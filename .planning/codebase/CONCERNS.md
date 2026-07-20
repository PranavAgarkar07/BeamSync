# Codebase Concerns

**Analysis Date:** 2026-07-20

## Tech Debt

### Monolithic server.go (1726 lines)

- **Issue:** `beamsync/server.go` contains HTTP server setup, upload handling, download handling, rate limiting, progress tracking, watchdog, event dispatching, file writing, and token validation — all in one file. This violates single responsibility and makes testing, reasoning about, and safely modifying any one concern difficult.
- **Files:** `beamsync/server.go`
- **Impact:** High-risk refactoring surface. Any change to one handler risks breaking others. The `StartServer()` function (lines 771-1360) alone is ~590 lines with deeply nested closures.
- **Fix approach:** Extract into separate files/structs: `upload_handler.go`, `download_handler.go`, `middleware.go`, `progress.go`, `rate_limiter.go`. Keep `server.go` as the orchestration entry point.

### Monolithic App.svelte (2160 lines)

- **Issue:** `desktop/frontend/src/App.svelte` contains all frontend state management, event binding, QR generation, drag-and-drop, settings UI, transfer history, progress tracking, and session logging — in one Svelte component.
- **Files:** `desktop/frontend/src/App.svelte`
- **Impact:** Hard to maintain, test, or reason about individual features. Component re-renders affect the entire app.
- **Fix approach:** Split into smaller components per concern: `TransferProgress.svelte`, `SettingsPanel.svelte`, `DevicePanel.svelte`, `TransferHistory.svelte`. Keep App.svelte as a thin shell.

### Duplicate progressWriter types

- **Issue:** `progressWriter` and `downloadProgressWriter` (`beamsync/server.go` lines 360-404) are nearly identical structs with the same fields and Write method. Only the event name differs.
- **Files:** `beamsync/server.go` (lines 360-404)
- **Impact:** Code duplication — changes must be made in sync across both types.
- **Fix approach:** Merge into a single `progressWriter` with a `direction` string field that selects "upload_progress" vs "download_progress".

### Inconsistent version strings

- **Issue:** The app version is hardcoded in two places with different values: `const currentVersion = "v2.4.0"` in `desktop/app.go` (line 29) and `appVersion="v2.2"` in `desktop/frontend/src/App.svelte` (line 944).
- **Files:** `desktop/app.go:29`, `desktop/frontend/src/App.svelte:944`
- **Impact:** Confusing to users. The version in the About panel may not match the actual release version.
- **Fix approach:** Derive the version from a single source (e.g., `wails.json` productVersion or a build-time injection) and pass it to the frontend via a bridge call.

### Duplicate share mode URL building logic

- **Issue:** The URL construction pattern `fmt.Sprintf("%s://%s:%s/?token=%s", beamsync.ServerScheme(), localIP, port, token)` is repeated in at least 6 places in `desktop/app.go` (e.g., lines 290, 321, 351, 380, 433).
- **Files:** `desktop/app.go:290,321,351,380,433`
- **Impact:** If the URL format changes (e.g., new query parameter), all callers must be updated. Easy to miss one.
- **Fix approach:** Create a helper method `func (a *App) shareURL(port, token string) string` on the App struct.

### Hardcoded version in frontend

- **Issue:** `appVersion="v2.2"` is hardcoded in the Svelte template at `App.svelte:944`, disconnected from the Go backend's `currentVersion`.
- **Files:** `desktop/frontend/src/App.svelte:944`
- **Impact:** Version shown to the user can drift from the actual binary version.
- **Fix approach:** Fetch version from the backend via a bridge method call.

### In-memory-only transfer history

- **Issue:** `beamsync/history.go` stores all transfer history in a ring buffer in memory. Restarting the app loses all history.
- **Files:** `beamsync/history.go`
- **Impact:** Users cannot review past transfer activity after restart.
- **Fix approach:** Persist history to disk (JSON file in config dir) on each new record, load on startup.

### Firewall script duplicated across codebase

- **Issue:** The firewall shell script exists in three locations: embedded in `beamsync/firewall.go`, on disk at `beamsync/firewall_setup.sh`, and at `build/linux/firewall_setup.sh`.
- **Files:** `beamsync/firewall.go`, `beamsync/firewall_setup.sh`, `build/linux/firewall_setup.sh`
- **Impact:** Scripts can drift; modifications must be made in multiple places.
- **Fix approach:** Use the embedded script as the single source of truth; delete the on-disk copies from the library package.

