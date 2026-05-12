package beamsync

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
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
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type EventCallback func(eventName string, data string)

//go:embed ui/*.html ui/*.png
var uiFS embed.FS

// serverState holds per-instance connection tracking (no more package-level globals).
type serverState struct {
	mu             sync.Mutex
	lastHeartbeat  time.Time
	isConnected    bool
	uploadingCount int32 // atomic: number of files currently being written
}

func (s *serverState) markHeartbeat() (wasConnected bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastHeartbeat = time.Now()
	wasConnected = s.isConnected
	s.isConnected = true
	return
}

// beginUpload increments the in-flight write counter.
// The watchdog will not fire device_disconnected while any write is in flight.
func (s *serverState) beginUpload() {
	if atomic.AddInt32(&s.uploadingCount, 1) == 1 {
		// First write starting — reset heartbeat so the 15s clock restarts when all finish
		s.mu.Lock()
		s.lastHeartbeat = time.Now()
		s.mu.Unlock()
	}
}

// endUpload decrements the in-flight write counter.
func (s *serverState) endUpload() {
	atomic.AddInt32(&s.uploadingCount, -1)
}

func (s *serverState) checkTimeout() (wasConnected bool, timedOut bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Never consider it a timeout while data is actively being received/written
	if s.isConnected && atomic.LoadInt32(&s.uploadingCount) == 0 && time.Since(s.lastHeartbeat) > 15*time.Second {
		s.isConnected = false
		return true, true
	}
	return s.isConnected, false
}

// PendingTransfer holds a blocked transfer awaiting user approval.
type PendingTransfer struct {
	ID             string `json:"id"`
	Filename       string `json:"filename"`
	SizeMB         string `json:"sizeMB"`
	SizeBytes      int64  `json:"sizeBytes"`
	MimeType       string `json:"mimeType"`
	SenderIP       string `json:"senderIP"`
	SenderName     string `json:"senderName"`
	AvailableBytes int64  `json:"availableBytes"`
	AvailableStr   string `json:"availableStr"`
	approved       chan bool
}

// HTTPServer wraps http.Server so we can shut it down cleanly.
type HTTPServer struct {
	server           *http.Server
	cancel           context.CancelFunc
	pendingMu        sync.Mutex
	pendingTransfers map[string]*PendingTransfer
	settings         *TransferSettings
}

// RespondToTransfer approves or rejects a pending transfer by ID.
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

// Settings returns a pointer to the live TransferSettings for in-place updates.
func (s *HTTPServer) Settings() *TransferSettings {
	return s.settings
}

func (s *HTTPServer) Shutdown() error {
	if s.cancel != nil {
		s.cancel()
	}
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

// progressWriter wraps an io.Writer and emits progress events.
// Uses an adaptive interval to avoid event flooding.
type progressWriter struct {
	mu          sync.Mutex
	w           io.Writer
	total       int64
	written     int64
	filename    string
	eventName   string
	emit        func(string, string)
	lastEmit    time.Time
	minInterval time.Duration
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.w.Write(p)

	pw.mu.Lock()
	pw.written += int64(n)
	now := time.Now()
	shouldEmit := now.Sub(pw.lastEmit) >= pw.minInterval
	if shouldEmit {
		pw.lastEmit = now
	}
	written := pw.written
	total := pw.total
	eventName := pw.eventName
	pw.mu.Unlock()

	if shouldEmit {
		data := fmt.Sprintf("%s|%d|%d", pw.filename, written, total)
		pw.emit(eventName, data)
	}
	return n, err
}

// copyBufferPool reduces heap allocations for the 8 MB copy buffer.
var copyBufferPool = sync.Pool{
	New: func() interface{} { return make([]byte, 8*1024*1024) },
}

// copyChunked reads src in large chunks before writing to dst.
// Go's multipart.Part has an internal 4 KB bufio, so Part.Read returns ≤4 KB
// per call regardless of the dst buffer size. Without this helper, we end up
// making thousands of tiny Write() syscalls per second which kills throughput.
// copyChunked accumulates those 4 KB reads into a single large Write(),
// giving the OS large sequential disk I/O instead of random small writes.
func copyChunked(dst io.Writer, src io.Reader) (int64, error) {
	buf := copyBufferPool.Get().([]byte)
	defer copyBufferPool.Put(buf)
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

// ── Concurrent write pipeline ─────────────────────────────────────────────────

// writeJob is a unit of work dispatched from the multipart-parsing goroutine
// to a disk-write worker. Only small files (≤64 MB) are dispatched this way;
// large files are written synchronously on the main goroutine.
type writeJob struct {
	dstPath   string
	savedName string
	totalSize int64
	buf       []byte // file data fully buffered in RAM
}

// writeWorkerCount is the number of goroutines writing files to disk in parallel.
const writeWorkerCount = 3

// largeFileThreshold is the maximum file size to buffer fully in RAM.
// Files larger than this are streamed directly to disk.
const largeFileThreshold = 64 * 1024 * 1024 // 64 MB

// startWriteWorkers launches writeWorkerCount goroutines that drain jobs and
// write files to disk. Returns a WaitGroup the caller can Wait() on.
func startWriteWorkers(
	jobs <-chan writeJob,
	state *serverState,
	emit func(string, string),
) *sync.WaitGroup {
	var wg sync.WaitGroup
	for i := 0; i < writeWorkerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				writeFileToDisk(job, state, emit)
			}
		}()
	}
	return &wg
}

