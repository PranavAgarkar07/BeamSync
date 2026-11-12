package beamsync

import (
	"bytes"
	"testing"
	"time"
)

// TestProgressWriterSendPathEmitsDownloadProgress is the test-first proof (D-07)
// that the send path emits the "download_progress" event name.
//
// progressWriter.Write used to hardcode "upload_progress" for every path, so the
// desktop send-direction listener (App.svelte: EventsOn("download_progress", ...))
// never fired and send-direction progress was dead in the shipped app.
//
// The writer below mirrors the single-file send construction shape from
// beamsync/server.go. With minInterval = 0 every write emits, so this test
// isolates the event name from the throttle behavior.
func TestProgressWriterSendPathEmitsDownloadProgress(t *testing.T) {
	now := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	var events []string
	pw := &progressWriter{
		w:           &bytes.Buffer{},
		total:       int64(100),
		filename:    "report.pdf",
		eventName:   "download_progress",
		emit:        func(event string, data string) { events = append(events, event+"|"+data) },
		minInterval: 0,
		now:         func() time.Time { return now },
	}

	if _, err := pw.Write([]byte("hello")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("emitted events = %d (%v), want 1", len(events), events)
	}
	if events[0] != "download_progress|report.pdf|5|100" {
		t.Fatalf("emitted event = %q, want %q", events[0], "download_progress|report.pdf|5|100")
	}
}
