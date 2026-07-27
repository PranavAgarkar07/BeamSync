package beamsync

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type EventCallback func(eventName string, data string)

const eventDispatcherBufferSize = 1024

type eventDispatchJob struct {
	emit  EventCallback
	event string
	data  string
}

type eventDispatcher struct {
	queue   chan eventDispatchJob
	dropped atomic.Int64
}

func newEventDispatcher(bufferSize int) *eventDispatcher {
	if bufferSize <= 0 {
		bufferSize = eventDispatcherBufferSize
	}
	dispatcher := &eventDispatcher{
		queue: make(chan eventDispatchJob, bufferSize),
	}
	go dispatcher.run()
	return dispatcher
}

func (d *eventDispatcher) run() {
	for job := range d.queue {
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Event callback panic: %v\n", r)
				}
			}()
			if job.emit != nil {
				job.emit(job.event, job.data)
			}
		}()
	}
}

func (d *eventDispatcher) emit(job eventDispatchJob) bool {
	if job.emit == nil {
		return true
	}
	select {
	case d.queue <- job:
		return true
	default:
		d.dropped.Add(1)
		return false
	}
}

func (d *eventDispatcher) DroppedCount() int64 {
	return d.dropped.Load()
}

var defaultEventDispatcher = newEventDispatcher(eventDispatcherBufferSize)

//go:embed ui/*.html ui/*.png
var uiFS embed.FS

var chunkBufferPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 8*1024*1024)
		return &buf
	},
}

type serverState struct {
	mu             sync.Mutex
	lastHeartbeat  time.Time
	isConnected    bool
	uploadingCount int
}

func (s *serverState) markHeartbeat() (wasConnected bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastHeartbeat = time.Now()
	wasConnected = s.isConnected
	s.isConnected = true
	return
}

func (s *serverState) beginUpload() int32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.uploadingCount++
	if s.uploadingCount == 1 {
		s.lastHeartbeat = time.Now()
	}
	return int32(s.uploadingCount)
}

func (s *serverState) endUpload() int32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.uploadingCount > 0 {
		s.uploadingCount--
	}
	return int32(s.uploadingCount)
}

func (s *serverState) activeUploads() int32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return int32(s.uploadingCount)
}

func (s *serverState) checkTimeout() (wasConnected bool, timedOut bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isConnected && s.uploadingCount == 0 && time.Since(s.lastHeartbeat) > 15*time.Second {
		s.isConnected = false
		return true, true
	}
	return s.isConnected, false
}

type PendingTransfer struct {
	ID         string `json:"id"`
	Filename   string `json:"filename"`
	SizeMB     string `json:"sizeMB"`
	SizeBytes  int64  `json:"sizeBytes"`
	MimeType   string `json:"mimeType"`
	SenderIP   string `json:"senderIP"`
	SenderName string `json:"senderName"`
	approved   chan bool
}

type HTTPServer struct {
	server           *http.Server
	cancel           context.CancelFunc
	pendingMu        sync.Mutex
	pendingTransfers map[string]*PendingTransfer
	settings         *TransferSettings
	history          *TransferHistory
	stats            *transferStatsTracker
	tokens           *tokenStore
}

func (s *HTTPServer) RespondToTransfer(id string, approved bool) {
	s.pendingMu.Lock()
	pt, ok := s.pendingTransfers[id]
	if ok {
		delete(s.pendingTransfers, id)
	}
	s.pendingMu.Unlock()
	if ok {
		pt.approved <- approved
	}
}

func (s *HTTPServer) Settings() *TransferSettings {
	return s.settings
}

func (s *HTTPServer) TransferHistory() []TransferRecord {
	if s == nil || s.history == nil {
		return nil
	}
	return s.history.List()
}

func (s *HTTPServer) Stats() TransferStats {
	if s.stats == nil {
		return TransferStats{}
	}
	return s.stats.snapshot(0)
}

func (s *HTTPServer) Shutdown() error {
	if s.cancel != nil {
		s.cancel()
	}
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.server.Shutdown(ctx); err != nil {
			if closeErr := s.server.Close(); closeErr != nil {
				return fmt.Errorf("graceful shutdown failed: %w; forced close failed: %v", err, closeErr)
			}
			return err
		}
	}
	return nil
}

