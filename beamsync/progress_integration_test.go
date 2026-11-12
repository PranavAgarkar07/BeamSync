package beamsync

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The unit tests in progress_test.go build their own progressWriter literal, so
// they prove "Write uses pw.eventName" but NOT "the production send path sets
// download_progress". A wrong eventName at the production construction sites
// left the whole suite green.
//
// These tests drive the REAL sender handler over HTTP and assert on the events
// the server actually emits, so a wrong value at beamsync/server.go's
// construction sites fails here.

func startSenderWithEvents(t *testing.T, filePath string) (*HTTPServer, string, string, <-chan string) {
	t.Helper()
	events := make(chan string, 64)
	server, port, bootstrapToken, err := StartSenderE([]string{filePath}, func(eventName, data string) {
		select {
		case events <- eventName + "|" + data:
		default:
			// Never block here: this callback runs on the SHARED global
			// event dispatcher worker. A full channel would wedge that
			// worker and hang every later event test instead of failing
			// one cleanly.
		}
	})
	if err != nil {
		t.Fatalf("StartSenderE returned error: %v", err)
	}
	if server == nil {
		t.Fatal("StartSenderE returned nil server")
	}
	t.Cleanup(func() { _ = server.Shutdown() }) //nolint:errcheck
	return server, "http://127.0.0.1:" + port + "/", bootstrapToken, events
}

// downloadTokenFromSenderPage extracts the single-use download token the
// sender page embeds.
func downloadTokenFromSenderPage(t *testing.T, pageBody string) string {
	t.Helper()
	const linkPrefix = "/download?token="
	start := strings.Index(pageBody, linkPrefix)
	if start == -1 {
		t.Fatal("sender page did not include a download link")
	}
	start += len(linkPrefix)
	end := strings.IndexAny(pageBody[start:], "'\"")
	if end == -1 {
		t.Fatal("download token was not terminated")
	}
	return pageBody[start : start+end]
}

// TestSenderPathEmitsDownloadProgress asserts the PRODUCTION send path emits
// "download_progress" (never "upload_progress").
//
// Regression guard: Phase 1 deleted downloadProgressWriter and pointed the
// sender at progressWriter, whose Write hardcoded "upload_progress". The
// desktop send-direction listener (App.svelte) therefore never fired.
func TestSenderPathEmitsDownloadProgress(t *testing.T) {
	t.Setenv(tlsEnvVar, "")
	filePath := filepath.Join(t.TempDir(), "payload.bin")
	payload := []byte("0123456789")
	if err := os.WriteFile(filePath, payload, 0600); err != nil {
		t.Fatalf("write sender fixture: %v", err)
	}

	_, baseURL, bootstrapToken, events := startSenderWithEvents(t, filePath)

	resp, err := http.Get(baseURL + "?token=" + bootstrapToken) //nolint:gosec,noctx
	if err != nil {
		t.Fatalf("GET sender page: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sender page status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	dlToken := downloadTokenFromSenderPage(t, string(body))
	dlResp, err := http.Get(baseURL + "download?token=" + dlToken) //nolint:gosec,noctx
	if err != nil {
		t.Fatalf("GET download: %v", err)
	}
	got, _ := io.ReadAll(dlResp.Body)
	dlResp.Body.Close()
	if dlResp.StatusCode != http.StatusOK {
		t.Fatalf("download status = %d, want %d", dlResp.StatusCode, http.StatusOK)
	}
	if string(got) != string(payload) {
		t.Fatalf("downloaded %q, want %q", got, payload)
	}

	// The send path must emit at least one download_progress event, even for a
	// transfer that completes inside the 500ms throttle window.
	deadline := time.After(2 * time.Second)
	sawDownloadProgress := false
	for !sawDownloadProgress {
		select {
		case ev := <-events:
			if strings.HasPrefix(ev, "download_progress|") {
				sawDownloadProgress = true
			} else if strings.HasPrefix(ev, "upload_progress|") {
				t.Fatalf("send path emitted upload_progress: %q — the send path must emit download_progress", ev)
			}
		case <-deadline:
			t.Fatal("send path emitted no download_progress event within 2s")
		}
	}
}

// TestSenderPathCompletionEventReportsFullLength asserts the send path emits a
// completion event whose written count equals the total.
//
// Without a completion event, a fast transfer (finished inside the 500ms
// throttle window) emits no progress at all and the desktop send-direction
// panel never moves.
//
// The fixture is deliberately LARGER than copyChunked's 8MB chunk size, so the
// transfer is written in more than one chunk. That matters: with a single-chunk
// file the very first Write already reports written == total, which would make
// the completion event indistinguishable from ordinary progress and let the
// event be deleted without failing this test. With a chunked file the first
// progress event reports written < total, so only the completion event can
// satisfy the assertion.
func TestSenderPathCompletionEventReportsFullLength(t *testing.T) {
	t.Setenv(tlsEnvVar, "")
	filePath := filepath.Join(t.TempDir(), "payload.bin")
	// 8MB + 1KB: crosses copyChunked's 8MB chunk boundary.
	payload := make([]byte, 8*1024*1024+1024)
	for i := range payload {
		payload[i] = byte(i % 251)
	}
	if err := os.WriteFile(filePath, payload, 0600); err != nil {
		t.Fatalf("write sender fixture: %v", err)
	}

	_, baseURL, bootstrapToken, events := startSenderWithEvents(t, filePath)

	resp, err := http.Get(baseURL + "?token=" + bootstrapToken) //nolint:gosec,noctx
	if err != nil {
		t.Fatalf("GET sender page: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	dlToken := downloadTokenFromSenderPage(t, string(body))
	dlResp, err := http.Get(baseURL + "download?token=" + dlToken) //nolint:gosec,noctx
	if err != nil {
		t.Fatalf("GET download: %v", err)
	}
	got, _ := io.ReadAll(dlResp.Body)
	dlResp.Body.Close()
	if len(got) != len(payload) {
		t.Fatalf("downloaded %d bytes, want %d", len(got), len(payload))
	}

	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev := <-events:
			if !strings.HasPrefix(ev, "download_progress|") {
				continue
			}
			parts := strings.Split(ev, "|")
			if len(parts) != 4 {
				continue
			}
			written, err := strconv.ParseInt(parts[2], 10, 64)
			if err != nil {
				continue
			}
			total, err := strconv.ParseInt(parts[3], 10, 64)
			if err != nil {
				continue
			}
			if written == total && total == int64(len(payload)) {
				return
			}
		case <-deadline:
			t.Fatal("send path emitted no download_progress completion event (written == total) within 5s")
		}
	}
}
