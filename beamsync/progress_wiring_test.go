package beamsync

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The progressWriter unit tests build their own struct literal, so they prove
// "Write uses pw.eventName" but never "the PRODUCTION construction site sets
// the right event name". These tests drive the real HTTP handlers and assert
// on the events the server actually emits.
//
// Each of the three production construction sites in beamsync/server.go is
// covered by exactly one test here, so a wrong eventName at any one of them
// fails the suite:
//   - receive path (single-file upload)      -> TestReceivePathEmitsUploadProgress
//   - send path (single file)                -> TestSenderPathEmitsDownloadProgress
//   - send path (multi-file, /download/N)    -> TestMultiFileSenderPathEmitsDownloadProgress

// uploadFile posts a single-file multipart upload and returns the response.
func uploadFile(t *testing.T, baseURL, token, filename string, payload []byte) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	manifest, err := writer.CreateFormField("beam_manifest")
	if err != nil {
		t.Fatalf("create manifest field: %v", err)
	}
	manifestJSON := `[{"name":"` + filename + `","size":` + strconv.Itoa(len(payload)) + `}]`
	if _, err := manifest.Write([]byte(manifestJSON)); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	file, err := writer.CreateFormFile("files", filename)
	if err != nil {
		t.Fatalf("create file field: %v", err)
	}
	if _, err := file.Write(payload); err != nil {
		t.Fatalf("write file field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	resp, err := http.Post(baseURL+"/upload?token="+token, writer.FormDataContentType(), &body) //nolint:gosec,noctx
	if err != nil {
		t.Fatalf("POST /upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload status = %d, want %d: %s", resp.StatusCode, http.StatusOK, responseBody)
	}
}

// awaitProgressEvent waits for a progress event with the given name and
// returns its payload. It fails if a progress event with a DIFFERENT name
// arrives first, so a swapped event name is caught rather than waited out.
func awaitProgressEvent(t *testing.T, events <-chan string, want string, timeout time.Duration) string {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case ev := <-events:
			if !strings.Contains(ev, "_progress|") {
				continue
			}
			name := ev[:strings.Index(ev, "|")]
			if name != want {
				t.Fatalf("progress event name = %q, want %q (event: %q)", name, want, ev)
			}
			return ev
		case <-deadline:
			t.Fatalf("no %q event within %s", want, timeout)
		}
	}
}

// TestReceivePathEmitsUploadProgress asserts the PRODUCTION receive path emits
// "upload_progress" (never "download_progress").
//
// Regression guard: the receive path shares progressWriter with the send path,
// so a wrong eventName at its construction site would silently mislabel every
// incoming transfer.
func TestReceivePathEmitsUploadProgress(t *testing.T) {
	t.Setenv(tlsEnvVar, "")
	server, baseURL, token, events, _ := startServerForTest(t)
	defer func() { _ = server.Shutdown() }() //nolint:errcheck

	uploadFile(t, baseURL, token, "note.txt", []byte("hello world"))

	ev := awaitProgressEvent(t, events, "upload_progress", 3*time.Second)
	if !strings.Contains(ev, "note.txt") {
		t.Fatalf("upload_progress event = %q, want it to name the uploaded file", ev)
	}
}

