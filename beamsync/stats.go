package beamsync

import (
	"encoding/json"
	"sync"
	"time"
)

type TransferStats struct {
	StartedAt       string `json:"startedAt"`
	FilesReceived   int    `json:"filesReceived"`
	BytesReceived   int64  `json:"bytesReceived"`
	FilesSent       int    `json:"filesSent"`
	BytesSent       int64  `json:"bytesSent"`
	ActiveUploads   int32  `json:"activeUploads"`
	ActiveDownloads int32  `json:"activeDownloads"`
	LastFilename    string `json:"lastFilename,omitempty"`
	LastDirection   string `json:"lastDirection,omitempty"`
	LastUpdatedAt   string `json:"lastUpdatedAt,omitempty"`
}

type transferStatsTracker struct {
	mu            sync.Mutex
	startedAt     time.Time
	filesReceived int
	bytesReceived int64
	filesSent     int
	bytesSent     int64
	lastFilename  string
	lastDirection string
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
	t.lastDirection = TransferDirectionReceive
	t.lastUpdatedAt = time.Now()

	return t.snapshotLocked(activeUploads, 0)
}

func (t *transferStatsTracker) recordSent(filename string, bytes int64, activeDownloads int32) TransferStats {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.filesSent++
	t.bytesSent += bytes
	t.lastFilename = filename
	t.lastDirection = TransferDirectionSend
	t.lastUpdatedAt = time.Now()

	return t.snapshotLocked(0, activeDownloads)
}

func (t *transferStatsTracker) snapshot(activeUploads int32) TransferStats {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.snapshotLocked(activeUploads, 0)
}

func (t *transferStatsTracker) snapshotDownloads(activeDownloads int32) TransferStats {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.snapshotLocked(0, activeDownloads)
}

func (t *transferStatsTracker) snapshotLocked(activeUploads int32, activeDownloads int32) TransferStats {
	stats := TransferStats{
		StartedAt:       t.startedAt.Format(time.RFC3339),
		FilesReceived:   t.filesReceived,
		BytesReceived:   t.bytesReceived,
		FilesSent:       t.filesSent,
		BytesSent:       t.bytesSent,
		ActiveUploads:   activeUploads,
		ActiveDownloads: activeDownloads,
		LastFilename:    t.lastFilename,
		LastDirection:   t.lastDirection,
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
