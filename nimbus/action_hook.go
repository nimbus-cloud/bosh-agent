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

// We are not unmounting on stop anymore
// Therefore for cut over with drbd to work we may need to detect if the disk is reqular-mounted or drbd-mounted
// unmount/remount as per config
// TODO: implement a hook method and call it from somewhere
// so that drbd cutover works: active drbd node that is being turned to needs to unmount
//

func (a ActionHook) OnStartAction() error {
	a.dualDCSupport.logger.Debug(nimbusLogTag, "OnStartAction - begin")

	passive, err := a.dualDCSupport.isPassiveSide()
	if err != nil {
		return bosherr.WrapError(err, "chcking if passive side")
	}

	if passive {
		return errors.New("Can not start services in passive mode!")
	}

	//	disk, found := a.dualDCSupport.persistentDiskSettings()
	//	if found {
	//		if err = a.platform.MountPersistentDisk(disk, a.platform.GetDirProvider().StoreDir()); err != nil {
	//			return bosherr.WrapErrorf(err, "Mounting persistent disk %s", disk)
	//		}
	//	}

	if err = a.dualDCSupport.StartDNSUpdatesIfRequired(); err != nil {
		return bosherr.WrapErrorf(err, "Startign DNS updates if required")
	}

	return nil
}

func (a ActionHook) OnStopAction() error {

	a.dualDCSupport.logger.Debug(nimbusLogTag, "OnStopAction - begin")
	//	disk, found := a.dualDCSupport.persistentDiskSettings()

	//	if found {
	//		if _, err := a.platform.UnmountPersistentDisk(disk); err != nil {
	//			return bosherr.WrapErrorf(err, "Unmounting persistent disk %s", disk)
	//		}
	//	}

	if err := a.dualDCSupport.StopDNSUpdatesIfRequired(); err != nil {
		return bosherr.WrapError(err, "Stopping DNS updates if required")
	}

	return nil
}