### Duplicate file info gathering in sender flow

- **Issue:** The `StartSender()` and `SendFiles()` methods in `desktop/app.go` both gather file metadata (name, size) and emit `sender_files` events with nearly identical code blocks (lines 392-407 and 444-459).
- **Files:** `desktop/app.go:392-407, 444-459`
- **Impact:** Code duplication — logical changes to file info gathering must be applied in two places.
- **Fix approach:** Extract a helper `gatherFileEntries(paths []string) []fileEntry` and reuse it.

## Security Considerations

### Firewall script launched with pkexec from temp directory

- **Risk:** `RunFirewallSetup()` in `beamsync/firewall.go` writes an embedded shell script to a world-readable temp directory and executes it with `pkexec` (root). A race condition exists where a local attacker could replace the script between write and execute.
- **Files:** `beamsync/firewall.go:19-30`
- **Current mitigation:** The temp directory is created with `os.MkdirTemp`, which is only readable by the current user. The race window is small.
- **Recommendations:** Use `pkexec` directly with the script as inline stdin (`echo "$SCRIPT" | pkexec sh`) to avoid writing to disk. Or use PolicyKit actions instead of script execution.

### Self-signed TLS with no certificate verification

- **Risk:** TLS is opt-in (`BEAMSYNC_ENABLE_TLS=1`) and uses a self-signed certificate generated at first run. No CA verification, no certificate pinning — any MITM can present their own cert.
- **Files:** `beamsync/tls.go:134-183`
- **Current mitigation:** The TLS certificate fingerprint is used in the token HMAC, so token replay requires knowledge of the fingerprint. However, the first connection has no trust-on-first-use verification.
- **Recommendations:** For a LAN-only tool, this is acceptable. Document the trust model explicitly.

### CORS allows any origin

- **Risk:** All endpoints set `Access-Control-Allow-Origin: *` (`beamsync/server.go:637`). On a LAN, any website loaded by any device can make requests to the BeamSync server.
- **Files:** `beamsync/server.go:637-640`
- **Current mitigation:** Token-based authentication on all sensitive endpoints prevents unauthorized access regardless of origin.
- **Recommendations:** If token auth is sufficient, this is acceptable for LAN use. Document that this is by design.

### Token secret is ephemeral per server start

- **Risk:** The token HMAC secret (`newTokenStore`, `auth_tokens.go:54-70`) is generated fresh via `crypto/rand` on each server start. If the server restarts during an active transfer, all in-flight sessions break.
- **Files:** `beamsync/auth_tokens.go:54-70`
- **Current mitigation:** Tokens have a 5-minute TTL, so sessions are short-lived anyway.
- **Recommendations:** Acceptable for the use case. Document that server restart during transfer requires re-scanning the QR code.

### No input size validation in resumable upload manifest

- **Risk:** The resumable upload manifest file (`*.json`) is loaded with `os.ReadFile` without size limits. A malicious peer could send a very large JSON that exhausts memory.
- **Files:** `beamsync/resumable_upload.go:185-198`
- **Current mitigation:** The manifest JSON is written by the server itself, so it's self-controlled.
- **Recommendations:** Add a `http.MaxBytesReader`-style limit to manifest reads, or use `json.NewDecoder` with `Decoder.Decode` instead of `ReadFile`.

### No session token invalidation on disconnect

- **Risk:** When a device disconnects (heartbeat timeout), session tokens issued to that client are not revoked. They remain valid until their 5-minute TTL expires.
- **Files:** `beamsync/server.go:716-742` (watchdog), `beamsync/auth_tokens.go:119-143`
- **Current mitigation:** Tokens expire within 5 minutes.
- **Recommendations:** Acceptable for LAN use. Lower the TTL or add token revocation on disconnect.

## Performance Bottlenecks

### Rate limiter acquires mutex per request

- **Problem:** `clientRateLimiter.allow()` (`beamsync/server.go:203-250`) holds a mutex for the entire decision-making process, including map lookups, insertions, and evictions. Under concurrent load from many clients, this becomes a contention point.
- **Files:** `beamsync/server.go:203-250`
- **Cause:** The entire rate limiter state is protected by a single `sync.Mutex`.
- **Improvement path:** Use a sharded map (e.g., 16 shards each with their own mutex, hashing the client IP for shard selection) to reduce lock contention.

