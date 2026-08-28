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
	"strings"
	"testing"
	"time"
)

func TestTokenMiddlewareRejectsMissingOrInvalidToken(t *testing.T) {
	store := newTokenStoreForTest(t)
	handler := tokenMiddleware(store, tokenScopeSession, false, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	for _, target := range []string{"/upload", "/upload?token=wrong"} {
		req := httptest.NewRequest(http.MethodPost, target, nil)
		rec := httptest.NewRecorder()

		handler(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s returned %d, want %d", target, rec.Code, http.StatusForbidden)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf("%s set Access-Control-Allow-Origin = %q, want empty", target, got)
		}
	}
}

func TestTokenMiddlewareAllowsValidTokenAndOptions(t *testing.T) {
	store := newTokenStoreForTest(t)
	token, err := store.issue(tokenScopeSession, 0, "")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	handler := tokenMiddleware(store, tokenScopeSession, false, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	req := httptest.NewRequest(http.MethodPost, "/upload?token="+token, nil)
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
	if err := os.WriteFile(filepath.Join(dir, "photo.png"), []byte("first"), 0600); err != nil { //nolint:gosec
		t.Fatalf("write existing file: %v", err)
	}

	got := autoRenamePath(dir, "photo.png")
	want := filepath.Join(dir, "photo(1).png")

	if got != want {
		t.Fatalf("autoRenamePath = %q, want %q", got, want)
	}
}

func TestGenerateIDReturnsHexValue(t *testing.T) {
	token := generateID()
	if len(token) != 32 {
		t.Fatalf("token length = %d, want 32", len(token))
	}
	for _, ch := range token {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			t.Fatalf("token contains non-hex character %q", ch)
		}
	}
}

func TestGenerateIDDoesNotCollideInSmallSample(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token := generateID()
		if seen[token] {
			t.Fatalf("generateID collision at value %q", token)
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

func TestHTTPServerShutdownWaitsForActiveRequest(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})

	httpSrv := &http.Server{
		ReadHeaderTimeout: 5 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(started)
			<-release
			w.WriteHeader(http.StatusNoContent)
		}),
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	serveDone := make(chan error, 1)
	go func() {
		if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			serveDone <- err
			return
		}
		serveDone <- nil
	}()

	clientDone := make(chan error, 1)
	go func() {
		resp, err := http.Get("http://" + ln.Addr().String())
		if err != nil {
			clientDone <- err
			return
		}
		defer resp.Body.Close()
		if _, err := io.Copy(io.Discard, resp.Body); err != nil {
			clientDone <- err
			return
		}
		if resp.StatusCode != http.StatusNoContent {
			clientDone <- fmt.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
			return
		}
		clientDone <- nil
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("handler did not receive request")
	}

	server := &HTTPServer{server: httpSrv}
	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- server.Shutdown()
	}()

	select {
	case err := <-shutdownDone:
		t.Fatalf("Shutdown returned before active request finished: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)

	select {
	case err := <-clientDone:
		if err != nil {
			t.Fatalf("client request: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("client request did not complete")
	}

	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Shutdown did not complete after request finished")
	}

	select {
	case err := <-serveDone:
		if err != nil {
			t.Fatalf("serve: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve did not exit after shutdown")
	}
}

func TestServerStateActiveUploadPreventsTimeout(t *testing.T) {
	state := &serverState{
		lastHeartbeat: time.Now().Add(-30 * time.Second),
		isConnected:   true,
	}

	if active := state.beginUpload(); active != 1 {
		t.Fatalf("beginUpload active count = %d, want 1", active)
	}
	wasConnected, timedOut := state.checkTimeout()
	if !wasConnected || timedOut {
		t.Fatalf("checkTimeout while upload active = (%v, %v), want connected without timeout", wasConnected, timedOut)
	}
	if active := state.endUpload(); active != 0 {
		t.Fatalf("endUpload active count = %d, want 0", active)
	}
}

func TestStartServerLifecycleRootAndHeartbeat(t *testing.T) {
	server, baseURL, bootstrapToken, events, _ := startRawServerForTest(t)

	resp, err := http.Get(baseURL + "/?token=" + bootstrapToken)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", resp.Header.Get("Content-Type"))
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("root Cache-Control = %q, want no-store", got)
	}
	token := extractInjectedToken(t, string(bodyBytes(resp)))

	replayResp, err := http.Get(baseURL + "/?token=" + bootstrapToken)
	if err != nil {
		t.Fatalf("replay bootstrap token: %v", err)
	}
	replayResp.Body.Close()
	if replayResp.StatusCode != http.StatusOK {
		t.Fatalf("replayed bootstrap status = %d, want %d; bootstrap token is no longer consumed on page load for Android app compat", replayResp.StatusCode, http.StatusOK)
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
	if heartbeatResp.Header.Get("X-BeamSync-Token") == "" {
		t.Fatal("heartbeat did not return a renewed session token")
	}

	if !waitForEvent(events, "device_connected", time.Second) {
		t.Fatal("device_connected event was not emitted")
	}
	if err := server.Shutdown(); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestStartSenderIssuesClientBoundSingleUseDownloadToken(t *testing.T) {
	t.Setenv(tlsEnvVar, "")
	filePath := filepath.Join(t.TempDir(), "secure.txt")
	if err := os.WriteFile(filePath, []byte("secure payload"), 0600); err != nil {
		t.Fatalf("write sender fixture: %v", err)
	}

	server, port, bootstrapToken := StartSender([]string{filePath}, nil)
	if server == nil {
		t.Fatal("StartSender returned nil server")
	}
	defer func() { _ = server.Shutdown() }() //nolint:errcheck
	baseURL := "http://127.0.0.1:" + port

	rootResp, err := http.Get(baseURL + "/?token=" + bootstrapToken)
	if err != nil {
		t.Fatalf("GET sender page: %v", err)
	}
	rootBody, err := io.ReadAll(rootResp.Body)
	rootResp.Body.Close()
	if err != nil || rootResp.StatusCode != http.StatusOK {
		t.Fatalf("sender page status=%d err=%v", rootResp.StatusCode, err)
	}

	replayRootResp, err := http.Get(baseURL + "/?token=" + bootstrapToken)
	if err != nil {
		t.Fatalf("replay sender bootstrap: %v", err)
	}
	replayRootResp.Body.Close()
	if replayRootResp.StatusCode != http.StatusOK {
		t.Fatalf("replayed sender bootstrap status=%d, want %d; bootstrap token is no longer consumed on page load for Android app compat", replayRootResp.StatusCode, http.StatusOK)
	}

	const linkPrefix = "/download?token="
	start := strings.Index(string(rootBody), linkPrefix)
	if start == -1 {
		t.Fatal("sender page did not include a secure download link")
	}
	start += len(linkPrefix)
	end := strings.IndexAny(string(rootBody[start:]), "'\"")
	if end == -1 {
		t.Fatal("secure download token was not terminated")
	}
	downloadURL := baseURL + linkPrefix + string(rootBody[start:start+end])

	firstResp, err := http.Get(downloadURL) //nolint:gosec,noctx
	if err != nil {
		t.Fatalf("first secure download: %v", err)
	}
	firstBody, _ := io.ReadAll(firstResp.Body)
	firstResp.Body.Close()
	if firstResp.StatusCode != http.StatusOK || string(firstBody) != "secure payload" {
		t.Fatalf("first download status=%d body=%q", firstResp.StatusCode, firstBody)
	}

	replayResp, err := http.Get(downloadURL) //nolint:gosec,noctx
	if err != nil {
		t.Fatalf("replayed secure download: %v", err)
	}
	replayResp.Body.Close()
	if replayResp.StatusCode != http.StatusForbidden {
		t.Fatalf("replayed download status=%d, want %d", replayResp.StatusCode, http.StatusForbidden)
	}
}

func TestUploadWithoutFileReturnsBadRequest(t *testing.T) {
	server, baseURL, token, _, _ := startServerForTest(t)
	defer func() { _ = server.Shutdown() }() //nolint:errcheck

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
	defer func() { _ = server.Shutdown() }() //nolint:errcheck

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
	server, baseURL, bootstrapToken, events, uploadDir := startRawServerForTest(t)
	resp, err := http.Get(baseURL + "/?token=" + bootstrapToken)
	if err != nil {
		_ = server.Shutdown() //nolint:errcheck
		t.Fatalf("exchange bootstrap token: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_ = server.Shutdown() //nolint:errcheck
		t.Fatalf("bootstrap exchange status=%d", resp.StatusCode)
	}
	token := extractInjectedToken(t, string(bodyBytes(resp)))
	return server, baseURL, token, events, uploadDir
}

func startRawServerForTest(t *testing.T) (*HTTPServer, string, string, <-chan string, string) {
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

func extractInjectedToken(t *testing.T, html string) string {
	t.Helper()
	const prefix = `let TOKEN = "`
	start := strings.Index(html, prefix)
	if start == -1 {
		t.Fatal("page did not include an injected session token")
	}
	start += len(prefix)
	end := strings.Index(html[start:], `"`)
	if end == -1 {
		t.Fatal("injected session token was not terminated")
	}
	return html[start : start+end]
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

func bodyBytes(resp *http.Response) []byte {
	b, _ := io.ReadAll(resp.Body)
	return b
}
