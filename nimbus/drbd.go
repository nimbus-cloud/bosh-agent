package nimbus

import (
	"fmt"
	"path/filepath"

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

func (d DualDCSupport) setupDRBD() error {
	configBody, err := d.drbdConfig()
	if err != nil {
		return err
	}
	d.fs.WriteFileString("/etc/drbd.d/r0.res", configBody)

	d.createLvm()
	d.drdbRestart()
	d.drbdCreatePartition()

	return nil
}

func (d DualDCSupport) createLvm() error {

	return nil
}

func (d DualDCSupport) drdbRestart() error {

	return nil
}

func (d DualDCSupport) drbdCreatePartition() error {

	return nil
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

func (d DualDCSupport) drbdConfig() (string, error) {
	spec, err := d.specService.Get()
	if err != nil {
		return "", bosherr.WrapError(err, "Fetching spec")
	}

	ips := d.settingsService.GetSettings().Networks.IPs()
	if len(ips) == 0 {
		return "", errors.New("DualDCSupport.drbdConfig() -> settingsService.GetSettings().Networks.IPs(), no ip found")
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

	return configBody, nil
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
