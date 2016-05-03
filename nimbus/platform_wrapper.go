package nimbus

import (
	"errors"
	"github.com/cloudfoundry/bosh-agent/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

var _ platform.Platform = (*PlatformWrapper)(nil)

type PlatformWrapper struct {
	platform.Platform
	dualDCSupport *DualDCSupport
}

func NewPlatformWrapper(platform platform.Platform, dualDCSupport *DualDCSupport) platform.Platform {
	return PlatformWrapper{platform, dualDCSupport}
}

func (w PlatformWrapper) MountPersistentDisk(diskSettings boshsettings.DiskSettings, mountPoint string) error {
	w.dualDCSupport.logger.Debug(nimbusLogTag, "MountPersistentDisk - begin")

	spec, err := w.dualDCSupport.specService.Get()
	if err != nil {
		return bosherr.WrapError(err, "getting spec to check if DRBD is enabled")
	}

	// DRBD logic
	if spec.DrbdEnabled {
		w.dualDCSupport.logger.Debug(nimbusLogTag, "MountPersistentDisk - drbd is enabled")

		if err = w.dualDCSupport.setupDRBD(); err != nil {
			return bosherr.WrapError(err, "setting up DRBD")
		}

		// mount only active side
		if spec.IsActiveSide() {
			if err = w.dualDCSupport.mountDRBD(); err != nil {
				return bosherr.WrapError(err, "DRBD mounting persistent share")
			}
		}
		return nil
	}

	// otherwise normal mount
	return w.Platform.MountPersistentDisk(diskSettings, mountPoint)
}

func (w PlatformWrapper) UnmountPersistentDisk(diskSettings boshsettings.DiskSettings) (didUnmount bool, err error) {
	w.dualDCSupport.logger.Debug(nimbusLogTag, "UnmountPersistentDisk - begin")

	spec, err := w.dualDCSupport.specService.Get()
	if err != nil {
		return false, bosherr.WrapError(err, "getting spec to check if DRBD is enabled")
	}

	// DRBD logic
	if spec.DrbdEnabled {
		w.dualDCSupport.logger.Debug(nimbusLogTag, "UnmountPersistentDisk - drbd is enabled")

		// always unmount
		didUnmount, err = w.dualDCSupport.unmountDRBD()
		if err != nil {
			return false, bosherr.WrapError(err, "DRBD unmounting persistent share")
		}
		return
	}

	// otherwise normal unmount
	return w.Platform.UnmountPersistentDisk(diskSettings)
}

func (w PlatformWrapper) MigratePersistentDisk(fromMountPoint, toMountPoint string) (err error) {

	spec, err := w.dualDCSupport.specService.Get()
	if err != nil {
		return bosherr.WrapError(err, "getting spec to check if DRBD is enabled")
	}

	if spec.DrbdEnabled {
		return errors.New("Migrating a drbd box is not supported.")
	}

	return w.Platform.MigratePersistentDisk(fromMountPoint, toMountPoint)
}

// for drbd enabled nodes checking if disk is mounted based on boshsettings.DiskSettings does not work.
// need to use /var/vcap/store to see if active side is mounted, returning true for passive side.

func (w PlatformWrapper) IsPersistentDiskMounted(diskSettings boshsettings.DiskSettings) (result bool, err error) {
	w.dualDCSupport.logger.Debug(nimbusLogTag, "IsPersistentDiskMounted - begin")

	spec, err := w.dualDCSupport.specService.Get()
	if err != nil {
		return false, bosherr.WrapError(err, "getting spec to check if DRBD is enabled")
	}

	if spec.DrbdEnabled {
		if spec.IsPassiveSide() {
			return true, nil
		}

		return w.dualDCSupport.mounter.IsMounted(w.dualDCSupport.dirProvider.StoreDir())
	}

	return w.Platform.IsPersistentDiskMounted(diskSettings)
}
