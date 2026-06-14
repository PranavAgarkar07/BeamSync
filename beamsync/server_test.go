package beamsync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTokenMiddlewareRejectsMissingOrInvalidToken(t *testing.T) {
	handler := tokenMiddleware("secret-token", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	for _, target := range []string{"/upload", "/upload?token=wrong"} {
		req := httptest.NewRequest(http.MethodPost, target, nil)
		rec := httptest.NewRecorder()

		handler(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s returned %d, want %d", target, rec.Code, http.StatusForbidden)
		}
	}
}

func TestTokenMiddlewareAllowsValidTokenAndOptions(t *testing.T) {
	handler := tokenMiddleware("secret-token", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	req := httptest.NewRequest(http.MethodPost, "/upload?token=secret-token", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("valid token status = %d, want %d", rec.Code, http.StatusAccepted)
	}

	optionsReq := httptest.NewRequest(http.MethodOptions, "/upload", nil)
	optionsRec := httptest.NewRecorder()
	handler(optionsRec, optionsReq)
	if optionsRec.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS status = %d, want %d", optionsRec.Code, http.StatusNoContent)
	}
}

func TestAutoRenamePathFindsNonCollidingName(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "photo.png"), []byte("first"), 0644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	got := autoRenamePath(dir, "photo.png")
	want := filepath.Join(dir, "photo(1).png")

	if got != want {
		t.Fatalf("autoRenamePath = %q, want %q", got, want)
	}
}

func TestGenerateTokenReturnsHexToken(t *testing.T) {
	token := generateToken()
	if len(token) != 32 {
		t.Fatalf("token length = %d, want 32", len(token))
	}
	for _, ch := range token {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			t.Fatalf("token contains non-hex character %q", ch)
		}
	}
}

func TestGenerateTokenDoesNotCollideInSmallSample(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token := generateToken()
		if seen[token] {
			t.Fatalf("generateToken collision at token %q", token)
		}
		seen[token] = true
	}
}

func TestSetCORSHeaders(t *testing.T) {
	rec := httptest.NewRecorder()

	setCORSHeaders(rec)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Allow-Origin = %q, want *", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, OPTIONS" {
		t.Fatalf("Allow-Methods = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type" {
		t.Fatalf("Allow-Headers = %q", got)
	}
}

func TestCopyChunkedCopiesDataAndReportsCount(t *testing.T) {
	payload := strings.Repeat("beam-sync", 1024)
	var dst bytes.Buffer

	n, err := copyChunked(&dst, strings.NewReader(payload), 17)
	if err != nil {
		t.Fatalf("copyChunked returned error: %v", err)
	}
	if n != int64(len(payload)) {
		t.Fatalf("copyChunked bytes = %d, want %d", n, len(payload))
	}
	if dst.String() != payload {
		t.Fatal("copyChunked did not preserve payload")
	}
}

func TestSafeEmitHandlesNilCallbackAndCallbackPanic(t *testing.T) {
	safeEmit(nil, "noop", "")

	done := make(chan struct{})
	safeEmit(func(string, string) {
		defer close(done)
		panic("boom")
	}, "panic", "payload")

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("safeEmit did not invoke callback")
	}
}

func TestSafeEmitDoesNotSpawnPerEventGoroutines(t *testing.T) {
	block := make(chan struct{})
	started := make(chan struct{}, 1)
	callback := func(string, string) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-block
	}

	before := runtime.NumGoroutine()
	for i := 0; i < 50; i++ {
		safeEmit(callback, "upload_progress", "file.txt|1|50")
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("dispatcher did not start processing events")
	}

	after := runtime.NumGoroutine()
	close(block)

	if growth := after - before; growth > 5 {
		t.Fatalf("goroutine count grew by %d; safeEmit should use one bounded dispatcher", growth)
	}
}

func TestProcessManifestCases(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := processManifest(strings.NewReader(`[{"name":"a.txt","size":12},{"name":"b.bin","size":34}]`))
		if err != nil {
			t.Fatalf("processManifest returned error: %v", err)
		}
		if got["a.txt"] != 12 || got["b.bin"] != 34 {
			t.Fatalf("processManifest = %#v", got)
		}
	})

	t.Run("empty manifest", func(t *testing.T) {
		got, err := processManifest(strings.NewReader(`[]`))
		if err != nil {
			t.Fatalf("processManifest returned error: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("processManifest len = %d, want 0", len(got))
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		if _, err := processManifest(strings.NewReader(`{`)); err == nil {
			t.Fatal("processManifest returned nil error for malformed JSON")
		}
	})
}

func TestWriteFileToDiskWritesFileAndEmitsProgress(t *testing.T) {
	dir := t.TempDir()
	state := &serverState{}
	stats := newTransferStatsTracker()
	history := NewTransferHistory(10)
	var mu sync.Mutex
	var events []string
	emit := func(name, data string) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, name+"|"+data)
	}

	writeFileToDisk(writeJob{
		dstPath:   filepath.Join(dir, "hello.txt"),
		savedName: "hello.txt",
		totalSize: 11,
		buf:       []byte("hello world"),
	}, state, stats, emit, history)

	got, err := os.ReadFile(filepath.Join(dir, "hello.txt"))
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(got) != "hello world" {
		t.Fatalf("written file = %q", got)
	}
	if state.uploadingCount != 0 {
		t.Fatalf("uploadingCount = %d, want 0", state.uploadingCount)
	}
	if !eventSeen(&mu, &events, "upload_progress|hello.txt|11|11") {
		t.Fatalf("upload_progress event missing: %#v", events)
	}
	if snapshot := stats.snapshot(0); snapshot.FilesReceived != 1 || snapshot.BytesReceived != 11 {
		t.Fatalf("stats snapshot = %#v", snapshot)
	}
	if records := history.List(); len(records) != 1 || records[0].Status != TransferStatusCompleted {
		t.Fatalf("history records = %#v", records)
	}
}

