# Phase 04: Go Structural Refactor — `server.go` Extraction - Pattern Map

**Mapped:** 2026-11-13
**Files analyzed:** 9 new, 6 modified (3 optional test splits)
**Analogs found:** 9 / 9 new files (7 share the established extracted-file shape; 2 have
role-matches only) / 6 modified test files (all covered)

---

## Scope note — three corrections to CONTEXT.md the planner MUST know

These were found by reading the live tree. Each one changes a plan instruction.

### C-1 — `endpointMu`, `serverMu`, `stateMu` are NOT in `beamsync/`

CONTEXT.md D-05 lists eleven Phase-3 artifacts that must survive the move, and names
`endpointMu`, `serverMu`, `stateMu` among them. Those three live in **`desktop/app.go`**:

```
desktop/app.go:47:	stateMu sync.RWMutex
desktop/app.go:55:	serverMu     sync.Mutex
desktop/app.go:64:	endpointMu   sync.RWMutex
```

This phase only touches `beamsync/`. The desktop module is not edited, so its three locks
cannot be dropped by this phase. The D-05 items that **are** in `beamsync/` — and therefore
must be carried intact — are:

| D-05 item | Lives at | Carried by |
|---|---|---|
| `settingsMu` guarding the live `TransferSettings` | `beamsync/server.go:311` | `server.go` (stays) |
| `HTTPServer.Settings()` returning a snapshot | `beamsync/server.go:342-350` | `server.go` (stays) |
| `HTTPServer.UpdateSettings` swapping atomically | `beamsync/server.go:356-360` | `server.go` (stays) |
| `rateLimitMiddleware` taking a **settings provider** | `beamsync/server.go:567` | `middleware.go` (moves) |
| `progressWriter.eventName` | `beamsync/server.go:433` | `progress.go` (moves) |
| nil-guarded `progressWriter.now` | `beamsync/server.go:437, 449-451` | `progress.go` (moves) |
| `progressWriter.Write`'s zero-byte guard | `beamsync/server.go:442-446` | `progress.go` (moves) |
| `eventDispatcher.stateMu` | `beamsync/server.go:45` | `events.go` (moves) |
| `eventDispatcher.close()` | `beamsync/server.go:105-113` | `events.go` (moves) |
| deterministic quit guard in `makeCallback` | `desktop/app.go` (desktop module) | **out of scope** |

**Planner action:** do not write a plan task that "preserves `endpointMu`/`serverMu`/`stateMu`
in the extracted files" — there is nothing to preserve. Keep `desktop/app.go` untouched and
keep `go test -race .` in `desktop/` in the verification gate (CI does this already).

### C-2 — `beamsync/localip_test.go` does not exist

CONTEXT.md `<code_context>` says *"beamsync/localip_test.go — the newest file, and the
cleanest example of a file plus its colocated test."* Verified with `ls beamsync/localip*`:

```
beamsync/localip.go      (76 lines — no test)
```

`localip.go` exists but has **no colocated test**. The real colocated-test precedents are
`auth_tokens_test.go`, `history_test.go`, `stats_test.go`, `tls_test.go`, `port_manager_test.go`,
`firewall_test.go`, `permissions_test.go` — all seven of which pair 1:1 with a source file of
the same concept. `localip.go` is still the right **primary analog for the source-file shape**
(newest, cleanest single-concept file), but it is **not** an example of the file-plus-test
pairing. Use `port_manager.go` / `port_manager_test.go` (32 + 46 lines — the tightest
source+test pair in the repo) for that.

### C-3 — ROADMAP 4.6 and CONTEXT D-06 contradict each other on `uiFS`

- ROADMAP.md Phase 4, plan 4.6: *"Move `generateID`, `sha256Hex`, `autoRenamePath`, the
  `uploadIntegrityHeader` constant and the `uiFS` embed declaration"* → `helpers.go`.
- CONTEXT.md D-06: *"`//go:embed ui/*.html ui/*.png` **stays in `server.go`** (PITFALLS.md 1d)"*.

**D-06 wins.** It is the locked decision and it is also the technically safer choice: all four
`uiFS` use sites are inside `server.go` today (`server.go:827, 853, 1321, 1346`), so keeping the
declaration in `server.go` leaves **zero** cross-file references to `uiFS`.

**Planner action:** plan 4.6 moves `generateID`, `sha256Hex`, `uploadIntegrityHeader` and the
two `var _ = …` blank assignments to `helpers.go`; it does **not** move `uiFS`. Flag this
divergence from the ROADMAP explicitly in the plan so a reviewer does not read it as an
omission.

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `beamsync/events.go` (NEW) | service (event infrastructure) | event-driven (channel queue + worker goroutine) | `beamsync/localip.go` (file shape) + `beamsync/auth_tokens.go` (bg-goroutine + mutex shape) | role-match |
| `beamsync/rate_limiter.go` (NEW) | service (rate limiting) | request-response (per-client window) | `beamsync/port_manager.go` (file shape) + `beamsync/stats.go` (mutex+constructor shape) | role-match |
| `beamsync/progress.go` (NEW) | model (writer) | streaming (byte stream → progress events) | `beamsync/stats.go` (struct + mutex + methods) | role-match |
| `beamsync/io_helpers.go` (NEW) | utility | file-I/O (chunked copy) + streaming | `beamsync/port_manager.go` (file shape) + `beamsync/tls.go` (file-I/O helpers) | role-match |
| `beamsync/middleware.go` (NEW) | middleware | request-response (HTTP chain) | `beamsync/auth_tokens.go` (token store consumed by mw) | role-match |
| `beamsync/helpers.go` (NEW) | utility | transform (ID gen) + file-I/O (path rename) | `beamsync/localip.go` (primary: file shape) | role-match |
| `beamsync/upload_handler.go` (NEW) | controller (HTTP handler method) | request-response + file-I/O + streaming | **none in-repo** (see "No Analog Found") | none → RESEARCH/PITFALLS 1c |
| `beamsync/download_handler.go` (NEW) | controller (HTTP handler method) | request-response + file-I/O + streaming | **none in-repo** (see "No Analog Found") | none → RESEARCH/PITFALLS 1c |
| `beamsync/server.go` (MODIFIED, trimmed) | controller (orchestration) | request-response | itself | exact |
| `beamsync/event_dispatcher_test.go` (MODIFIED) | test | event-driven | itself | exact |
| `beamsync/rate_limiter_test.go` (MODIFIED) | test | request-response | itself | exact |
| `beamsync/progress_test.go` (MODIFIED) | test | streaming | itself | exact |
| `beamsync/progress_integration_test.go` (MODIFIED) | test | request-response + streaming | itself | exact |
| `beamsync/progress_wiring_test.go` (MODIFIED) | test | request-response + streaming | itself | exact |
| `beamsync/server_test.go` (MODIFIED) | test | mixed | itself | exact |

---

## The established convention (primary analog: `beamsync/localip.go`)

Every file extracted in Phase 1/2 follows the same shape. `beamsync/localip.go` is the newest
and cleanest — use it as the template.

### `beamsync/localip.go` — the reference file shape (76 lines, read in full)

