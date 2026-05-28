package beamsync

import (
	"encoding/json"
	"testing"
)

func TestTransferStatsTrackerRecordsReceivedFiles(t *testing.T) {
	tracker := newTransferStatsTracker()

	first := tracker.recordReceived("one.txt", 128, 1)
	second := tracker.recordReceived("two.txt", 256, 0)

	if first.FilesReceived != 1 || first.BytesReceived != 128 {
		t.Fatalf("first snapshot = %+v", first)
	}
	if second.FilesReceived != 2 {
		t.Fatalf("files received = %d, want 2", second.FilesReceived)
	}
	if second.BytesReceived != 384 {
		t.Fatalf("bytes received = %d, want 384", second.BytesReceived)
	}
	if second.LastFilename != "two.txt" {
		t.Fatalf("last filename = %q, want two.txt", second.LastFilename)
	}
	if second.ActiveUploads != 0 {
		t.Fatalf("active uploads = %d, want 0", second.ActiveUploads)
	}
}

func TestTransferStatsJSON(t *testing.T) {
	tracker := newTransferStatsTracker()
	stats := tracker.recordReceived("file.bin", 42, 0)
	raw := transferStatsJSON(stats)

	var decoded TransferStats
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("stats JSON should decode: %v", err)
	}
	if decoded.FilesReceived != 1 || decoded.BytesReceived != 42 {
		t.Fatalf("decoded stats = %+v", decoded)
	}
}