func TestStartWriteWorkersProcessesJobsAndIgnoresCreateErrors(t *testing.T) {
	dir := t.TempDir()
	state := &serverState{}
	stats := newTransferStatsTracker()
	history := NewTransferHistory(10)
	jobs := make(chan writeJob, 2)
	wg := startWriteWorkers(jobs, state, stats, func(string, string) {}, history)

	jobs <- writeJob{dstPath: filepath.Join(dir, "one.txt"), savedName: "one.txt", totalSize: 3, buf: []byte("one")}
	jobs <- writeJob{dstPath: filepath.Join(dir, "missing", "bad.txt"), savedName: "bad.txt", totalSize: 3, buf: []byte("bad")}
	close(jobs)
	wg.Wait()

	if got, err := os.ReadFile(filepath.Join(dir, "one.txt")); err != nil || string(got) != "one" {
		t.Fatalf("worker did not write good job: data=%q err=%v", got, err)
	}
	if state.uploadingCount != 0 {
		t.Fatalf("uploadingCount = %d, want 0", state.uploadingCount)
	}
	if records := history.List(); len(records) != 2 {
		t.Fatalf("history len = %d, want 2: %#v", len(records), records)
	}
}

func TestStartServerLifecycleRootAndHeartbeat(t *testing.T) {
	server, baseURL, token, events, _ := startServerForTest(t)

	resp, err := http.Get(baseURL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", resp.Header.Get("Content-Type"))
	}
	if !strings.Contains(string(body), token) {
		t.Fatal("root page did not include session token")
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/heartbeat?token="+token, nil)
	if err != nil {
		t.Fatalf("create heartbeat request: %v", err)
	}
	heartbeatResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /heartbeat: %v", err)
	}
	heartbeatResp.Body.Close()
	if heartbeatResp.StatusCode != http.StatusOK {
		t.Fatalf("heartbeat status = %d, want %d", heartbeatResp.StatusCode, http.StatusOK)
	}

	if !waitForEvent(events, "device_connected", time.Second) {
		t.Fatal("device_connected event was not emitted")
	}
	if err := server.Shutdown(); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestUploadWithoutFileReturnsBadRequest(t *testing.T) {
	server, baseURL, token, _, _ := startServerForTest(t)
	defer server.Shutdown()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	resp, err := http.Post(baseURL+"/upload?token="+token, writer.FormDataContentType(), &body)
	if err != nil {
		t.Fatalf("POST /upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("upload status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestUploadWithFileSavesToDiskAndEmitsEvents(t *testing.T) {
	server, baseURL, token, events, uploadDir := startServerForTest(t)
	defer server.Shutdown()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	manifest, err := writer.CreateFormField("beam_manifest")
	if err != nil {
		t.Fatalf("create manifest field: %v", err)
	}
	if _, err := manifest.Write([]byte(`[{"name":"note.txt","size":11}]`)); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	file, err := writer.CreateFormFile("files", "note.txt")
	if err != nil {
		t.Fatalf("create file field: %v", err)
	}
	if _, err := file.Write([]byte("hello world")); err != nil {
		t.Fatalf("write file field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	resp, err := http.Post(baseURL+"/upload?token="+token, writer.FormDataContentType(), &body)
	if err != nil {
		t.Fatalf("POST /upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload status = %d, want %d: %s", resp.StatusCode, http.StatusOK, responseBody)
	}

	got, err := os.ReadFile(filepath.Join(uploadDir, "note.txt"))
	if err != nil {
		t.Fatalf("read uploaded file: %v", err)
	}
	if string(got) != "hello world" {
		t.Fatalf("uploaded file = %q", got)
	}
	if !waitForEvent(events, "file_received", time.Second) {
		t.Fatal("file_received event was not emitted")
	}

	statsResp, err := http.Get(baseURL + "/stats?token=" + token)
	if err != nil {
		t.Fatalf("GET /stats: %v", err)
	}
	defer statsResp.Body.Close()
	if statsResp.StatusCode != http.StatusOK {
		t.Fatalf("stats status = %d, want %d", statsResp.StatusCode, http.StatusOK)
	}
	var stats TransferStats
	if err := json.NewDecoder(statsResp.Body).Decode(&stats); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	if stats.FilesReceived != 1 || stats.BytesReceived != 11 {
		t.Fatalf("stats = %#v, want one 11-byte received file", stats)
	}
}

func eventSeen(mu *sync.Mutex, events *[]string, want string) bool {
	mu.Lock()
	defer mu.Unlock()
	for _, event := range *events {
		if event == want {
			return true
		}
	}
	return false
}

func waitForEvent(events <-chan string, want string, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case event := <-events:
			if strings.HasPrefix(event, want+"|") {
				return true
			}
		case <-timer.C:
			return false
		}
	}
}

func startServerForTest(t *testing.T) (*HTTPServer, string, string, <-chan string, string) {
	t.Helper()
	uploadDir := t.TempDir()
	startPort := freePort(t)
	events := make(chan string, 20)
	server, port, token := StartServer(uploadDir, startPort, DefaultTransferSettings(), func(eventName, data string) {
		events <- eventName + "|" + data
	})
	if server == nil {
		t.Fatal("StartServer returned nil server")
	}
	if token == "" {
		t.Fatal("StartServer returned empty token")
	}
	if port != fmt.Sprint(startPort) {
		t.Fatalf("StartServer port = %q, want %d", port, startPort)
	}
	return server, "http://127.0.0.1:" + port, token, events, uploadDir
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
