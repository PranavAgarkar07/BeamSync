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

func TestTransferStatsTrackerRecordsSentFiles(t *testing.T) {
	tracker := newTransferStatsTracker()

	first := tracker.recordSent("report.pdf", 1024, 1)
	second := tracker.recordSent("photo.jpg", 2048, 0)

	if first.FilesSent != 1 || first.BytesSent != 1024 {
		t.Fatalf("first sent snapshot = %+v", first)
	}
	if second.FilesSent != 2 {
		t.Fatalf("files sent = %d, want 2", second.FilesSent)
	}
	if second.BytesSent != 3072 {
		t.Fatalf("bytes sent = %d, want 3072", second.BytesSent)
	}
	if second.LastFilename != "photo.jpg" {
		t.Fatalf("last filename = %q, want photo.jpg", second.LastFilename)
	}
	if second.LastDirection != TransferDirectionSend {
		t.Fatalf("last direction = %q, want %q", second.LastDirection, TransferDirectionSend)
	}
	if second.ActiveDownloads != 0 {
		t.Fatalf("active downloads = %d, want 0", second.ActiveDownloads)
	}
}

func TestTransferStatsJSON(t *testing.T) {
	tracker := newTransferStatsTracker()
	tracker.recordReceived("file.bin", 42, 0)
	stats := tracker.recordSent("download.bin", 84, 0)
	raw := transferStatsJSON(stats)

	var decoded TransferStats
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("stats JSON should decode: %v", err)
	}
	if decoded.FilesReceived != 1 || decoded.BytesReceived != 42 || decoded.FilesSent != 1 || decoded.BytesSent != 84 {
		t.Fatalf("decoded stats = %+v", decoded)
	}
}
