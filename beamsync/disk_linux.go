//go:build linux

package beamsync

import "syscall"

func GetDiskFreeSpace(path string) (DiskSpaceInfo, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return DiskSpaceInfo{}, err
	}
	info := DiskSpaceInfo{
		AvailableBytes: int64(stat.Bavail) * stat.Bsize,
		TotalBytes:     int64(stat.Blocks) * stat.Bsize,
		UsedBytes:      int64(stat.Blocks-stat.Bavail) * stat.Bsize,
	}
	return info.withStrings(), nil
}
