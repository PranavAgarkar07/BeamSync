package beamsync

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

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

//go:embed ui/*.html ui/*.png
var uiFS embed.FS

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
	return int32(s.uploadingCount) //nolint:gosec // small counter
}

func (s *serverState) endUpload() int32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.uploadingCount > 0 {
		s.uploadingCount--
	}
	return int32(s.uploadingCount) //nolint:gosec // small counter
}

func (s *serverState) activeUploads() int32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return int32(s.uploadingCount) //nolint:gosec // small counter
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

type HTTPServer struct {
	server           *http.Server
	cancel           context.CancelFunc
	pendingMu        sync.Mutex
	pendingTransfers map[string]*PendingTransfer
	// settingsMu guards settings. The desktop can replace the transfer
	// settings (blocklist, trustlist) while HTTP handlers are reading them,
	// and the struct carries slice fields — so the handlers must read a
	// consistent snapshot, never a pointer that is being written through.
	settingsMu       sync.RWMutex
	settings         *TransferSettings
	history          *TransferHistory
	stats            *transferStatsTracker
	tokens           *tokenStore
	pageLimiter      *clientRateLimiter
	heartbeatLimiter *clientRateLimiter
	transferLimiter  *clientRateLimiter
	uploadLimiter    *clientRateLimiter
	bootstrapMu      sync.Mutex
	bootstrapToken   string
	state            *serverState
	// uploadDir is the directory StartServerE writes uploads into. Only
	// StartServerE sets it: the sender server registers no upload route, so
	// the field stays empty there.
	uploadDir string
	// emit is this server's safeEmit-wrapped callback, stored so the handler
	// methods can emit without each one taking the callback as a parameter.
	// Both constructors set it — the download routes registered by StartSenderE
	// emit transfer stats.
	emit EventCallback
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

// Settings returns a snapshot of the current transfer settings.
//
// It returns a COPY: the caller must not mutate the server's live settings
// through the returned pointer, because HTTP handlers read it concurrently.
// Use UpdateSettings to change them.
func (s *HTTPServer) Settings() *TransferSettings {
	s.settingsMu.RLock()
	defer s.settingsMu.RUnlock()
	if s.settings == nil {
		return nil
	}
	snapshot := *s.settings
	return &snapshot
}

// UpdateSettings replaces the server's transfer settings atomically.
//
// The whole struct is swapped under the lock, so a concurrent reader either
// sees the old settings or the new ones — never a half-written mix.
func (s *HTTPServer) UpdateSettings(settings TransferSettings) {
	s.settingsMu.Lock()
	s.settings = &settings
	s.settingsMu.Unlock()
}

// currentSettings returns a snapshot for use inside request handlers.
func (s *HTTPServer) currentSettings() *TransferSettings {
	return s.Settings()
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
				return fmt.Errorf("graceful shutdown failed: %w; forced close failed: %w", err, closeErr) //nolint:errorlint // both wrapped
			}
			return err
		}
	}
	return nil
}

func (s *HTTPServer) CurrentBootstrapToken() string {
	if s == nil || s.tokens == nil {
		return ""
	}
	s.bootstrapMu.Lock()
	defer s.bootstrapMu.Unlock()
	return s.bootstrapToken
}

func (s *HTTPServer) RotateBootstrapToken() (string, error) {
	if s == nil || s.tokens == nil {
		return "", fmt.Errorf("no token store")
	}
	newToken, err := s.tokens.issue(tokenScopeBootstrap, 1, "")
	if err != nil {
		return "", err
	}
	s.bootstrapMu.Lock()
	s.bootstrapToken = newToken
	s.bootstrapMu.Unlock()
	s.tokens.revokeScopeExcept(tokenScopeBootstrap, newToken)
	return newToken, nil
}