### Resumable upload cleanup runs on every chunk request

- **Problem:** `cleanupResumableUploads()` (`beamsync/resumable_upload.go:233-252`) reads the entire resume directory on every single chunk upload, which includes an `os.ReadDir` call.
- **Files:** `beamsync/resumable_upload.go:67,166,233-252`
- **Cause:** Called at the start of every chunk handler invocation (line 67) and status check (line 166).
- **Improvement path:** Move cleanup to a periodic background goroutine (similar to token cleanup) instead of per-request. Or at minimum, skip cleanup if it ran within the last minute.

### Single Go 1.25.5 module structure with workspace replace directive

- **Problem:** The project uses a Go workspace (`beamsync/go.mod` module `beamsync + desktop/go.mod` module `desktop`) with a `replace beamsync => ../beamsync` directive. This is fine but the entire core library is one package (`package beamsync`) — no sub-package organization.
- **Files:** `beamsync/go.mod`, `desktop/go.mod`
- **Cause:** No sub-package structure within the `beamsync` module.
- **Improvement path:** Break the core library into sub-packages (`beamsync/auth`, `beamsync/transfer`, `beamsync/network`) for better build caching and compilation parallelism.

### 8 MB chunk buffer pool may cause memory pressure

- **Problem:** `chunkBufferPool` (`beamsync/server.go:88-93`) holds 8 MB buffers. Under concurrent large-file transfers, many of these stay allocated.
- **Files:** `beamsync/server.go:88-93`
- **Cause:** Pool does not cap the number of buffers. Under load, the pool can grow unbounded (though `sync.Pool` is GC-aware).
- **Improvement path:** Acceptable with `sync.Pool` — GC pressure is minimal. Only if profiling shows high memory use should this be revisited.

## Fragile Areas

### eventDispatcher with dropped events

- **Files:** `beamsync/server.go:39-80`
- **Why fragile:** The `eventDispatcher` uses a 256-capacity buffered channel. If events are produced faster than the callback can process them, events are silently dropped (line 77: `default: return false`). The only signal is a `fmt.Printf` line (line 751).
- **Safe modification:** Increase buffer size or use a blocking send in the `safeEmit` wrapper. Consider adding a metric/counter for dropped events.
- **Test coverage:** No unit tests for eventDispatcher behavior under load.

### app.go event channel with 100 buffer capacity

- **Files:** `desktop/app.go:70`, `232-237`
- **Why fragile:** The `eventChan` has a 100-capacity buffer (line 70). The `makeCallback` method drops events when full (line 235). During a burst of events (e.g., many files received), events like `device_connected` or `transfer_stats` can be silently dropped.
- **Safe modification:** Use a larger buffer or a goroutine-per-event pattern. The current non-blocking send is a deliberate choice to avoid blocking the event producer.
- **Test coverage:** No unit tests for event channel behavior.

### Drag-and-drop race between HTML5 and Wails native handler

- **Files:** `desktop/frontend/src/App.svelte:814-841`
- **Why fragile:** Two drag-and-drop paths can both fire — the HTML5 `window.ondrop` (line 852) and the Wails `OnFileDrop` binding (line 479). The `dropGuard` mechanism (lines 822-823, 837-838) with a 500ms timeout attempts to prevent double-firing but is timing-dependent.
- **Safe modification:** Use only one drop path. If Wails `OnFileDrop` supports the `.path` property, use it exclusively and remove the HTML5 handler.
- **Test coverage:** Not tested — drag-and-drop is difficult to automate.

### Progress timeout resets after 30 seconds of inactivity

- **Files:** `desktop/frontend/src/App.svelte:422-440`
- **Why fragile:** `_progressTimeout` (30s) resets progress UI to idle if no progress update arrives. For very large files over slow networks, the interval between progress events could exceed 30 seconds, causing the UI to show "Idle" during an active transfer.
- **Safe modification:** Increase the timeout to 120 seconds, or tie it to the heartbeat mechanism.

## Scaling Limits

### Transfer history maxes at 100 records

- **Current capacity:** 100 entries per session (`defaultTransferHistoryLimit`, `beamsync/history.go:9`).
- **Limit:** History ring buffer evicts old entries when full. No persistence.
- **Scaling path:** Persist to disk and allow configurable limits.

### Active transfer count unbounded

