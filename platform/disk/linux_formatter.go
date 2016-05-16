package disk

import (
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	"regexp"
	"strings"
)

type linuxFormatter struct {
	runner boshsys.CmdRunner
	fs     boshsys.FileSystem
	logger boshlog.Logger
}

func NewLinuxFormatter(runner boshsys.CmdRunner, fs boshsys.FileSystem, logger boshlog.Logger) Formatter {
	return linuxFormatter{
		runner: runner,
		fs:     fs,
		logger: logger,
	}
}

func (f linuxFormatter) Format(partitionPath string, fsType FileSystemType) (err error) {

	// do not format existing drbd partitions
	if f.isDrbdPartition(partitionPath) {
		f.logger.Info("LinuxFormatter", "LVM2_member partition discovered, exiting")
		return
	}

	f.logger.Info("LinuxFormatter", "Non LVM2_member partition discovered, continuing")

	existingFsType, err := f.getPartitionFormatType(partitionPath)
	if err != nil {
		return bosherr.WrapError(err, "Checking filesystem format of partition")
	}

	if fsType == FileSystemSwap {
		if existingFsType == FileSystemSwap {
			return
		}
		// swap is not user-configured, so we're not concerned about reformatting
	} else if existingFsType == FileSystemExt4 || existingFsType == FileSystemXFS {
		// never reformat if it is already formatted in a supported format
		return
	}

	switch fsType {
	case FileSystemSwap:
		_, _, _, err = f.runner.RunCommand("mkswap", partitionPath)
		if err != nil {
			err = bosherr.WrapError(err, "Shelling out to mkswap")
		}

	case FileSystemExt4:
		if f.fs.FileExists("/sys/fs/ext4/features/lazy_itable_init") {
			_, _, _, err = f.runner.RunCommand("mke2fs", "-t", string(fsType), "-j", "-E", "lazy_itable_init=1", partitionPath)
		} else {
			_, _, _, err = f.runner.RunCommand("mke2fs", "-t", string(fsType), "-j", partitionPath)
		}
		if err != nil {
			err = bosherr.WrapError(err, "Shelling out to mke2fs")
		}

	case FileSystemXFS:
		_, _, _, err = f.runner.RunCommand("mkfs.xfs", partitionPath)
		if err != nil {
			err = bosherr.WrapError(err, "Shelling out to mkfs.xfs")
		}
	}
	return
}

func (f linuxFormatter) getPartitionFormatType(partitionPath string) (FileSystemType, error) {
	stdout, stderr, exitStatus, err := f.runner.RunCommand("blkid", "-p", partitionPath)

	if err != nil {
		if exitStatus == 2 && stderr == "" {
			// in that case we expect the device not to have any file system
			return "", nil
		}
		return "", err
	}

	re := regexp.MustCompile(" TYPE=\"([^\"]+)\"")
	match := re.FindStringSubmatch(stdout)

	if nil == match {
		return "", nil
	}

	return FileSystemType(match[1]), nil
}

func (f linuxFormatter) isDrbdPartition(partitionPath string) bool {
	stdout, _, _, _ := f.runner.RunCommand("blkid", "-p", partitionPath)
	return strings.Contains(stdout, `TYPE="LVM2_member"`)
}