func StartServerE(uploadDir string, startPort int, settings TransferSettings, callback EventCallback) (*HTTPServer, string, string, error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("PANIC IN StartServer: %v\n%s\n", r, debug.Stack())
		}
	}()

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, "", "", fmt.Errorf("cannot create save folder: %w", err)
	}

	fingerprint, err := serverTLSFingerprint()
	if err != nil {
		return nil, "", "", fmt.Errorf("cannot prepare secure context: %w", err)
	}
	tokens, err := newTokenStore(fingerprint)
	if err != nil {
		return nil, "", "", fmt.Errorf("cannot create session credentials: %w", err)
	}
	token, err := tokens.issue(tokenScopeBootstrap, 1, "")
	if err != nil {
		return nil, "", "", fmt.Errorf("cannot create session credentials: %w", err)
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
		bootstrapToken:   token,
		state:            state,
		pageLimiter:      newClientRateLimiter(60, time.Minute),
		heartbeatLimiter: newClientRateLimiter(120, time.Minute),
		transferLimiter:  newClientRateLimiter(30, time.Minute),
		uploadLimiter:    newClientRateLimiter(12, time.Minute),
		uploadDir:        uploadDir,
		emit:             emit,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/heartbeat", rateLimitMiddleware(httpServer.heartbeatLimiter, httpServer.currentSettings, tokenMiddlewareAny(tokens, []tokenScope{tokenScopeSession, tokenScopeBootstrap}, false, func(w http.ResponseWriter, r *http.Request) {
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
	})))

	mux.HandleFunc("/", rateLimitMiddleware(httpServer.pageLimiter, httpServer.currentSettings, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if err := tokens.validate(extractToken(r), "", tokenScopeBootstrap, false); err != nil {
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
		w.Write([]byte(html)) //nolint:errcheck

		wasConnected := state.markHeartbeat()
		if !wasConnected {
			emit("device_connected", "Android Device")
		}
	}))

	mux.HandleFunc("/logo.png", rateLimitMiddleware(httpServer.pageLimiter, httpServer.currentSettings, func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		w.Header().Set("Content-Type", "image/png")
		content, err := uiFS.ReadFile("ui/logo.png")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Write(content) //nolint:errcheck
	}))

	mux.HandleFunc("/stats", rateLimitMiddleware(httpServer.pageLimiter, httpServer.currentSettings, tokenMiddlewareAny(tokens, []tokenScope{tokenScopeSession, tokenScopeBootstrap}, false, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Content-Type", "application/json")
		snapshot := httpServer.stats.snapshot(state.activeUploads())
		w.Write([]byte(transferStatsJSON(snapshot))) //nolint:errcheck,gosec
	})))

	mux.HandleFunc("/request-transfer", rateLimitMiddleware(httpServer.transferLimiter, httpServer.currentSettings, tokenMiddlewareAny(tokens, []tokenScope{tokenScopeSession, tokenScopeBootstrap}, false, func(w http.ResponseWriter, r *http.Request) {
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

		s := httpServer.currentSettings()
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
			w.Write([]byte("approved")) //nolint:errcheck,gosec
			return
		}

		if s.Mode == TransferModeTrustedOnly {
			if s.isDeviceTrusted(senderIP) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("approved")) //nolint:errcheck,gosec
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
				w.Write([]byte("approved")) //nolint:errcheck,gosec
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
	})))

	mux.HandleFunc("/upload", rateLimitMiddleware(httpServer.uploadLimiter, httpServer.currentSettings, tokenMiddlewareAny(tokens, []tokenScope{tokenScopeSession, tokenScopeBootstrap}, false, httpServer.handleUpload)))

	portInt, listener, err := FindAvailablePort(startPort, 2, 50)
	if err != nil {
		if strings.Contains(err.Error(), "permission") || strings.Contains(err.Error(), "access") {
			if fwErr := RunFirewallSetup(); fwErr == nil {
				portInt, listener, err = FindAvailablePort(startPort, 2, 50)
				if err != nil {
					cancel()
					return nil, "", "", fmt.Errorf("no free local port even after firewall setup: %w. %s", err, FirewallManualSteps())
				}
			} else {
				cancel()
				return nil, "", "", fmt.Errorf("cannot open a local port (%w) and automatic firewall setup failed: %w. %s", err, fwErr, FirewallManualSteps())
			}
		} else {
			cancel()
			return nil, "", "", fmt.Errorf("no free local port (TCP 3000-3100 busy): %w. Close the app using the port and retry.", err)
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
		return nil, "", "", fmt.Errorf("cannot start secure listener: %w", tlsErr)
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

	startBootstrapRotator(ctx, httpServer, portStr, emit)

	return httpServer, portStr, token, nil
}

func StartSenderE(filePaths []string, callback EventCallback) (*HTTPServer, string, string, error) {
	fingerprint, err := serverTLSFingerprint()
	if err != nil {
		return nil, "", "", fmt.Errorf("cannot prepare secure context: %w", err)
	}
	tokens, err := newTokenStore(fingerprint)
	if err != nil {
		return nil, "", "", fmt.Errorf("cannot create session credentials: %w", err)
	}
	token, err := tokens.issue(tokenScopeBootstrap, 1, "")
	if err != nil {
		return nil, "", "", fmt.Errorf("cannot create session credentials: %w", err)
	}
	emit := func(evt, data string) { safeEmit(callback, evt, data) }

	state := &serverState{}
	ctx, cancel := context.WithCancel(context.Background())
	tokens.startCleanup(ctx)
	httpServer := &HTTPServer{
		cancel:           cancel,
		history:          NewTransferHistory(defaultTransferHistoryLimit),
		stats:            newTransferStatsTracker(),
		tokens:           tokens,
		bootstrapToken:   token,
		state:            state,
		pageLimiter:      newClientRateLimiter(60, time.Minute),
		heartbeatLimiter: newClientRateLimiter(120, time.Minute),
		transferLimiter:  newClientRateLimiter(30, time.Minute),
		uploadLimiter:    newClientRateLimiter(12, time.Minute),
		emit:             emit,
	}
	startWatchdog(ctx, state, emit)

	mux := http.NewServeMux()

	mux.HandleFunc("/heartbeat", rateLimitMiddleware(httpServer.heartbeatLimiter, nil, tokenMiddlewareAny(tokens, []tokenScope{tokenScopeSession, tokenScopeBootstrap}, false, func(w http.ResponseWriter, r *http.Request) {
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
	})))

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

	mux.HandleFunc("/", rateLimitMiddleware(httpServer.pageLimiter, nil, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if err := tokens.validate(extractToken(r), "", tokenScopeBootstrap, false); err != nil {
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
		w.Write([]byte(html)) //nolint:errcheck
	}))

	mux.HandleFunc("/logo.png", rateLimitMiddleware(httpServer.pageLimiter, nil, func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		w.Header().Set("Content-Type", "image/png")
		content, err := uiFS.ReadFile("ui/logo.png")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Write(content) //nolint:errcheck
	}))

	if len(filePaths) == 1 {
		mux.HandleFunc("/download", rateLimitMiddleware(httpServer.uploadLimiter, nil, tokenMiddlewareAny(tokens, []tokenScope{tokenScopeTransfer, tokenScopeBootstrap}, true, func(w http.ResponseWriter, r *http.Request) {
			httpServer.handleDownload(filePaths[0], w, r)
		})))
	} else {
		for i, path := range filePaths {
			idx := i
			filePath := path
			mux.HandleFunc(fmt.Sprintf("/download/%d", idx), rateLimitMiddleware(httpServer.uploadLimiter, nil, tokenMiddlewareAny(tokens, []tokenScope{tokenScopeTransfer, tokenScopeBootstrap}, true, func(w http.ResponseWriter, r *http.Request) {
				httpServer.handleDownload(filePath, w, r)
			})))
		}
	}

	portInt, listener, err := FindAvailablePort(3005, 2, 50)
	if err != nil {
		if strings.Contains(err.Error(), "permission") || strings.Contains(err.Error(), "access") {
			if fwErr := RunFirewallSetup(); fwErr == nil {
				portInt, listener, err = FindAvailablePort(3005, 2, 50)
				if err != nil {
					cancel()
					return nil, "", "", fmt.Errorf("no free local port even after firewall setup: %w. %s", err, FirewallManualSteps())
				}
			} else {
				cancel()
				return nil, "", "", fmt.Errorf("cannot open a local port (%w) and automatic firewall setup failed: %w. %s", err, fwErr, FirewallManualSteps())
			}
		} else {
			cancel()
			return nil, "", "", fmt.Errorf("no free local port (TCP 3000-3100 busy): %w. Close the app using the port and retry.", err)
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
		return nil, "", "", fmt.Errorf("cannot start secure listener: %w", tlsErr)
	}

	go func() {
		if err := srv.Serve(serveListener); err != nil && err != http.ErrServerClosed {
			fmt.Println("Sender error:", err)
		}
	}()

	startBootstrapRotator(ctx, httpServer, portStr, emit)

	return httpServer, portStr, token, nil
}