type progressWriter struct {
	w         io.Writer
	total     int64
	written   int64
	filename  string
	emit      func(string, string)
	lastEmit  time.Time
	minInterval time.Duration
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.w.Write(p)
	pw.written += int64(n)
	now := time.Now()
	if now.Sub(pw.lastEmit) >= pw.minInterval {
		pw.emit("upload_progress", fmt.Sprintf("%s|%d|%d", pw.filename, pw.written, pw.total))
		pw.lastEmit = now
	}
	return n, err
}

func copyChunked(dst io.Writer, src io.Reader, chunkSize int) (int64, error) {
	var buf []byte
	if chunkSize == 8*1024*1024 {
		bufPtr := chunkBufferPool.Get().(*[]byte)
		defer chunkBufferPool.Put(bufPtr)
		buf = *bufPtr
	} else {
		buf = make([]byte, chunkSize)
	}
	var total int64
	for {
		n, err := io.ReadFull(src, buf)
		if n > 0 {
			nw, werr := dst.Write(buf[:n])
			total += int64(nw)
			if werr != nil {
				return total, werr
			}
		}
		if err == io.ErrUnexpectedEOF || err == io.EOF {
			return total, nil
		}
		if err != nil {
			return total, err
		}
	}
}

const uploadIntegrityHeader = "X-BeamSync-File-SHA256"

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func tokenMiddleware(tokens *tokenStore, scope tokenScope, consume bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if err := tokens.validate(r.URL.Query().Get("token"), "", scope, consume); err != nil {
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, "403 Forbidden: invalid token", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func tokenMiddlewareAny(tokens *tokenStore, scopes []tokenScope, consume bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if err := tokens.validateAny(r.URL.Query().Get("token"), scopes, consume); err != nil {
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, "403 Forbidden: invalid token", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

func autoRenamePath(dir, filename string) string {
	dst := filepath.Join(dir, filename)
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		return dst
	}
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	for i := 1; i < 1000; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s(%d)%s", base, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
	return filepath.Join(dir, fmt.Sprintf("%s_%d%s", base, time.Now().UnixNano(), ext))
}

func startWatchdog(ctx context.Context, state *serverState, emit func(string, string)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Watchdog panic: %v\n", r)
			}
		}()
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, timedOut := state.checkTimeout()
				if timedOut {
					emit("device_disconnected", "")
				}
			}
		}
	}()
}

func safeEmit(emit EventCallback, event, data string) {
	if emit == nil {
		return
	}
	if !defaultEventDispatcher.emit(eventDispatchJob{emit: emit, event: event, data: data}) {
		fmt.Printf("Dropping event because dispatch queue is full: %s\n", event)
	}
}

func logTransfer(history *TransferHistory, emit func(string, string), record TransferRecord) {
	if history == nil {
		return
	}
	saved := history.Add(record)
	data, err := json.Marshal(saved)
	if err != nil {
		fmt.Println("Failed to marshal transfer history record:", err)
		return
	}
	emit("transfer_logged", string(data))
}