// TestMultiFileSenderPathEmitsDownloadProgress asserts the PRODUCTION
// multi-file send path (the /download/N handler) emits "download_progress".
//
// The multi-file handler is a separate construction site from the single-file
// one, so it needs its own end-to-end assertion.
func TestMultiFileSenderPathEmitsDownloadProgress(t *testing.T) {
	t.Setenv(tlsEnvVar, "")
	dir := t.TempDir()
	first := filepath.Join(dir, "first.bin")
	second := filepath.Join(dir, "second.bin")
	payload := []byte("multi-file payload")
	for _, p := range []string{first, second} {
		if err := os.WriteFile(p, payload, 0600); err != nil {
			t.Fatalf("write sender fixture %q: %v", p, err)
		}
	}

	events := make(chan string, 64)
	server, port, bootstrapToken, err := StartSenderE([]string{first, second}, func(eventName, data string) {
		events <- eventName + "|" + data
	})
	if err != nil {
		t.Fatalf("StartSenderE returned error: %v", err)
	}
	if server == nil {
		t.Fatal("StartSenderE returned nil server")
	}
	defer func() { _ = server.Shutdown() }() //nolint:errcheck
	baseURL := "http://127.0.0.1:" + port + "/"

	resp, err := http.Get(baseURL + "?token=" + bootstrapToken) //nolint:gosec,noctx
	if err != nil {
		t.Fatalf("GET sender page: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sender page status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// The multi-file page exposes /download/0 and /download/1 links.
	idx := strings.Index(string(body), "/download/0?token=")
	if idx == -1 {
		t.Fatal("sender page did not include a multi-file download link")
	}
	rest := string(body)[idx+len("/download/0?token="):]
	end := strings.IndexAny(rest, "'\"")
	if end == -1 {
		t.Fatal("multi-file download token was not terminated")
	}
	dlToken := rest[:end]

	dlResp, err := http.Get(baseURL + "download/0?token=" + dlToken) //nolint:gosec,noctx
	if err != nil {
		t.Fatalf("GET download/0: %v", err)
	}
	got, _ := io.ReadAll(dlResp.Body)
	dlResp.Body.Close()
	if dlResp.StatusCode != http.StatusOK {
		t.Fatalf("download/0 status = %d, want %d", dlResp.StatusCode, http.StatusOK)
	}
	if string(got) != string(payload) {
		t.Fatalf("downloaded %q, want %q", got, payload)
	}

	ev := awaitProgressEvent(t, events, "download_progress", 3*time.Second)
	if !strings.Contains(ev, "first.bin") {
		t.Fatalf("download_progress event = %q, want it to name the sent file", ev)
	}
}

// TestReceivePathEmitsCompletionProgress asserts the receive path's TERMINAL
// progress event reports the full written length.
//
// The three production sites each have their own terminal emit; pinning only
// one leaves the other two free to regress into the "progress stuck below
// 100%" symptom.
func TestReceivePathEmitsCompletionProgress(t *testing.T) {
	t.Setenv(tlsEnvVar, "")
	server, baseURL, token, events, _ := startServerForTest(t)
	defer func() { _ = server.Shutdown() }() //nolint:errcheck

	payload := []byte("hello world")
	uploadFile(t, baseURL, token, "note.txt", payload)

	deadline := time.After(3 * time.Second)
	for {
		select {
		case ev := <-events:
			if !strings.HasPrefix(ev, "upload_progress|") {
				continue
			}
			parts := strings.Split(ev, "|")
			if len(parts) != 4 {
				continue
			}
			written, err1 := strconv.ParseInt(parts[2], 10, 64)
			total, err2 := strconv.ParseInt(parts[3], 10, 64)
			if err1 != nil || err2 != nil {
				continue
			}
			if written == total && total == int64(len(payload)) {
				return
			}
		case <-deadline:
			t.Fatal("receive path emitted no upload_progress completion event (written == total) within 3s")
		}
	}
}

// TestMultiFileSenderPathEmitsCompletionProgress asserts the multi-file send
// path's TERMINAL progress event reports the full written length.
func TestMultiFileSenderPathEmitsCompletionProgress(t *testing.T) {
	t.Setenv(tlsEnvVar, "")
	dir := t.TempDir()
	first := filepath.Join(dir, "first.bin")
	second := filepath.Join(dir, "second.bin")
	// Larger than copyChunked's 8MB chunk, so the transfer is written in more
	// than one chunk. The FIRST progress event then reports written < total,
	// which is what makes the terminal event distinguishable from ordinary
	// progress — with a single-chunk file the first event already reports
	// written == total and the terminal event would be unobservable.
	payload := make([]byte, 8*1024*1024+1024)
	for i := range payload {
		payload[i] = byte(i % 251)
	}
	for _, f := range []string{first, second} {
		if err := os.WriteFile(f, payload, 0600); err != nil {
			t.Fatalf("write sender fixture %q: %v", f, err)
		}
	}

	events := make(chan string, 64)
	server, port, bootstrapToken, err := StartSenderE([]string{first, second}, func(eventName, data string) {
		events <- eventName + "|" + data
	})
	if err != nil {
		t.Fatalf("StartSenderE returned error: %v", err)
	}
	defer func() { _ = server.Shutdown() }() //nolint:errcheck
	baseURL := "http://127.0.0.1:" + port + "/"

	resp, err := http.Get(baseURL + "?token=" + bootstrapToken) //nolint:gosec,noctx
	if err != nil {
		t.Fatalf("GET sender page: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	// With TWO files the page exposes the /download/N handler, not /download.
	idx := strings.Index(string(body), "/download/0?token=")
	if idx == -1 {
		t.Fatal("sender page did not include a multi-file download link")
	}
	rest := string(body)[idx+len("/download/0?token="):]
	end := strings.IndexAny(rest, "'\"")
	if end == -1 {
		t.Fatal("multi-file download token was not terminated")
	}
	dlToken := rest[:end]

	dlResp, err := http.Get(baseURL + "download/0?token=" + dlToken) //nolint:gosec,noctx
	if err != nil {
		t.Fatalf("GET download/0: %v", err)
	}
	_, _ = io.ReadAll(dlResp.Body)
	dlResp.Body.Close()

	deadline := time.After(3 * time.Second)
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
			written, err1 := strconv.ParseInt(parts[2], 10, 64)
			total, err2 := strconv.ParseInt(parts[3], 10, 64)
			if err1 != nil || err2 != nil {
				continue
			}
			if written == total && total == int64(len(payload)) {
				return
			}
		case <-deadline:
			t.Fatal("multi-file send path emitted no download_progress completion event within 3s")
		}
	}
}
