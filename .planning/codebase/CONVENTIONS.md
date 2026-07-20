# Coding Conventions

**Analysis Date:** 2026-07-20

## Languages

| Language | Location | Scope |
|----------|----------|-------|
| Go | `beamsync/`, `desktop/` (except `frontend/`) | Core library + Wails desktop app |
| JavaScript (Svelte 3) | `desktop/frontend/src/` | Wails frontend UI |
| Astro | `community/` | Marketing / community site |

---

## Go Conventions

### Naming Patterns

**Files:**
- Snake case with single-word package names matching directory: `beamsync/server.go`, `beamsync/auth_tokens.go`, `beamsync/rate_limiter_test.go`
- Desktop package is `main` in `desktop/main.go` and `desktop/app.go`

**Functions:**
- PascalCase for exported functions (`StartServer`, `StartSender`, `FindAvailablePort`, `ServerScheme`)
- camelCase for unexported functions (`newEventDispatcher`, `startBackgroundPruner`, `safeEmit`, `logTransfer`, `generateID`)

**Variables:**
- camelCase for local variables (`savePath`, `startPort`, `portStr`)
- Short names for local scope (`n`, `err`, `buf`, `wg`)
- Unexported package-level vars use camelCase (`defaultEventDispatcher`, `chunkBufferPool`, `writeWorkerCount`)

**Types:**
- PascalCase for exported types (`TransferSettings`, `HTTPServer`, `PendingTransfer`, `TransferRecord`, `TransferHistory`)
- PascalCase for unexported types (`serverState`, `clientRateLimiter`, `tokenStore`, `eventDispatcher`)
- Sentinel errors use `err` prefix with PascalCase (`errInvalidToken`, `errExpiredToken`, `errUsedToken`, `errWrongClient`)

**Constants:**
- PascalCase for exported constants (`TransferModeAcceptAll`, `TransferDirectionReceive`, `TransferStatusCompleted`)
- camelCase for unexported constants (`defaultTokenTTL`, `rateLimitPruneInterval`, `largeFileThreshold`, `writeWorkerCount`, `eventDispatcherBufferSize`)

### Code Style

**Formatting:**
- `gofmt` enforced — CI blocks PRs on unformatted Go files (`test -z "$(gofmt -l .)"`)
- No manual formatting decisions; all Go code matches `gofmt` output exactly

**Vetting:**
- `go vet ./...` runs in CI for both `beamsync/` and `desktop/`

**Imports:**
- Standard library imports grouped first, then third-party
- Blank imports for embedded assets (`_ "embed"`)
- No import aliases used

### Error Handling

**Patterns:**
```go
// Sentinel errors defined as package-level vars
var errInvalidToken = errors.New("invalid token")

// Error wrapping with %w for sentinel comparison
return nil, fmt.Errorf("generate token secret: %w", err)

// Return errors from functions; caller checks
func (s *tokenStore) validate(value, clientIP string, scope tokenScope, consume bool) error {
}
if err := tokens.validate(...); err != nil {
    http.Error(w, "403 Forbidden: invalid token", http.StatusForbidden)
    return
}
```

**Panic Recovery:**
- Goroutines that can panic are wrapped with defer/recover:
```go
defer func() {
    if r := recover(); r != nil {
        fmt.Printf("Event callback panic: %v\n", r)
    }
}()
```
- Used in: `eventDispatcher.run()`, `startWatchdog`, `StartServer`, upload handler, `safeEmit`

**Logging:**
- `fmt.Printf` and `fmt.Println` for all logging (no structured logging library)
- Emoji prefixes for categorizing messages: `❌` errors, `✅` success, `⚠️` warnings, `💓` heartbeat, `📤` sender, etc.
- No log levels — all output goes to stdout

### Function Design

**Size:**
- Small focused functions preferred (e.g., `generateID`, `clientIP`, `hashMatches`, `sha256Hex` — under 10 lines)
- Larger orchestrator functions exist where necessary: `StartServer` (~590 lines), `StartSender` (~300 lines)

**Parameters:**
- Structs used to group related parameters (`writeJob`, `PendingTransfer`, `rateLimitDecision`)
- Callback functions passed as `EventCallback` type

**Return Values:**
- Multiple return values with `(value, error)` pattern standard
- Named returns used sparingly (e.g., `(wasConnected bool, timedOut bool)` in `checkTimeout`)

### Module Design

