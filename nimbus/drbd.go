package nimbus

import (
	"fmt"
	"path/filepath"

	boshas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type DualDCSupport struct {
	cmdRunner   boshsys.CmdRunner
	fs          boshsys.FileSystem
	dirProvider boshdir.Provider
	specService boshas.V1Service
	logger      boshlog.Logger
}

func NewDualDCSupport(
	cmdRunner boshsys.CmdRunner,
	fs boshsys.FileSystem,
	dirProvider boshdir.Provider,
	logger boshlog.Logger,
) DualDCSupport {
	return DualDCSupport{
		cmdRunner:   cmdRunner,
		fs:          fs,
		dirProvider: dirProvider,
		specService: boshas.NewConcreteV1Service(fs, filepath.Join(dirProvider.BoshDir(), "spec.json")),
		logger:      logger,
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

func (d DualDCSupport) Mount() error {

	return nil
}

func (d DualDCSupport) Umount() error {

	return nil
}

func (d DualDCSupport) setupDRBD() error {
	return nil
}

func drbdConfig(replicationType, secret, thisHostName, thisHostIP, otherHostIP string) string {
	// this needs to be written to: /etc/drbd.d/r0.res
	configBody := fmt.Sprintf(drbdConfigTemplate, replicationType, secret, thisHostName, thisHostIP, otherHostIP)
	return configBody
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
