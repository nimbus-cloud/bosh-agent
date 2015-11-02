package nimbus

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	boshas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	boshdisk "github.com/cloudfoundry/bosh-agent/platform/disk"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

// TODO: test DNS - local DNS server??? fake?
// TODO: pointer receiver for methods? Pass DualDCSupport instance as pointer???

type DualDCSupport struct {
	cmdRunner       boshsys.CmdRunner
	fs              boshsys.FileSystem
	dirProvider     boshdir.Provider
	specService     boshas.V1Service
	settingsService boshsettings.Service
	mounter         boshdisk.Mounter
	formatter       boshdisk.Formatter
	cancelChan      chan struct{}
	logger          boshlog.Logger
}

func NewDualDCSupport(
	cmdRunner boshsys.CmdRunner,
	fs boshsys.FileSystem,
	dirProvider boshdir.Provider,
	specService boshas.V1Service,
	settingsService boshsettings.Service,
	logger boshlog.Logger,
) DualDCSupport {

	linuxMounter := boshdisk.NewLinuxMounter(cmdRunner, boshdisk.NewCmdMountsSearcher(cmdRunner), 1*time.Second)
	linuxFormatter := boshdisk.NewLinuxFormatter(cmdRunner, fs)

	return DualDCSupport{
		cmdRunner:       cmdRunner,
		fs:              fs,
		dirProvider:     dirProvider,
		specService:     specService,
		settingsService: settingsService,
		mounter:         linuxMounter,
		formatter:       linuxFormatter,
		logger:          logger,
	}
}

func (d DualDCSupport) setupDRBD() (err error) {
	d.logger.Info(nimbusLogTag, "setupDRBD - begin")

	if err = d.writeDrbdConfig(); err != nil {
		return bosherr.WrapError(err, "DualDCSupport.setupDRBD() error calling writeDrbdConfig()")
	}

	if err = d.createLvm(); err != nil {
		return bosherr.WrapError(err, "DualDCSupport.setupDRBD() error calling createLvm()")
	}

	// looks like there is no need for this
	//	if err = d.drdbRestart(); err != nil {
	//		return bosherr.WrapError(err, "DualDCSupport.setupDRBD() error calling drdbRestart()")
	//	}

	if err = d.drbdCreatePartition(); err != nil {
		return bosherr.WrapError(err, "DualDCSupport.setupDRBD() error calling drbdCreatePartition()")
	}

	return
}

func (d DualDCSupport) mountDRBD(mountPoint string) (err error) {
	d.logger.Info(nimbusLogTag, "Drbd mounting %s", mountPoint)

	isMounted, err := d.mounter.IsMounted(mountPoint)
	if err != nil {
		return bosherr.WrapErrorf(err, "mountDRBD() -> error calling mounter.IsMounted(%s)", mountPoint)
	}

	if isMounted {
		return
	}

	err = d.drbdMakePrimary()
	if err != nil {
		return bosherr.WrapError(err, "mountDRBD() -> error calling drbdMakePrimary()")
	}

	err = d.fs.MkdirAll(mountPoint, os.FileMode(0755))
	if err != nil {
		return bosherr.WrapError(err, "mountDRBD() -> error calling fs.MkdirAll()")
	}

	out, _, _, err := d.cmdRunner.RunCommand("file -s /dev/drbd1")
	if err != nil {
		return bosherr.WrapError(err, "mountDRBD() -> error checking if filesystem exists")
	}
	if strings.HasPrefix(out, "/dev/drbd1: data") {
		err = d.formatter.Format("/dev/drbd1", boshdisk.FileSystemExt4)
		if err != nil {
			return bosherr.WrapError(err, "mountDRBD() -> error calling formatter.Format")
		}
	}

	err = d.mounter.Mount("/dev/drbd1", mountPoint)
	if err != nil {
		return bosherr.WrapError(err, "mountDRBD() -> error calling mounter.Mount()")
	}

	return
}

func (d DualDCSupport) unmountDRBD(mountPoint string) (didUnmount bool, err error) {
	d.logger.Info(nimbusLogTag, "Drbd unmounting %s", mountPoint)

	didUnmount, err = d.mounter.Unmount(mountPoint)
	if err != nil {
		return false, bosherr.WrapErrorf(err, "unmountDRBD() -> error calling mounter.Unmount(%s)", mountPoint)
	}

	// In certain scenarios drbd may not have been setup yet: non-drbd job changed to drbd-enabled job - OnStartAction
	// un-mounts disk first (now drbd enabled according to job spec) but drbd has not been setup yet.
	// It will be when the disk is mounted - hence the check below
	if d.isDRBDConfigWritten() {
		err = d.drbdMakeSecondary()
		if err != nil {
			return false, bosherr.WrapError(err, "unmountDRBD() -> error calling drbdMakeSecondary")
		}
	}

	return
}

func (d DualDCSupport) isDRBDConfigWritten() bool {
	return d.fs.FileExists(drbdConfigLocation)
}

func (d DualDCSupport) persistentDiskSettings() (persistentDisk boshsettings.DiskSettings, found bool) {
	settings := d.settingsService.GetSettings()

	persistentDisks := settings.Disks.Persistent
	if len(persistentDisks) == 1 {
		for diskID := range persistentDisks {
			persistentDisk, found = settings.PersistentDiskSettings(diskID)
		}
	}

	return
}