**Package structure** (`beamsync/`):
- Single `package beamsync` for all core library files
- Files separated by concern: `server.go`, `history.go`, `auth_tokens.go`, `permissions.go`, `stats.go`, `tls.go`, `port_manager.go`, `resumable_upload.go`, `firewall.go`
- Audio as sub-package: `beamsync/audio/audio.go`
- UI assets embedded via `//go:embed ui/*.html ui/*.png`

**Desktop** (`desktop/`):
- `package main` with `main.go` (entry point) and `app.go` (Wails App struct and bridge methods)
- Separated by section markers:
```go
// ── Concurrent write pipeline ─────────────────────────────────────────────────
// ── Heartbeat ────────────────────────────────────────────────────────────
// ── HELPERS ─────────────────────────────────────────────────────────────
```

### Struct Patterns

**Constructor functions:**
```go
func NewTransferHistory(maxEntries int) *TransferHistory
func newClientRateLimiter(limit int, window time.Duration) *clientRateLimiter
func newTokenStore(fingerprint string) (*tokenStore, error)
func newTransferStatsTracker() *transferStatsTracker
```

**Mutex-based concurrency safety:**
```go
type transferStatsTracker struct {
    mu     sync.Mutex
    // ... fields
}
func (t *transferStatsTracker) recordReceived(...) TransferStats {
    t.mu.Lock()
    defer t.mu.Unlock()
    // ...
}
```

**Default value constructors:**
```go
func DefaultTransferSettings() TransferSettings {
    return TransferSettings{
        Mode: TransferModeAskFirst,
        MaxFileSizeMB: 0,
        // ...
    }
}
```

---

## Frontend (Svelte) Conventions

### Naming

**Files:**
- PascalCase `.svelte` files: `App.svelte`, `TopNavBar.svelte`, `FileDropZone.svelte`, `SplashScreen.svelte`
- camelCase `.js` files: `main.js`, `vite.config.js`
- CSS files: `app.css`, `tokens.css`

**Functions/Variables:**
- camelCase for all JS identifiers (`activeTab`, `connectionState`, `qrImage`, `transferHistory`)
- Uppercase constants (`QR_GENERATION_DEBOUNCE_MS`, `TRANSFER_STATS_THROTTLE_MS`)
- Destructured imports from Wails bindings

### Import Organization

1. CSS imports
2. Wails bridge imports (`../wailsjs/go/main/App.js`, `../wailsjs/runtime/runtime.js`)
3. Third-party (`qrcode`, `svelte` modules)
4. Local components (`./design-system/index.js`)
5. Static assets (`../assets/images/icon.png`)

### Component Patterns

**Props:**
```svelte
export let activeTab     = 'receive';
export let networkStatus = 'idle';
export let serverUrl     = '';
```

**Events:**
```svelte
const dispatch = createEventDispatcher();
// ...
on:click={() => dispatch('tabChange', { tab: tab.id })}
```

**Scoped CSS:**
- Component styles inside `<style>` tags (Svelte-scoped by default)
- BEM-like naming: `.navbar__logo`, `.navbar__tab--active`
- CSS custom properties from `tokens.css` using `var(--nb-*)`

### CSS Architecture

**Global:** `app.css` — resets, scrollbar, toast, update banner
**Tokens:** `tokens.css` — design tokens (colors, fonts, spacing, shadows, borders) — the single source of truth
**Component-level:** Scoped `<style>` blocks using token variables

### Design System Classes

- `.nb-btn`, `.nb-btn--primary`, `.nb-btn--secondary`, `.nb-btn--ghost`, `.nb-btn--danger`
- `.nb-input`
- `.nb-card`, `.nb-card--interactive`
- `.nb-badge`, `.nb-badge--success`, `.nb-badge--danger`, etc.
- `.nb-mono`

### Linting / Formatting

- No ESLint or Prettier config files detected in the project
- `jsconfig.json` enables type checking (`checkJs: true`) but no auto-formatter enforced
- CI runs `npm run lint --if-present` (no-op since no lint script is configured)

---

## Astro (Community Site) Conventions

**Files:**
- PascalCase `.astro` components: `Hero.astro`, `Features.astro`, `BaseLayout.astro`
- Component `<style>` sections are global (Astro default — not scoped like Svelte)
- Static assets in `community/public/`
- Config in `astro.config.mjs`

---

*Convention analysis: 2026-07-20*
