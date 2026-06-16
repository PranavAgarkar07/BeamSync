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
	"math"
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

const eventDispatcherBufferSize = 256

type eventDispatchJob struct {
	emit  EventCallback
	event string
	data  string
}

type eventDispatcher struct {
	queue chan eventDispatchJob
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
		return false
	}
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

type rateLimitState struct {
	windowStart time.Time
	count       int
	lastSeen    time.Time
}

type rateLimitDecision struct {
	allowed    bool
	retryAfter time.Duration
	limit      int
	remaining  int
	resetAt    time.Time
}

type clientRateLimiter struct {
	mu         sync.Mutex
	limit      int
	window     time.Duration
	maxClients int
	clients    map[string]*rateLimitState
	now        func() time.Time
}

func newClientRateLimiter(limit int, window time.Duration) *clientRateLimiter {
	return &clientRateLimiter{
		limit:      limit,
		window:     window,
		maxClients: 4096,
		clients:    make(map[string]*rateLimitState),
		now:        time.Now,
	}
}

func (l *clientRateLimiter) allow(client string) rateLimitDecision {
	if l == nil || l.limit <= 0 || l.window <= 0 {
		return rateLimitDecision{allowed: true}
	}

	now := l.now()
	resetAt := now.Add(l.window)
	l.mu.Lock()
	defer l.mu.Unlock()

	l.prune(now)

	state, ok := l.clients[client]
	if !ok || now.Sub(state.windowStart) >= l.window {
		if !ok {
			l.evictOldestIfFull()
		}
		l.clients[client] = &rateLimitState{
			windowStart: now,
			count:       1,
			lastSeen:    now,
		}
		return rateLimitDecision{
			allowed:   true,
			limit:     l.limit,
			remaining: l.limit - 1,
			resetAt:   now.Add(l.window),
		}
	}

	state.lastSeen = now
	resetAt = state.windowStart.Add(l.window)
	if state.count >= l.limit {
		return rateLimitDecision{
			allowed:    false,
			retryAfter: l.window - now.Sub(state.windowStart),
			limit:      l.limit,
			remaining:  0,
			resetAt:    resetAt,
		}
	}

	state.count++
	return rateLimitDecision{
		allowed:   true,
		limit:     l.limit,
		remaining: l.limit - state.count,
		resetAt:   resetAt,
	}
}

func (l *clientRateLimiter) prune(now time.Time) {
	for client, state := range l.clients {
		if now.Sub(state.lastSeen) > 2*l.window {
			delete(l.clients, client)
		}
	}
}

func (l *clientRateLimiter) evictOldestIfFull() {
	if l.maxClients <= 0 || len(l.clients) < l.maxClients {
		return
	}

	var oldestClient string
	var oldestSeen time.Time
	for client, state := range l.clients {
		if oldestClient == "" ||
			state.lastSeen.Before(oldestSeen) ||
			(state.lastSeen.Equal(oldestSeen) && client < oldestClient) {
			oldestClient = client
			oldestSeen = state.lastSeen
		}
	}

	if oldestClient != "" {
		delete(l.clients, oldestClient)
	}
}

// PendingTransfer holds a blocked transfer awaiting user approval.
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

