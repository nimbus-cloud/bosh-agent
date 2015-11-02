package nimbus

import (
	"errors"
	boshdpresolv "github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver"
	"github.com/cloudfoundry/bosh-agent/platform"
	"github.com/cloudfoundry/bosh-agent/platform/cert"
	boshvitals "github.com/cloudfoundry/bosh-agent/platform/vitals"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshcmd "github.com/cloudfoundry/bosh-utils/fileutil"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

var _ platform.Platform = (*PlatformWrapper)(nil)

type PlatformWrapper struct {
	platform      platform.Platform
	dualDCSupport DualDCSupport
}

func NewPlatformWrapper(platform platform.Platform, dualDCSupport DualDCSupport) platform.Platform {
	return PlatformWrapper{platform: platform, dualDCSupport: dualDCSupport}
}

func (w PlatformWrapper) GetFs() boshsys.FileSystem {
	return w.platform.GetFs()
}

func (w PlatformWrapper) GetRunner() boshsys.CmdRunner {
	return w.platform.GetRunner()
}

func (w PlatformWrapper) GetCompressor() boshcmd.Compressor {
	return w.platform.GetCompressor()
}

func (w PlatformWrapper) GetCopier() boshcmd.Copier {
	return w.platform.GetCopier()
}

func (w PlatformWrapper) GetDirProvider() boshdir.Provider {
	return w.platform.GetDirProvider()
}

func (w PlatformWrapper) GetVitalsService() boshvitals.Service {
	return w.platform.GetVitalsService()
}

func (w PlatformWrapper) GetDevicePathResolver() (devicePathResolver boshdpresolv.DevicePathResolver) {
	return w.platform.GetDevicePathResolver()
}

// User management

func (w PlatformWrapper) CreateUser(username, password, basePath string) (err error) {
	return w.platform.CreateUser(username, password, basePath)
}

func (w PlatformWrapper) AddUserToGroups(username string, groups []string) (err error) {
	return w.platform.AddUserToGroups(username, groups)
}

func (w PlatformWrapper) DeleteEphemeralUsersMatching(regex string) (err error) {
	return w.platform.DeleteEphemeralUsersMatching(regex)
}

// Bootstrap functionality

func (w PlatformWrapper) SetupRootDisk(ephemeralDiskPath string) (err error) {
	return w.platform.SetupRootDisk(ephemeralDiskPath)
}

func (w PlatformWrapper) SetupSSH(publicKey, username string) (err error) {
	return w.platform.SetupSSH(publicKey, username)
}

func (w PlatformWrapper) SetUserPassword(user, encryptedPwd string) (err error) {
	return w.platform.SetUserPassword(user, encryptedPwd)
}

func (w PlatformWrapper) SetupHostname(hostname string) (err error) {
	return w.platform.SetupHostname(hostname)
}

func (w PlatformWrapper) SetupNetworking(networks boshsettings.Networks) (err error) {
	return w.platform.SetupNetworking(networks)
}

func (w PlatformWrapper) SetupLogrotate(groupName, basePath, size string) (err error) {
	return w.platform.SetupLogrotate(groupName, basePath, size)
}

func (w PlatformWrapper) SetTimeWithNtpServers(servers []string) (err error) {
	return w.platform.SetTimeWithNtpServers(servers)
}

func (w PlatformWrapper) SetupEphemeralDiskWithPath(devicePath string) (err error) {
	return w.platform.SetupEphemeralDiskWithPath(devicePath)
}

func (w PlatformWrapper) SetupRawEphemeralDisks(devices []boshsettings.DiskSettings) (err error) {
	return w.platform.SetupRawEphemeralDisks(devices)
}

func (w PlatformWrapper) SetupDataDir() (err error) {
	return w.platform.SetupDataDir()
}

func (w PlatformWrapper) SetupTmpDir() (err error) {
	return w.platform.SetupTmpDir()
}

func (w PlatformWrapper) SetupMonitUser() (err error) {
	return w.platform.SetupMonitUser()
}

func (w PlatformWrapper) StartMonit() (err error) {
	return w.platform.StartMonit()
}

func (w PlatformWrapper) SetupRuntimeConfiguration() (err error) {
	return w.platform.SetupRuntimeConfiguration()
}

// Disk management

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
	return w.platform.MountPersistentDisk(diskSettings, mountPoint)
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
	return w.platform.UnmountPersistentDisk(diskSettings)
}

func (w PlatformWrapper) MigratePersistentDisk(fromMountPoint, toMountPoint string) (err error) {

	spec, err := w.dualDCSupport.specService.Get()
	if err != nil {
		return bosherr.WrapError(err, "getting spec to check if DRBD is enabled")
	}

	if spec.DrbdEnabled {
		return errors.New("Migrating a drbd box is not supported.")
	}

	return w.platform.MigratePersistentDisk(fromMountPoint, toMountPoint)
}

func (w PlatformWrapper) GetEphemeralDiskPath(diskSettings boshsettings.DiskSettings) string {
	return w.platform.GetEphemeralDiskPath(diskSettings)
}

func (w PlatformWrapper) IsMountPoint(path string) (result bool, err error) {
	return w.platform.IsMountPoint(path)
}

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

	return w.platform.IsPersistentDiskMounted(diskSettings)
}

func (w PlatformWrapper) GetFileContentsFromCDROM(filePath string) (contents []byte, err error) {
	return w.platform.GetFileContentsFromCDROM(filePath)
}

func (w PlatformWrapper) GetFilesContentsFromDisk(diskPath string, fileNames []string) (contents [][]byte, err error) {
	return w.platform.GetFilesContentsFromDisk(diskPath, fileNames)
}

// Network misc

func (w PlatformWrapper) GetDefaultNetwork() (boshsettings.Network, error) {
	return w.platform.GetDefaultNetwork()
}

func (w PlatformWrapper) GetConfiguredNetworkInterfaces() ([]string, error) {
	return w.platform.GetConfiguredNetworkInterfaces()
}

func (w PlatformWrapper) PrepareForNetworkingChange() error {
	return w.platform.PrepareForNetworkingChange()
}

// Additional monit management

func (w PlatformWrapper) GetMonitCredentials() (username, password string, err error) {
	return w.platform.GetMonitCredentials()
}

func (w PlatformWrapper) GetCertManager() cert.Manager {
	return w.platform.GetCertManager()
}

func (w PlatformWrapper) GetHostPublicKey() (string, error) {
	return w.platform.GetHostPublicKey()
}
