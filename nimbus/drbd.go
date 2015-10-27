package nimbus

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	settingsService boshsettings.Service,
	logger boshlog.Logger,
) DualDCSupport {

	specService := boshas.NewConcreteV1Service(fs, filepath.Join(dirProvider.BoshDir(), "spec.json"))
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

func (d DualDCSupport) DRBDEnabled() (bool, error) {
	spec, err := d.specService.Get()
	if err != nil {
		return false, bosherr.WrapError(err, "Fetching spec")
	}

	return spec.DrbdEnabled, nil
}

func (d DualDCSupport) SetupDRBD() (err error) {

	if err = d.writeDrbdConfig(); err != nil {
		return bosherr.WrapError(err, "DualDCSupport.setupDRBD() error calling writeDrbdConfig()")
	}

	if err = d.createLvm(); err != nil {
		return bosherr.WrapError(err, "DualDCSupport.setupDRBD() error calling createLvm()")
	}

	if err = d.drdbRestart(); err != nil {
		return bosherr.WrapError(err, "DualDCSupport.setupDRBD() error calling drdbRestart()")
	}

	if err = d.drbdCreatePartition(); err != nil {
		return bosherr.WrapError(err, "DualDCSupport.setupDRBD() error calling drbdCreatePartition()")
	}

	return
}

func (d DualDCSupport) DRBDMount(mountPoint string) (err error) {
	d.logger.Info(nimbusLogTag, "Drbd mounting %s", mountPoint)

	isMounted, err := d.mounter.IsMounted(mountPoint)
	if err != nil {
		return bosherr.WrapErrorf(err, "DRBDMount() -> error calling mounter.IsMounted(%s)", mountPoint)
	}

	if isMounted {
		return
	}

	err = d.drbdMakePrimary()
	if err != nil {
		return bosherr.WrapError(err, "DRBDMount() -> error calling drbdMakePrimary()")
	}

	err = d.fs.MkdirAll(mountPoint, os.FileMode(0755))
	if err != nil {
		return bosherr.WrapError(err, "DRBDMount() -> error calling fs.MkdirAll()")
	}

	out, _, _, err := d.cmdRunner.RunCommand("file -s /dev/drbd1")
	if err != nil {
		return bosherr.WrapError(err, "DRBDMount() -> error checking if filesystem exists")
	}
	if strings.HasPrefix(out, "/dev/drbd1: data") {
		err = d.formatter.Format("/dev/drbd1", boshdisk.FileSystemExt4)
		if err != nil {
			return bosherr.WrapError(err, "DRBDMount() -> error calling formatter.Format")
		}
	}

	err = d.mounter.Mount("/dev/drbd1", mountPoint)
	if err != nil {
		return bosherr.WrapError(err, "DRBDMount() -> error calling mounter.Mount()")
	}

	return
}

func (d DualDCSupport) DRBDUmount(mountPoint string) (err error) {
	d.logger.Info(nimbusLogTag, "Drbd unmounting %s", mountPoint)

	_, err = d.mounter.Unmount(mountPoint)
	if err != nil {
		return bosherr.WrapErrorf(err, "DRBDUmount() -> error calling mounter.Unmount(%s)", mountPoint)
	}

	err = d.drbdMakeSecondary()
	if err != nil {
		return bosherr.WrapError(err, "DRBDUmount() -> error calling drbdMakeSecondary")
	}

	return
}

func (d DualDCSupport) createLvm() (err error) {

	settings := d.settingsService.GetSettings()

	disks := settings.Disks.Persistent
	if len(disks) != 1 {
		return errors.New("DualDCSupport.createLvm(): expected exactly 1 persistent disk")
	}

	var diskSettings boshsettings.DiskSettings
	for diskID := range disks {
		diskSettings, _ = settings.PersistentDiskSettings(diskID)
	}

	out, _, _, _ := d.cmdRunner.RunCommand("pvs")
	if !strings.Contains(out, diskSettings.Path) {
		d.cmdRunner.RunCommand("pvcreate", diskSettings.Path)
		d.cmdRunner.RunCommand("vgcreate", "vgStoreData", diskSettings.Path)
	}

	out, _, _, _ = d.cmdRunner.RunCommand("lvs")
	matchFound, _ := regexp.MatchString("StoreData\\s+vgStoreData", out)
	if !matchFound {
		d.cmdRunner.RunCommand("lvcreate -n StoreData -l 40%FREE vgStoreData")
	}

	return
}

func (d DualDCSupport) drdbRestart() (err error) {
	_, _, _, err = d.cmdRunner.RunCommand("/etc/init.d/drbd", "restart")
	return
}

func (d DualDCSupport) drbdCreatePartition() (err error) {

	out, _, _, err := d.cmdRunner.RunCommand("drbdadm", "dstate", "r0")
	if err != nil {
		return
	}
	if !strings.HasPrefix(out, "Diskless") {
		return
	}

	out, _, _, err = d.cmdRunner.RunCommand("drbdadm", "dump-md", "r0")
	if err != nil {
		return bosherr.WrapErrorf(err, "Failure: drbdadm dump-md r0. Output: %s", out)
	}
	if strings.Contains(out, "No valid meta data found") {
		_, _, _, err = d.cmdRunner.RunCommand("echo 'no' | drbdadm create-md r0")
		if err != nil {
			return
		}
	}

	_, _, _, err = d.cmdRunner.RunCommand("drbdadm", "down", "r0")
	if err != nil {
		return
	}

	_, _, _, err = d.cmdRunner.RunCommand("drbdadm", "up", "r0")

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

	err = d.fs.WriteFileString("/etc/drbd.d/r0.res", configBody)

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