// HTTPServer wraps http.Server so we can shut it down cleanly.
type HTTPServer struct {
	server           *http.Server
	cancel           context.CancelFunc
	pendingMu        sync.Mutex
	pendingTransfers map[string]*PendingTransfer
	settings         *TransferSettings
	history          *TransferHistory
	stats            *transferStatsTracker
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

func (s *HTTPServer) TransferHistory() []TransferRecord {
	if s == nil || s.history == nil {
		return nil
	}
	return s.history.List()
}

// Stats returns the current receiver transfer statistics snapshot.
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

// progressWriter wraps an io.Writer and emits upload_progress events
// as bytes are written. Uses an adaptive interval to avoid event flooding.
type progressWriter struct {
	w           io.Writer
	total       int64
	written     int64
	filename    string
	emit        func(string, string)
	lastEmit    time.Time
	minInterval time.Duration
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.w.Write(p)
	pw.written += int64(n)
	now := time.Now()
	if now.Sub(pw.lastEmit) >= pw.minInterval {
		data := fmt.Sprintf("%s|%d|%d", pw.filename, pw.written, pw.total)
		pw.emit("upload_progress", data)
		pw.lastEmit = now
	}
	return n, err
}

// downloadProgressWriter wraps an io.Writer and emits download_progress events
// as bytes are written. Uses an adaptive interval to avoid event flooding.
type downloadProgressWriter struct {
	w           io.Writer
	total       int64
	written     int64
	filename    string
	emit        func(string, string)
	lastEmit    time.Time
	minInterval time.Duration
}

func (dw *downloadProgressWriter) Write(p []byte) (int, error) {
	n, err := dw.w.Write(p)
	dw.written += int64(n)
	now := time.Now()
	if now.Sub(dw.lastEmit) >= dw.minInterval {
		data := fmt.Sprintf("%s|%d|%d", dw.filename, dw.written, dw.total)
		dw.emit("download_progress", data)
		dw.lastEmit = now
	}
	return n, err
}

// copyChunked reads src in large chunks before writing to dst.
// Go's multipart.Part has an internal 4 KB bufio, so Part.Read returns ≤4 KB
// per call regardless of the dst buffer size. Without this helper, we end up
// making thousands of tiny Write() syscalls per second which kills throughput.
// copyChunked accumulates those 4 KB reads into a single chunkSize Write(),
// giving the OS large sequential disk I/O instead of random small writes.
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

type manifestEntry struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
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
	stats *transferStatsTracker,
	emit func(string, string),
	history *TransferHistory,
) *sync.WaitGroup {
	var wg sync.WaitGroup
	for i := 0; i < writeWorkerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				writeFileToDisk(job, state, stats, emit, history)
			}
		}()
	}
	return &wg
}

// writeFileToDisk performs the actual file write for one job and emits events.
// Only small files (fully buffered in buf) are dispatched here.
func writeFileToDisk(job writeJob, state *serverState, stats *transferStatsTracker, emit func(string, string), history *TransferHistory) {
	startedAt := time.Now()
	state.beginUpload()
	defer state.endUpload()

	dst, err := os.Create(job.dstPath)
	if err != nil {
		fmt.Println("❌ File creation error:", err)
		logTransfer(history, emit, TransferRecord{
			Filename:  job.savedName,
			Direction: TransferDirectionReceive,
			Status:    TransferStatusFailed,
			SizeBytes: job.totalSize,
			StartedAt: startedAt,
			Error:     err.Error(),
		})
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
		emit:        emit,
		minInterval: 200 * time.Millisecond,
	}
	n, werr := pw.Write(job.buf)
	written := int64(n)
	var transferErr error
	if werr != nil {
		fmt.Println("❌ Write error:", werr)
		transferErr = werr
	}

	if flushErr := diskBuf.Flush(); flushErr != nil {
		fmt.Println("❌ Disk flush error:", flushErr)
		if transferErr == nil {
			transferErr = flushErr
		}
	}

	status := TransferStatusCompleted
	errMsg := ""
	if transferErr != nil {
		status = TransferStatusFailed
		errMsg = transferErr.Error()
	}

	emit("upload_progress", fmt.Sprintf("%s|%d|%d", job.savedName, written, written))
	logTransfer(history, emit, TransferRecord{
		Filename:  job.savedName,
		Direction: TransferDirectionReceive,
		Status:    status,
		SizeBytes: written,
		StartedAt: startedAt,
		Error:     errMsg,
	})
	if status != TransferStatusCompleted {
		return
	}
	if stats != nil {
		snapshot := stats.recordReceived(job.savedName, written, atomic.LoadInt32(&state.uploadingCount))
		emit("transfer_stats", transferStatsJSON(snapshot))
	}

	fmt.Printf("✅ File saved: %s (%d bytes)\n", job.savedName, written)
	go func(fname string) {
		time.Sleep(100 * time.Millisecond)
		emit("file_received", fname)
	}(job.savedName)
}