// writeFileToDisk performs the actual file write for one job and emits events.
// Only small files (fully buffered in buf) are dispatched here.
func writeFileToDisk(job writeJob, state *serverState, emit func(string, string)) {
	state.beginUpload()
	defer state.endUpload()

	dst, err := os.Create(job.dstPath)
	if err != nil {
		Error("File creation error: %v", err)
		return
	}
	defer dst.Close()

	// 8 MB disk write buffer for large sequential I/O
	diskBuf := bufio.NewWriterSize(dst, 8*1024*1024)

	// Data is already in RAM — one large write into the buffered writer
	pw := &progressWriter{
		w:           diskBuf,
		total:       int64(len(job.buf)),
		filename:    job.savedName,
		eventName:   "upload_progress",
		emit:        emit,
		minInterval: 200 * time.Millisecond,
	}
	n, werr := pw.Write(job.buf)
	written := int64(n)
	if werr != nil {
		Error("Write error: %v", werr)
	}

	if flushErr := diskBuf.Flush(); flushErr != nil {
		Error("Disk flush error: %v", flushErr)
	}

	emit("upload_progress", fmt.Sprintf("%s|%d|%d", job.savedName, written, written))
	fmt.Printf("✅ File saved: %s (%d bytes)\n", job.savedName, written)

	go func(fname string) {
		time.Sleep(100 * time.Millisecond)
		emit("file_received", fname)
	}(job.savedName)
}

// generateToken creates a 16-byte (32 hex char) crypto-random session token.
func generateToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback: timestamp-based (unlikely but safe)
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// validateToken middleware — returns 403 if the Authorization header doesn't match.
// Exempt routes: "/" (serves UI page).
func tokenMiddleware(token string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		got := r.Header.Get("Authorization")
		if got == "" || got != "Bearer "+token {
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

// autoRenamePath returns a non-colliding file path by appending (1), (2), …
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
	// Absolute fallback: timestamp suffix
	return filepath.Join(dir, fmt.Sprintf("%s_%d%s", base, time.Now().UnixNano(), ext))
}

// startWatchdog launches the heartbeat watchdog goroutine.
func startWatchdog(ctx context.Context, state *serverState, emit func(string, string)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("⚠️ Watchdog panic: %v\n", r)
			}
		}()

		fmt.Println("👁️ Watchdog started")
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				fmt.Println("🛑 Watchdog stopped")
				return
			case <-ticker.C:
				_, timedOut := state.checkTimeout()
				if timedOut {
					emit("device_disconnected", "")
					fmt.Println("💔 Device Disconnected (Timeout)")
				}
			}
		}
	}()
}

// safeEmit dispatches an event in its own goroutine with panic recovery.
func safeEmit(emit EventCallback, event, data string) {
	go func(evt, dt string) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("⚠️ Event callback panic: %v\n", r)
			}
		}()
		fmt.Printf("📡 Emitting event: %s | data: %s\n", evt, dt)
		if emit != nil {
			emit(evt, dt)
			fmt.Printf("✅ Event emitted: %s\n", evt)
		}
	}(event, data)
}

