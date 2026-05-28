package beamsync

import (
	"encoding/json"
	"sync"
	"time"
)

type TransferStats struct {
	StartedAt     string `json:"startedAt"`
	FilesReceived int    `json:"filesReceived"`
	BytesReceived int64  `json:"bytesReceived"`
	ActiveUploads int32  `json:"activeUploads"`
	LastFilename  string `json:"lastFilename,omitempty"`
	LastUpdatedAt string `json:"lastUpdatedAt,omitempty"`
}

type transferStatsTracker struct {
	mu            sync.Mutex
	startedAt     time.Time
	filesReceived int
	bytesReceived int64
	lastFilename  string
	lastUpdatedAt time.Time
}

func newTransferStatsTracker() *transferStatsTracker {
	return &transferStatsTracker{startedAt: time.Now()}
}

func (t *transferStatsTracker) recordReceived(filename string, bytes int64, activeUploads int32) TransferStats {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.filesReceived++
	t.bytesReceived += bytes
	t.lastFilename = filename
	t.lastUpdatedAt = time.Now()

	return t.snapshotLocked(activeUploads)
}

func (t *transferStatsTracker) snapshot(activeUploads int32) TransferStats {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.snapshotLocked(activeUploads)
}

func (t *transferStatsTracker) snapshotLocked(activeUploads int32) TransferStats {
	stats := TransferStats{
		StartedAt:     t.startedAt.Format(time.RFC3339),
		FilesReceived: t.filesReceived,
		BytesReceived: t.bytesReceived,
		ActiveUploads: activeUploads,
		LastFilename:  t.lastFilename,
	}
	if !t.lastUpdatedAt.IsZero() {
		stats.LastUpdatedAt = t.lastUpdatedAt.Format(time.RFC3339)
	}
	return stats
}

func transferStatsJSON(stats TransferStats) string {
	data, err := json.Marshal(stats)
	if err != nil {
		return "{}"
	}
	return string(data)
}