- **Current capacity:** No hard limit on concurrent uploads/downloads. The rate limiter throttles new requests but within a window, unlimited goroutines can be spawned.
- **Limit:** Memory and file descriptor exhaustion under extreme concurrent load.
- **Scaling path:** Add a semaphore to cap concurrent file operations (`beamsync/server.go` around the `startWriteWorkers` path).

### Resumable uploads never age out mid-transfer

- **Current capacity:** Stale resumable uploads are cleaned up after 24 hours (`resumableUploadTTL`, `resumable_upload.go:21`). No limit on total number of partial uploads.
- **Limit:** A malicious client could start many partial uploads to fill disk with `.part` files.
- **Scaling path:** Add a maximum number of concurrent resumable uploads per client.

## Missing Critical Features

### No encrypted transfer option

- **Problem:** All transfer data uses plain HTTP (TLS is opt-in and not default). Files travel in the clear over the LAN.
- **Blocks:** Users on untrusted networks (public Wi-Fi, corporate networks) cannot ensure transfer privacy without manually enabling TLS.

### No transfer cancellation by user

- **Problem:** Once a transfer starts, there is no API or UI to cancel it mid-flight. The only way to stop is to disconnect the server.
- **Blocks:** Users cannot abort a mistaken or too-large file transfer.

### No file selection on mobile

- **Problem:** The mobile receiver UI (`ui/upload.html`) is served by `embed.FS` and cannot be modified without rebuilding the Go binary. No way to customize the mobile UI flow.
- **Blocks:** Feature requests for the mobile side require full binary rebuilds.

## Test Coverage Gaps

### eventDispatcher not tested

- **What's not tested:** The `eventDispatcher` struct (`server.go:39-80`) has no unit tests. Buffer full behavior, panic recovery, and concurrent emit safety are untested.
- **Files:** `beamsync/server.go:39-80`
- **Risk:** Silent event drops or panics could cause UI to show stale data.
- **Priority:** Medium

### progressWriter and downloadProgressWriter not tested

- **What's not tested:** Neither progress tracking type has unit tests. The event emission timing and content format are untested.
- **Files:** `beamsync/server.go:360-404`
- **Risk:** Wrong progress values could confuse users. Format changes could break the frontend parser.
- **Priority:** Low

### tokenStore edge cases not tested

- **What's not tested:** Token expiry, concurrent issue/validate, cleanup of expired tokens, race conditions in consume vs non-consume behavior.
- **Files:** `beamsync/auth_tokens.go`
- **Risk:** Auth bypass or session hijacking.
- **Priority:** High

### Firewall setup not tested

- **What's not tested:** `RunFirewallSetup()` has no tests. The firewall script itself is untested.
- **Files:** `beamsync/firewall.go`, `beamsync/firewall_setup.sh`
- **Risk:** FW setup could fail silently or degrade system firewall rules.
- **Priority:** Low

### Frontend has no tests

- **What's not tested:** The entire Svelte frontend (`App.svelte`, all design system components) has zero tests. No component tests, no integration tests.
- **Files:** `desktop/frontend/src/`
- **Risk:** Bugs in UI state management, event handling, or rendering go undetected.
- **Priority:** High

### No end-to-end tests

- **What's not tested:** No cross-module integration tests or E2E tests covering the full send-receive flow across the Go backend and Svelte frontend.
- **Files:** Entire project
- **Risk:** Integration bugs between backend and frontend go undetected.
- **Priority:** Medium

## Dependencies at Risk

### github.com/faiface/beep v1.1.0 (audio engine)

- **Risk:** Audio playback library. Last release appears old (v1.1.0). The `beep` library has known limitations on non-Linux platforms and the `speaker` package has caused panics in the past (app.go line 131 has a fallback).
- **Impact:** Audio feedback is a non-critical feature. If the library breaks, the app degrades gracefully (sounds fail silently).
- **Migration plan:** Acceptable risk. If audio becomes critical, consider replacing with a more maintained alternative or platform-native audio.

### github.com/wailsapp/wails/v2 v2.12.0

- **Risk:** Core desktop framework. Major version 2 has known limitations in accessibility, HiDPI support on Linux, and native menu support.
- **Impact:** Full application dependency — hard to migrate.
- **Migration plan:** Monitor Wails v3 progress. No immediate action needed.

---

*Concerns audit: 2026-07-20*
