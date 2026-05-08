package beamsync

import "fmt"

type DiskSpaceInfo struct {
	AvailableBytes int64  `json:"availableBytes"`
	TotalBytes     int64  `json:"totalBytes"`
	UsedBytes      int64  `json:"usedBytes"`
	AvailableStr   string `json:"availableStr"`
	TotalStr       string `json:"totalStr"`
}

func formatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func (d DiskSpaceInfo) withStrings() DiskSpaceInfo {
	d.AvailableStr = formatBytes(d.AvailableBytes)
	d.TotalStr = formatBytes(d.TotalBytes)
	return d
}
