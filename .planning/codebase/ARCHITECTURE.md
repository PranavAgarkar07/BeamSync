<!-- refreshed: 2026-07-20 -->
# Architecture

**Analysis Date:** 2026-07-20

## System Overview

BeamSync is a cross-platform desktop application for offline-first peer-to-peer file transfer over LAN. It is structured as a **Go workspace** with two modules: a core library (`beamsync/`) and a Wails desktop shell (`desktop/`). No external databases or cloud services are used — all state is in-memory with JSON config persistence.

```text
┌──────────────────────────────────────────────────────────────────┐
│              DESKTOP UI (Svelte 3 + Vite)                        │
│  `desktop/frontend/src/App.svelte` + `design-system/*.svelte`    │
│  Wails JS bridge calls -> Go App methods                         │
└──────────────────────────┬───────────────────────────────────────┘
                           │ Wails IPC (WebSocket/HTTP)
                           ▼
┌──────────────────────────────────────────────────────────────────┐
│              APPLICATION SHELL (Go + Wails v2)                   │
│  `desktop/main.go` → `desktop/app.go`                            │
│  App struct: lifecycle, config, event bridge, audio              │
│  Exposes Go methods to frontend via Wails Bind                   │
├────────────────┬────────────────┬───────────────────────────────┤
│  StartReceiver │  StartSender   │  Event Callback Bridge         │
│  `app.go:296`  │  `app.go:357`  │  `makeCallback()` + channel    │
└────────┬───────┴───────┬────────┴───────────────────────────────┘
         │               │
         ▼               ▼
┌──────────────────────────────────────────────────────────────────┐
│              CORE NETWORK ENGINE (Go net/http)                   │
│  `beamsync/server.go`                                            │
│  StartServer() — receiver mode HTTP file server                  │
│  StartSender() — sender mode HTTP file server                    │
├──────────────┬───────────────────┬────────────────┬──────────────┤
│  Auth Tokens │  Rate Limiting    │  TLS (opt)     │  Resumable   │
│  `auth_tok..`│  `server.go:203`  │  `tls.go`      │  `resumable..`│
├──────────────┼───────────────────┼────────────────┼──────────────┤
│  Permissions │  Transfer History │  Stats         │  Firewall    │
│  `perm..go`  │  `history.go`     │  `stats.go`    │  `firewall.go`│
└──────────────┴───────────────────┴────────────────┴──────────────┘
         │
         ▼
┌──────────────────────────────────────────────────────────────────┐
│  EMBEDDED MOBILE WEB UI (HTML/CSS/JS served by Go server)        │
│  `beamsync/ui/upload.html` — mobile upload page                  │
│  `beamsync/ui/download.html` — mobile download page              │
│  Clients connect via browser (QR code or URL)                    │
└──────────────────────────────────────────────────────────────────┘
```

## Component Responsibilities

| Component | Responsibility | File |
|-----------|----------------|------|
| **App Shell** | Lifecycle, config persistence, Wails event bridge, audio | `desktop/app.go` |
| **Wails Entry** | Window creation, Go↔JS binding, drag-and-drop | `desktop/main.go` |
| **Receiver Server** | HTTP server that accepts file uploads from mobile clients | `beamsync/server.go:770` |
| **Sender Server** | HTTP server that serves file downloads to mobile clients | `beamsync/server.go:1364` |
| **Auth Token Store** | HMAC-signed session tokens with scope, expiry, IP binding | `beamsync/auth_tokens.go` |
| **Rate Limiter** | Per-client token-bucket rate limiting for all endpoints | `beamsync/server.go:167` |
| **TLS Layer** | Self-signed ECDSA certificate generation & optional HTTPS | `beamsync/tls.go` |
| **Permissions** | Transfer mode, device trust/block lists, file extension blocks | `beamsync/permissions.go` |
| **Transfer History** | Ring-buffer transfer records with in-memory storage | `beamsync/history.go` |
| **Transfer Stats** | Session-level transfer statistics tracking | `beamsync/stats.go` |
| **Port Manager** | Dynamic port scanning with step-based allocation | `beamsync/port_manager.go` |
| **Firewall Setup** | Linux firewall rule automation via pkexec | `beamsync/firewall.go` |
| **Resumable Upload** | Chunked upload with SHA-256 integrity per chunk | `beamsync/resumable_upload.go` |
| **Audio Engine** | Native WAV playback using beep/speaker | `beamsync/audio/audio.go` |
| **Mobile Upload UI** | Embedding web page served by receiver for mobile browsers | `beamsync/ui/upload.html` |
| **Mobile Download UI** | Embedding web page served by sender for mobile browsers | `beamsync/ui/download.html` |
| **Desktop UI** | Svelte 3 app with neubrutalism design system | `desktop/frontend/src/App.svelte` |
| **Design System** | CSS custom properties, UI components (card, button, badge, etc.) | `desktop/frontend/src/design-system/` |
| **Community Site** | Astro-based static site for community | `community/` |

## Pattern Overview

**Overall:** Two-server architecture — one per transfer direction — each running an embedded HTTP stack. The desktop app acts as a bridge between the Wails Svelte UI and the Go HTTP engine.

**Key Characteristics:**
- **No external dependencies** — No database, no cloud, no internet required
- **Event-driven IPC** — Go HTTP server emits events through callback → channel → Wails runtime → Svelte UI
- **Embedded web UI** — Mobile clients never install an app; they load HTML/JS served directly by the Go HTTP server
- **HMAC token auth** — All endpoints protected by short-lived, client-bound, scope-scoped HMAC tokens
- **Concurrent file I/O** — 3-worker goroutine pool for small files, synchronous streaming for large files with 8MB chunk buffers
- **In-memory state** — Config persisted to `~/.config/beamsync/config.json`, everything else (tokens, history, stats) is in-memory

## Layers

**Desktop UI Layer:**
- Purpose: User interface for starting/stopping transfers, displaying QR codes, showing progress
- Location: `desktop/frontend/src/`
- Contains: Svelte components, CSS design system, static assets
- Depends on: Wails runtime JS bridge (`wailsjs/go/main/App.js`)
- Used by: End user (desktop app window)

**Application Shell Layer:**
- Purpose: Desktop app lifecycle, config persistence, event routing, audio
- Location: `desktop/app.go`, `desktop/main.go`
- Contains: `App` struct with Wails-bound methods, event processing goroutine, IP monitor goroutine
- Depends on: `beamsync` core library, `beamsync/audio`, Wails v2
- Used by: Desktop UI (via Wails Bind)

**Network Engine Layer:**
- Purpose: HTTP file transfer servers with auth, rate limiting, TLS
- Location: `beamsync/server.go`
- Contains: HTTP handlers, middleware (token, rate-limit), concurrent write pipeline, watchdog
- Depends on: `auth_tokens.go`, `permissions.go`, `history.go`, `stats.go`, `tls.go`, `port_manager.go`, `firewall.go`
- Used by: Mobile browsers (HTTP clients)

**Mobile Web UI Layer:**
- Purpose: Browser-based upload/download interface for mobile devices
- Location: `beamsync/ui/` (embedded via Go `embed.FS`)
- Contains: Standalone HTML pages with inline CSS/JS, served by the Go HTTP server
- Depends on: Network Engine (serves the HTML)
- Used by: Mobile device browsers (scan QR code)

## Data Flow

### Primary Request Path — Receiver Mode (mobile → desktop)

1. User launches BeamSync → `App.startup()` loads config, initializes audio, starts IP monitor
2. `StartReceiverDefault()` calls `beamsync.StartServer()` → creates HTTP server with auth tokens
3. QR code URL generated and displayed in Svelte UI: `http://<ip>:<port>/?token=<bootstrap>`
4. Mobile device scans QR (or opens URL) → loads `beamsync/ui/upload.html` from embedded FS
5. Bootstrap token exchanged for session token; heartbeat loop begins
6. Mobile selects files, calls `/request-transfer` → server checks permissions mode, emits `transfer_request` event
7. Desktop user approves/rejects via `ApproveTransfer()`/`RejectTransfer()` in Svelte UI
8. If approved, mobile POSTs multipart data to `/upload` → server parses parts, dispatches to write workers
9. Progress events flow: `progressWriter` → emit → `eventDispatcher` → `App.eventChan` → `processEvents()` → `EventsEmit` → Svelte UI

### Secondary Flow — Sender Mode (desktop → mobile)

1. User clicks "Send" or drags files → `StartSender()` or `SendFiles()` called
2. `beamsync.StartSender()` creates HTTP server with download endpoints
3. QR code displayed; mobile loads `beamsync/ui/download.html`
4. Each file gets a single-use `tokenScopeTransfer` token
5. Mobile clicks download → `/download?token=<transfer>` → server streams file with progress tracking
6. `downloadProgressWriter` emits events → same event chain as receiver

### State Management:
- All state is in-memory within Go structs (no database)
- Config persisted as JSON: `~/.config/beamsync/config.json`
- Transfer history: ring buffer in `TransferHistory` (`beamsync/history.go`)
- Auth tokens: in-memory map in `tokenStore` (`beamsync/auth_tokens.go`)
- Rate limiter state: in-memory map in `clientRateLimiter` (`beamsync/server.go`)
- Transfer stats: in-memory in `transferStatsTracker` (`beamsync/stats.go`)

## Key Abstractions

**`HTTPServer`:**
- Purpose: Wraps `*http.Server` with transfer settings, history, stats, token store, and pending transfer management
- Location: `beamsync/server.go:297`
- Pattern: Aggregate struct that references all sub-components; exposes `Shutdown()`, `RespondToTransfer()`, `Stats()`, `Settings()`, `TransferHistory()`

**`eventDispatcher`:**
- Purpose: Buffered channel-based event queue that serializes callback invocations with panic recovery
- Location: `beamsync/server.go:39`
- Pattern: Goroutine + channel fan-in with `recover()` per event

**`tokenStore`:**
- Purpose: Issues and validates HMAC-signed tokens with scope, IP binding, and single-use semantics
- Location: `beamsync/auth_tokens.go:44`
- Pattern: In-memory map + HMAC-SHA256 signing, with periodic cleanup goroutine

**`TransferSettings`:**
- Purpose: User-customizable transfer permission rules (mode, blocked extensions, trusted/blocked devices)
- Location: `beamsync/permissions.go:22`
- Pattern: Value object with query methods, persisted as JSON

**`progressWriter` / `downloadProgressWriter`:**
- Purpose: `io.Writer` wrappers that emit progress events at throttled intervals during file I/O
- Location: `beamsync/server.go:360`, `:384`
- Pattern: Decorator over `io.Writer` with adaptive emit interval

**`AudioEngine`:**
- Purpose: Native WAV playback using `faiface/beep` with named sound bank
- Location: `beamsync/audio/audio.go:15`
- Pattern: Map of pre-decoded `beep.Buffer` instances, `speaker.Play()` for playback

## Entry Points

**Desktop Application:**
- Location: `desktop/main.go:14`
- Triggers: User launches the native desktop binary
- Responsibilities: Creates `App` instance, configures Wails window, binds `App` methods to frontend

**Backend Event Processing:**
- Location: `desktop/app.go:159` (`processEvents()`)
- Triggers: Started as goroutine in `startup()`
- Responsibilities: Dequeues events from channel, detects IP changes, forwards to Wails runtime

**HTTP Receiver Server:**
- Location: `beamsync/server.go:770` (`StartServer()`)
- Triggers: Called from `App.StartReceiverDefault()` or `App.StartReceiver()`
- Responsibilities: Creates HTTP server with all handlers, middleware, watchdog, returns URL and token

**HTTP Sender Server:**
- Location: `beamsync/server.go:1364` (`StartSender()`)
- Triggers: Called from `App.StartSender()` or `App.SendFiles()`
- Responsibilities: Creates HTTP server with download endpoints, returns URL and token

## Architectural Constraints

- **Threading:** Single-threaded event loop via Go goroutines + channels. HTTP server runs on its own goroutine; event dispatch on one goroutine; watchdog on one goroutine; 3 write worker goroutines for small files; IP monitor on one goroutine.
- **Global state:** Package-level `defaultEventDispatcher` singleton in `beamsync/server.go:81`. `chunkBufferPool` sync.Pool at `beamsync/server.go:88`.
- **Circular imports:** None. `desktop` depends on `beamsync`; `beamsync` has no external project dependencies.
- **Embedded assets:** UI HTML and images compiled into Go binary via `//go:embed ui/*.html ui/*.png`. Sound WAV files via `//go:embed sounds/*.wav` in desktop module.
- **No database:** All state is ephemeral in-memory. Only config is persisted to disk as JSON.

## Anti-Patterns

### Package-level singleton dispatcher

**What happens:** `defaultEventDispatcher` is a package-level variable in `beamsync/server.go:81`, created at init time. All `StartServer` and `StartSender` calls share this single dispatcher. If multiple servers are created concurrently (possible via the desktop shell), events compete for the same buffer.

**Why it's wrong:** Limits isolation between multiple server instances. The buffer of 256 entries is shared across all instances.

**Do this instead:** Pass the dispatcher as a parameter to `StartServer`/`StartSender`, or have each `HTTPServer` hold its own dispatcher instance.

### In-memory tokens with no persistence

**What happens:** All issued tokens are stored in an in-memory map. If the server restarts, all active sessions are invalidated. Mobile clients must re-scan the QR code.

**Why it's wrong:** Session continuity is lost on restart. The 5-minute TTL mitigates this but doesn't eliminate the UX interruption.

**Do this instead:** Accept this as a deliberate trade-off (no database dependency). The restart scenario is rare in normal desktop usage.

### Monolithic server.go

**What happens:** `beamsync/server.go` contains ~1700 lines encompassing server startup, HTTP handlers, middleware, rate limiter, progress writers, write pipeline, event dispatcher, watchdog, and utilities.

**Why it's wrong:** Low cohesion — the file mixes concerns that could be separate packages (rate limiter, progress tracking, event dispatch).

**Do this instead:** Split rate limiter into its own file (partial — some rate limiter code is at the bottom of `server.go`), progress writers into `progress.go`, event dispatcher into `events.go`.

## Error Handling

**Strategy:** Defensive — panic recovery at all goroutine boundaries (`recover()` in server handler, watchdog, event dispatcher, write workers). Errors are logged to stdout; HTTP errors return appropriate status codes.

**Patterns:**
- Panic recovery with `debug.Stack()` on server handler goroutines
- `safeEmit()` drops events when dispatch queue is full instead of blocking
- `writeFileToDisk` handles file I/O errors and logs transfer failures to history
- All external API calls (GitHub update check) have timeouts and degrade gracefully

## Cross-Cutting Concerns

**Logging:** Simple `fmt.Printf` to stdout with emoji prefixes (🚀, ✅, ❌, ⚠️, 💚). No structured logging library.

**Validation:** Input validation at trust boundaries — token validation in middleware, multipart parsing, Content-Type checking, filename sanitization, Content-Range validation for resumable uploads.

**Authentication:** Three-tier token scopes — `bootstrap` (single-use, QR code), `session` (renewable via heartbeat), `transfer` (single-use per download). HMAC-SHA256 signed with per-server secret.

---

*Architecture analysis: 2026-07-20*
