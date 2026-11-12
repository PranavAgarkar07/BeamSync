package beamsync

import (
	"bytes"
	"io"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// progressWriterFixture couples a progressWriter with the bytes it forwarded and
// the events it emitted, so the tests below can assert on both without
// repeating the production struct literal in every case.
type progressWriterFixture struct {
	writer *progressWriter
	sink   bytes.Buffer
	events []string
}

// newProgressWriterForTest builds a progressWriter over an in-memory sink.
//
// The literal mirrors the production construction sites in beamsync/server.go:
// the caller picks the per-path event name, the filename, the total, the throttle
// window, and the clock. lastEmit is left at its zero value unless the caller
// pins it — which is exactly what the production receive path does.
func newProgressWriterForTest(eventName string, filename string, total int64, minInterval time.Duration, now func() time.Time) *progressWriterFixture {
	f := &progressWriterFixture{}
	f.writer = &progressWriter{
		w:           &f.sink,
		total:       total,
		filename:    filename,
		eventName:   eventName,
		emit:        func(event string, data string) { f.events = append(f.events, event+"|"+data) },
		minInterval: minInterval,
		now:         now,
	}
	return f
}

// progressPayloadFields splits a progress payload into its event name, filename,
// written, and total fields.
//
// It requires the filename to be free of the '|' delimiter — filenames that
// contain '|' are a known limitation of this payload format, covered by
// TestProgressWriterPayloadFormat.
func progressPayloadFields(t *testing.T, payload string) (string, string, int64, int64) {
	t.Helper()
	parts := strings.Split(payload, "|")
	if len(parts) != 4 {
		t.Fatalf("payload %q has %d '|'-separated parts, want 4 (event|filename|written|total)", payload, len(parts))
	}
	written, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		t.Fatalf("payload %q has a non-numeric written field: %v", payload, err)
	}
	total, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		t.Fatalf("payload %q has a non-numeric total field: %v", payload, err)
	}
	return parts[0], parts[1], written, total
}

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
	f := newProgressWriterForTest("download_progress", "report.pdf", 100, 0, func() time.Time { return now })

	if _, err := f.writer.Write([]byte("hello")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	if len(f.events) != 1 {
		t.Fatalf("emitted events = %d (%v), want 1", len(f.events), f.events)
	}
	if f.events[0] != "download_progress|report.pdf|5|100" {
		t.Fatalf("emitted event = %q, want %q", f.events[0], "download_progress|report.pdf|5|100")
	}
}

