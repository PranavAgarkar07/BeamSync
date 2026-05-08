//go:build darwin

package beamsync

import "syscall"

func GetDiskFreeSpace(path string) (DiskSpaceInfo, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return DiskSpaceInfo{}, err
	}
	// Darwin Statfs_t: Bsize=int32, Blocks=int64, Bavail=int64
	bsize := int64(stat.Bsize)
	info := DiskSpaceInfo{
		AvailableBytes: stat.Bavail * bsize,
		TotalBytes:     stat.Blocks * bsize,
		UsedBytes:      (stat.Blocks - stat.Bavail) * bsize,
	}
	return info.withStrings(), nil
}
