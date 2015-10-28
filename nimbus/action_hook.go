package nimbus

import (
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

func (a ActionHook) OnStartAction() (err error) {

	// TODO: starting a passive job should return error - director should never call start action on the agent

	disk, found := a.dualDCSupport.persistentDiskSettings()
	if found {
		if err = a.platform.MountPersistentDisk(disk, a.platform.GetDirProvider().StoreDir()); err != nil {
			err = bosherr.WrapErrorf(err, "Mounting persistent disk %s", disk)
			return
		}
	}
	if err = a.dualDCSupport.StartDNSUpdatesIfRequired(); err != nil {
		err = bosherr.WrapErrorf(err, "Startign DNS updates if required")
		return
	}

	return
}

func (a ActionHook) OnStopAction() (err error) {

	disk, found := a.dualDCSupport.persistentDiskSettings()

	if found {
		if _, err = a.platform.UnmountPersistentDisk(disk); err != nil {
			err = bosherr.WrapErrorf(err, "Unmounting persistent disk %s", disk)
			return
		}
	}

	if err = a.dualDCSupport.StopDNSUpdatesIfRequired(); err != nil {
		err = bosherr.WrapError(err, "Stopping DNS updates if required")
		return
	}

	return
}
