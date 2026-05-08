//go:build windows

package beamsync

import "syscall"

func GetDiskFreeSpace(path string) (DiskSpaceInfo, error) {
	var free, total, totalFree int64
	// Convert to uint64 pointers as expected by GetDiskFreeSpaceEx
	_, err := syscall.GetDiskFreeSpaceEx(
		syscall.StringToUTF16Ptr(path),
		(*uint64)(&free),
		(*uint64)(&total),
		(*uint64)(&totalFree),
	)
	if err != nil {
		return DiskSpaceInfo{}, err
	}
	info := DiskSpaceInfo{
		AvailableBytes: free,
		TotalBytes:     total,
		UsedBytes:      total - free,
	}
	return info.withStrings(), nil
}