func StartServer(uploadDir string, startPort int, settings TransferSettings, callback EventCallback) (*HTTPServer, string, string) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("PANIC IN StartServer: %v\n%s\n", r, debug.Stack())
		}
	}()

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, "", ""
	}

	fingerprint, err := serverTLSFingerprint()
	if err != nil {
		return nil, "", ""
	}
	tokens, err := newTokenStore(fingerprint)
	if err != nil {
		return nil, "", ""
	}
	token, err := tokens.issue(tokenScopeBootstrap, 1, "")
	if err != nil {
		return nil, "", ""
	}
	emit := func(evt, data string) { safeEmit(callback, evt, data) }

	state := &serverState{}
	ctx, cancel := context.WithCancel(context.Background())
	tokens.startCleanup(ctx)
	startWatchdog(ctx, state, emit)

	settingsCopy := settings
	httpServer := &HTTPServer{
		cancel:           cancel,
		pendingTransfers: make(map[string]*PendingTransfer),
		settings:         &settingsCopy,
		history:          NewTransferHistory(defaultTransferHistoryLimit),
		stats:            newTransferStatsTracker(),
		tokens:           tokens,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/heartbeat", tokenMiddlewareAny(tokens, []tokenScope{tokenScopeSession, tokenScopeBootstrap}, false, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		wasConnected := state.markHeartbeat()
		if !wasConnected {
			emit("device_connected", "Android Device")
		}
		refreshedToken, err := tokens.issue(tokenScopeSession, 0, "")
		if err != nil {
			http.Error(w, "Failed to renew session", http.StatusInternalServerError)
			return
		}
		w.Header().Set("X-BeamSync-Token", refreshedToken)
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
	}))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if err := tokens.validate(r.URL.Query().Get("token"), "", tokenScopeBootstrap, true); err != nil {
			http.Error(w, "403 Forbidden: reconnect by scanning the current QR code", http.StatusForbidden)
			return
		}
		setCORSHeaders(w)
		content, err := uiFS.ReadFile("ui/upload.html")
		if err != nil {
			http.Error(w, "UI Load Error", http.StatusInternalServerError)
			return
		}
		sessionToken, err := tokens.issue(tokenScopeSession, 0, "")
		if err != nil {
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			return
		}
		html := strings.Replace(string(content), "{{TOKEN}}", sessionToken, 1)
		w.Header().Set("X-BeamSync-Token", sessionToken)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))

		wasConnected := state.markHeartbeat()
		if !wasConnected {
			emit("device_connected", "Android Device")
		}
	})

	mux.HandleFunc("/logo.png", func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		w.Header().Set("Content-Type", "image/png")
		content, err := uiFS.ReadFile("ui/logo.png")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Write(content)
	})

	mux.HandleFunc("/stats", tokenMiddlewareAny(tokens, []tokenScope{tokenScopeSession, tokenScopeBootstrap}, false, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Content-Type", "application/json")
		snapshot := httpServer.stats.snapshot(state.activeUploads())
		w.Write([]byte(transferStatsJSON(snapshot)))
	}))

	mux.HandleFunc("/request-transfer", tokenMiddlewareAny(tokens, []tokenScope{tokenScopeSession, tokenScopeBootstrap}, false, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		senderIP := clientIP(r)
		var req struct {
			Filename  string `json:"filename"`
			SizeBytes int64  `json:"sizeBytes"`
			MimeType  string `json:"mimeType"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		s := httpServer.settings
		senderName := s.friendlyNameForIP(senderIP)

		if s.isDeviceBlocked(senderIP) {
			http.Error(w, "403 Forbidden: device is blocked", http.StatusForbidden)
			return
		}
		if s.Mode == TransferModeBlockAll {
			http.Error(w, "403 Forbidden: all transfers are blocked", http.StatusForbidden)
			return
		}
		if s.isExtensionBlocked(req.Filename) {
			http.Error(w, "403 Forbidden: file type is blocked", http.StatusForbidden)
			return
		}
		if s.MaxFileSizeMB > 0 && req.SizeBytes > s.MaxFileSizeMB*1024*1024 {
			http.Error(w, fmt.Sprintf("403 Forbidden: file exceeds max size of %d MB", s.MaxFileSizeMB), http.StatusForbidden)
			return
		}

		if s.Mode == TransferModeAcceptAll {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("approved"))
			return
		}

		if s.Mode == TransferModeTrustedOnly {
			if s.isDeviceTrusted(senderIP) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("approved"))
			} else {
				http.Error(w, "403 Forbidden: device is not trusted", http.StatusForbidden)
			}
			return
		}

		id := generateID()
		sizeMB := fmt.Sprintf("%.2f MB", float64(req.SizeBytes)/1024/1024)

		pt := &PendingTransfer{
			ID:         id,
			Filename:   req.Filename,
			SizeMB:     sizeMB,
			SizeBytes:  req.SizeBytes,
			MimeType:   req.MimeType,
			SenderIP:   senderIP,
			SenderName: senderName,
			approved:   make(chan bool, 1),
		}

		httpServer.pendingMu.Lock()
		httpServer.pendingTransfers[id] = pt
		httpServer.pendingMu.Unlock()

		evtData, _ := json.Marshal(pt)
		emit("transfer_request", string(evtData))

		select {
		case approved := <-pt.approved:
			if approved {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("approved"))
			} else {
				http.Error(w, "403 Forbidden: transfer rejected by user", http.StatusForbidden)
			}
		case <-time.After(60 * time.Second):
			httpServer.pendingMu.Lock()
			delete(httpServer.pendingTransfers, id)
			httpServer.pendingMu.Unlock()
			emit("transfer_request_timeout", id)
			http.Error(w, "408 Request Timeout: no response from user", http.StatusRequestTimeout)
		}
	}))

	mux.HandleFunc("/upload", tokenMiddlewareAny(tokens, []tokenScope{tokenScopeSession, tokenScopeBootstrap}, false, func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("PANIC in upload handler: %v\n%s\n", r, debug.Stack())
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Accept-Ranges", "bytes")
		state.markHeartbeat()
		r.Body = http.MaxBytesReader(w, r.Body, 100*1024*1024*1024)

		contentType := r.Header.Get("Content-Type")
		mediaType, params, ctErr := mime.ParseMediaType(contentType)
		if ctErr != nil || !strings.HasPrefix(mediaType, "multipart/") {
			http.Error(w, "Expected multipart/form-data", http.StatusBadRequest)
			return
		}
		boundary := params["boundary"]

		netReader := bufio.NewReaderSize(r.Body, 8*1024*1024)
		mr := multipart.NewReader(netReader, boundary)

		fileSizes := make(map[string]int64)
		fileCount := 0
		var parseErr error

		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				parseErr = err
				break
			}

			formName := part.FormName()
			filename := part.FileName()

			if formName == "beam_manifest" && filename == "" {
				var manifest []struct {
					Name string `json:"name"`
					Size int64  `json:"size"`
				}
				if err := json.NewDecoder(part).Decode(&manifest); err == nil {
					for _, f := range manifest {
						fileSizes[f.Name] = f.Size
					}
				}
				part.Close()
				continue
			}

			if filename == "" {
				part.Close()
				continue
			}

			fileCount++
			rawName := filepath.Base(filename)
			if rawName == "" || rawName == "." {
				rawName = fmt.Sprintf("upload_%d.bin", time.Now().Unix())
			}

			dstPath := autoRenamePath(uploadDir, rawName)
			savedName := filepath.Base(dstPath)

			dst, createErr := os.Create(dstPath)
			if createErr != nil {
				io.Copy(io.Discard, part)
				part.Close()
				continue
			}

			diskBuf := bufio.NewWriterSize(dst, 8*1024*1024)
			var pw *progressWriter
			pw = &progressWriter{
				w:           diskBuf,
				filename:    savedName,
				emit:        emit,
				minInterval: 500 * time.Millisecond,
			}

			startedAt := time.Now()
			state.beginUpload()

			// ponytail: no integrity hashing on upload path - adds complexity for LAN transfer where corruption is near-zero. Add if transfer integrity issues reported.
			written, copyErr := copyChunked(pw, part, 8*1024*1024)

			flushErr := diskBuf.Flush()
			dst.Close()
			part.Close()
			state.endUpload()

			var transferErr error
			switch {
			case copyErr != nil:
				transferErr = copyErr
			case flushErr != nil:
				transferErr = flushErr
			}

			if transferErr != nil {
				logTransfer(httpServer.history, emit, TransferRecord{
					Filename:  savedName,
					Direction: TransferDirectionReceive,
					Status:    TransferStatusFailed,
					SizeBytes: written,
					StartedAt: startedAt,
					Error:     transferErr.Error(),
				})
				continue
			}

			emit("upload_progress", fmt.Sprintf("%s|%d|%d", savedName, written, written))
			snapshot := httpServer.stats.recordReceived(savedName, written, state.activeUploads())
			emit("transfer_stats", transferStatsJSON(snapshot))

			logTransfer(httpServer.history, emit, TransferRecord{
				Filename:  savedName,
				Direction: TransferDirectionReceive,
				Status:    TransferStatusCompleted,
				SizeBytes: written,
				StartedAt: startedAt,
			})

			go func(fname string) {
				time.Sleep(100 * time.Millisecond)
				emit("file_received", fname)
			}(savedName)
		}

		if parseErr != nil {
			http.Error(w, "Multipart read error", http.StatusBadRequest)
			return
		}

		if fileCount == 0 {
			http.Error(w, "No files uploaded", http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Upload Complete"))
	}))

	portInt, listener, err := FindAvailablePort(startPort, 2, 50)
	if err != nil {
		if strings.Contains(err.Error(), "permission") || strings.Contains(err.Error(), "access") {
			if fwErr := RunFirewallSetup(); fwErr == nil {
				portInt, listener, err = FindAvailablePort(startPort, 2, 50)
				if err != nil {
					cancel()
					return nil, "", ""
				}
			} else {
				cancel()
				return nil, "", ""
			}
		} else {
			cancel()
			return nil, "", ""
		}
	}
	portStr := fmt.Sprintf("%d", portInt)

	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  4 * time.Hour,
		WriteTimeout: 4 * time.Hour,
		IdleTimeout:  60 * time.Second,
	}
	httpServer.server = srv
	serveListener, _, tlsErr := maybeTLSListener(listener)
	if tlsErr != nil {
		cancel()
		listener.Close()
		return nil, "", ""
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Server panic: %v\n", r)
			}
		}()
		if err := srv.Serve(serveListener); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Server error: %v\n", err)
		}
	}()

	return httpServer, portStr, token
}

func StartSender(filePaths []string, callback EventCallback) (*HTTPServer, string, string) {
	fingerprint, err := serverTLSFingerprint()
	if err != nil {
		return nil, "", ""
	}
	tokens, err := newTokenStore(fingerprint)
	if err != nil {
		return nil, "", ""
	}
	token, err := tokens.issue(tokenScopeBootstrap, 1, "")
	if err != nil {
		return nil, "", ""
	}
	emit := func(evt, data string) { safeEmit(callback, evt, data) }

	state := &serverState{}
	ctx, cancel := context.WithCancel(context.Background())
	tokens.startCleanup(ctx)
	httpServer := &HTTPServer{
		cancel:  cancel,
		history: NewTransferHistory(defaultTransferHistoryLimit),
		stats:   newTransferStatsTracker(),
		tokens:  tokens,
	}
	startWatchdog(ctx, state, emit)

	mux := http.NewServeMux()

	mux.HandleFunc("/heartbeat", tokenMiddleware(tokens, tokenScopeSession, false, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		wasConnected := state.markHeartbeat()
		if !wasConnected {
			emit("device_connected", "Mobile (Downloader)")
		}
		refreshedToken, err := tokens.issue(tokenScopeSession, 0, "")
		if err != nil {
			http.Error(w, "Failed to renew session", http.StatusInternalServerError)
			return
		}
		w.Header().Set("X-BeamSync-Token", refreshedToken)
		w.WriteHeader(http.StatusOK)
	}))

	buildFileBlock := func(filePaths []string) (string, error) {
		fmtSize := func(sz int64) string {
			switch {
			case sz >= 1073741824:
				return fmt.Sprintf("%.2f GB", float64(sz)/1073741824)
			case sz >= 1048576:
				return fmt.Sprintf("%.2f MB", float64(sz)/1048576)
			case sz >= 1024:
				return fmt.Sprintf("%.1f KB", float64(sz)/1024)
			default:
				return fmt.Sprintf("%d B", sz)
			}
		}
		dlIcon := `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="square"><path d="M21 15v4H3v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="3" x2="12" y2="15"/></svg>`
		card := func(name, sizeStr, cardID, downloadURL string) string {
			return fmt.Sprintf(`<div class="file-card" id="card-%s">
				<div class="file-card__row">
					<div class="file-card__info">
						<div class="file-card__name" id="name-%s">%s</div>
						<div class="file-card__size">%s</div>
					</div>
					<div class="file-card__actions">
						<button class="download-btn" id="btn-%s" onclick="event.preventDefault(); startDownload('%s', '%s'); return false;">%s DOWNLOAD</button>
						<span class="file-card__chip" id="chip-%s"></span>
					</div>
				</div>
				<div class="file-card__progress-track" id="track-%s">
					<div class="file-card__progress-fill" id="fill-%s"></div>
				</div>
			</div>`, cardID, cardID, name, sizeStr, cardID, downloadURL, cardID, dlIcon, cardID, cardID, cardID)
		}

		var b strings.Builder
		if len(filePaths) == 1 {
			name := filepath.Base(filePaths[0])
			sizeStr := ""
			if info, err := os.Stat(filePaths[0]); err == nil {
				sizeStr = fmtSize(info.Size())
			}
			transferToken, err := tokens.issue(tokenScopeTransfer, 1, "")
			if err != nil {
				return "", err
			}
			b.WriteString(card(name, sizeStr, "single", fmt.Sprintf("/download?token=%s", transferToken)))
		} else {
			for i, path := range filePaths {
				name := filepath.Base(path)
				sizeStr := ""
				if info, err := os.Stat(path); err == nil {
					sizeStr = fmtSize(info.Size())
				}
				cardID := fmt.Sprintf("multi-%d", i)
				transferToken, err := tokens.issue(tokenScopeTransfer, 1, "")
				if err != nil {
					return "", err
				}
				downloadURL := fmt.Sprintf("/download/%d?token=%s", i, transferToken)
				b.WriteString(card(name, sizeStr, cardID, downloadURL))
			}
		}
		return b.String(), nil
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if err := tokens.validate(r.URL.Query().Get("token"), "", tokenScopeBootstrap, true); err != nil {
			http.Error(w, "403 Forbidden: reconnect by scanning the current QR code", http.StatusForbidden)
			return
		}
		setCORSHeaders(w)
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Content-Type", "text/html")
		content, err := uiFS.ReadFile("ui/download.html")
		if err != nil {
			http.Error(w, "UI Load Error", http.StatusInternalServerError)
			return
		}
		fileBlock, err := buildFileBlock(filePaths)
		if err != nil {
			http.Error(w, "Failed to create transfer links", http.StatusInternalServerError)
			return
		}
		sessionToken, err := tokens.issue(tokenScopeSession, 0, "")
		if err != nil {
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			return
		}
		html := strings.Replace(string(content), "{{FILES}}", fileBlock, 1)
		html = strings.Replace(html, "{{TOKEN}}", sessionToken, 1)
		w.Header().Set("X-BeamSync-Token", sessionToken)
		w.Write([]byte(html))
	})

	mux.HandleFunc("/logo.png", func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		w.Header().Set("Content-Type", "image/png")
		content, err := uiFS.ReadFile("ui/logo.png")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Write(content)
	})

	if len(filePaths) == 1 {
		filePath := filePaths[0]
		filename := filepath.Base(filePath)
		mux.HandleFunc("/download", tokenMiddleware(tokens, tokenScopeTransfer, true, func(w http.ResponseWriter, r *http.Request) {
			activeDownloads := state.beginUpload()
			emit("transfer_stats", transferStatsJSON(httpServer.stats.snapshotDownloads(activeDownloads)))
			defer func() {
				activeDownloads := state.endUpload()
				emit("transfer_stats", transferStatsJSON(httpServer.stats.snapshotDownloads(activeDownloads)))
			}()
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
			w.Header().Set("X-Filename", filename)
			w.Header().Set("Access-Control-Expose-Headers", "X-Filename")

			file, err := os.Open(filePath)
			if err != nil {
				http.Error(w, "File not found", http.StatusNotFound)
				return
			}
			defer file.Close()

			fileInfo, err := file.Stat()
			if err != nil {
				http.Error(w, "Failed to stat file", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
			mimeType := mime.TypeByExtension(filepath.Ext(filename))
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}
			w.Header().Set("Content-Type", mimeType)

			startedAt := time.Now()
			bufReader := bufio.NewReaderSize(file, 8*1024*1024)
			pw := &progressWriter{
				w:           w,
				total:       fileInfo.Size(),
				filename:    filename,
				emit:        emit,
				lastEmit:    time.Now(),
				minInterval: 500 * time.Millisecond,
			}
			written, copyErr := copyChunked(pw, bufReader, 8*1024*1024)
			status := TransferStatusCompleted
			errMsg := ""
			if copyErr != nil {
				status = TransferStatusFailed
				errMsg = copyErr.Error()
			} else {
				snapshot := httpServer.stats.recordSent(filename, written, state.activeUploads())
				emit("transfer_stats", transferStatsJSON(snapshot))
			}
			logTransfer(httpServer.history, emit, TransferRecord{
				Filename:  filename,
				Direction: TransferDirectionSend,
				Status:    status,
				SizeBytes: written,
				StartedAt: startedAt,
				Error:     errMsg,
			})
		}))
	} else {
		for i, path := range filePaths {
			idx := i
			filePath := path
			mux.HandleFunc(fmt.Sprintf("/download/%d", idx), tokenMiddleware(tokens, tokenScopeTransfer, true, func(w http.ResponseWriter, r *http.Request) {
				realName := filepath.Base(filePath)
				activeDownloads := state.beginUpload()
				emit("transfer_stats", transferStatsJSON(httpServer.stats.snapshotDownloads(activeDownloads)))
				defer func() {
					activeDownloads := state.endUpload()
					emit("transfer_stats", transferStatsJSON(httpServer.stats.snapshotDownloads(activeDownloads)))
				}()
				w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
				w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, realName))
				w.Header().Set("X-Filename", realName)
				w.Header().Set("Access-Control-Expose-Headers", "X-Filename")

				file, err := os.Open(filePath)
				if err != nil {
					http.Error(w, "File not found", http.StatusNotFound)
					return
				}
				defer file.Close()

				fileInfo, err := file.Stat()
				if err != nil {
					http.Error(w, "Failed to stat file", http.StatusInternalServerError)
					return
				}

				w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
				mimeType := mime.TypeByExtension(filepath.Ext(realName))
				if mimeType == "" {
					mimeType = "application/octet-stream"
				}
				w.Header().Set("Content-Type", mimeType)

				startedAt := time.Now()
				bufReader := bufio.NewReaderSize(file, 8*1024*1024)
				pw := &progressWriter{
					w:           w,
					total:       fileInfo.Size(),
					filename:    realName,
					emit:        emit,
					lastEmit:    time.Now(),
					minInterval: 500 * time.Millisecond,
				}
				written, copyErr := copyChunked(pw, bufReader, 8*1024*1024)
				status := TransferStatusCompleted
				errMsg := ""
				if copyErr != nil {
					status = TransferStatusFailed
					errMsg = copyErr.Error()
				} else {
					snapshot := httpServer.stats.recordSent(realName, written, state.activeUploads())
					emit("transfer_stats", transferStatsJSON(snapshot))
				}
				logTransfer(httpServer.history, emit, TransferRecord{
					Filename:  realName,
					Direction: TransferDirectionSend,
					Status:    status,
					SizeBytes: written,
					StartedAt: startedAt,
					Error:     errMsg,
				})
			}))
		}
	}

	portInt, listener, err := FindAvailablePort(3005, 2, 50)
	if err != nil {
		if strings.Contains(err.Error(), "permission") || strings.Contains(err.Error(), "access") {
			if fwErr := RunFirewallSetup(); fwErr == nil {
				portInt, listener, err = FindAvailablePort(3005, 2, 50)
				if err != nil {
					cancel()
					return nil, "", ""
				}
			} else {
				cancel()
				return nil, "", ""
			}
		} else {
			cancel()
			return nil, "", ""
		}
	}
	portStr := fmt.Sprintf("%d", portInt)

	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  4 * time.Hour,
		WriteTimeout: 4 * time.Hour,
		IdleTimeout:  60 * time.Second,
	}
	httpServer.server = srv
	serveListener, _, tlsErr := maybeTLSListener(listener)
	if tlsErr != nil {
		cancel()
		listener.Close()
		return nil, "", ""
	}

	go func() {
		if err := srv.Serve(serveListener); err != nil && err != http.ErrServerClosed {
			fmt.Println("Sender error:", err)
		}
	}()

	return httpServer, portStr, token
}
