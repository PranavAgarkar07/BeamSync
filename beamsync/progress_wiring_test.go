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

// uploadFileWithManifestName posts a multipart upload whose manifest declares
// manifestName while the part's filename is partName.
func uploadFileWithManifestName(t *testing.T, baseURL, token, manifestName, partName string, payload []byte) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	manifest, err := writer.CreateFormField("beam_manifest")
	if err != nil {
		t.Fatalf("create manifest field: %v", err)
	}
	manifestJSON := `[{"name":"` + manifestName + `","size":` + strconv.Itoa(len(payload)) + `}]`
	if _, err := manifest.Write([]byte(manifestJSON)); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	file, err := writer.CreateFormFile("files", partName)
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

// TestReceivePathReportsDeclaredTotal asserts the receive path reports the
// file's DECLARED size as the progress total.
//
// The manifest carries each file's size and it was parsed into fileSizes —
// then discarded, leaving progressWriter.total at 0. With total 0 the
// desktop cannot compute a percentage and shows the upload as indeterminate.
//
// The fixture is larger than copyChunked's 8MB chunk so the transfer spans
// more than one write: the first progress event then reports written < total,
// which is what makes the terminal event distinguishable.
func TestReceivePathReportsDeclaredTotal(t *testing.T) {
	t.Setenv(tlsEnvVar, "")
	server, baseURL, token, events, _ := startServerForTest(t)
	defer func() { _ = server.Shutdown() }() //nolint:errcheck

	payload := make([]byte, 8*1024*1024+1024)
	for i := range payload {
		payload[i] = byte(i % 251)
	}
	uploadFile(t, baseURL, token, "big.bin", payload)

	deadline := time.After(10 * time.Second)
	sawPartial := false
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
			if total != int64(len(payload)) {
				t.Fatalf("progress event %q reports total=%d, want the declared size %d", ev, total, len(payload))
			}
			if written < total {
				sawPartial = true
			}
			if written == total {
				if !sawPartial {
					t.Fatal("no partial progress event was seen before the completion event")
				}
				return
			}
		case <-deadline:
			t.Fatal("receive path emitted no completion event with the declared total within 10s")
		}
	}
}

// TestReceivePathFindsDeclaredSizeForPathQualifiedManifestName pins that the
// declared size is found even when the manifest carries a path-qualified file
// name.
//
// The manifest key is stored verbatim while the multipart part name is
// reduced with filepath.Base. Without normalising both forms the lookup
// silently misses and the progress total falls back to 0 — the indeterminate
// upload progress bar.
func TestReceivePathFindsDeclaredSizeForPathQualifiedManifestName(t *testing.T) {
	t.Setenv(tlsEnvVar, "")
	server, baseURL, token, events, _ := startServerForTest(t)
	defer func() { _ = server.Shutdown() }() //nolint:errcheck

	payload := []byte("qualified name payload")
	// The manifest uses a path-qualified name; the multipart part uses the
	// base name only.
	uploadFileWithManifestName(t, baseURL, token, "sub/dir/note.txt", "note.txt", payload)

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
			total, err := strconv.ParseInt(parts[3], 10, 64)
			if err != nil {
				continue
			}
			if total != int64(len(payload)) {
				t.Fatalf("progress event %q reports total=%d, want the declared size %d", ev, total, len(payload))
			}
			return
		case <-deadline:
			t.Fatal("receive path emitted no upload_progress event within 3s")
		}
	}
}

// TestReceivePathGivesEachFileItsOwnDeclaredSize pins that two files uploaded
// in the SAME request, whose names share a base name, each report their own
// size.
//
// Keying the manifest by base name alone collapses them onto one entry, so
// the second file would report the first file's size and its progress total
// would be wrong (written > total).
func TestReceivePathGivesEachFileItsOwnDeclaredSize(t *testing.T) {
	t.Setenv(tlsEnvVar, "")
	server, baseURL, token, events, _ := startServerForTest(t)
	defer func() { _ = server.Shutdown() }() //nolint:errcheck

	firstPayload := []byte("aaaaaaaaaa")     // 10 bytes
	secondPayload := []byte("bbbbbbbbbbbbb") // 13 bytes

	// ONE request, TWO parts, manifest declaring two path-qualified names
	// that share the base name "report.txt".
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	manifest, err := writer.CreateFormField("beam_manifest")
	if err != nil {
		t.Fatalf("create manifest field: %v", err)
	}
	manifestJSON := `[{"name":"a/report.txt","size":10},{"name":"b/report.txt","size":13}]`
	if _, err := manifest.Write([]byte(manifestJSON)); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	for _, payload := range [][]byte{firstPayload, secondPayload} {
		file, err := writer.CreateFormFile("files", "report.txt")
		if err != nil {
			t.Fatalf("create file field: %v", err)
		}
		if _, err := file.Write(payload); err != nil {
			t.Fatalf("write file field: %v", err)
		}
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

	totals := map[int64]bool{}
	deadline := time.After(5 * time.Second)
	for len(totals) < 2 {
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
			if written > total {
				t.Fatalf("progress event %q reports written=%d > total=%d", ev, written, total)
			}
			totals[total] = true
		case <-deadline:
			t.Fatalf("expected totals for both uploads, saw %v", totals)
		}
	}

	if !totals[int64(len(firstPayload))] {
		t.Fatalf("totals %v missing the first file's size %d", totals, len(firstPayload))
	}
	if !totals[int64(len(secondPayload))] {
		t.Fatalf("totals %v missing the second file's size %d", totals, len(secondPayload))
	}
}