func processManifest(r io.Reader) (map[string]int64, error) {
	var manifest []manifestEntry
	if err := json.NewDecoder(r).Decode(&manifest); err != nil {
		return nil, err
	}

	fileSizes := make(map[string]int64, len(manifest))
	for _, f := range manifest {
		fileSizes[f.Name] = f.Size
	}
	return fileSizes, nil
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

// validateToken middleware — returns 403 if the token query-param doesn't match.
// Exempt routes: "/" (serves UI page).
func tokenMiddleware(token string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if got := r.URL.Query().Get("token"); got != token {
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

func setRateLimitHeaders(w http.ResponseWriter, decision rateLimitDecision) {
	if decision.limit <= 0 {
		return
	}
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(decision.limit))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(decision.remaining))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(decision.resetAt.Unix(), 10))
}

func rateLimitMiddleware(limiter *clientRateLimiter, settings *TransferSettings, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isUI := r.URL.Path == "/" || r.URL.Path == "/logo.png"
		if !isUI {
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}

		ip := clientIP(r)
		if settings != nil && settings.isDeviceTrusted(ip) {
			next(w, r)
			return
		}

		decision := limiter.allow(ip)
		setRateLimitHeaders(w, decision)
		if !decision.allowed {
			retryAfter := decision.retryAfter
			if retryAfter < time.Second {
				retryAfter = time.Second
			}
			seconds := int64(math.Ceil(retryAfter.Seconds()))
			w.Header().Set("Retry-After", strconv.FormatInt(seconds, 10))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error":      "rate_limit_exceeded",
				"message":    "429 Too Many Requests: rate limit exceeded",
				"retryAfter": seconds,
			})
			return
		}

		next(w, r)
	}
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

