package beamsync

import (
	"encoding/json"
	"fmt"
	"sync"
)

// TransferDirection indicates whether a recorded transfer was inbound or outbound.
type TransferDirection string

const (
	TransferDirectionReceive TransferDirection = "receive"
	TransferDirectionSend    TransferDirection = "send"
)

// TransferStats is an immutable snapshot of cumulative transfer activity.
type TransferStats struct {
	FilesReceived     int               `json:"files_received"`
	BytesReceived     int64             `json:"bytes_received"`
	FilesSent         int               `json:"files_sent"`
	BytesSent         int64             `json:"bytes_sent"`
	LastFilename      string            `json:"last_filename"`
	LastDirection     TransferDirection `json:"last_direction"`
	ActiveUploads     int               `json:"active_uploads"`
	ActiveDownloads   int               `json:"active_downloads"`
	IntegrityFailures int               `json:"integrity_failures"`
}

// TotalFiles returns the combined count of received and sent files.
func (s TransferStats) TotalFiles() int {
	return s.FilesReceived + s.FilesSent
}

// TotalBytes returns the combined byte count of received and sent transfers.
func (s TransferStats) TotalBytes() int64 {
	return s.BytesReceived + s.BytesSent
}

// HasFailures reports whether any integrity failures have been recorded.
func (s TransferStats) HasFailures() bool {
	return s.IntegrityFailures > 0
}

// transferStatsTracker accumulates transfer statistics in a thread-safe manner.
type transferStatsTracker struct {
	mu    sync.Mutex
	stats TransferStats
}

func newTransferStatsTracker() *transferStatsTracker {
	return &transferStatsTracker{}
}

// recordReceived records an inbound file transfer and returns the updated snapshot.
func (t *transferStatsTracker) recordReceived(filename string, bytes int64, activeUploads int) TransferStats {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stats.FilesReceived++
	t.stats.BytesReceived += bytes
	t.stats.LastFilename = filename
	t.stats.LastDirection = TransferDirectionReceive
	t.stats.ActiveUploads = activeUploads

	return t.stats
}

// recordSent records an outbound file transfer and returns the updated snapshot.
func (t *transferStatsTracker) recordSent(filename string, bytes int64, activeDownloads int) TransferStats {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stats.FilesSent++
	t.stats.BytesSent += bytes
	t.stats.LastFilename = filename
	t.stats.LastDirection = TransferDirectionSend
	t.stats.ActiveDownloads = activeDownloads

	return t.stats
}

// recordIntegrityFailure increments the integrity failure counter and returns the updated snapshot.
func (t *transferStatsTracker) recordIntegrityFailure() TransferStats {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stats.IntegrityFailures++

	return t.stats
}

// snapshot returns the current stats without mutating any counters.
func (t *transferStatsTracker) snapshot() TransferStats {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.stats
}

// reset clears all counters back to zero and returns the cleared snapshot.
func (t *transferStatsTracker) reset() TransferStats {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stats = TransferStats{}

	return t.stats
}

// beginUpload increments the active upload count, e.g. when a remote peer
// starts pushing a file to us.
func (t *transferStatsTracker) beginUpload() TransferStats {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stats.ActiveUploads++

	return t.stats
}

// endUpload decrements the active upload count, never going below zero.
func (t *transferStatsTracker) endUpload() TransferStats {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.stats.ActiveUploads > 0 {
		t.stats.ActiveUploads--
	}

	return t.stats
}

// beginDownload increments the active download count, e.g. when we start
// pulling a file from a remote peer.
func (t *transferStatsTracker) beginDownload() TransferStats {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stats.ActiveDownloads++

	return t.stats
}

// endDownload decrements the active download count, never going below zero.
func (t *transferStatsTracker) endDownload() TransferStats {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.stats.ActiveDownloads > 0 {
		t.stats.ActiveDownloads--
	}

	return t.stats
}

// transferStatsJSON serializes a TransferStats snapshot to a JSON string.
func transferStatsJSON(stats TransferStats) string {
	data, err := json.Marshal(stats)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// transferStatsSummary renders a short human-readable summary line, useful
// for logging or CLI status output.
func transferStatsSummary(stats TransferStats) string {
	return fmt.Sprintf(
		"received %d file(s) / %d bytes, sent %d file(s) / %d bytes, last=%q (%s), active up/down=%d/%d, failures=%d",
		stats.FilesReceived, stats.BytesReceived,
		stats.FilesSent, stats.BytesSent,
		stats.LastFilename, stats.LastDirection,
		stats.ActiveUploads, stats.ActiveDownloads,
		stats.IntegrityFailures,
	)
}