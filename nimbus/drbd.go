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
) *DualDCSupport {

	linuxMounter := boshdisk.NewLinuxMounter(cmdRunner, boshdisk.NewCmdMountsSearcher(cmdRunner), 1*time.Second)
	linuxFormatter := boshdisk.NewLinuxFormatter(cmdRunner, fs)

	return &DualDCSupport{
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

	if err = d.drbdCreatePartition(); err != nil {
		return bosherr.WrapError(err, "DualDCSupport.setupDRBD() error calling drbdCreatePartition()")
	}

	return
}

func (d DualDCSupport) mountDRBD() (err error) {
	d.logger.Info(nimbusLogTag, "Drbd mount - begin")

	mountPoint := d.dirProvider.StoreDir()

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

	out, _, _, err := d.cmdRunner.RunCommand("sh", "-c", "file -s /dev/drbd1")
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

func (d DualDCSupport) unmountDRBD() (didUnmount bool, err error) {
	d.logger.Info(nimbusLogTag, "Drbd unmount - begin")

	mountPoint := d.dirProvider.StoreDir()

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
		if _, err = d.mounter.Unmount(device); err != nil {
			return bosherr.WrapError(err, "Unmounting device before creating physical volume")
		}
		if _, _, _, err = d.cmdRunner.RunCommand("pvcreate", device); err != nil {
			return bosherr.WrapError(err, "Creating physical volume")
		}
		if _, _, _, err := d.cmdRunner.RunCommand("vgcreate", "vgStoreData", device); err != nil {
			return bosherr.WrapError(err, "Creating volume group")
		}
	}

	out, _, _, _ = d.cmdRunner.RunCommand("lvs")
	matchFound, _ := regexp.MatchString("StoreData\\s+vgStoreData", out)
	if !matchFound {
		if _, err := d.mounter.Unmount(device); err != nil {
			return bosherr.WrapError(err, "Unmounting device before creating logical volume")
		}
		if _, _, _, err := d.cmdRunner.RunCommand("sh", "-c", "lvcreate -n StoreData -l 40%FREE vgStoreData"); err != nil {
			return bosherr.WrapError(err, "when running: lvcreate -n StoreData -l 40%FREE vgStoreData")
		}
	}

	return
}

func (d DualDCSupport) drbdCreatePartition() (err error) {

	// TODO: looks like none of this is needed
	//	out, _, _, _ := d.cmdRunner.RunCommand("drbdadm dstate r0")
	//	if err != nil {
	//		return
	//	}
	//	if !strings.HasPrefix(out, "Diskless") {
	//		return
	//	}

	out, _, _, err := d.cmdRunner.RunCommand("sh", "-c", "echo 'yes' | drbdadm dump-md r0 2>&1")
	//	if err != nil {
	//		return bosherr.WrapErrorf(err, "Failure: drbdadm dump-md r0. Output: %s", out)
	//	}
	if strings.Contains(out, "No valid meta data found") {
		_, _, _, err = d.cmdRunner.RunCommand("sh", "-c", "echo 'no' | drbdadm create-md r0")
		if err != nil {
			return
		}
	}

	_, _, _, err = d.cmdRunner.RunCommand("sh", "-c", "drbdadm down r0")
	if err != nil {
		return
	}

	_, _, _, err = d.cmdRunner.RunCommand("sh", "-c", "drbdadm up r0")

	return
}

func (d DualDCSupport) drbdMakePrimary() (err error) {
	d.logger.Info(nimbusLogTag, "Drbd making primary")

	spec, err := d.specService.Get()
	if err != nil {
		return
	}

	if spec.DrbdForceMaster {
		_, _, _, err = d.cmdRunner.RunCommand("sh", "-c", "drbdadm primary --force r0")
	} else {
		_, _, _, err = d.cmdRunner.RunCommand("sh", "-c", "drbdadm primary r0")
	}

	return
}

func (d DualDCSupport) drbdMakeSecondary() (err error) {
	d.logger.Info(nimbusLogTag, "Drbd making secondary")

	// make sure that dstate is UpToDate on both sides
	for i := 0; i <= 10; i++ {
		d.logger.Debug(nimbusLogTag, "Checking if both sides are in sync before demoting to secondary")
		out, _, _, err := d.cmdRunner.RunCommand("sh", "-c", "drbdadm dstate r0")
		if err != nil {
			return bosherr.WrapError(err, "Checking 'drbdadm dstate r0' before making secondary")
		}
		if matchFound, _ := regexp.MatchString("UpToDate/UpToDate", out); matchFound {
			break
		}
		if i == 10 {
			return errors.New("Checked 'drbdadm dstate r0' 10 times, still not in sync, can not make secondary...")
		}
		time.Sleep(3 * time.Second)
	}
	_, _, _, err = d.cmdRunner.RunCommand("sh", "-c", "drbdadm secondary r0")
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

// TODO: add data-integrity-alg sha1; to net section??? kind of makes sense with A protocol???
// TODO: split brain hanlder: split-brain "/usr/lib/drbd/notify-split-brain.sh root";
// TODO: congestion policy: https://drbd.linbit.com/users-guide/s-configure-congestion-policy.html

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
