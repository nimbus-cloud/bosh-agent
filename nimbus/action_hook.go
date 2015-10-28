package nimbus

import (
	"errors"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type ActionHook struct {
	platform      boshplatform.Platform
	dualDCSupport DualDCSupport
}

func NewActionHook(platform boshplatform.Platform, dualDCSupport DualDCSupport) ActionHook {
	return ActionHook{platform: platform, dualDCSupport: dualDCSupport}
}

func (a ActionHook) OnStartAction() error {

	passive, err := a.dualDCSupport.isPassiveSide()
	if err != nil {
		return bosherr.WrapError(err, "chcking if passive side")
	}

	if passive {
		return errors.New("Can not start services in passive mode!")
	}

	disk, found := a.dualDCSupport.persistentDiskSettings()
	if found {
		if err = a.platform.MountPersistentDisk(disk, a.platform.GetDirProvider().StoreDir()); err != nil {
			return bosherr.WrapErrorf(err, "Mounting persistent disk %s", disk)
		}
	}

	if err = a.dualDCSupport.StartDNSUpdatesIfRequired(); err != nil {
		return bosherr.WrapErrorf(err, "Startign DNS updates if required")
	}

	return nil
}

func (a ActionHook) OnStopAction() error {

	disk, found := a.dualDCSupport.persistentDiskSettings()

	if found {
		if _, err := a.platform.UnmountPersistentDisk(disk); err != nil {
			return bosherr.WrapErrorf(err, "Unmounting persistent disk %s", disk)
		}
	}

	if err := a.dualDCSupport.StopDNSUpdatesIfRequired(); err != nil {
		return bosherr.WrapError(err, "Stopping DNS updates if required")
	}

	return nil
}