// StartServer starts the file-receiver HTTP server.
// Returns (server handle, port string, session token).
func StartServer(uploadDir string, startPort int, settings TransferSettings, callback EventCallback) (*HTTPServer, string, string) {
	fmt.Println("🚀 StartServer() called")

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("🚨 PANIC IN StartServer: %v\n%s\n", r, debug.Stack())
		}
	}()

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		Error("Failed to create upload directory: %v", err)
		return nil, "", ""
	}
	fmt.Printf("📁 Upload directory: %s\n", uploadDir)

	token := generateToken()
	emit := func(evt, data string) { safeEmit(callback, evt, data) }

	state := &serverState{}
	ctx, cancel := context.WithCancel(context.Background())

	startWatchdog(ctx, state, emit)

	settingsCopy := settings
	httpServer := &HTTPServer{
		cancel:           cancel,
		pendingTransfers: make(map[string]*PendingTransfer),
		settings:         &settingsCopy,
	}

	mux := http.NewServeMux()

	// ── Heartbeat ────────────────────────────────────────────────────────────
	mux.HandleFunc("/heartbeat", tokenMiddleware(token, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		fmt.Println("💓 Heartbeat received")
		wasConnected := state.markHeartbeat()
		if !wasConnected {
			emit("device_connected", "Android Device")
			fmt.Println("💚 Device Connected!")
		}
		w.WriteHeader(http.StatusOK)
	}))

	// ── Serve UI (no token required — this IS the page that shows the token) ─
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if r.Method != http.MethodGet || r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		fmt.Println("🌐 GET / - Serving upload UI")
		content, err := uiFS.ReadFile("ui/upload.html")
		if err != nil {
			http.Error(w, "UI Load Error", http.StatusInternalServerError)
			return
		}
		// Inject token so the upload page knows it
		html := strings.Replace(string(content), "{{TOKEN}}", token, 1)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))

		// Emit device_connected immediately — the phone loading this page
		// is already proof of connection; no need to wait for first heartbeat.
		wasConnected := state.markHeartbeat()
		if !wasConnected {
			fmt.Println("💚 Device Connected (page load)!")
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

	// ── Request Transfer (ask before accepting) ──────────────────────────────
	mux.HandleFunc("/request-transfer", tokenMiddleware(token, func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract sender IP
		senderIP, _, _ := net.SplitHostPort(r.RemoteAddr)
		if senderIP == "" {
			senderIP = r.RemoteAddr
		}

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

		// ── Check blocked devices ─────────────────────────────────────────────
		if s.isDeviceBlocked(senderIP) {
			Warn("Blocked device tried to send: %s", senderIP)
			http.Error(w, "403 Forbidden: device is blocked", http.StatusForbidden)
			return
		}

		// ── Check block_all mode ──────────────────────────────────────────────
		if s.Mode == TransferModeBlockAll {
			http.Error(w, "403 Forbidden: all transfers are blocked", http.StatusForbidden)
			return
		}

		// ── Check file extension ──────────────────────────────────────────────
		if s.isExtensionBlocked(req.Filename) {
			Warn("Blocked file extension: %s", req.Filename)
			http.Error(w, "403 Forbidden: file type is blocked", http.StatusForbidden)
			return
		}

		// ── Check max file size ───────────────────────────────────────────────
		if s.MaxFileSizeMB > 0 && req.SizeBytes > s.MaxFileSizeMB*1024*1024 {
			Warn("File too large: %d bytes (max %d MB)", req.SizeBytes, s.MaxFileSizeMB)
			http.Error(w, fmt.Sprintf("403 Forbidden: file exceeds max size of %d MB", s.MaxFileSizeMB), http.StatusForbidden)
			return
		}

		// ── Check available disk space ──────────────────────────────────────
		if s.MinFreeSpaceMB > 0 {
			space, err := GetDiskFreeSpace(uploadDir)
			if err == nil {
				needed := req.SizeBytes + s.MinFreeSpaceMB*1024*1024
				if space.AvailableBytes < needed {
					msg := fmt.Sprintf("Insufficient disk space: need %s, have %s available",
						formatBytes(needed), space.AvailableStr)
					fmt.Printf("🚫 Disk space insufficient: need %s, have %s\n",
						formatBytes(needed), space.AvailableStr)
					http.Error(w, msg, http.StatusInsufficientStorage)
					return
				}
			} else {
				Warn("Could not check disk space: %v", err)
			}
		}

		// ── accept_all: approve immediately ───────────────────────────────────
		if s.Mode == TransferModeAcceptAll {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("approved"))
			return
		}

		// ── trusted_only: approve if trusted, else reject ─────────────────────
		if s.Mode == TransferModeTrustedOnly {
			if s.isDeviceTrusted(senderIP) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("approved"))
			} else {
				http.Error(w, "403 Forbidden: device is not trusted", http.StatusForbidden)
			}
			return
		}

		// ── ask_first: emit event and wait for user response ──────────────────
		id := generateToken()
		sizeMB := fmt.Sprintf("%.2f MB", float64(req.SizeBytes)/1024/1024)

		availBytes := int64(0)
		availStr := ""
		if space, err := GetDiskFreeSpace(uploadDir); err == nil {
			availBytes = space.AvailableBytes
			availStr = space.AvailableStr
		}

		pt := &PendingTransfer{
			ID:             id,
			Filename:       req.Filename,
			SizeMB:         sizeMB,
			SizeBytes:      req.SizeBytes,
			MimeType:       req.MimeType,
			SenderIP:       senderIP,
			SenderName:     senderName,
			AvailableBytes: availBytes,
			AvailableStr:   availStr,
			approved:       make(chan bool, 1),
		}

		httpServer.pendingMu.Lock()
		httpServer.pendingTransfers[id] = pt
		httpServer.pendingMu.Unlock()

		// Emit event to desktop UI
		evtData, _ := json.Marshal(pt)
		emit("transfer_request", string(evtData))

		fmt.Printf("⏳ Waiting for user approval: %s from %s\n", req.Filename, senderIP)

		// Wait up to 60 seconds for user response
		select {
		case approved := <-pt.approved:
			if approved {
				fmt.Printf("✅ Transfer approved: %s\n", req.Filename)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("approved"))
			} else {
				fmt.Printf("❌ Transfer rejected: %s\n", req.Filename)
				http.Error(w, "403 Forbidden: transfer rejected by user", http.StatusForbidden)
			}
		case <-time.After(60 * time.Second):
			// Auto-reject on timeout
			httpServer.pendingMu.Lock()
			delete(httpServer.pendingTransfers, id)
			httpServer.pendingMu.Unlock()
			Warn("Transfer request timed out: %s", req.Filename)
			emit("transfer_request_timeout", id)
			http.Error(w, "408 Request Timeout: no response from user", http.StatusRequestTimeout)
		}
	}))

	// ── Upload ────────────────────────────────────────────────────────────────
	mux.HandleFunc("/upload", tokenMiddleware(token, func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("📤 POST /upload - Upload started")

		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("❌ PANIC in upload handler: %v\n%s\n", r, debug.Stack())
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Update heartbeat on upload activity
		state.markHeartbeat()

		// 100 GB max — guard runaway clients
		r.Body = http.MaxBytesReader(w, r.Body, 100*1024*1024*1024)

		// ── Periodic disk space monitor ──────────────────────────────────────
		lastSpaceCheck := time.Now()
		minFree := httpServer.settings.MinFreeSpaceMB
		checkDiskSpace := func() error {
			if minFree <= 0 {
				return nil
			}
			if time.Since(lastSpaceCheck) < 10*time.Second {
				return nil
			}
			lastSpaceCheck = time.Now()
			space, err := GetDiskFreeSpace(uploadDir)
			if err != nil {
				return nil
			}
			if space.AvailableBytes < minFree*1024*1024 {
				return fmt.Errorf("disk space below minimum: %s available, need %d MB reserve",
					space.AvailableStr, minFree)
			}
			return nil
		}

		// ── High-throughput streaming multipart ───────────────────────────────
		contentType := r.Header.Get("Content-Type")
		mediaType, params, ctErr := mime.ParseMediaType(contentType)
		if ctErr != nil || !strings.HasPrefix(mediaType, "multipart/") {
			Error("Invalid Content-Type: %s", contentType)
			http.Error(w, "Expected multipart/form-data", http.StatusBadRequest)
			return
		}
		boundary := params["boundary"]

		// 8 MB network read buffer — reduces TCP recv() syscalls dramatically.
		netReader := bufio.NewReaderSize(r.Body, 8*1024*1024)
		mr := multipart.NewReader(netReader, boundary)

		// ── Concurrent write pipeline ─────────────────────────────────────────
		jobs := make(chan writeJob, writeWorkerCount)
		wg := startWriteWorkers(jobs, state, emit)

		fileCount := 0
		var parseErr error
		// Map of filename -> size in bytes, provided by the mobile client manifest
		fileSizes := make(map[string]int64)

		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				Error("Multipart read error: %v", err)
				parseErr = err
				break
			}

			formName := part.FormName()
			filename := part.FileName()

			// ── Case A: Metadata Manifest ─────────────────────────────────────
			// The mobile UI sends a JSON manifest of all files in this batch
			// as the first field. We use this to get accurate 'total' sizes.
			if formName == "beam_manifest" && filename == "" {
				var manifest []struct {
					Name string `json:"name"`
					Size int64  `json:"size"`
				}
				if err := json.NewDecoder(part).Decode(&manifest); err == nil {
					for _, f := range manifest {
						fileSizes[f.Name] = f.Size
					}
					fmt.Printf("📦 Manifest received: %d files registered\n", len(manifest))
				}
				part.Close()
				continue
			}

			// ── Case B: File Part ─────────────────────────────────────────────
			if filename == "" {
				part.Close()
				continue
			}

			fileCount++
			fmt.Printf("📄 Processing file #%d: %s\n", fileCount, filename)

			rawName := filepath.Base(filename)
			if rawName == "" || rawName == "." {
				rawName = fmt.Sprintf("upload_%d.bin", time.Now().Unix())
			}

			// Auto-rename on conflict (safe to do on main goroutine — sequential)
			dstPath := autoRenamePath(uploadDir, rawName)
			savedName := filepath.Base(dstPath)
			fmt.Printf("💾 Queuing write: %s\n", dstPath)

			// Check disk space before each file during multi-file batches
			if err := checkDiskSpace(); err != nil {
				fmt.Printf("🚫 Aborting upload — %v\n", err)
				io.Copy(io.Discard, part)
				part.Close()
				parseErr = err
				break
			}

			// Read up to largeFileThreshold bytes to determine dispatch strategy.
			var buf bytes.Buffer
			buf.Grow(largeFileThreshold)
			readLimit := int64(largeFileThreshold)
			n, readErr := io.CopyN(&buf, part, readLimit)

			if readErr == nil && n == readLimit {
				// Large file — write synchronously on main goroutine to avoid
				// racing on the shared bufio.Reader (netReader).
				fmt.Printf("📦 Large file — writing synchronously: %s\n", savedName)
				state.beginUpload()

				// Final space check before starting the big write
				if err := checkDiskSpace(); err != nil {
					fmt.Printf("🚫 Aborting large file write — %v\n", err)
					io.Copy(io.Discard, part)
					part.Close()
					state.endUpload()
					parseErr = err
					break
				}

				dst, createErr := os.Create(dstPath)
				if createErr != nil {
					fmt.Println("❌ File creation error:", createErr)
					io.Copy(io.Discard, part) // must drain before NextPart()
					part.Close()
					state.endUpload()
					continue
				}

				diskBuf := bufio.NewWriterSize(dst, 8*1024*1024)
				estTotal := int64(-1)
				writtenTotal := int64(len(buf.Bytes()))

				// Order of size preference:
				// 1. Explicit size from manifest (sent by mobile JS)
				// 2. Part header Content-Length
				// 3. Request Content-Length (only accurate for single-file uploads)
				if sz, ok := fileSizes[filename]; ok {
					estTotal = sz
				} else if cl, _ := strconv.ParseInt(part.Header.Get("Content-Length"), 10, 64); cl > 0 {
					estTotal = cl
				} else if r.ContentLength > 0 && r.ContentLength < 2*1024*1024*1024 {
					estTotal = r.ContentLength
				}

				if estTotal > 0 {
					fmt.Printf("📊 Total size for %s: %d bytes\n", savedName, estTotal)
				}

				lpw := &progressWriter{
					w:           diskBuf,
					total:       estTotal,
					filename:    savedName,
					eventName:   "upload_progress",
					emit:        emit,
					minInterval: 500 * time.Millisecond,
				}
				// Write the already-buffered prefix first.
				prefixBytes := buf.Bytes()
				lpw.Write(prefixBytes)

				// Stream the remainder from the network — check space every ~100 MB.
				var spaceCheckBudget int64 = 100 * 1024 * 1024
				var lErr error
				for {
					chunkSize := int64(8 * 1024 * 1024)
					nw, rErr := io.CopyN(lpw, part, chunkSize)
					writtenTotal += nw
					spaceCheckBudget -= nw
					if rErr != nil {
						lErr = rErr
						if lErr == io.EOF || lErr == io.ErrUnexpectedEOF {
							lErr = nil
						}
					}
					if spaceCheckBudget <= 0 && lErr == nil {
						spaceCheckBudget = 100 * 1024 * 1024
						if err := checkDiskSpace(); err != nil {
							fmt.Printf("🚫 Aborting during streaming — %v\n", err)
							io.Copy(io.Discard, part)
							lErr = err
						}
					}
					if lErr != nil {
						break
					}
					if writtenTotal >= estTotal && estTotal > 0 {
						break
					}
				}
				diskBuf.Flush()
				dst.Close()
				part.Close()
				state.endUpload()

				if lErr != nil {
					fmt.Println("❌ Large file copy error:", lErr)
					if lErr != nil && strings.Contains(lErr.Error(), "disk space below minimum") {
						parseErr = lErr
					}
					continue
				}
				emit("upload_progress", fmt.Sprintf("%s|%d|%d", savedName, writtenTotal, writtenTotal))
				fmt.Printf("✅ Large file saved: %s (%d bytes)\n", savedName, writtenTotal)
				go func(fname string) {
					time.Sleep(100 * time.Millisecond)
					emit("file_received", fname)
				}(savedName)
			} else {
				// Small file (or EOF before threshold): fully buffered — dispatch to worker.
				part.Close()
				if readErr != nil && readErr != io.EOF {
					fmt.Println("❌ Part read error:", readErr)
					continue
				}
				jobs <- writeJob{
					dstPath:   dstPath,
					savedName: savedName,
					totalSize: int64(buf.Len()),
					buf:       buf.Bytes(),
				}
			}
		}

		// Signal workers that no more jobs are coming, then wait for all writes.
		close(jobs)
		wg.Wait()

		if parseErr != nil {
			http.Error(w, "Multipart read error", http.StatusBadRequest)
			return
		}

		if fileCount == 0 {
			http.Error(w, "No files uploaded", http.StatusBadRequest)
			return
		}

		fmt.Println("✅ Upload handler completed")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("✅ Upload Complete"))
	}))

	portInt, listener, err := FindAvailablePort(startPort, 2, 50)
	if err != nil {
		Error("Failed to find available port for Receiver: %v", err)
		if strings.Contains(err.Error(), "permission") || strings.Contains(err.Error(), "access") {
			Info("Permission error — attempting firewall setup...")
			if fwErr := RunFirewallSetup(); fwErr != nil {
				Error("Firewall setup failed: %v", fwErr)
			} else {
				portInt, listener, err = FindAvailablePort(startPort, 2, 50)
				if err != nil {
					fmt.Println("❌ Still failed after firewall setup:", err)
					cancel()
					return nil, "", ""
				}
			}
		} else {
			cancel()
			return nil, "", ""
		}
	}
	portStr := fmt.Sprintf("%d", portInt)

	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  4 * time.Hour, // support 100 GB over slow Wi-Fi (~7 MB/s)
		WriteTimeout: 4 * time.Hour,
		IdleTimeout:  60 * time.Second,
	}
	httpServer.server = srv

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("❌ Server panic: %v\n", r)
			}
		}()
		fmt.Printf("🚀 Starting HTTP receiver on :%s...\n", portStr)
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Printf("❌ Server error: %v\n", err)
		}
	}()

	fmt.Println("✅ StartServer() completed")
	return httpServer, portStr, token
}