```go
package beamsync                                    // 1  — no file-level doc comment, ever

import (                                            // 3
	"net"                                     // 4  — parenthesized block; stdlib only
)                                                  // 5

// GetLocalIP returns the host's LAN IP for building share URLs.   // 7  — doc comment
//
// Primary: a UDP "dial" to 8.8.8.8:80. No packet is sent; the kernel just
// selects the source address from the routing table (like `ip route get`).
// ...                                                  // 10-21 — multi-paragraph rationale
func GetLocalIP() string {                          // 22 — exported func: doc comment REQUIRED
	if ip := dialOutIP(); ip != "" {      // 23
		return ip
	}
	if ip := firstUnicastIPv4(); ip != "" { // 26
		return ip
	}
	return "127.0.0.1"
}

func dialOutIP() string {                            // 32 — unexported func: NO doc comment
	conn, err := net.Dial("udp", "8.8.8.8:80")
	...
}

func firstUnicastIPv4() string {                     // 45 — unexported: NO doc comment
	...
}
```

### The eight conventions every extracted file obeys

Verified across `localip.go`, `auth_tokens.go`, `permissions.go`, `history.go`, `stats.go`,
`tls.go`, `port_manager.go`, `firewall.go`:

| # | Convention | Evidence |
|---|---|---|
| C1 | `package beamsync` on line 1; **no file-level doc comment** | all 8 files, line 1 |
| C2 | Blank line, then a single `import ( … )` block; stdlib only; no aliases | `localip.go:3-5`, `auth_tokens.go:3-10` |
| C3 | Doc comment on **exported** symbols only; unexported symbols get **no** doc comment | `localip.go:7` vs `localip.go:32`; `firewall.go:18` vs `firewall.go:73` |
| C4 | A doc comment on an *unexported* symbol is allowed **only** when it records a non-obvious invariant (see the `eventDispatcher.stateMu` field comment at `server.go:42-44` and `progressWriter`'s doc comment at `server.go:422-427`) | `server.go:42-44`, `server.go:422-427` |
| C5 | Constructor named `newXxx` returning `*xxx` for unexported types; `NewXxx` for exported | `auth_tokens.go:60`, `stats.go:36`, `history.go:42` vs `history.go:42` `NewTransferHistory` |
| C6 | Mutex-guarded struct: `mu sync.Mutex` as the **first** field, `Lock()`/`defer Unlock()` in every method | `stats.go:24,41-42`, `history.go:34,54-55`, `auth_tokens.go:54,88-90` |
| C7 | `//go:embed` sits in the file that **owns** the asset | `firewall.go:12-13` (`//go:embed firewall_setup.sh`) |
| C8 | Test file named `{source}_test.go`, same package (`package beamsync`, **not** `_test` suffix) | `auth_tokens.go`↔`auth_tokens_test.go` ×7 pairs |

**Package clause is non-negotiable:** `package beamsync`, never `package beamsync_test`
(PITFALLS.md §2a — `eventDispatcher`, `clientRateLimiter`, `progressWriter` are all unexported).

### Import grouping

One `import ( … )` block, stdlib only, alphabetical within the block. Two idioms exist in the
repo — **pick the parenthesized form for every new file**, because every new file has ≥2
imports:

```go
// parenthesized — used by localip.go, auth_tokens.go, stats.go, history.go, tls.go,
//                  port_manager.go, firewall.go  (7 of 8)
import (
	"net"
)
```

```go
// single-line — used ONLY by permissions.go (1 of 8)
import "strings"
```

> Do **not** add a second import group or a blank line separating stdlib from third-party in
> the new files. No new file needs a third-party import.

---

## Pattern Assignments

### `beamsync/events.go` (NEW — service, event-driven)

**Analog (file shape):** `beamsync/localip.go`
**Analog (bg goroutine + mutex):** `beamsync/auth_tokens.go:167-180` (`startCleanup`)
**Production code moved:** `beamsync/server.go:28-113`, `:227`, `:229-239`, `:654-675`,
`:728-748`

#### Moved block A — the dispatcher type + its singleton (PITFALLS 1a)

`beamsync/server.go:28-113` + `:227`:

```go
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

	// stateMu guards closed. emit takes the read lock and close takes the
	// write lock, so closing the dispatcher can never race with an emit
	// (which would panic on a send to a closed channel).
	stateMu sync.RWMutex
	closed  bool
}

func newEventDispatcher(bufferSize int) *eventDispatcher { ... }   // 49-58
func (d *eventDispatcher) run() { ... }                          // 60-73
func (d *eventDispatcher) emit(job eventDispatchJob) bool { ... }// 75-91
func (d *eventDispatcher) DroppedCount() int64 { ... }            // 93-95
func (d *eventDispatcher) close() { ... }                         // 97-113
```

**`var defaultEventDispatcher = newEventDispatcher(eventDispatcherBufferSize)` (`server.go:227`)
MUST travel in the same block.** PITFALLS 1a is exactly this trap: if the `var` stays behind
while the type moves, there are two dispatchers and events are silently dropped — **no compile
error, no vet error**.

Verification gate (from ROADMAP Phase 4 Risks + CONTEXT D-02):

```bash
rg "defaultEventDispatcher" beamsync/
# MUST show exactly 1 declaration (the `var` line) and N > 0 usages.
# Current baseline: server.go:227 (decl) + server.go:732 (use).
# After the move: events.go (decl) + events.go:732-equivalent (use).
```

The use site is `safeEmit` (`server.go:732`), which also moves into `events.go`. So after the
move, `rg` must show **2 hits, both in `events.go`** — one declaration, one usage.

#### Moved block B — `PendingTransfer` (CONTEXT agent discretion)

`beamsync/server.go:229-239`:

```go
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
```

**Recommendation: `events.go`** (ROADMAP 4.1 lists it there). Rationale: the type exists to
carry a `transfer_request` event to the desktop and its `approved chan bool` is the approval
handshake — it is the event-flow's payload type, and the `transfer_request` /
`transfer_request_timeout` emits live in the same file.
**Alternative (also allowed by CONTEXT):** leave it in `server.go` next to
`HTTPServer.pendingTransfers map[string]*PendingTransfer` (`server.go:306`) and
`RespondToTransfer` (`server.go:325-335`) — its only three consumers.

> Cross-file references inside one package are legal Go; either home compiles. The planner must
> pin ONE. Do not let the two files both claim it.

#### Moved block C — the background event producers

`beamsync/server.go:654-675` (`startWatchdog`) and `:728-748` (`safeEmit`, `logTransfer`):

```go
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
```

`startWatchdog` takes `*serverState` — which **stays in `server.go`**. That is fine (same
package); it is a cross-file type reference, not a cross-package one.

#### Moved block D — `startBootstrapRotator` (planner must pin this)

`beamsync/server.go:681-726` is **not assigned to any file** by CONTEXT or ROADMAP. It is a
background goroutine that emits `url_changed` and `token_rotated` — the same shape as
`startWatchdog`, which ROADMAP 4.1 puts in `events.go`.

**Recommendation: `events.go`** (alongside `startWatchdog`). It needs `*HTTPServer`
(`hs.RotateBootstrapToken()`, `hs.state.activeUploads()`) — a same-package reference, fine.
**Alternative: `helpers.go`** (it is a free function, and `helpers.go` is the designated
catch-all), or leave it in `server.go` as orchestration.

Whichever is chosen, the function body moves verbatim; only its *home file* changes.

#### `events.go` import block (exact)

```go
package beamsync

import (
	"context"        // startWatchdog, startBootstrapRotator
	"encoding/json"  // logTransfer
	"fmt"            // run(), safeEmit, logTransfer, watchdog panic print
	"sync"           // eventDispatcher.stateMu sync.RWMutex
	"sync/atomic"    // eventDispatcher.dropped atomic.Int64
	"time"           // startWatchdog ticker
)
```

If `startBootstrapRotator` goes to `helpers.go` instead, `events.go` drops `"context"` and
`helpers.go` gains it.

---

### `beamsync/rate_limiter.go` (NEW — service, request-response)

**Analog (file shape):** `beamsync/port_manager.go` (32 lines, one concept, stdlib-only imports)
**Analog (mutex + injected clock + constructor):** `beamsync/auth_tokens.go:53-66`
**Production code moved:** `beamsync/server.go:115-226` — **one contiguous block**, type +
constructor + all three methods (PITFALLS 1e: move the *entire* type or the package breaks).

#### Moved block (verbatim, `beamsync/server.go:115-225`)

```go
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

func (l *clientRateLimiter) allow(client string) rateLimitDecision { ... }  // 148-196
func (l *clientRateLimiter) prune(now time.Time) { ... }                   // 198-204
func (l *clientRateLimiter) evictOldestIfFull() { ... }                   // 206-225
```

#### The `now func() time.Time` injection pattern (must not be lost)

`clientRateLimiter.now` is the repo's canonical clock-injection seam. `beamsync/rate_limiter_test.go:15`
overwrites it (`limiter.now = func() time.Time { return now }`) and the Phase-3 mutant #33
(`rateLimitMiddleware` captures a pointer) is killed by
`TestRateLimitMiddlewareReadsCurrentSettings`. Move the field **and** the `now: time.Now`
default in the constructor together.

#### `rate_limiter.go` import block (exact)

```go
package beamsync

import (
	"sync"   // clientRateLimiter.mu
	"time"   // rateLimitState, rateLimitDecision, window
)
```

#### Test co-location

`beamsync/rate_limiter_test.go` already carries the right name. It splits into two concerns:

| Test | Subject | Lands in |
|---|---|---|
| `TestClientRateLimiterBlocksAfterLimit` (`:12`) | `rate_limiter.go` | `rate_limiter_test.go` |
| `TestClientRateLimiterResetsAfterWindow` (`:28`) | `rate_limiter.go` | `rate_limiter_test.go` |
| `TestClientRateLimiterEvictsOldestClientWhenFull` (`:46`) | `rate_limiter.go` | `rate_limiter_test.go` |
| `TestRateLimitMiddlewareReturns429` (`:74`) | `middleware.go` | `rate_limiter_test.go` or new `middleware_test.go` |
| `TestRateLimitMiddlewareUsesXForwardedFor` (`:114`) | `middleware.go` | idem |
| `TestRateLimitMiddlewareBypassesTrustedDevice` (`:151`) | `middleware.go` | idem |
| `TestRateLimitMiddlewareRoundsRetryAfterUp` (`:182`) | `middleware.go` | idem |
| `TestRateLimitMiddlewareReadsCurrentSettings` (`:234`) — **mutant #33** | `middleware.go` | idem |

**Optional:** create `beamsync/middleware_test.go` and move the five `TestRateLimitMiddleware*`
functions into it. **Test function names must not change** (see "Shared Patterns → the mutant
table keys on test names"). Moving a function between `_test.go` files in the same package is
safe; renaming one is not.

---

### `beamsync/progress.go` (NEW — model, streaming)

**Analog:** `beamsync/stats.go` (struct + constructor + mutex-guarded methods, one concept)
**Production code moved:** `beamsync/server.go:422-458`

#### Moved block (verbatim, `beamsync/server.go:422-458`)

```go
// progressWriter wraps the destination of a transfer and emits a progress event
// as bytes flow through it.
//
// Single-writer type: Write mutates written/lastEmit and is NOT safe for
// concurrent use. Each transfer path owns exactly one progressWriter and writes
// to it from a single goroutine (receive path and each send path).
type progressWriter struct {
	w           io.Writer
	total       int64
	written     int64
	filename    string
	eventName   string
	emit        func(string, string)
	lastEmit    time.Time
	minInterval time.Duration
	now         func() time.Time
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.w.Write(p) //nolint:errcheck,gosec
	if n == 0 {
		// A zero-byte write transfers nothing; emitting progress for it would
		// report "0 bytes written" as a real update.
		return n, err
	}
	pw.written += int64(n)
	clock := time.Now
	if pw.now != nil {
		clock = pw.now
	}
	now := clock()
	if now.Sub(pw.lastEmit) >= pw.minInterval {
		pw.emit(pw.eventName, fmt.Sprintf("%s|%d|%d", pw.filename, pw.written, pw.total))
		pw.lastEmit = now
	}
	return n, err
}
```

#### The three Phase-3 invariants inside this block (D-05)

1. **`eventName` field** (`server.go:433`) — mutant #1/#2/#3 kill it. Without it, `Write` would
   hardcode `"upload_progress"` and the send path dies again (the exact Phase-1 defect).
2. **nil-guarded `now`** (`server.go:437, 449-451`) — `progressWriter` has **no constructor**; it
   is built by struct literals at three sites. A zero-value `now` is `nil` → `pw.now()` panics.
   The guard (`clock := time.Now; if pw.now != nil { clock = pw.now }`) is the Phase-3 fix and
   must move verbatim.
3. **zero-byte guard** (`server.go:442-446`) — mutant #5. Must move verbatim.

> The doc comment on `progressWriter` (`server.go:422-427`) is an *unexported* symbol with a
> doc comment — permitted by convention C4 because it records the single-writer invariant.
> Move it as-is.

#### `progress.go` import block (exact)

```go
package beamsync

import (
	"fmt"   // Write's payload format
	"io"    // progressWriter.w io.Writer
	"time"  // lastEmit, minInterval, now func() time.Time
)
```

#### Test co-location

`beamsync/progress_test.go` already carries the right name. **One test is misplaced** and
should move (function name unchanged):

| Test | Currently in | Should be in |
|---|---|---|
| `TestEventDispatcherCloseStopsWorkerAndIsIdempotent` | `progress_test.go:431-470` | `event_dispatcher_test.go` — it tests `eventDispatcher.close()`, i.e. `events.go` (mutant #7) |

The remaining eight `TestProgressWriter*` functions stay in `progress_test.go`. The helper
`progressWriterFixture` / `newProgressWriterForTest` (`progress_test.go:16-40`) stays with them.

---

### `beamsync/io_helpers.go` (NEW — utility, file-I/O + streaming)

**Analog (file shape):** `beamsync/port_manager.go`
**Production code moved:** `beamsync/server.go:244-249` (the pool) + `:460-486` (`copyChunked`)

#### The pool declaration — PITFALLS 1b, exactly ONE declaration

`beamsync/server.go:244-249`:

```go
var chunkBufferPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 8*1024*1024)
		return &buf
	},
}
```

Current state (verified by `rg`):

```
server.go:244:var chunkBufferPool = sync.Pool{        ← declaration
server.go:463:		bufPtr := chunkBufferPool.Get().(*[]byte)   ← use (inside copyChunked)
server.go:464:		defer chunkBufferPool.Put(bufPtr)           ← use (inside copyChunked)
```

**After the move there must still be exactly one `var chunkBufferPool = sync.Pool{` line**, and
it must live in a file that both handler files can reach. `io_helpers.go` is the only file
imported (i.e. same package) by both — the pool is a package-level `var`, so **any file in
`package beamsync` can reach it**. The trap is not visibility; it is *declaring it twice*.

Verification gate:

```bash
rg -n "var chunkBufferPool" beamsync/     # MUST return exactly 1 hit
```

> PITFALLS 1b suggests `pool.go` as the home. ROADMAP 4.4 says `io_helpers.go`. **Follow
> ROADMAP** (`io_helpers.go`) — `copyChunked` is the pool's only consumer, so the file that
> owns the copy loop owns the pool. The single-declaration rule is what matters, not the file
> name.

#### The copy loop (verbatim, `beamsync/server.go:460-486`)

```go
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
```

#### `io_helpers.go` import block (exact)

```go
package beamsync

import (
	"io"   // copyChunked signature
	"sync" // chunkBufferPool
)
```

#### Test co-location

`TestCopyChunkedCopiesDataAndReportsCount` currently lives in `beamsync/server_test.go:118-132`.
**Optional but recommended:** move it to a new `beamsync/io_helpers_test.go` — this is exactly
the file name the original ROADMAP Phase 3 plan 3.3 specified (`beamsync/io_helpers_test.go`),
which was never created because the test landed in `server_test.go` instead. Creating it now
realizes the original intent and matches convention C8. Function name unchanged.

---

### `beamsync/middleware.go` (NEW — middleware, request-response)

**Analog:** `beamsync/auth_tokens.go` (the `tokenStore` this middleware validates against;
the `tokenScope` constants it consumes)
**Production code moved:** `beamsync/server.go:506-636` — one contiguous block

#### Moved block (verbatim, `beamsync/server.go:506-636`)

```go
func extractToken(r *http.Request) string { ... }                          // 506-517
func tokenMiddleware(tokens *tokenStore, scope tokenScope, consume bool, next http.HandlerFunc) http.HandlerFunc { ... }        // 519-534
func tokenMiddlewareAny(tokens *tokenStore, scopes []tokenScope, consume bool, next http.HandlerFunc) http.HandlerFunc { ... }     // 536-551
func setRateLimitHeaders(w http.ResponseWriter, decision rateLimitDecision) { ... }                                                 // 553-560
func rateLimitMiddleware(lim *clientRateLimiter, settings func() *TransferSettings, next http.HandlerFunc) http.HandlerFunc { ... }// 562-606
func setCORSHeaders(w http.ResponseWriter) { ... }                          // 608-612
func clientIP(r *http.Request) string { ... }                              // 614-636
```

#### The settings-provider signature — the Phase-3 hardening (D-05)

`beamsync/server.go:562-567` — **the signature must not change**:

```go
// rateLimitMiddleware wraps a handler with per-client rate limiting.
//
// settings is a PROVIDER, not a pointer: the settings can be replaced while
// the server runs, so each request must read the current snapshot rather than
// a pointer captured at route-registration time.
func rateLimitMiddleware(lim *clientRateLimiter, settings func() *TransferSettings, next http.HandlerFunc) http.HandlerFunc {
```

And the body reads it per request (`server.go:576-579`):

```go
		var current *TransferSettings
		if settings != nil {
			current = settings()
		}
		if current != nil && current.isDeviceTrusted(ip) {
```

**Mutant #33** (`rateLimitMiddleware` captures a pointer) is killed by
`TestRateLimitMiddlewareReadsCurrentSettings` (`rate_limiter_test.go:234-270`). Changing the
signature to `settings *TransferSettings` re-opens that defect silently — the test still
compiles if the call sites are updated, and then fails. Good: the test *does* catch it. But
the plan must not make that change.

> The two call sites pass **different** providers today:
> `httpServer.currentSettings` (server.go:798, 817, 849, 861, 872, 963 — receiver side) and
> `nil` (server.go:1227, 1309, 1342, 1357, 1427 — sender side). Both must be preserved
> verbatim.

#### `middleware.go` import block (exact)

```go
package beamsync

import (
	"encoding/json"  // rateLimitMiddleware's 429 JSON body
	"fmt"            // tokenMiddleware's reject print
	"math"           // math.Ceil(retryAfter.Seconds())
	"net"            // clientIP: net.ParseIP, net.SplitHostPort
	"net/http"       // all middleware
	"strconv"        // setRateLimitHeaders, Retry-After
	"strings"        // extractToken: strings.HasPrefix; clientIP: strings.Split/TrimSpace
	"time"           // rateLimitMiddleware: time.Second
)
```

---

### `beamsync/helpers.go` (NEW — utility, transform + file-I/O)

**Analog (primary):** `beamsync/localip.go`
**Production code moved:** `beamsync/server.go:488-504`, `:638-652`, `:677-679`
(+ `:681-726` if the planner assigns `startBootstrapRotator` here)

#### Moved block A — the reserved-integrity helpers (verbatim, `server.go:488-496`)

```go
const uploadIntegrityHeader = "X-BeamSync-File-SHA256" //nolint:unused // reserved

func sha256Hex(data []byte) string { //nolint:unused // reserved
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

var _ = uploadIntegrityHeader
var _ = sha256Hex
```

**Move all four pieces together.** CONTEXT's `<deferred>` section says these are currently
unused and Phase 3 deliberately left them alone — removing them is *not* a structural move and
is out of scope. The `//nolint:unused` markers and the two `var _ = …` blank assignments exist
to keep the linter quiet; dropping the blank assignments while keeping the `//nolint:unused`
markers (or vice-versa) changes the lint posture of the file.

#### Moved block B — `generateID` (verbatim, `server.go:498-504`)

```go
func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
```

#### Moved block C — `autoRenamePath` (verbatim, `server.go:638-652`)

```go
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
```

#### Moved block D — `getLocalIPForURL` (verbatim, `server.go:677-679`)

```go
func getLocalIPForURL() string {
	return GetLocalIP()
}
```

**Recommendation: `helpers.go`.** **Alternative (conceptually cleaner):** `localip.go` — it is a
one-line wrapper over `GetLocalIP()`, which lives there (`localip.go:22`). `localip.go` is not
in CONTEXT's target file list, so `helpers.go` is the default. Either is a one-line move.

#### `helpers.go` import block (exact)

```go
package beamsync

import (
	"context"        // ONLY if startBootstrapRotator lands here
	"crypto/rand"    // generateID
	"crypto/sha256"  // sha256Hex
	"encoding/hex"   // generateID, sha256Hex
	"fmt"            // generateID fallback, autoRenamePath
	"os"             // autoRenamePath: os.Stat
	"path/filepath"  // autoRenamePath
	"strings"        // autoRenamePath: strings.TrimSuffix
	"time"           // generateID + autoRenamePath fallbacks
)
```

Drop `"context"` if `startBootstrapRotator` goes to `events.go`.

> **`uiFS` does NOT go here** — see correction C-3.

---

### `beamsync/upload_handler.go` (NEW — controller, request-response + file-I/O + streaming)

**Analog:** none in-repo for the closure→method conversion. See "No Analog Found".
**Guide:** `.planning/research/PITFALLS.md` §1c (the two-step extraction: make them methods on
`*HTTPServer` so the diff stays reviewable).
**Production code moved:** `beamsync/server.go:963-1141` (the `/upload` route + handler body)

#### The route registration that stays in `server.go` (server.go:963)

```go
	mux.HandleFunc("/upload", rateLimitMiddleware(httpServer.uploadLimiter, httpServer.currentSettings, tokenMiddlewareAny(tokens, []tokenScope{tokenScopeSession, tokenScopeBootstrap}, false, func(w http.ResponseWriter, r *http.Request) {
		... body ...
	})))
```

becomes:

```go
	mux.HandleFunc("/upload", rateLimitMiddleware(httpServer.uploadLimiter, httpServer.currentSettings, tokenMiddlewareAny(tokens, []tokenScope{tokenScopeSession, tokenScopeBootstrap}, false, httpServer.handleUpload)))
```

**The middleware chain and its order stay in `server.go`** — `rateLimitMiddleware` →
`tokenMiddlewareAny` → handler. CONTEXT D-04 says the *handler body* becomes a method; the
route registration stays in `server.go` (CONTEXT: "server.go keeps … route registration").

#### The handler body as a method (server.go:964-1140 → `upload_handler.go`)

The body moves **verbatim**. Five captured names change to receiver fields:

| Captured name | In the closure | Becomes | Source of truth |
|---|---|---|---|
| `state` | local `state := &serverState{}` (`server.go:775`) | `s.state` | `HTTPServer.state` — `server.go:322` ✓ already a field |
| `httpServer.history` | `httpServer.history` | `s.history` | `server.go:313` ✓ |
| `httpServer.stats` | `httpServer.stats` | `s.stats` | `server.go:314` ✓ |
| `emit` | local `emit := func(evt, data string) { safeEmit(callback, evt, data) }` (`server.go:773`) | `s.emit` | **NOT a field today — see the gap below** |
| `uploadDir` | `StartServerE` parameter (`server.go:750`) | `s.uploadDir` | **NOT a field today — CONTEXT D-04 requires adding it** |

**The `emit` gap — the planner must close it.** CONTEXT D-04 names only `uploadDir` as the new
field, but the `/upload` body also emits (`emit(pw.eventName, …)` at `server.go:1111`,
`emit("transfer_stats", …)` at `:1113`, `emit("file_received", fname)` at `:1125`) and the
`/download` bodies emit too. `emit` is **not** currently a field on `HTTPServer`.

Two ways to close it:

| Option | Shape | Notes |
|---|---|---|
| **A (recommended)** | Add `emit EventCallback` to `HTTPServer`; set it in both constructors | Serves **both** handler files. `StartServerE` sets `emit: emit` (the `safeEmit`-wrapped closure, `server.go:773`); `StartSenderE` sets `emit: emit` (`server.go:1206`). `StartSenderE` is called with a `nil` callback in `server_test.go:321` — `safeEmit(nil, …)` returns early, so a nil `callback` is safe. |
| B | Pass `emit` as a method parameter: `func (s *HTTPServer) handleUpload(emit EventCallback, w http.ResponseWriter, r *http.Request)` | 3 params + receiver — still well under PITFALLS 1c's 8-param ceiling. But then `handleDownload` needs it too. |

Option A also lets `startBootstrapRotator` read `hs.emit` instead of taking a parameter —
**but do not make that change**; keep its signature as-is to avoid an unnecessary behavior
surface. (Passing `s.emit` and `hs.emit` are the same closure, so it would be equivalent —
but "no behavior changes" argues for the minimal move.)

#### The `uploadDir` field (CONTEXT D-04)

CONTEXT: *"it is added as an `HTTPServer` field so the method signature stays short"* and
*"The exact `HTTPServer` field name for the stored upload directory (D-04)"* is agent
discretion.

**Recommendation:** `uploadDir string`.

```go
type HTTPServer struct {
	server           *http.Server
	cancel           context.CancelFunc
	pendingMu        sync.Mutex
	pendingTransfers map[string]*PendingTransfer
	// settingsMu guards settings. …
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
	// NEW in Phase 4:
	uploadDir        string   // the directory StartServerE writes uploads into
	emit             EventCallback // the safeEmit-wrapped callback for this server
}
```

`StartSenderE` does **not** set `uploadDir` — it stays `""`. The sender never writes uploads,
so the field is unused there. Document that with a one-line comment.

> The `settingsMu` field comment at `server.go:307-310` is a Phase-3 artifact (mutant #14).
> It must survive verbatim — do not "tidy" the struct's comments.

#### The body — the pieces that must not change

```go
func (s *HTTPServer) handleUpload(w http.ResponseWriter, r *http.Request) {
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
	s.state.markHeartbeat()
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
	// … manifest parsing, per-part loop, progressWriter construction, copyChunked,
	// logTransfer, stats, emit — all verbatim from server.go:990-1127 …
}
```

**Three Phase-3 artifacts inside this body (mutants #1, #20, #21, #18):**

1. **The ordered manifest matching** (`server.go:990-999` and `:1058-1069`) — the
   `manifestEntry{name, size, used}` slice, consumed **in order**, not keyed by base name.
   Mutant #21 (`ordered manifest matching → lossy keying`) is killed by
   `TestReceivePathGivesEachFileItsOwnDeclaredSize` (`progress_wiring_test.go:454`).
2. **The declared size** (`server.go:1058-1069`) — `declaredSize` feeds `progressWriter.total`.
   Mutant #20 is killed by `TestReceivePathReportsDeclaredTotal` (`progress_wiring_test.go:358`).
3. **The terminal emit** (`server.go:1111`) — `emit(pw.eventName, fmt.Sprintf("%s|%d|%d", savedName, written, written))`.
   Mutant #18 is killed by `TestReceivePathEmitsCompletionProgress` (`progress_wiring_test.go:225`).

Do not "simplify" any of these while moving the body.

#### `upload_handler.go` import block (exact)

```go
package beamsync

import (
	"bufio"           // bufio.NewReaderSize, bufio.NewWriterSize
	"encoding/json"   // json.NewDecoder(part).Decode(&manifest)
	"fmt"             // fmt.Sprintf for the fallback filename + the payload
	"io"              // io.EOF, io.Copy(io.Discard, part)
	"mime"            // mime.ParseMediaType
	"mime/multipart"  // multipart.NewReader
	"net/http"        // handler signature, http.Error, http.MaxBytesReader
	"os"              // os.Create
	"path/filepath"   // filepath.Base, autoRenamePath call
	"runtime/debug"   // debug.Stack() in the recover
	"strings"         // strings.HasPrefix(mediaType, "multipart/")
	"time"            // time.Now, 500*time.Millisecond
)
```

---

### `beamsync/download_handler.go` (NEW — controller, request-response + file-I/O + streaming)

**Analog:** none in-repo for the closure→method conversion (see "No Analog Found").
**Guide:** PITFALLS §1c.
**Production code moved:** `beamsync/server.go:1354-1493` (both download route registrations)

#### The two near-identical bodies

`server.go:1357-1422` (single file, `/download`) and `server.go:1427-1491` (multi file,
`/download/N`) are **the same logic** with one cosmetic difference: the single-file version
names the variable `filename`, the multi-file version names it `realName` — and both compute it
as `filepath.Base(filePath)`:

```go
	// single-file (server.go:1355-1356)
	filePath := filePaths[0]
	filename := filepath.Base(filePath)

	// multi-file (server.go:1424-1428)
	filePath := path
	realName := filepath.Base(filePath)
```

Everything else is identical: the same headers, the same `os.Open`/`Stat`, the same
`progressWriter` with `eventName: "download_progress"`, the same `copyChunked(pw, bufReader,
8*1024*1024)`, the same terminal emit, the same `logTransfer`.

**Recommendation: ONE method, `func (s *HTTPServer) handleDownload(filePath string, w
http.ResponseWriter, r *http.Request)`.** This is provably behavior-identical and removes the
duplicate-code risk that made the original Phase-1 bug possible. Route registration:

```go
	// single file (server.go:1354-1357)
	if len(filePaths) == 1 {
		filePath := filePaths[0]
		mux.HandleFunc("/download", rateLimitMiddleware(httpServer.uploadLimiter, nil, tokenMiddlewareAny(tokens, []tokenScope{tokenScopeTransfer, tokenScopeBootstrap}, true, func(w http.ResponseWriter, r *http.Request) {
			httpServer.handleDownload(filePath, w, r)
		})))
	} else {
		// multi file (server.go:1423-1492)
		for i, path := range filePaths {
			idx := i
			filePath := path
			mux.HandleFunc(fmt.Sprintf("/download/%d", idx), rateLimitMiddleware(httpServer.uploadLimiter, nil, tokenMiddlewareAny(tokens, []tokenScope{tokenScopeTransfer, tokenScopeBootstrap}, true, func(w http.ResponseWriter, r *http.Request) {
				httpServer.handleDownload(filePath, w, r)
			})))
		}
	}
```

**Alternative: two methods** (`handleDownloadSingle` / `handleDownloadIndexed`), each a literal
copy of its original body. Safer as a *move*, but keeps the duplication. CONTEXT's "no behavior
changes" charter does not forbid de-duplication, and the single method is provably identical —
but the planner must pick one and the plan must state which.

> ⚠️ **The `idx` capture.** In the multi-file loop, `idx := i` and `filePath := path` are
> **required** — `i`/`path` are the loop variables and Go < 1.22 semantics would alias them.
> `go.mod` says `go 1.25.5`, so per-iteration loop variables apply and the copies are
> redundant — **but keep them**. They are existing code and removing them is an unrelated
> change.

#### The body (verbatim from `server.go:1358-1421`, with `filename` → local `filepath.Base`)

```go
func (s *HTTPServer) handleDownload(filePath string, w http.ResponseWriter, r *http.Request) {
	activeDownloads := s.state.beginUpload()
	s.emit("transfer_stats", transferStatsJSON(s.stats.snapshotDownloads(activeDownloads)))
	defer func() {
		activeDownloads := s.state.endUpload()
		s.emit("transfer_stats", transferStatsJSON(s.stats.snapshotDownloads(activeDownloads)))
	}()
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	filename := filepath.Base(filePath)
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
		eventName:   "download_progress",
		emit:        s.emit,
		minInterval: 500 * time.Millisecond,
		now:         time.Now,
	}
	written, copyErr := copyChunked(pw, bufReader, 8*1024*1024)
	status := TransferStatusCompleted
	errMsg := ""
	if copyErr != nil {
		status = TransferStatusFailed
		errMsg = copyErr.Error()
	} else {
		// Completion event. Without it a fast transfer (finished inside the
		// 500ms throttle window) emits NO progress event at all, so the
		// desktop send-direction panel would never move.
		s.emit(pw.eventName, fmt.Sprintf("%s|%d|%d", filename, written, written))
		snapshot := s.stats.recordSent(filename, written, s.state.activeUploads())
		s.emit("transfer_stats", transferStatsJSON(snapshot))
	}
	logTransfer(s.history, s.emit, TransferRecord{
		Filename:  filename,
		Direction: TransferDirectionSend,
		Status:    status,
		SizeBytes: written,
		StartedAt: startedAt,
		Error:     errMsg,
	})
}
```

#### Phase-3 artifacts inside this body (mutants #2, #19)

1. **`eventName: "download_progress"`** (`server.go:1395` / `:1466`) — mutants #2 and #3.
   The Phase-1 defect was `progressWriter` hardcoding `"upload_progress"`; the field is the fix.
2. **The terminal completion emit** (`server.go:1410` / `:1479`) — mutants #19 and #18.
   `emit(pw.eventName, fmt.Sprintf("%s|%d|%d", filename, written, written))` — note it uses
   `pw.eventName`, **not** a literal. Keep that.
3. **`now: time.Now`** in the `progressWriter` literal — the nil-guard partner. Without it
   `pw.now` is `nil` and `Write` panics.

#### `download_handler.go` import block (exact)

```go
package beamsync

import (
	"bufio"          // bufio.NewReaderSize(file, 8*1024*1024)
	"fmt"            // fmt.Sprintf for Content-Disposition / Content-Length / payload
	"mime"           // mime.TypeByExtension
	"net/http"       // handler signature, http.Error
	"os"             // os.Open
	"path/filepath"  // filepath.Base, filepath.Ext
	"time"           // time.Now, 500*time.Millisecond
)
```

---

### `beamsync/server.go` (MODIFIED — trimmed to orchestration)

**Analog:** itself. Nothing else in the repo is an orchestration file.

#### What stays (CONTEXT + ROADMAP 4.9)

| Kept item | Current lines | Note |
|---|---|---|
| `package beamsync` + the import block | 1-26 → shrinks to 11 imports | see below |
| `//go:embed ui/*.html ui/*.png` + `var uiFS embed.FS` | 241-242 | **PITFALLS 1d / D-06 — must stay here** |
| `serverState` type + its 5 methods | 251-300 | `sync.Mutex` + `time` |
| `HTTPServer` struct + all its methods | 302-420 | `RespondToTransfer`, `Settings`, `UpdateSettings`, `currentSettings`, `TransferHistory`, `Stats`, `Shutdown`, `CurrentBootstrapToken`, `RotateBootstrapToken` |
| `StartServerE` | 750-1191 | route registration + construction + `srv.Serve` |
| `StartSenderE` | 1193-1538 | route registration + construction + `srv.Serve` |

#### The final import block (exact — this is the plan's acceptance check)

```go
import (
	"context"       // StartServerE/StartSenderE ctx, Shutdown's context.WithTimeout
	"embed"         // var uiFS embed.FS
	"encoding/json" // /request-transfer: json.NewDecoder, json.Marshal
	"fmt"           // fmt.Errorf, fmt.Sprintf, fmt.Printf
	"net/http"      // mux, http.Server, http.HandlerFunc
	"os"            // os.MkdirAll(uploadDir), buildFileBlock's os.Stat
	"path/filepath" // buildFileBlock's filepath.Base
	"runtime/debug" // StartServerE's recover: debug.Stack()
	"strings"       // strings.Replace, strings.Contains, strings.Builder
	"sync"          // serverState.mu
	"time"          // everywhere
)
```

**11 imports, down from 22.** The 11 that leave: `bufio`, `crypto/rand`, `crypto/sha256`,
`encoding/hex`, `io`, `math`, `mime`, `mime/multipart`, `net`, `strconv`, `sync/atomic`.

> ⚠️ `"embed"` (named) stays in `server.go` because `uiFS` is `embed.FS`. Do **not** switch to
> the blank-import form `_ "embed"` — that form (used at `firewall.go:4`) is only for
> embedding into `[]byte`. A blank import would make `embed.FS` unnameable and break the build.

#### The `uiFS` verification

All four `uiFS` reads are in `server.go` and stay there:

```
server.go:827   uiFS.ReadFile("ui/upload.html")
server.go:853   uiFS.ReadFile("ui/logo.png")
server.go:1321  uiFS.ReadFile("ui/download.html")
server.go:1346  uiFS.ReadFile("ui/logo.png")
```

So after the split, `rg "uiFS" beamsync/` must return **exactly 5 hits, all in `server.go`**
(1 declaration + 4 uses). No other file may reference `uiFS`.

#### The embed directory constraint

`//go:embed ui/*.html ui/*.png` resolves relative to the **directory of the file that contains
it** — `beamsync/`. The embedded files are `beamsync/ui/{download.html,logo.png,upload.html}`.
Moving the directive to any other directory breaks the build with
`pattern ui/*.html: no matching files found`. Moving it to another *file in the same directory*
compiles fine but violates D-06.

---

## Shared Patterns

### The Phase-3 hardening that must survive the move (CONTEXT D-05)

**Apply to:** every file in this phase.

| Invariant | Lives at | Moves to | Pinned by |
|---|---|---|---|
| `settingsMu` guards the live `TransferSettings` | `server.go:311` | `server.go` (stays) | `TestUpdateSettingsIsRaceFreeWithConcurrentReaders` (`permissions_test.go:75`) — mutant #14 |
| `Settings()` returns a **snapshot**, not the live pointer | `server.go:342-350` | `server.go` (stays) | `TestSettingsReturnsSnapshotNotLivePointer` (`permissions_test.go:106`) — mutant #15 |
| `UpdateSettings` swaps the whole struct under the write lock | `server.go:356-360` | `server.go` (stays) | same two |
| `rateLimitMiddleware` takes a **settings provider** | `server.go:567` | `middleware.go` | `TestRateLimitMiddlewareReadsCurrentSettings` (`rate_limiter_test.go:234`) — mutant #33 |
| `progressWriter.eventName` | `server.go:433` | `progress.go` | `TestReceivePathEmitsUploadProgress`, `TestSenderPathEmitsDownloadProgress`, `TestMultiFileSenderPathEmitsDownloadProgress` — mutants #1-3 |
| nil-guarded `progressWriter.now` | `server.go:437, 449-451` | `progress.go` | `progress_test.go` (all `TestProgressWriter*`) |
| `progressWriter.Write` zero-byte guard | `server.go:442-446` | `progress.go` | `TestProgressWriterZeroByteWriteEmitsNothing` — mutant #5 |
| `eventDispatcher.stateMu` (RWMutex) | `server.go:45` | `events.go` | `TestEventDispatcherCloseRacesEmitSafely` — mutant #8 (real DATA RACE) |
| `eventDispatcher.close()` idempotent | `server.go:105-113` | `events.go` | `TestEventDispatcherCloseStopsWorkerAndIsIdempotent` — mutant #7 |
| ordered manifest matching (upload) | `server.go:990-999, 1058-1069` | `upload_handler.go` | `TestReceivePathGivesEachFileItsOwnDeclaredSize` — mutant #21 |
| declared size → `progressWriter.total` (upload) | `server.go:1058-1069` | `upload_handler.go` | `TestReceivePathReportsDeclaredTotal` — mutant #20 |
| terminal completion emit (upload) | `server.go:1111` | `upload_handler.go` | `TestReceivePathEmitsCompletionProgress` — mutant #18 |
| terminal completion emit (download) | `server.go:1410, 1479` | `download_handler.go` | `TestMultiFileSenderPathEmitsCompletionProgress` — mutant #19 |

### The mutant table keys on TEST FUNCTION NAMES

`.planning/phases/03-tests-before-refactoring/03-REVIEW.md` (33 mutants, all killed) names the
killing test for each mutant. **Test function names must not be renamed.** Moving a test
function between `_test.go` files in the same package is safe (Go resolves package-scope
identifiers across files). Renaming one silently invalidates the table.

### Error handling — no new error style

**Source:** `beamsync/server.go:1045-1050`, `beamsync/firewall.go:56`, `beamsync/auth_tokens.go:71`
**Apply to:** every moved block.

```go
	// wrap with %w
	return fmt.Errorf("cannot create save folder: %w", err)

	// swallow the error and continue the loop
	dst, createErr := os.Create(dstPath)
	if createErr != nil {
		_, _ = io.Copy(io.Discard, part) //nolint:errcheck,gosec
		part.Close()
		continue
	}

	// nolint where the linter requires it
	if err := os.WriteFile(scriptPath, firewallScript, 0755); err != nil { //nolint:gosec // script needs exec
		return fmt.Errorf("cannot write firewall script: %w", err)
	}
```

Rules:
- `%w` wrapping for sentinel-comparable errors (`errorlint` is enabled).
- `//nolint:errcheck` / `//nolint:gosec` / `//nolint:errorlint` **with a reason comment** when
  the error is deliberately dropped. The existing reason-comment style is
  `//nolint:gosec // script needs exec` — keep the `// reason` suffix.
- `fmt.Printf` / `fmt.Println` for all logging. Emoji prefixes by category
  (`🔑` token, `🛡️` firewall, `🔄` rotation, `⚠️` warnings). No structured logging.

### Panic recovery — every goroutine and every handler

**Source:** `beamsync/server.go:62-67` (dispatcher), `:656-660` (watchdog), `:690-694` (rotator),
`:751-755` (`StartServerE`), `:964-969` (upload handler)

```go
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("PANIC in upload handler: %v\n%s\n", r, debug.Stack())
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}()
```

The dispatcher and the two background goroutines use the short form (no `debug.Stack()`); the
HTTP handlers use the long form (with a 500 response). Keep each variant with its original.

### `gofmt` + golangci-lint

CI (`.github/workflows/ci.yml`) runs, for the `beamsync` module:

```
golangci-lint run ./...          (working-directory: beamsync)
go test -race ./...              (cd beamsync)
go build ./...                   (cd beamsync)
```

Enabled linters (`beamsync/.golangci.yml`): `govet`, `staticcheck`, `errcheck`, `gosec`,
`errorlint`. Implications for the new files:

- Every returned error is checked or explicitly ignored with a `//nolint:errcheck`.
- `os.WriteFile` with a literal permission needs `//nolint:gosec`.
- `fmt.Sprintf` into a header needs `//nolint:gosec` where the value is untrusted
  (precedent: `server.go:869`, `:1140`).
- `gofmt` must be clean — CI blocks on `test -z "$(gofmt -l .)"`.

### The per-module verification gate (CONTEXT D-08)

After **every** extraction step:

```bash
cd beamsync && go build ./... && go vet ./... && go test -race ./...
cd desktop  && go build .    && go vet .    && go test -race .
```

The `desktop/` commands are scoped to `.` (not `./...`) because `go list ./...` in `desktop/`
picks up a stray Go package inside `desktop/frontend/node_modules/flatted/golang/pkg/flatted`
(PITFALLS 5d; see `03-PATTERNS.md`).

### `git mv` and one commit per extraction (CONTEXT D-09)

Each extraction is its own commit so a revert is surgical. Suggested commit shape, matching
the repo's existing style (`test(beamsync): …`, `fix(beamsync): …`, `docs(…): …`):

```
refactor(beamsync): extract the event dispatcher into events.go
refactor(beamsync): extract the client rate limiter into rate_limiter.go
refactor(beamsync): extract progressWriter into progress.go
refactor(beamsync): extract the chunked-copy helpers into io_helpers.go
refactor(beamsync): extract the HTTP middleware into middleware.go
refactor(beamsync): extract the shared helpers into helpers.go
refactor(beamsync): extract the upload handler into upload_handler.go
refactor(beamsync): extract the download handler into download_handler.go
refactor(beamsync): trim server.go to orchestration
```

`git mv` applies to **renamed** files. For the source split, the new files are new files and
`server.go` is edited — `git mv` is not applicable there. `git mv` **is** applicable if a test
file is renamed (e.g. creating `beamsync/io_helpers_test.go` from a chunk of `server_test.go`:
`git mv` the file, then move the function inside it).

### Risk-ascending order (CONTEXT D-07) mapped to files

| Step | File | Risk | Why |
|---|---|---|---|
| 1 | `events.go` | Low | pure move + one package var travelling with its type |
| 2 | `rate_limiter.go` | Low | pure move; type + all methods together |
| 3 | `progress.go` | Low | pure move of a type + one method |
| 4 | `io_helpers.go` | Low | pure move + the pool declaration |
| 5 | `middleware.go` | Low | pure move; the settings-provider signature must not change |
| 6 | `helpers.go` | Low | pure move; the `uiFS` embed does NOT come with it |
| 7 | `upload_handler.go` | **Medium** | closure → method; new `HTTPServer` fields |
| 8 | `download_handler.go` | **Medium** | closure → method; possible de-duplication |
| 9 | trim `server.go` | **Medium** | every remaining reference must be accounted for |

### The exported API must not change

`desktop/app.go` imports `beamsync` and calls `StartServerE`, `StartSenderE`,
`RunFirewallSetup`, `ServerScheme`. `beamsync`'s exported surface — `StartServerE`,
`StartSenderE`, `EventCallback`, `HTTPServer`, `PendingTransfer`, `TransferSettings`,
`TransferHistory`, `TransferRecord`, `TransferStats`, `FindAvailablePort`, `ServerScheme`,
`TLSEnabled`, `NewTransferHistory`, `DefaultTransferSettings`, `FirewallManualSteps`,
`RunFirewallSetup`, `GetLocalIP` — is unchanged by this phase. Every moved symbol is
unexported or already exported and unchanged.

Verify:

```bash
rg -n "beamsync\.\w+" desktop/ --type go
```

The result must be byte-identical before and after the phase.

---

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `beamsync/upload_handler.go` | controller | request-response + file-I/O + streaming | **No method-based HTTP handler exists in this repo.** `rg "http.HandlerFunc"` finds only the three middleware constructors in `server.go` (`:519, :536, :567`) — all free functions, none a method. The closest precedent for the *shape* is `desktop/app.go:463` `func (a *App) handleIPChange(currentRealIP string) string` — a method on a struct that owns its state — but it is not an HTTP handler. **Use PITFALLS §1c as the guide**, and copy the middleware-chain shape from `server.go:963`. |
| `beamsync/download_handler.go` | controller | request-response + file-I/O + streaming | Same as above. Its *body* has a strong analog in the repo — the `/upload` body it is a near-mirror of — but there is no second method-based handler to copy the conversion from. |
| `beamsync/localip_test.go` | test | n/a | **Does not exist** (correction C-2). CONTEXT names it as the "file plus colocated test" exemplar; the tree has `localip.go` with no test. Use `port_manager.go` + `port_manager_test.go` (32 + 46 lines) as the tightest source+test pair instead. |
| `beamsync/middleware_test.go` | test | request-response | Does not exist. Optional new file if the five `TestRateLimitMiddleware*` functions are split out of `rate_limiter_test.go`. |
| `beamsync/io_helpers_test.go` | test | file-I/O | Does not exist, but was specified by ROADMAP Phase 3 plan 3.3. Optional: move `TestCopyChunkedCopiesDataAndReportsCount` out of `server_test.go` into it. |
| `beamsync/helpers_test.go` | test | transform + file-I/O | Does not exist. Optional: move `TestAutoRenamePathFindsNonCollidingName`, `TestGenerateIDReturnsHexValue`, `TestGenerateIDDoesNotCollideInSmallSample` out of `server_test.go`. |
| `beamsync/events_test.go` | test | event-driven | Does not exist. The dispatcher tests already live in `event_dispatcher_test.go` — keep that name (it is the established name and `event_dispatcher_test.go` already pairs 1:1 with the dispatcher concept). |

---

## Metadata

**Analog search scope:**
`beamsync/*.go` (23 files: 11 source, 12 test), `beamsync/audio/audio.go`,
`desktop/app.go`, `desktop/app_test.go`, `.github/workflows/ci.yml`,
`.planning/research/PITFALLS.md` §1a-1e, `.planning/codebase/CONVENTIONS.md`,
`.planning/phases/03-tests-before-refactoring/03-REVIEW.md` (mutant table),
`.planning/ROADMAP.md` (Phase 4), `.planning/REQUIREMENTS.md` (STR-01…STR-09),
`CONTRIBUTING.md`.

**Files scanned:** 11 `beamsync` source files, 12 `beamsync` test files, `desktop/app.go`,
1 CI workflow, 6 planning documents.

**Analogs read in full:** `beamsync/server.go` (1538), `beamsync/localip.go` (76),
`beamsync/auth_tokens.go` (180), `beamsync/permissions.go` (80), `beamsync/history.go` (100),
`beamsync/stats.go` (115), `beamsync/tls.go` (150), `beamsync/port_manager.go` (32),
`beamsync/firewall.go` (85), `beamsync/server_test.go` (554),
`beamsync/rate_limiter_test.go` (270), `beamsync/progress_test.go` (517),
`beamsync/progress_integration_test.go` (193), `beamsync/progress_wiring_test.go` (528),
`beamsync/event_dispatcher_test.go` (260).

**Verification performed (read-only):**
- `rg "defaultEventDispatcher" beamsync/` → 1 declaration (`server.go:227`) + 1 usage
  (`server.go:732`). Matches PITFALLS 1a exactly (the "12+ call sites" figure in PITFALLS is
  stale; CONTEXT D-02 already re-derived it as 1).
- `rg "chunkBufferPool" beamsync/` → 1 declaration (`server.go:244`) + 2 uses (`:463`, `:464`).
- `rg "uiFS" beamsync/` → 1 declaration (`server.go:242`) + 4 uses, **all in `server.go`**
  (`:827, :853, :1321, :1346`).
- `rg "serverState" beamsync/` → declaration + 5 methods + 1 field + 2 constructor sites +
  1 test — all inside `server.go` except `server_test.go:245`.
- `grep -rn "endpointMu\|serverMu\|stateMu\|settingsMu" desktop/ beamsync/` →
  `endpointMu`/`serverMu`/`stateMu` **only** in `desktop/app.go`; `settingsMu` only in
  `beamsync/server.go`. (Correction C-1.)
- `ls beamsync/localip*` → `localip.go` only, no test file. (Correction C-2.)
- Test-function inventory per `_test.go` file (56 test functions across 12 files) — used for
  the mutant-table cross-check.
- Import-block derivation for all 8 new files + the trimmed `server.go` — each import traced
  to the exact line that needs it.

**Pattern extraction date:** 2026-11-13
