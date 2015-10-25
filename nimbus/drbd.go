package nimbus

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"errors"

	boshas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

const nimbusLogTag = "Nimbus"

type DualDCSupport struct {
	cmdRunner       boshsys.CmdRunner
	fs              boshsys.FileSystem
	dirProvider     boshdir.Provider
	specService     boshas.V1Service
	settingsService boshsettings.Service
	logger          boshlog.Logger
}

func NewDualDCSupport(
	cmdRunner boshsys.CmdRunner,
	fs boshsys.FileSystem,
	dirProvider boshdir.Provider,
	settingsService boshsettings.Service,
	logger boshlog.Logger,
) DualDCSupport {
	return DualDCSupport{
		cmdRunner:       cmdRunner,
		fs:              fs,
		dirProvider:     dirProvider,
		specService:     boshas.NewConcreteV1Service(fs, filepath.Join(dirProvider.BoshDir(), "spec.json")),
		settingsService: settingsService,
		logger:          logger,
	}
}

func (d DualDCSupport) SetupDRBDIfRequired() error {

	spec, err := d.specService.Get()
	if err != nil {
		return bosherr.WrapError(err, "Fetching spec")
	}

	if spec.DrbdEnabled {
		err = d.setupDRBD()
		if err != nil {
			return bosherr.WrapError(err, "Drbd.StartupIfRequired() -> error calling d.startup()")
		}
	}

	return nil
}

func (d DualDCSupport) DRBDMount() error {

	return nil
}

func (d DualDCSupport) DRBDUmount() error {

	return nil
}

func (d DualDCSupport) setupDRBD() (err error) {

	if err = d.writeDrbdConfig(); err != nil {
		return bosherr.WrapError(err, "Failure: DualDCSupport.writeDrbdConfig()")
	}

	if err = d.createLvm(); err != nil {
		return bosherr.WrapError(err, "Failure: DualDCSupport.createLvm()")
	}

	if err = d.drdbRestart(); err != nil {
		return bosherr.WrapError(err, "Failure: DualDCSupport.drdbRestart()")
	}

	if err = d.drbdCreatePartition(); err != nil {
		return bosherr.WrapError(err, "Failure: DualDCSupport.drbdCreatePartition()")
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

	ips := d.settingsService.GetSettings().Networks.IPs()
	if len(ips) == 0 {
		return errors.New("DualDCSupport.drbdConfig() -> settingsService.GetSettings().Networks.IPs(), no ip found")
	}

	thisHostIP := ips[0]
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