// StartSender starts the file-sender HTTP server.
// Returns (server handle, port string, session token).
func StartSender(filePaths []string, callback EventCallback) (*HTTPServer, string, string) {
	defer func() {
		if r := recover(); r != nil {
			Error("PANIC IN StartSender: %v\n%s", r, debug.Stack())
		}
	}()

	token := generateToken()
	emit := func(evt, data string) { safeEmit(callback, evt, data) }

	state := &serverState{}
	ctx, cancel := context.WithCancel(context.Background())

	// Sender also gets a watchdog
	startWatchdog(ctx, state, emit)

	mux := http.NewServeMux()

	// ── Heartbeat ─────────────────────────────────────────────────────────────
	mux.HandleFunc("/heartbeat", tokenMiddleware(token, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		fmt.Println("💓 Sender Heartbeat received")
		wasConnected := state.markHeartbeat()
		if !wasConnected {
			emit("device_connected", "Mobile (Downloader)")
			fmt.Println("💚 Device Connected to Sender!")
		}
		w.WriteHeader(http.StatusOK)
	}))

	// ── Serve files (no token on / — mobile opens the download page directly) ─
	// The generated download URL embedded in the QR already carries the token.

	buildFileBlock := func(filePaths []string) string {
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
			b.WriteString(card(name, sizeStr, "single", "/download"))
		} else {
			for i, path := range filePaths {
				name := filepath.Base(path)
				sizeStr := ""
				if info, err := os.Stat(path); err == nil {
					sizeStr = fmtSize(info.Size())
				}
				cardID := fmt.Sprintf("multi-%d", i)
				downloadURL := fmt.Sprintf("/download/%d", i)
				b.WriteString(card(name, sizeStr, cardID, downloadURL))
			}
		}
		return b.String()
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Content-Type", "text/html")
		content, err := uiFS.ReadFile("ui/download.html")
		if err != nil {
			http.Error(w, "UI Load Error", http.StatusInternalServerError)
			return
		}
		html := strings.Replace(string(content), "{{FILES}}", buildFileBlock(filePaths), 1)
		html = strings.Replace(html, "{{TOKEN}}", token, 1)
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
		mux.HandleFunc("/download", tokenMiddleware(token, func(w http.ResponseWriter, r *http.Request) {
			setCORSHeaders(w)
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
			// Expose the real filename so the mobile JS can use it for link.download
			w.Header().Set("X-Filename", filename)
			w.Header().Set("Access-Control-Expose-Headers", "X-Filename")

			// Track download progress
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
			// Detect real MIME type from extension so mobile can open the file directly.
			// Fall back to octet-stream only for unknown/binary types.
			mimeType := mime.TypeByExtension(filepath.Ext(filename))
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}
			w.Header().Set("Content-Type", mimeType)

			// 8 MB read buffer — avoids many small read syscalls when streaming large files.
			bufReader := bufio.NewReaderSize(file, 8*1024*1024)
			pw := &progressWriter{
				w:           w,
				total:       fileInfo.Size(),
				filename:    filename,
				eventName:   "download_progress",
				emit:        emit,
				lastEmit:    time.Now(),
				minInterval: 500 * time.Millisecond,
			}
			copyChunked(pw, bufReader)
		}))
	} else {
		for i, path := range filePaths {
			idx := i
			filePath := path
			mux.HandleFunc(fmt.Sprintf("/download/%d", idx), tokenMiddleware(token, func(w http.ResponseWriter, r *http.Request) {
				setCORSHeaders(w)
				realName := filepath.Base(filePath)
				w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
				w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, realName))
				// Expose the real filename so the mobile JS can use it for link.download
				w.Header().Set("X-Filename", realName)
				w.Header().Set("Access-Control-Expose-Headers", "X-Filename")

				// Track download progress
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
				// Detect real MIME type from extension so mobile can open the file directly.
				// Fall back to octet-stream only for unknown/binary types.
				mimeType := mime.TypeByExtension(filepath.Ext(filePath))
				if mimeType == "" {
					mimeType = "application/octet-stream"
				}
				w.Header().Set("Content-Type", mimeType)

				// 8 MB read buffer — avoids many small read syscalls when streaming large files.
				bufReader := bufio.NewReaderSize(file, 8*1024*1024)
				pw := &progressWriter{
					w:           w,
					total:       fileInfo.Size(),
					filename:    realName,
					eventName:   "download_progress",
					emit:        emit,
					lastEmit:    time.Now(),
					minInterval: 500 * time.Millisecond,
				}
				copyChunked(pw, bufReader)
			}))
		}
	}

	portInt, listener, err := FindAvailablePort(3005, 2, 50)
	if err != nil {
		fmt.Println("❌ Failed to find available port for Sender:", err)
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
		Handler: mux,
		// 4-hour timeouts — large video files over Wi-Fi can easily exceed 10 min.
		ReadTimeout:  4 * time.Hour,
		WriteTimeout: 4 * time.Hour,
		IdleTimeout:  60 * time.Second,
	}
	httpServer := &HTTPServer{server: srv, cancel: cancel}

	go func() {
		fmt.Printf("🚀 Starting sender on :%s...\n", portStr)
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Println("❌ Sender error:", err)
		}
	}()

	return httpServer, portStr, token
}