func (d DualDCSupport) isPassiveSide() (passive bool, err error) {
	spec, err := d.specService.Get()
	if err != nil {
		err = bosherr.WrapError(err, "Fetching spec")
		return
	}
	passive = spec.IsPassiveSide()
	return
}

func (d DualDCSupport) createLvm() (err error) {

	//	diskSettings, found := d.persistentDiskSettings()
	//	if !found {
	//		return errors.New("DualDCSupport.createLvm(): persistent disk not found")
	//	}

	device := "/dev/sdc1"

	out, _, _, _ := d.cmdRunner.RunCommand("pvs")
	if !strings.Contains(out, device) {
		d.cmdRunner.RunCommand("pvcreate", device)
		d.cmdRunner.RunCommand("vgcreate", "vgStoreData", device)
	}

	out, _, _, _ = d.cmdRunner.RunCommand("lvs")
	matchFound, _ := regexp.MatchString("StoreData\\s+vgStoreData", out)
	if !matchFound {
		_, _, _, err := d.cmdRunner.RunCommand("lvcreate -n StoreData -l 40%FREE vgStoreData")
		if err != nil {
			return bosherr.WrapError(err, "when running: lvcreate -n StoreData -l 40%FREE vgStoreData")
		}
	}

	return
}

//func (d DualDCSupport) drdbRestart() (err error) {
//	_, _, _, err = d.cmdRunner.RunCommand("/etc/init.d/drbd", "restart")
//	return
//}

func (d DualDCSupport) drbdCreatePartition() (err error) {

	// TODO: looks like none of this is needed
	//	out, _, _, _ := d.cmdRunner.RunCommand("drbdadm dstate r0")
	//	if err != nil {
	//		return
	//	}
	//	if !strings.HasPrefix(out, "Diskless") {
	//		return
	//	}

	out, _, _, err := d.cmdRunner.RunCommand("echo 'yes' | drbdadm dump-md r0")
	//	if err != nil {
	//		return bosherr.WrapErrorf(err, "Failure: drbdadm dump-md r0. Output: %s", out)
	//	}
	if strings.Contains(out, "No valid meta data found") {
		_, _, _, err = d.cmdRunner.RunCommand("echo 'no' | drbdadm create-md r0")
		if err != nil {
			return
		}
	}

	_, _, _, err = d.cmdRunner.RunCommand("drbdadm down r0")
	if err != nil {
		return
	}

	_, _, _, err = d.cmdRunner.RunCommand("drbdadm up r0")

	return
}

func (d DualDCSupport) drbdMakePrimary() (err error) {
	d.logger.Info(nimbusLogTag, "Drbd making primary")

	spec, err := d.specService.Get()
	if err != nil {
		return
	}

	forceFlag := ""
	if spec.DrbdForceMaster {
		forceFlag = "--force"
	}
	_, _, _, err = d.cmdRunner.RunCommand("drbdadm", "primary", forceFlag, "r0")
	return
}

func (d DualDCSupport) drbdMakeSecondary() (err error) {
	d.logger.Info(nimbusLogTag, "Drbd making secondary")

	// TODO: invalidate on secondary ???
	// drbdadm invalidate r0
	_, _, _, err = d.cmdRunner.RunCommand("drbdadm", "secondary", "r0")
	return
}

func (d DualDCSupport) writeDrbdConfig() (err error) {
	spec, err := d.specService.Get()
	if err != nil {
		return bosherr.WrapError(err, "Fetching spec")
	}

	thisHostIP, err := d.thisHostIP()
	if err != nil {
		return
	}
	otherHostIP := ""

	if thisHostIP == spec.DrbdReplicationNode1 {
		otherHostIP = spec.DrbdReplicationNode2
	} else {
		otherHostIP = spec.DrbdReplicationNode1
	}

	configBody := fmt.Sprintf(
		drbdConfigTemplate,
		spec.DrbdReplicationType,
		spec.DrbdSecret,
		d.settingsService.GetSettings().AgentID,
		thisHostIP,
		otherHostIP,
	)

	err = d.fs.WriteFileString(drbdConfigLocation, configBody)

	return
}

func (d DualDCSupport) thisHostIP() (ip string, err error) {
	ips := d.settingsService.GetSettings().Networks.IPs()

	if len(ips) == 0 {
		return "", errors.New("DualDCSupport.thisHostIP() -> settingsService.GetSettings().Networks.IPs(), no ip found")
	}

	// TODO: is this correct??? what if multiple networks are configured???
	return ips[0], nil
}

const drbdConfigLocation = "/etc/drbd.d/r0.res"

const nimbusLogTag = "Nimbus"

const drbdConfigTemplate = `
resource r0 {
  net {
    protocol %s;
    shared-secret %s;
    verify-alg sha1;
  }
  disk {
    resync-rate 24M;
  }
  handlers {
    before-resync-target "/lib/drbd/snapshot-resync-target-lvm.sh";
    after-resync-target "/lib/drbd/unsnapshot-resync-target-lvm.sh";
  }
  startup {
    wfc-timeout 3;
    degr-wfc-timeout 3;
    outdated-wfc-timeout 2;
  }
  on %s {
    device    drbd1;
    disk      /dev/mapper/vgStoreData-StoreData;
    address   %s:7789;
    meta-disk internal;
  }
  on host2 {
    device    drbd1;
    disk      /dev/mapper/vgStoreData-StoreData;
    address   %s:7789;
    meta-disk internal;
  }
}
`
