package disk

type FileSystemType string

const (
	FileSystemSwap          FileSystemType = "swap"
	FileSystemExt4          FileSystemType = "ext4"
	FileSystemXFS           FileSystemType = "xfs"
	FileSystemDefault       FileSystemType = ""
	FileSystemDrbdPartition FileSystemType = "LVM2_member"
)

type Formatter interface {
	Format(partitionPath string, fsType FileSystemType) (err error)
}