// safeEmit queues an event without spawning a goroutine per notification.
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
		fmt.Println("⚠️ Failed to marshal transfer history record:", err)
		return
	}
	emit("transfer_logged", string(data))
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
		fmt.Println("❌ Failed to create upload directory:", err)
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
		history:          NewTransferHistory(defaultTransferHistoryLimit),
		stats:            newTransferStatsTracker(),
	}

	mux := http.NewServeMux()
	pageLimiter := newClientRateLimiter(60, time.Minute)
	heartbeatLimiter := newClientRateLimiter(120, time.Minute)
	transferLimiter := newClientRateLimiter(30, time.Minute)
	uploadLimiter := newClientRateLimiter(12, time.Minute)

	// ── Heartbeat ────────────────────────────────────────────────────────────
	mux.HandleFunc("/heartbeat", rateLimitMiddleware(heartbeatLimiter, httpServer.settings, tokenMiddleware(token, func(w http.ResponseWriter, r *http.Request) {
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
	})))

	// ── Serve UI (no token required — this IS the page that shows the token) ─
	mux.HandleFunc("/", rateLimitMiddleware(pageLimiter, httpServer.settings, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		fmt.Println("🌐 GET / - Serving upload UI")
		setCORSHeaders(w)
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
	}))

	mux.HandleFunc("/logo.png", rateLimitMiddleware(pageLimiter, httpServer.settings, func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		w.Header().Set("Content-Type", "image/png")
		content, err := uiFS.ReadFile("ui/logo.png")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Write(content)
	}))

	mux.HandleFunc("/stats", tokenMiddleware(token, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Content-Type", "application/json")
		snapshot := httpServer.stats.snapshot(atomic.LoadInt32(&state.uploadingCount))
		w.Write([]byte(transferStatsJSON(snapshot)))
	}))

	// ── Request Transfer (ask before accepting) ──────────────────────────────
	mux.HandleFunc("/request-transfer", rateLimitMiddleware(transferLimiter, httpServer.settings, tokenMiddleware(token, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract sender IP from the actual TCP connection.
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

		// ── Check blocked devices ─────────────────────────────────────────────
		if s.isDeviceBlocked(senderIP) {
			fmt.Printf("🚫 Blocked device tried to send: %s\n", senderIP)
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
			fmt.Printf("🚫 Blocked file extension: %s\n", req.Filename)
			http.Error(w, "403 Forbidden: file type is blocked", http.StatusForbidden)
			return
		}

		// ── Check max file size ───────────────────────────────────────────────
		if s.MaxFileSizeMB > 0 && req.SizeBytes > s.MaxFileSizeMB*1024*1024 {
			fmt.Printf("🚫 File too large: %d bytes (max %d MB)\n", req.SizeBytes, s.MaxFileSizeMB)
			http.Error(w, fmt.Sprintf("403 Forbidden: file exceeds max size of %d MB", s.MaxFileSizeMB), http.StatusForbidden)
			return
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
			fmt.Printf("⏰ Transfer request timed out: %s\n", req.Filename)
			emit("transfer_request_timeout", id)
			http.Error(w, "408 Request Timeout: no response from user", http.StatusRequestTimeout)
		}
	})))

	// ── Upload ────────────────────────────────────────────────────────────────
	mux.HandleFunc("/upload", rateLimitMiddleware(uploadLimiter, httpServer.settings, tokenMiddleware(token, func(w http.ResponseWriter, r *http.Request) {
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
		w.Header().Set("Accept-Ranges", "bytes")

		// Update heartbeat on upload activity
		state.markHeartbeat()

		// 100 GB max — guard runaway clients
		r.Body = http.MaxBytesReader(w, r.Body, 100*1024*1024*1024)

		// ── High-throughput streaming multipart ───────────────────────────────
		contentType := r.Header.Get("Content-Type")
		mediaType, params, ctErr := mime.ParseMediaType(contentType)
		if ctErr != nil || !strings.HasPrefix(mediaType, "multipart/") {
			fmt.Println("❌ Invalid Content-Type:", contentType)
			http.Error(w, "Expected multipart/form-data", http.StatusBadRequest)
			return
		}
		boundary := params["boundary"]

		// 8 MB network read buffer — reduces TCP recv() syscalls dramatically.
		netReader := bufio.NewReaderSize(r.Body, 8*1024*1024)
		mr := multipart.NewReader(netReader, boundary)

		// ── Concurrent write pipeline ─────────────────────────────────────────
		jobs := make(chan writeJob, writeWorkerCount)
		wg := startWriteWorkers(jobs, state, httpServer.stats, emit, httpServer.history)

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
				fmt.Println("❌ Multipart read error:", err)
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

			// Read up to largeFileThreshold bytes to determine dispatch strategy.
			var buf bytes.Buffer
			buf.Grow(largeFileThreshold)
			readLimit := int64(largeFileThreshold)
			n, readErr := io.CopyN(&buf, part, readLimit)

			if readErr == nil && n == readLimit {
				// Large file — write synchronously on main goroutine to avoid
				// racing on the shared bufio.Reader (netReader).
				fmt.Printf("📦 Large file — writing synchronously: %s\n", savedName)
				startedAt := time.Now()
				prefixSize := n
				state.beginUpload()

				dst, createErr := os.Create(dstPath)
				if createErr != nil {
					fmt.Println("❌ File creation error:", createErr)
					io.Copy(io.Discard, part) // must drain before NextPart()
					part.Close()
					state.endUpload()
					size := fileSizes[filename]
					if size == 0 {
						size = prefixSize
					}
					logTransfer(httpServer.history, emit, TransferRecord{
						Filename:  savedName,
						Direction: TransferDirectionReceive,
						Status:    TransferStatusFailed,
						SizeBytes: size,
						StartedAt: startedAt,
						Error:     createErr.Error(),
					})
					continue
				}

				diskBuf := bufio.NewWriterSize(dst, 8*1024*1024)
				estTotal := int64(-1)

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
					emit:        emit,
					minInterval: 500 * time.Millisecond,
				}
				// Write the already-buffered prefix first.
				prefixBytes := buf.Bytes()
				prefixWritten, prefixErr := lpw.Write(prefixBytes)
				// Stream the remainder from the network.
				lWritten, lErr := copyChunked(lpw, part, 8*1024*1024)
				lWritten += int64(prefixWritten)
				flushErr := diskBuf.Flush()
				dst.Close()
				part.Close()
				state.endUpload()

				var transferErr error
				switch {
				case prefixErr != nil:
					transferErr = prefixErr
				case lErr != nil:
					transferErr = lErr
				case flushErr != nil:
					transferErr = flushErr
				}

				if transferErr != nil {
					fmt.Println("❌ Large file copy error:", transferErr)
					logTransfer(httpServer.history, emit, TransferRecord{
						Filename:  savedName,
						Direction: TransferDirectionReceive,
						Status:    TransferStatusFailed,
						SizeBytes: lWritten,
						StartedAt: startedAt,
						Error:     transferErr.Error(),
					})
					continue
				}
				emit("upload_progress", fmt.Sprintf("%s|%d|%d", savedName, lWritten, lWritten))
				snapshot := httpServer.stats.recordReceived(savedName, lWritten, atomic.LoadInt32(&state.uploadingCount))
				emit("transfer_stats", transferStatsJSON(snapshot))
				fmt.Printf("✅ Large file saved: %s (%d bytes)\n", savedName, lWritten)
				logTransfer(httpServer.history, emit, TransferRecord{
					Filename:  savedName,
					Direction: TransferDirectionReceive,
					Status:    TransferStatusCompleted,
					SizeBytes: lWritten,
					StartedAt: startedAt,
				})
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
	})))

	mux.HandleFunc("/upload/resumable", tokenMiddleware(token, handleResumableUpload(uploadDir, state, emit)))
	mux.HandleFunc("/upload-status/", tokenMiddleware(token, handleResumableUploadStatus(uploadDir)))

	portInt, listener, err := FindAvailablePort(startPort, 2, 50)
	if err != nil {
		fmt.Println("❌ Failed to find available port for Receiver:", err)
		if strings.Contains(err.Error(), "permission") || strings.Contains(err.Error(), "access") {
			fmt.Println("🔒 Permission error — attempting firewall setup...")
			if fwErr := RunFirewallSetup(); fwErr != nil {
				fmt.Printf("❌ Firewall setup failed: %v\n", fwErr)
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
	serveListener, tlsEnabled, tlsErr := maybeTLSListener(listener)
	if tlsErr != nil {
		fmt.Printf("❌ Failed to configure TLS receiver: %v\n", tlsErr)
		cancel()
		listener.Close()
		return nil, "", ""
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("❌ Server panic: %v\n", r)
			}
		}()
		protocol := "HTTP"
		if tlsEnabled {
			protocol = "HTTPS"
		}
		fmt.Printf("🚀 Starting %s receiver on :%s...\n", protocol, portStr)
		if err := srv.Serve(serveListener); err != nil && err != http.ErrServerClosed {
			fmt.Printf("❌ Server error: %v\n", err)
		}
	}()

	fmt.Println("✅ StartServer() completed")
	return httpServer, portStr, token
}

