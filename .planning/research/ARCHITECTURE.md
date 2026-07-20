# Architecture Refactoring Research

**Date:** 2026-07-20
**Focus:** Decomposition strategy for `beamsync/server.go` (1726 lines), `desktop/app.go` (734 lines), and `desktop/frontend/src/App.svelte` (2160 lines)

---

## 1. Monolithic server.go → Focused Files

### Current organization (all in one file)

| Lines | Concern | Extraction target |
|-------|---------|-------------------|
| 33–81 | `eventDispatcher` | `events.go` (also use per-server instance, kill global singleton) |
| 86–93, 153–282 | `clientRateLimiter` | `rate_limiter.go` (partial — some rate limiter already exists in tests) |
| 95–140 | `serverState` (connection state) | `server_state.go` or keep in `server.go` |
| 153–282 | Rate limiter types + logic | `rate_limiter.go` |
| 284–306, 308–356 | `HTTPServer` struct + methods | `server.go` (keeps orchestration + the aggregate struct) |
| 358–404 | `progressWriter` / `downloadProgressWriter` | `progress.go` (merge into one type with direction field) |
| 406–438 | `copyChunked` + chunk buffer pool | `io_helpers.go` |
| 440–594 | Write pipeline (workers, `writeFileToDisk`, job types) | `upload_handler.go` |
| 596–648 | `processManifest`, `generateID`, `sha256Hex`, `hashMatches`, `clientIP` | `helpers.go` (shared utilities) |
| 619–696 | Middleware (token, rate-limit, CORS, headers) | `middleware.go` |
| 698–714 | `autoRenamePath` | `helpers.go` |
| 716–743 | Watchdog | `events.go` |
| 745–767 | `safeEmit`, `logTransfer` | `events.go` |
| 770–1360 | `StartServer()` (receiver) | `server.go` (stays here — it's the orchestration hub) |
| 1364–1726 | `StartSender()` (sender) | `server.go` (stays here) |

### Suggested extraction order (dependency-respecting)

| Step | Extraction | Dependencies | Risk |
|------|------------|-------------|------|
| 1 | `helpers.go` — `generateID`, `sha256Hex`, `hashMatches`, `clientIP`, `autoRenamePath`, `processManifest`, `uploadBufferInitialCapacity` | None | **Low** — pure functions, no struct references |
| 2 | `rate_limiter.go` — `rateLimitState`, `rateLimitDecision`, `clientRateLimiter` (move tests too) | None | **Low** — rate limiter is self-contained |
| 3 | `progress.go` — merge `progressWriter` + `downloadProgressWriter` into one with a `direction` field | None | **Low** — both have identical implementation; renaming the field is safe |
| 4 | `io_helpers.go` — `copyChunked`, `chunkBufferPool` | None | **Low** — pure I/O helpers |
| 5 | `middleware.go` — `tokenMiddleware`, `rateLimitMiddleware`, `setCORSHeaders`, `setRateLimitHeaders` | `rate_limiter.go`, `auth_tokens.go` | **Low** — no cross-file state |
| 6 | `events.go` — `EventCallback`, `eventDispatchJob`, `eventDispatcher`, `startWatchdog`, `safeEmit`, `logTransfer` | None | **Medium** — removing the `defaultEventDispatcher` singleton requires updating all callers in `StartServer`/`StartSender` |
| 7 | `upload_handler.go` — `writeJob`, `manifestEntry`, `writeWorkerCount`, `largeFileThreshold`, `startWriteWorkers`, `writeFileToDisk` | `progress.go`, `io_helpers.go`, `helpers.go`, `events.go` | **Medium** — depends on many extracted pieces; test the integration end-to-end |
| 8 | `server.go` — keep `HTTPServer`, `StartServer`, `StartSender`, `serverState`, remove everything else | Everything above | **High** — last step, must import all new files |

### Pattern for extracted files

Each extracted file keeps `package beamsync` — no new sub-packages. Only file-level reorganization:

```go
// events.go
package beamsync

type EventCallback func(eventName string, data string)
type eventDispatcher struct { ... }
func newEventDispatcher(...) *eventDispatcher { ... }
func startWatchdog(...) { ... }
func safeEmit(...) { ... }
func logTransfer(...) { ... }
```

```go
// progress.go
package beamsync

type progressWriter struct {
    w           io.Writer
    total       int64
    written     int64
    filename    string
    emit        func(string, string)
    lastEmit    time.Time
    minInterval time.Duration
    direction   string  // "upload" or "download" — replaces the duplicate struct
}

func newProgressWriter(...) *progressWriter { ... }
func (pw *progressWriter) Write(p []byte) (int, error) { ... }
```

### Kill global singleton

Replace `var defaultEventDispatcher` in `server.go:81` with instance-level dispatchers:
- `HTTPServer` should hold its own `*eventDispatcher`
- `StartServer`/`StartSender` create dispatcher and pass to the server struct

Impact: changes `safeEmit` signature from using `defaultEventDispatcher` to accepting a dispatcher parameter. All callers in `StartServer`/`StartSender` pass the server's dispatcher.

```go
type HTTPServer struct {
    dispatcher *eventDispatcher
    // ...
}
```

---

## 2. Organizing Handlers, Middleware, Services

### Recommended structure within `beamsync/`

```
beamsync/
├── server.go                    # HTTPServer struct, StartServer(), StartSender()
├── rate_limiter.go              # clientRateLimiter
├── middleware.go                 # tokenMiddleware, rateLimitMiddleware, CORS, headers
├── events.go                    # eventDispatcher, watchdog, safeEmit, logTransfer
├── progress.go                  # progressWriter (merged direction-aware)
├── upload_handler.go            # write pipeline, writeFileToDisk
├── io_helpers.go                # copyChunked, chunkBufferPool
├── helpers.go                   # generateID, sha256Hex, clientIP, autoRenamePath, processManifest
├── auth_tokens.go               # (already separate)
├── permissions.go               # (already separate)
├── history.go                   # (already separate)
├── stats.go                     # (already separate)
├── tls.go                       # (already separate)
├── port_manager.go              # (already separate)
├── firewall.go                  # (already separate)
├── resumable_upload.go          # (already separate)
└── ui/                          # (already separate)
```

### Handler wiring pattern

The `StartServer` and `StartSender` functions are handler factories. Extract the inline closures into named functions that receive dependencies via parameters:

**Before:**
```go
mux.HandleFunc("/heartbeat", rateLimitMiddleware(heartbeatLimiter, settings, tokenMiddleware(tokens, tokenScopeSession, false, func(w http.ResponseWriter, r *http.Request) {
    // 20 lines of inline logic
})))
```

**After:**
```go
func handleHeartbeat(state *serverState, tokens *tokenStore, emit func(string, string)) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // 20 lines
    }
}

// In StartServer:
mux.HandleFunc("/heartbeat", rateLimitMiddleware(heartbeatLimiter, settings,
    tokenMiddleware(tokens, tokenScopeSession, false, handleHeartbeat(state, tokens, emit))))
```

This pattern keeps each handler testable in isolation without needing to spin up `StartServer`.

### Handler naming convention

```
beamsync/
├── handlers.go                  # All handler constructors (one per endpoint)
```

Or keep handlers close to their domain:
- `upload_handler.go` → `handleUpload`, `handleResumableUpload`, `handleRequestTransfer`
- `download_handler.go` → `handleDownload` (or inline in `server.go` sender section)
- `heartbeat_handler.go` → `handleHeartbeat`
- `page_handler.go` → `handleRootPage`, `handleDownloadPage`, `handleLogo`

---

## 3. Event-Driven Go Code

### Current state

Events flow: `progressWriter.Write()` → `safeEmit(callback, ...)` → `defaultEventDispatcher.queue ←` → goroutine runs `EventCallback` → `app.go:229` pushes to `eventChan (cap 100)` → `processEvents goroutine` → `runtime.EventsEmit()` → Svelte `EventsOn`.

Three serialized queues with possible drops at each:

```
progressWriter (per-write emit)
  → safeEmit → defaultEventDispatcher (cap 256, non-blocking send, drops on full)
    → app.makeCallback → eventChan (cap 100, non-blocking select, drops on full)
      → processEvents goroutine → runtime.EventsEmit → Svelte frontend
```

### Risk: silent drops at two levels

- `defaultEventDispatcher.emit()` drops when queue is full (line 76-78 of server.go)
- `makeCallback()` drops when `eventChan` is full (line 234-236 of app.go)

Under burst load (e.g., 50 files at once), `device_connected` or intermediate progress events can be silently dropped. The only signal is a `fmt.Printf`.

### Suggested improvements

**Short-term: increase buffer sizes**
- `eventDispatcher` buffer: 256 → 1024
- `eventChan` buffer: 100 → 500

**Medium-term: add drop counters**
```go
// Add to eventDispatcher
type eventDispatcher struct {
    queue      chan eventDispatchJob
    dropped    atomic.Int64
}
// Expose for health checks
func (d *eventDispatcher) DroppedCount() int64 { return d.dropped.Load() }
```

**Long-term: reduce serial queues**
- Remove `defaultEventDispatcher` — move dispatch into `eventChan` directly.
- `progressWriter` calls `makeCallback` directly instead of going through the dispatcher.
- The dispatcher was useful if multiple callers needed fan-in; if there's one caller (App), it's an unnecessary hop.

### Progress writer consolidation

Merge `progressWriter` + `downloadProgressWriter`:
```go
type progressWriter struct {
    w           io.Writer
    total       int64
    written     int64
    filename    string
    emit        func(string, string)
    lastEmit    time.Time
    minInterval time.Duration
    eventName   string // "upload_progress" or "download_progress"
}
```

---

## 4. Component Decomposition for App.svelte (2160 lines)

### Current structure breakdown

| Section | Lines | Concern |
|---------|-------|---------|
| Imports + state declarations | 1–147 | All imports, all state variables |
| Utility functions | 148–252 | `toast`, `addSessionEntry`, `normalizeTransferStats`, `formatDuration`, etc. |
| Event handlers + lifecycle | 254–492 | `onMount`, `onDestroy`, all `EventsOn` callbacks |
| Action functions | 494–845 | `initReceiver`, `switchMode`, `startSend`, `sendSelectedFiles`, `generateQR`, settings CRUD, drag-and-drop |
| Template — transfer request modal | 855–888 | Modal for incoming transfer approval |
| Template — main layout | 890–1335 | Splash, app shell, all 4 modes, progress overlay |
| Styles | 1337–2160 | Component CSS |

### Extraction plan

| Step | New component | What to extract | Lines removed from App.svelte | Risk |
|------|---------------|-----------------|-------------------------------|------|
| 1 | `ReceiveView.svelte` | Receive mode template + state (QR, connection state, file list, instructions) | ~1000–1080, plus related state vars | **Low** — self-contained view with clear props interface |
| 2 | `SendView.svelte` | Send mode template + state (sender dialog, QR, file list) | ~1082–1141, plus related state vars | **Low** — self-contained view |
| 3 | `SettingsView.svelte` | Settings mode template + handlers (transfer mode, devices, blocked extensions, sounds, save path) | ~1142–1250, plus related state + handlers | **Medium** — many handler callbacks pass back up; need clear event interface |
| 4 | `AboutView.svelte` | About mode template | ~1251–1297 | **Low** — static view |
| 5 | `ToastManager.svelte` | Toast state + template | 144–154, 912–916 | **Low** — standalone concern |
| 6 | `UpdateBanner.svelte` | Update banner | 133–136, 918–937 | **Low** — standalone concern |
| 7 | `ProgressOverlay.svelte` | Global floating progress bar | 1311–1334 | **Low** — displays computed data |
| 8 | `DragDropManager.svelte` or svelte:window handlers in App.svelte | Drag/drop handlers (keep in App shell since they're global) | 778–841 | **Medium** — needs coordination with both Receive and Send views |

### After extraction

```
desktop/frontend/src/
├── App.svelte                    # Thin shell: routing, global drag-drop, event binding, state owner
├── main.js
├── app.css
├── views/
│   ├── ReceiveView.svelte        # QR, connection state, file list
│   ├── SendView.svelte           # Sender dialog, QR, file list
│   ├── SettingsView.svelte       # All settings forms
│   └── AboutView.svelte          # Version, links
├── components/
│   ├── ProgressOverlay.svelte    # Floating progress bar
│   ├── TransferRequestModal.svelte # Incoming transfer approval modal
│   ├── ToastManager.svelte       # Toast notification system
│   └── UpdateBanner.svelte       # Update available banner
├── lib/
│   └── utils.js                  # formatDuration, formatSize, fileIcon, isValidIPv4, etc.
├── design-system/                # Existing design system components
│   ├── tokens.css
│   ├── TopNavBar.svelte
│   ├── ...
```

### Shared state pattern

Keep all shared state in `App.svelte` and pass down as props + events for now. If state management becomes unwieldy, consider Svelte stores (`writable` stores). The current pattern (parent owner → pass down) is fine for a single-page app with 4 views.

**Props interface for ReceiveView:**
```svelte
<ReceiveView
  {connectionState}
  {qrImage}
  {displayUrl}
  {receivedFiles}
  {transferStats}
  {transferStatsNow}
  {transferSpeeds}
  {activeSpeedDirection}
  {transferHistory}
  {sessionLog}
  on:changeSavePath
  on:disconnectReset
/>
```

**Events back up:**
- `changeSavePath` → App calls `SetSavePath()`
- `disconnectReset` → App calls `resetAll()` + `initReceiver()`
- `openFile(name)` → App calls `OpenFile()`

---

## 5. Shared State: Go Backend ↔ Svelte Frontend (Wails Bind)

### Current pattern

```
Go App method (Wails Bind)
  → returns string/struct directly to frontend
  → synchronous call via import from "../wailsjs/go/main/App.js"

Events (async push from Go to frontend):
  → Go: runtime.EventsEmit(ctx, eventName, data)
  → JS: EventsOn(eventName, callback)
```

### Strengths
- Wails Bind handles serialization automatically (structs → JSON)
- Events channel pattern works for async progress
- `makeCallback` → `eventChan` → `processEvents` → `EventsEmit` keeps UI updates off the event producer goroutines

### Weaknesses
- Version string duplicated (Go: `app.go:29`, JS: `App.svelte:944`)
- URL format string `fmt.Sprintf("%s://%s:%s/?token=%s", ...)` duplicated 6+ times in `app.go`
- Event names (strings) are magic — no type safety between Go and frontend
- `makeCallback` drops events silently when `eventChan` is full

### Recommendations

**Single version source:**
- Remove `const currentVersion` from `app.go`
- Remove hardcoded `"v2.2"` from `App.svelte`
- Read version from `wails.json` at build time (Wails provides `app.GetVersion()` or similar). Common pattern:
  ```go
  // In app.go
  //go:embed wails.json
  var wailsConfig []byte
  
  var appVersion string // initialized in startup from wails.json
  ```

**URL helper:**
```go
// In desktop/app.go
func (a *App) shareURL(port, token string) string {
    return fmt.Sprintf("%s://%s:%s/?token=%s", beamsync.ServerScheme(), a.currentIP, port, token)
}
```

**Event name constants:**
```go
// In beamsync/events.go (after extraction)
const (
    EventDeviceConnected    = "device_connected"
    EventDeviceDisconnected = "device_disconnected"
    EventUploadProgress     = "upload_progress"
    EventDownloadProgress   = "download_progress"
    EventTransferRequest    = "transfer_request"
    EventTransferLogged     = "transfer_logged"
    EventTransferStats      = "transfer_stats"
    EventFileReceived       = "file_received"
    EventURLChanged         = "url_changed"
    EventSenderStarted      = "sender_started"
    EventSenderFiles        = "sender_files"
    EventUpdateAvailable    = "update_available"
    EventTransferTimeout    = "transfer_request_timeout"
)
```

**App.go extraction:**

`desktop/app.go` (734 lines) could also benefit from splitting:

| Lines | Concern | Extraction |
|-------|---------|------------|
| 75–121 | Config persistence | `config.go` |
| 122–156 | Startup (audio init) | Keep in `app.go` |
| 159–237 | Event bridge (`processEvents`, `safeEmit`, `makeCallback`) | `events_bridge.go` |
| 240–488 | Wails-bound bridge methods | Keep in `app.go` |
| 576–621 | Transfer permission methods | `permissions_bridge.go` |
| 626–697 | Update checker | `updater.go` |
| 700–734 | Helpers (IP monitor) | Keep in `app.go` |

But this is lower priority than server.go and App.svelte. Only extract if working on related concerns.

---

## Refactoring Risk Assessment

### Phase 1: Low risk (4 files, no behavior change)

| Refactoring | Risk | Effort | Tests exist? |
|-------------|------|--------|-------------|
| Extract `helpers.go` | Low | 15 min | Partial (sha256Hex tested indirectly) |
| Extract `rate_limiter.go` | Low | 15 min | Yes (rate_limiter_test.go) |
| Extract `progress.go` (merge writers) | Low | 20 min | No (untested) |
| Extract `io_helpers.go` | Low | 10 min | No (untested) |
| Extract `middleware.go` | Low | 15 min | No (tested via server_test.go) |

### Phase 2: Medium risk (needs integration test pass)

| Refactoring | Risk | Effort | Notes |
|-------------|------|--------|-------|
| Extract `events.go` | **Medium** | 30 min | Must update all `safeEmit` callers + replace global dispatcher |
| Extract `upload_handler.go` | **Medium** | 30 min | Many implicit dependencies on server state |
| Extract handlers into named functions | **Medium** | 45 min | Changes the closure pattern; verify all routes still work |

### Phase 3: Frontend extraction

| Refactoring | Risk | Effort | Notes |
|-------------|------|--------|-------|
| Extract `ToastManager` | Low | 20 min | Standalone |
| Extract `ProgressOverlay` | Low | 15 min | Standalone |
| Extract `ReceiveView` | **Medium** | 1 hr | Many state variables need re-routing through props |
| Extract `SendView` | **Medium** | 1 hr | Same pattern |
| Extract `SettingsView` | **Medium** | 1 hr | Complex bidirectional state (settings CRUD) |
| Extract `AboutView` | Low | 15 min | Static |

---

## Preconditions

Before refactoring server.go:

1. **Write tests for untested code first** — `progressWriter`, `eventDispatcher`, `safeEmit`, `copyChunked` have no unit tests. Adding them before extraction catches regressions.
2. **Freeze feature work** on the refactored files until all extractions land. Concurrent feature branches touching server.go will create merge conflicts.
3. **Extract in the order above** — dependency order matters. Rate limiter has no deps on other files; events.go depends on nothing new. Extract those first.
4. **Use `git mv` for renamed files** — keep git history continuous.
5. **Run `go test ./...` after each extraction** to verify no accidental breakage.

---

## Summary

| File | Lines | Extraction targets | Suggested order |
|------|-------|-------------------|-----------------|
| `beamsync/server.go` | 1726 | 9 files (helpers, rate_limiter, progress, io_helpers, middleware, events, upload_handler, handlers, trim server.go) | Phase 1 → Phase 2 |
| `desktop/app.go` | 734 | 3 files (config, events_bridge, updater) | Phase 4 (lower priority) |
| `desktop/frontend/src/App.svelte` | 2160 | 7 components (ReceiveView, SendView, SettingsView, AboutView, ToastManager, ProgressOverlay, UpdateBanner) + utils.js | Phase 3 |

All extractions are file-level reorganizations — no structural changes to `package beamsync`. The goal is readability and testability without introducing interface abstractions or sub-packages.