// TestProgressWriterReceivePathEmitsUploadProgress pins the receive-path event
// name and the first-write-emits asymmetry.
//
// The receive path leaves lastEmit unset, so the zero time.Time is roughly two
// millennia in the past and the very first write always emits — whatever
// minInterval says.
func TestProgressWriterReceivePathEmitsUploadProgress(t *testing.T) {
	now := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	f := newProgressWriterForTest("upload_progress", "note.txt", 100, 500*time.Millisecond, func() time.Time { return now })

	if _, err := f.writer.Write([]byte("hello")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	if len(f.events) != 1 {
		t.Fatalf("emitted events = %d (%v), want 1", len(f.events), f.events)
	}
	if f.events[0] != "upload_progress|note.txt|5|100" {
		t.Fatalf("emitted event = %q, want %q", f.events[0], "upload_progress|note.txt|5|100")
	}
}

// TestProgressWriterEmitsEveryWriteWithoutThrottle pins the unthrottled
// behavior: with minInterval = 0 every write emits its own progress event.
func TestProgressWriterEmitsEveryWriteWithoutThrottle(t *testing.T) {
	now := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	f := newProgressWriterForTest("upload_progress", "note.txt", 100, 0, func() time.Time { return now })

	for _, chunk := range []string{"he", "ll", "o"} {
		if _, err := f.writer.Write([]byte(chunk)); err != nil {
			t.Fatalf("Write returned error: %v", err)
		}
	}

	want := []string{
		"upload_progress|note.txt|2|100",
		"upload_progress|note.txt|4|100",
		"upload_progress|note.txt|5|100",
	}
	if len(f.events) != len(want) {
		t.Fatalf("emitted events = %d (%v), want %d", len(f.events), f.events, len(want))
	}
	for i := range want {
		if f.events[i] != want[i] {
			t.Fatalf("emitted event %d = %q, want %q", i, f.events[i], want[i])
		}
	}
}

// TestProgressWriterCoalescesWritesWithinInterval pins the throttle: writes
// inside the interval coalesce into a single emit, and the first write after the
// interval emits again — carrying the coalesced byte count.
//
// The clock is injected (D-09); no real time.Sleep is used, so the test is
// neither slow nor flaky under CI load.
func TestProgressWriterCoalescesWritesWithinInterval(t *testing.T) {
	now := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	f := newProgressWriterForTest("upload_progress", "note.txt", 100, 500*time.Millisecond, func() time.Time { return now })
	// Pin lastEmit to the injected clock so the first write starts inside the
	// throttle window instead of the zero-value "always emit" state.
	f.writer.lastEmit = now

	if _, err := f.writer.Write([]byte("a")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if len(f.events) != 0 {
		t.Fatalf("emitted events = %d (%v), want 0 inside the throttle window", len(f.events), f.events)
	}

	now = now.Add(499 * time.Millisecond)
	if _, err := f.writer.Write([]byte("b")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if len(f.events) != 0 {
		t.Fatalf("emitted events = %d (%v), want 0 inside the throttle window", len(f.events), f.events)
	}

	now = now.Add(time.Millisecond) // 500ms since the pinned lastEmit
	if _, err := f.writer.Write([]byte("c")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if len(f.events) != 1 {
		t.Fatalf("emitted events = %d (%v), want 1 once the throttle window elapsed", len(f.events), f.events)
	}
	if f.events[0] != "upload_progress|note.txt|3|100" {
		t.Fatalf("emitted event = %q, want %q", f.events[0], "upload_progress|note.txt|3|100")
	}

	now = now.Add(100 * time.Millisecond)
	if _, err := f.writer.Write([]byte("d")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if len(f.events) != 1 {
		t.Fatalf("emitted events = %d (%v), want 1 inside the next throttle window", len(f.events), f.events)
	}

	now = now.Add(400 * time.Millisecond) // 500ms since the last emit
	if _, err := f.writer.Write([]byte("e")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if len(f.events) != 2 {
		t.Fatalf("emitted events = %d (%v), want 2 once the second throttle window elapsed", len(f.events), f.events)
	}
	if f.events[1] != "upload_progress|note.txt|5|100" {
		t.Fatalf("emitted event = %q, want %q", f.events[1], "upload_progress|note.txt|5|100")
	}
}

// TestProgressWriterZeroByteWriteEmitsNothing pins that a zero-byte write is a
// no-op: it emits no progress event and leaves the written counter and the
// destination untouched.
//
// The throttle window is deliberately left OPEN (lastEmit is not pinned) so the
// assertion proves the zero-byte guard in Write, not the throttle. Pinning
// lastEmit inside the window would make the test pass for the wrong reason.
func TestProgressWriterZeroByteWriteEmitsNothing(t *testing.T) {
	now := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	f := newProgressWriterForTest("upload_progress", "note.txt", 100, 500*time.Millisecond, func() time.Time { return now })
	// lastEmit stays at its zero value, so now.Sub(lastEmit) >= minInterval and
	// the throttle would permit an emit if Write did not guard against n == 0.

	if _, err := f.writer.Write(nil); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	if len(f.events) != 0 {
		t.Fatalf("emitted events = %d (%v), want 0 for a zero-byte write", len(f.events), f.events)
	}
	if f.writer.written != 0 {
		t.Fatalf("written = %d, want 0 for a zero-byte write", f.writer.written)
	}
	if f.sink.Len() != 0 {
		t.Fatalf("sink bytes = %d, want 0 for a zero-byte write", f.sink.Len())
	}
}

// TestProgressWriterPayloadFormat pins the wire format of the progress payload:
// "<eventName>|<filename>|<written>|<total>", where written and total are byte
// counts.
func TestProgressWriterPayloadFormat(t *testing.T) {
	now := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	t.Run("payload is filename|written|total", func(t *testing.T) {
		f := newProgressWriterForTest("upload_progress", "note.txt", 100, 0, clock)

		if _, err := f.writer.Write([]byte("hello")); err != nil {
			t.Fatalf("Write returned error: %v", err)
		}

		if len(f.events) != 1 {
			t.Fatalf("emitted events = %d (%v), want 1", len(f.events), f.events)
		}
		if f.events[0] != "upload_progress|note.txt|5|100" {
			t.Fatalf("emitted event = %q, want %q", f.events[0], "upload_progress|note.txt|5|100")
		}
	})

	t.Run("written counts bytes not code points", func(t *testing.T) {
		f := newProgressWriterForTest("upload_progress", "note.txt", 100, 0, clock)

		// "héllo" is 5 code points but 6 UTF-8 bytes.
		if _, err := f.writer.Write([]byte("héllo")); err != nil {
			t.Fatalf("Write returned error: %v", err)
		}

		if len(f.events) != 1 {
			t.Fatalf("emitted events = %d (%v), want 1", len(f.events), f.events)
		}
		if f.events[0] != "upload_progress|note.txt|6|100" {
			t.Fatalf("emitted event = %q, want %q", f.events[0], "upload_progress|note.txt|6|100")
		}
	})

	t.Run("completion emit reports written == total", func(t *testing.T) {
		f := newProgressWriterForTest("upload_progress", "note.txt", 5, 0, clock)

		if _, err := f.writer.Write([]byte("hello")); err != nil {
			t.Fatalf("Write returned error: %v", err)
		}

		if len(f.events) != 1 {
			t.Fatalf("emitted events = %d (%v), want 1", len(f.events), f.events)
		}
		if f.events[0] != "upload_progress|note.txt|5|5" {
			t.Fatalf("emitted event = %q, want %q", f.events[0], "upload_progress|note.txt|5|5")
		}
	})

	t.Run("byte counts stay exact at realistic file sizes", func(t *testing.T) {
		const total = int64(4) << 30 // 4 GiB
		f := newProgressWriterForTest("upload_progress", "movie.mkv", total, 0, clock)

		if _, err := f.writer.Write([]byte("x")); err != nil {
			t.Fatalf("Write returned error: %v", err)
		}

		want := "upload_progress|movie.mkv|1|4294967296"
		if len(f.events) != 1 {
			t.Fatalf("emitted events = %d (%v), want 1", len(f.events), f.events)
		}
		if f.events[0] != want {
			t.Fatalf("emitted event = %q, want %q", f.events[0], want)
		}
	})

	t.Run("filename containing the delimiter is a known limitation", func(t *testing.T) {
		// KNOWN LIMITATION (not fixed here): the payload format is '|'
		// delimited, so a filename that itself contains '|' makes the payload
		// ambiguous — the frontend splits on '|' and reads the first three
		// parts (App.svelte: handleProgressUpdate), so the filename is
		// truncated and the byte counts shift. Changing the wire format would
		// break the Go->Svelte contract, so this is recorded, not fixed.
		f := newProgressWriterForTest("upload_progress", "a|b.txt", 100, 0, clock)

		if _, err := f.writer.Write([]byte("hello")); err != nil {
			t.Fatalf("Write returned error: %v", err)
		}

		if len(f.events) != 1 {
			t.Fatalf("emitted events = %d (%v), want 1", len(f.events), f.events)
		}
		if f.events[0] != "upload_progress|a|b.txt|5|100" {
			t.Fatalf("emitted event = %q, want %q", f.events[0], "upload_progress|a|b.txt|5|100")
		}
		// The frontend destructures [filename, written, total] from the split
		// array, so 5 parts leave filename="a", written="b.txt" (NaN) and
		// total="5" — the byte counts are misread.
		if parts := strings.Split(f.events[0], "|"); len(parts) != 5 {
			t.Fatalf("payload %q splits into %d '|'-separated parts, want 5 — the filename delimiter is ambiguous", f.events[0], len(parts))
		}
	})
}

// TestProgressWriterSingleWriterPerTransferPath pins the single-writer contract:
// progressWriter.Write is not safe for concurrent use, so each transfer path
// owns exactly one writer and drives it from a single goroutine. Under that
// discipline the byte accounting stays exact and the emitted payloads stay
// monotonic.
func TestProgressWriterSingleWriterPerTransferPath(t *testing.T) {
	payload := strings.Repeat("beam-sync", 12) // 108 bytes
	now := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	t.Run("receive path", func(t *testing.T) {
		// The receive path streams a multipart part whose size is not plumbed
		// into the writer, so total stays 0 for the duration of the transfer
		// and the frontend shows an unknown total until the final emit.
		f := newProgressWriterForTest("upload_progress", "note.txt", 0, 0, clock)

		written, err := copyChunked(f.writer, strings.NewReader(payload), 17)
		if err != nil {
			t.Fatalf("copyChunked returned error: %v", err)
		}
		if written != int64(len(payload)) {
			t.Fatalf("copyChunked bytes = %d, want %d", written, len(payload))
		}
		if f.writer.written != int64(len(payload)) {
			t.Fatalf("progressWriter written = %d, want %d", f.writer.written, len(payload))
		}
		if f.sink.String() != payload {
			t.Fatal("progressWriter did not forward the payload to the sink")
		}
		if len(f.events) == 0 {
			t.Fatal("receive path emitted no progress events")
		}

		event, filename, lastWritten, _ := progressPayloadFields(t, f.events[len(f.events)-1])
		if event != "upload_progress" {
			t.Fatalf("final event name = %q, want %q", event, "upload_progress")
		}
		if filename != "note.txt" {
			t.Fatalf("final event filename = %q, want %q", filename, "note.txt")
		}
		if lastWritten != int64(len(payload)) {
			t.Fatalf("final event written = %d, want %d", lastWritten, len(payload))
		}
	})

	t.Run("send path", func(t *testing.T) {
		f := newProgressWriterForTest("download_progress", "report.pdf", int64(len(payload)), 0, clock)

		written, err := copyChunked(f.writer, strings.NewReader(payload), 17)
		if err != nil {
			t.Fatalf("copyChunked returned error: %v", err)
		}
		if written != int64(len(payload)) {
			t.Fatalf("copyChunked bytes = %d, want %d", written, len(payload))
		}
		if f.writer.written != int64(len(payload)) {
			t.Fatalf("progressWriter written = %d, want %d", f.writer.written, len(payload))
		}
		if f.sink.String() != payload {
			t.Fatal("progressWriter did not forward the payload to the sink")
		}
		if len(f.events) == 0 {
			t.Fatal("send path emitted no progress events")
		}

		event, filename, lastWritten, total := progressPayloadFields(t, f.events[len(f.events)-1])
		if event != "download_progress" {
			t.Fatalf("final event name = %q, want %q", event, "download_progress")
		}
		if filename != "report.pdf" {
			t.Fatalf("final event filename = %q, want %q", filename, "report.pdf")
		}
		if lastWritten != int64(len(payload)) {
			t.Fatalf("final event written = %d, want %d", lastWritten, len(payload))
		}
		if total != int64(len(payload)) {
			t.Fatalf("final event total = %d, want %d", total, len(payload))
		}
	})

	t.Run("emitted byte counts stay monotonic", func(t *testing.T) {
		f := newProgressWriterForTest("upload_progress", "note.txt", int64(len(payload)), 0, clock)

		if _, err := copyChunked(f.writer, strings.NewReader(payload), 17); err != nil {
			t.Fatalf("copyChunked returned error: %v", err)
		}

		var previous int64
		for i, event := range f.events {
			_, _, written, _ := progressPayloadFields(t, event)
			if written < previous {
				t.Fatalf("emitted event %d written = %d, want >= %d (byte counts must not go backwards)", i, written, previous)
			}
			previous = written
		}
	})
}

// TestEventDispatcherCloseStopsWorkerAndIsIdempotent pins the dispatcher's
// shutdown behaviour: close() must stop the worker goroutine (no leak), must
// be safe to call twice, and must make later emits no-ops rather than
// panicking on a send to a closed channel.
//
// The leak is detected by creating many dispatchers at once: each adds one
// worker goroutine, so the signal is large enough that ordinary goroutine
// churn elsewhere in the test binary cannot mask it.
func TestEventDispatcherCloseStopsWorkerAndIsIdempotent(t *testing.T) {
	const dispatchers = 20

	baseline := runtime.NumGoroutine()
	created := make([]*eventDispatcher, 0, dispatchers)
	for i := 0; i < dispatchers; i++ {
		d := newEventDispatcher(4)
		created = append(created, d)
	}

	// Each dispatcher owns one worker goroutine; wait for them to start.
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() < baseline+dispatchers {
		if time.Now().After(deadline) {
			t.Fatalf("workers did not start: baseline=%d now=%d", baseline, runtime.NumGoroutine())
		}
		time.Sleep(5 * time.Millisecond)
	}

	for _, d := range created {
		d.close()
		// Idempotent: a second close must not panic on an already-closed channel.
		d.close()
	}

	// Emitting after close must be a safe no-op, not a panic.
	if created[0].emit(eventDispatchJob{emit: func(string, string) {}, event: "after_close"}) {
		t.Fatal("emit after close returned true, want false")
	}

	// The worker goroutines must be gone.
	deadline = time.Now().Add(3 * time.Second)
	for runtime.NumGoroutine() > baseline+dispatchers/2 {
		if time.Now().After(deadline) {
			t.Fatalf("goroutines did not settle after close: baseline=%d now=%d (leaked workers?)",
				baseline, runtime.NumGoroutine())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// shortWriter reports a short write, as the io.Writer contract allows.
type shortWriter struct {
	n int
}

func (w *shortWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	w.n = len(p) / 2
	return w.n, io.ErrShortWrite
}

// TestProgressWriterCountsBytesActuallyWritten pins that the written counter
// tracks the bytes the destination actually accepted (n), not the length of
// the caller's buffer.
//
// The send path writes into an http.ResponseWriter, which may legitimately
// accept fewer bytes than were offered.
func TestProgressWriterCountsBytesActuallyWritten(t *testing.T) {
	now := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	sink := &shortWriter{}
	f := &progressWriterFixture{}
	f.writer = &progressWriter{
		w:           sink,
		total:       100,
		filename:    "note.txt",
		eventName:   "upload_progress",
		emit:        func(event string, data string) { f.events = append(f.events, event+"|"+data) },
		minInterval: 0,
		now:         func() time.Time { return now },
	}

	payload := []byte("0123456789") // 10 bytes
	if _, err := f.writer.Write(payload); err == nil {
		t.Fatal("Write returned nil error, want io.ErrShortWrite")
	}

	// The destination accepted only half; the counter must reflect that.
	if f.writer.written != int64(sink.n) {
		t.Fatalf("written = %d, want %d (the bytes the destination accepted)", f.writer.written, sink.n)
	}
	if f.writer.written != 5 {
		t.Fatalf("written = %d, want 5", f.writer.written)
	}
}