// StartSender starts the file-sender HTTP server.
// Returns (server handle, port string, session token).
func StartSender(filePaths []string, callback EventCallback) (*HTTPServer, string, string) {
	token := generateToken()
	emit := func(evt, data string) { safeEmit(callback, evt, data) }

	state := &serverState{}
	ctx, cancel := context.WithCancel(context.Background())
	httpServer := &HTTPServer{
		cancel:  cancel,
		history: NewTransferHistory(defaultTransferHistoryLimit),
		stats:   newTransferStatsTracker(),
	}

	// Sender also gets a watchdog
	startWatchdog(ctx, state, emit)

	mux := http.NewServeMux()
	pageLimiter := newClientRateLimiter(60, time.Minute)
	heartbeatLimiter := newClientRateLimiter(120, time.Minute)
	downloadLimiter := newClientRateLimiter(30, time.Minute)

	// ── Heartbeat ─────────────────────────────────────────────────────────────
	mux.HandleFunc("/heartbeat", rateLimitMiddleware(heartbeatLimiter, httpServer.settings, tokenMiddleware(token, func(w http.ResponseWriter, r *http.Request) {
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
	})))

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
			b.WriteString(card(name, sizeStr, "single", fmt.Sprintf("/download?token=%s", token)))
		} else {
			for i, path := range filePaths {
				name := filepath.Base(path)
				sizeStr := ""
				if info, err := os.Stat(path); err == nil {
					sizeStr = fmtSize(info.Size())
				}
				cardID := fmt.Sprintf("multi-%d", i)
				downloadURL := fmt.Sprintf("/download/%d?token=%s", i, token)
				b.WriteString(card(name, sizeStr, cardID, downloadURL))
			}
		}
		return b.String()
	}

	mux.HandleFunc("/", rateLimitMiddleware(pageLimiter, httpServer.settings, func(w http.ResponseWriter, r *http.Request) {
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
	}))

	mux.HandleFunc("/logo.png", rateLimitMiddleware(pageLimiter, httpServer.settings, func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		w.Header().Set("Content-Type", "image/png")
		content, err := uiFS.ReadFile("ui/logo.png")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Write(content)
	}))

	if len(filePaths) == 1 {
		filePath := filePaths[0]
		filename := filepath.Base(filePath)
		mux.HandleFunc("/download", rateLimitMiddleware(downloadLimiter, httpServer.settings, tokenMiddleware(token, func(w http.ResponseWriter, r *http.Request) {
			activeDownloads := atomic.AddInt32(&state.uploadingCount, 1)
			emit("transfer_stats", transferStatsJSON(httpServer.stats.snapshotDownloads(activeDownloads)))
			defer func() {
				activeDownloads := atomic.AddInt32(&state.uploadingCount, -1)
				emit("transfer_stats", transferStatsJSON(httpServer.stats.snapshotDownloads(activeDownloads)))
			}()
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

			startedAt := time.Now()
			// 8 MB read buffer — avoids many small read syscalls when streaming large files.
			bufReader := bufio.NewReaderSize(file, 8*1024*1024)
			progressWriter := &downloadProgressWriter{
				w:           w,
				total:       fileInfo.Size(),
				written:     0,
				filename:    filename,
				emit:        emit,
				lastEmit:    time.Now(),
				minInterval: 500 * time.Millisecond,
			}
			written, copyErr := copyChunked(progressWriter, bufReader, 8*1024*1024)
			status := TransferStatusCompleted
			errMsg := ""
			if copyErr != nil {
				status = TransferStatusFailed
				errMsg = copyErr.Error()
			} else {
				snapshot := httpServer.stats.recordSent(filename, written, atomic.LoadInt32(&state.uploadingCount))
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
		})))
	} else {
		for i, path := range filePaths {
			idx := i
			filePath := path
			mux.HandleFunc(fmt.Sprintf("/download/%d", idx), rateLimitMiddleware(downloadLimiter, httpServer.settings, tokenMiddleware(token, func(w http.ResponseWriter, r *http.Request) {
				realName := filepath.Base(filePath)
				activeDownloads := atomic.AddInt32(&state.uploadingCount, 1)
				emit("transfer_stats", transferStatsJSON(httpServer.stats.snapshotDownloads(activeDownloads)))
				defer func() {
					activeDownloads := atomic.AddInt32(&state.uploadingCount, -1)
					emit("transfer_stats", transferStatsJSON(httpServer.stats.snapshotDownloads(activeDownloads)))
				}()
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

				startedAt := time.Now()
				// 8 MB read buffer — avoids many small read syscalls when streaming large files.
				bufReader := bufio.NewReaderSize(file, 8*1024*1024)
				progressWriter := &downloadProgressWriter{
					w:           w,
					total:       fileInfo.Size(),
					written:     0,
					filename:    realName,
					emit:        emit,
					lastEmit:    time.Now(),
					minInterval: 500 * time.Millisecond,
				}
				written, copyErr := copyChunked(progressWriter, bufReader, 8*1024*1024)
				status := TransferStatusCompleted
				errMsg := ""
				if copyErr != nil {
					status = TransferStatusFailed
					errMsg = copyErr.Error()
				} else {
					snapshot := httpServer.stats.recordSent(realName, written, atomic.LoadInt32(&state.uploadingCount))
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
			})))
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
	httpServer.server = srv
	serveListener, tlsEnabled, tlsErr := maybeTLSListener(listener)
	if tlsErr != nil {
		fmt.Printf("❌ Failed to configure TLS sender: %v\n", tlsErr)
		cancel()
		listener.Close()
		return nil, "", ""
	}

	go func() {
		protocol := "HTTP"
		if tlsEnabled {
			protocol = "HTTPS"
		}
		fmt.Printf("🚀 Starting %s sender on :%s...\n", protocol, portStr)
		if err := srv.Serve(serveListener); err != nil && err != http.ErrServerClosed {
			fmt.Println("❌ Sender error:", err)
		}
	}()

	return httpServer, portStr, token
}
