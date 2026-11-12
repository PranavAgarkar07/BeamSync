# Phase 03: Tests Before Refactoring — Pattern Map

**Mapped:** 2026-11-12
**Files analyzed:** 8 (5 new, 3 modified)
**Analogs found:** 6 / 8 (2 config files have no in-repo analog)

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `beamsync/event_dispatcher_test.go` (NEW) | test | event-driven (channel queue + worker goroutine) | `beamsync/rate_limiter_test.go` (style/injection) + `beamsync/server_test.go` (async wait) | role-match (exact) |
| `beamsync/progress_test.go` (NEW) | test | event-driven (callback emit) + streaming write | `beamsync/rate_limiter_test.go` (clock injection) + `beamsync/server_test.go` (assert style) | role-match (exact) |
| `beamsync/server.go` (MODIFIED) | model (writer) | streaming | self, `clientRateLimiter` at `beamsync/server.go:100-117` | exact (same-file precedent) |
| `desktop/app_test.go` (NEW) | test | transform (pure string fns) + file-I/O (config round-trip) | `beamsync/server_test.go` (`t.TempDir`/`t.Setenv`) + `beamsync/tls_test.go:61` (`t.Setenv("HOME", …)`) | role-match (cross-module) |
| `desktop/frontend/dist/.gitkeep` (NEW) | config/placeholder | n/a | none — no `.gitkeep` exists in repo | none |
| `desktop/.gitignore` (MODIFIED) | config | n/a | none | none |
| `.gitignore` (root) (MODIFIED) | config | n/a | none | none |
| `.github/workflows/ci.yml` (MODIFIED) | config | n/a | none | none |

> **Scope note:** the root `.gitignore` is **not** in CONTEXT.md's file list, but it must change
> alongside `desktop/.gitignore` — see the `desktop/frontend/dist/.gitkeep` section for the
> empirically-verified reason. This is a required addition to the plan's file list.

---

## Pattern Assignments

### `beamsync/event_dispatcher_test.go` (NEW — test, event-driven)

**Analog:** `beamsync/rate_limiter_test.go` (assertion style, no external deps)
**Analog:** `beamsync/server_test.go` (async channel wait pattern)
**Production code under test:** `beamsync/server.go:28-84`

#### Imports pattern

`beamsync/rate_limiter_test.go:1-10` — stdlib only, no assertion library:

```go
package beamsync

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)
```

Required for this file: `package beamsync` (white-box — `eventDispatcher` and `eventDispatchJob`
are unexported), plus `sync`, `sync/atomic`, `testing`, `time`.

> **Constraint from `.planning/research/PITFALLS.md` §2a:** the package clause MUST be
> `package beamsync`, NOT `package beamsync_test`. The dispatcher type is unexported.

#### Production code under test — `beamsync/server.go:28-84`

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
```

#### Black-box seam (D-04) — do NOT read `d.queue`

The only permitted assertions are:

| Assertion | API |
|---|---|
| enqueue succeeded | `d.emit(job) == true` |
| enqueue dropped | `d.emit(job) == false` |
| drop counter | `d.DroppedCount() int64` |

`d.queue` is `chan eventDispatchJob` and is about to move to `beamsync/events.go` in Phase 4 —
reading it couples the test to a representation that changes.

#### Core pattern — async delivery wait

Copy this from `beamsync/server_test.go:143-147` (select-with-timeout, never `time.Sleep`):

```go
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("safeEmit did not invoke callback")
	}
```

And the channel-name-prefix variant at `beamsync/server_test.go:464-477`:

```go
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
```

#### Core pattern — blocking callback to force deterministic drops (D-05)

`newEventDispatcher` starts `run()` immediately (`beamsync/server.go:50`), so the queue drains
unless the callback blocks. To get a *deterministic* drop count, block inside the callback:

```go
// Blocking the callback stalls the single dispatcher goroutine, so the queue
// fills to its buffer size and every further emit() must return false.
d := newEventDispatcher(1)
release := make(chan struct{})
d.emit(eventDispatchJob{emit: func(event, data string) {
	<-release
}})
```

D-05 requires an **exact** accounting assertion — the number of `emit()` calls that returned
`false` must equal `d.DroppedCount()`:

```go
var failed atomic.Int64
// ... concurrently emit N jobs ...
if got, want := d.DroppedCount(), failed.Load(); got != want {
	t.Fatalf("DroppedCount() = %d, want %d (failed emit count)", got, want)
}
```

Use `sync/atomic` (already imported in `beamsync/server.go:24`) for the shared counter — the
concurrency test runs under `-race` (D-13), so a plain `int++` would itself be a data race and
fail CI.

#### Core pattern — panic recovery survives (D-06)

```go
d := newEventDispatcher(8)
d.emit(eventDispatchJob{emit: func(event, data string) { panic("boom") }})

delivered := make(chan string, 1)
d.emit(eventDispatchJob{emit: func(event, data string) { delivered <- event }})

select {
case got := <-delivered:
	if got != "later_event" {
		t.Fatalf("delivered event = %q, want %q", got, "later_event")
	}
case <-time.After(time.Second):
	t.Fatal("dispatcher did not survive the panicking callback")
}
```

`run()`'s `recover()` (`beamsync/server.go:57-61`) prints `"Event callback panic: %v\n"` to stdout —
**do not assert on stdout**; assert on the *survival* of the next event instead.

#### Assertion style

From `beamsync/server_test.go` (repo-wide convention — no assertion library):

```go
if got != want {
	t.Fatalf("autoRenamePath = %q, want %q", got, want)
}
```
```go
if len(token) != 32 {
	t.Fatalf("token length = %d, want 32", len(token))
}
```

`%q` for strings, `%d` for ints, `%#v` for structs (`beamsync/server_test.go:460`).

#### Existing concurrency-test precedent

`beamsync/history_test.go:169-187` — the repo's existing concurrency test shape (goroutine +
`close(done)` + `select`/`default` spin):

```go
func TestTransferHistoryConcurrentAccess(t *testing.T) {
	history := NewTransferHistory(100)
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			history.Add(TransferRecord{Filename: "concurrent.txt"})
		}
		close(done)
	}()

	for {
		select {
		case <-done:
			return
		default:
			history.List()
		}
	}
}
```

#### Test-helper placement

Per CONTEXT "The agent's Discretion": keep helpers in the test file that uses them.
Precedent: `newTokenStoreForTest` in `beamsync/auth_tokens_test.go:9-16`,
`freePort`/`startRawServerForTest` in `beamsync/server_test.go:496-542`. All helpers call
`t.Helper()` first:

```go
func newTokenStoreForTest(t *testing.T) *tokenStore {
	t.Helper()
	store, err := newTokenStore("test-fingerprint")
	if err != nil {
		t.Fatalf("newTokenStore: %v", err)
	}
	return store
}
```

No `export_test.go` needed (Go allows any file in the same package to use the helpers).

---

### `beamsync/progress_test.go` (NEW — test, event-driven + streaming)

**Analog:** `beamsync/rate_limiter_test.go` (clock injection — the exact pattern D-09 mandates)
**Analog:** `beamsync/server_test.go` (assert style, `t.TempDir`, `bytes.Buffer`)
**Production code under test:** `beamsync/server.go:362-381`

#### Production code under test — `beamsync/server.go:362-381` (current, pre-fix)

```go
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
	n, err := pw.w.Write(p) //nolint:errcheck,gosec
	pw.written += int64(n)
	now := time.Now()
	if now.Sub(pw.lastEmit) >= pw.minInterval {
		pw.emit("upload_progress", fmt.Sprintf("%s|%d|%d", pw.filename, pw.written, pw.total))
		pw.lastEmit = now
	}
	return n, err
}
```

#### Clock-injection pattern (D-09) — the analog to copy

`beamsync/rate_limiter_test.go:12-26` is the precedent D-09 names explicitly:

```go
func TestClientRateLimiterBlocksAfterLimit(t *testing.T) {
	now := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	limiter := newClientRateLimiter(2, time.Minute)
	limiter.now = func() time.Time { return now }

	if decision := limiter.allow("192.168.1.10"); !decision.allowed {
		t.Fatal("first request should be allowed")
	}
	...
	now = now.Add(time.Minute)   // advance the injected clock, no real sleep
	...
}
```

And the production side it mirrors — `beamsync/server.go:100-117`:

```go
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
```

`progressWriter` has **no constructor function** — it is built with struct literals at three
sites — so the `now: time.Now` default must be written explicitly at each site (see
`beamsync/server.go` MODIFIED below).

#### Test split (D-09)

**(a) Payload/behavior test — `minInterval = 0`.** With `minInterval == 0`,
`now.Sub(pw.lastEmit) >= 0` is always true, so every `Write` emits. No clock needed:

```go
var got []string
pw := &progressWriter{
	w:           &bytes.Buffer{},
	total:       100,
	filename:    "note.txt",
	emit:        func(event, data string) { got = append(got, event+"|"+data) },
	minInterval: 0,
}
if _, err := pw.Write([]byte("hello")); err != nil {
	t.Fatalf("Write returned error: %v", err)
}
```

Expected payload shape: `"<eventName>|<filename>|<written>|<total>"` — i.e.
`fmt.Sprintf("%s|%d|%d", pw.filename, pw.written, pw.total)` produces `note.txt|5|100`.

**(b) Coalescing test — injected clock.** Set `now` to a fixed instant, write, advance the
clock by less than `minInterval`, write again → second write must NOT emit; advance past
`minInterval`, write → must emit.

> **Critical detail — `lastEmit` zero value.** `progressWriter.lastEmit` is the zero `time.Time`
> unless set explicitly. `now.Sub(zeroTime)` is ~2026 years, so the **first** write always emits
> regardless of `minInterval`. Two of the three production construction sites set
> `lastEmit: time.Now()` (`beamsync/server.go:1280`, `:1346`) but the receive path
> (`beamsync/server.go:957-962`) does **not**. Tests must therefore either set `lastEmit`
> explicitly to the injected `now` (to test coalescing) or rely on the zero-value behavior
> (to test first-write-always-emits). Plan must pin this down explicitly — it is the difference
> between a passing and a flaky test.

#### Event-name assertion (D-07 / D-08) — the actual bug

`progressWriter.Write` hardcodes `"upload_progress"` (`beamsync/server.go:377`). The desktop
send-direction handler listens for `"download_progress"`:

`desktop/frontend/src/App.svelte:458-459`
```js
    EventsOn("upload_progress", (data) => handleProgressUpdate(data, "receive"));
    EventsOn("download_progress", (data) => handleProgressUpdate(data, "send"));
```

So `download_progress` is emitted by **nothing** — send-direction progress is dead in the
shipped app. The test must assert the **event name**, not just the payload (D-08).

Assert both names:

```go
if got[0] != "upload_progress|note.txt|5|100" {
	t.Fatalf("receive-path event = %q, want %q", got[0], "upload_progress|note.txt|5|100")
}
```

#### Existing `bytes.Buffer` sink precedent

`beamsync/server_test.go:118-132` — the repo's existing way to sink writes and count bytes:

```go
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
```

> **Nil-guard requirement.** `pw.emit` is called unconditionally at `beamsync/server.go:377`.
> Every test MUST set `emit` to a non-nil func, or the test panics with a nil-pointer
> dereference instead of a clean failure.

---

### `beamsync/server.go` (MODIFIED — model, streaming)

**Analog for the new field:** `clientRateLimiter` at `beamsync/server.go:100-117` (same file).

#### Change 1 — add `eventName` and `now` to `progressWriter` (`beamsync/server.go:362-370`)

```go
type progressWriter struct {
	w           io.Writer
	total       int64
	written     int64
	filename    string
	eventName   string            // NEW: "upload_progress" | "download_progress"
	emit        func(string, string)
	lastEmit    time.Time
	minInterval time.Duration
	now         func() time.Time  // NEW: clock injection (mirrors clientRateLimiter.now)
}
```

#### Change 2 — use the injected clock and the injected event name in `Write` (`beamsync/server.go:372-381`)

```go
func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.w.Write(p) //nolint:errcheck,gosec
	pw.written += int64(n)
	now := pw.now()
	if now.Sub(pw.lastEmit) >= pw.minInterval {
		pw.emit(pw.eventName, fmt.Sprintf("%s|%d|%d", pw.filename, pw.written, pw.total))
		pw.lastEmit = now
	}
	return n, err
}
```

> **Defensive nil-guard required.** Because `progressWriter` is built by struct literal (not a
> constructor), a zero-value `now` field is `nil` → `pw.now()` panics. Either:
> (a) set `now: time.Now` at all three construction sites, **and** guard in `Write`:
> ```go
> now := time.Now
> if pw.now != nil {
> 	now = pw.now
> }
> ```
> or (b) add a `newProgressWriter(...)` constructor. Option (a) is smaller and matches the
> existing struct-literal style at the three call sites. The plan MUST choose one — a nil
> `pw.now` is a latent panic in production.

#### Change 3 — set `eventName` + `now` at all THREE construction sites

| Site | Path | `eventName` | Notes |
|---|---|---|---|
| `beamsync/server.go:957-962` | receive path (upload) | `"upload_progress"` | **No `lastEmit` set** — zero value; first write always emits. Also **no `total`** set (total stays 0 during streaming). |
| `beamsync/server.go:1275-1282` | single-file send (download) | `"download_progress"` | Sets `total: fileInfo.Size()` and `lastEmit: time.Now()`. |
| `beamsync/server.go:1341-1348` | multi-file send (download) | `"download_progress"` | Sets `total: fileInfo.Size()` and `lastEmit: time.Now()`. |

Site 1 today (receive path):
```go
			diskBuf := bufio.NewWriterSize(dst, 8*1024*1024)
			pw := &progressWriter{
				w:           diskBuf,
				filename:    savedName,
				emit:        emit,
				minInterval: 500 * time.Millisecond,
			}
```
→ becomes:
```go
			pw := &progressWriter{
				w:           diskBuf,
				filename:    savedName,
				eventName:   "upload_progress",
				emit:        emit,
				lastEmit:    time.Now(),
				minInterval: 500 * time.Millisecond,
				now:         time.Now,
			}
```

Site 2 today (single-file send):
```go
			pw := &progressWriter{
				w:           w,
				total:       fileInfo.Size(),
				filename:    filename,
				emit:        emit,
				lastEmit:    time.Now(),
				minInterval: 500 * time.Millisecond,
			}
```
→ becomes (adds `eventName: "download_progress"` and `now: time.Now`).

Site 3 today (multi-file send) is identical in shape to site 2 but uses `realName` → becomes
`eventName: "download_progress"`, `now: time.Now`.

> **Missed-site risk (CONTEXT "Integration Points").** A missed site reproduces D-07.
> The plan should add a grep-based verification step:
> `grep -n "progressWriter{" beamsync/server.go` must return exactly 3 hits, and
> `grep -n "eventName:" beamsync/server.go` must return exactly 3 hits.

#### Related emit that is NOT a `progressWriter` — do not touch

`beamsync/server.go:995` emits the final receive-path progress directly:
```go
			emit("upload_progress", fmt.Sprintf("%s|%d|%d", savedName, written, written))
```
This is already correct (`upload_progress` on the receive path) and must stay as-is.

#### Test-first ordering (D-07)

CONTEXT "Specific Ideas" requires the failing test to land **before** the fix so the fix is
provably what turned it green. Plan order per site: (1) write `progressWriter` tests asserting
per-path event names → (2) run and observe failure → (3) apply `server.go` change → (4) re-run.

---

### `desktop/app_test.go` (NEW — test, transform + file-I/O)

**Analog:** `beamsync/server_test.go` (repo test style, `t.TempDir`, `t.Setenv`)
**Analog:** `beamsync/tls_test.go:57-63` (HOME redirection for filesystem-rooted code)
**Production code under test:** `desktop/app.go:49-55, 106-175, 265-292`

> **No analog exists inside `desktop/` — this is the first test file for that module.**
> `desktop/` currently contains exactly two Go files (`desktop/main.go`, `desktop/app.go`), both
> `package main`, and **zero** `*_test.go` files. The analog to copy is therefore the *style*
> from `beamsync/`, not a same-module precedent. `desktop/go.mod` is `module desktop` with
> `replace beamsync => ../beamsync`, so `beamsync` symbols are importable from the test.

#### Package clause

```go
package main
```

`desktop/app.go:1` is `package main`. The pure functions are unexported
(`shareURL`, `configPath`, `loadConfig`, `saveConfig`, `ensureSaveDir`,
`extractTokenFromURL`, `extractPortFromURL`), so the test must be `package main`
(not `main_test`) to reach them. `&App{}` can be constructed directly — no Wails runtime
context is needed (D-10):

```go
app := &App{currentIP: "192.168.1.10"}
```

#### Pure functions under test — `desktop/app.go:49-55, 265-292`

```go
func (a *App) shareURL(port, token string) string {
	ip := a.currentIP
	if ip == "" {
		ip = getLocalIP()
	}
	return fmt.Sprintf("%s://%s:%s/?token=%s", beamsync.ServerScheme(), ip, port, token)
}
```

```go
func extractTokenFromURL(raw string) string {
	idx := strings.Index(raw, "token=")
	if idx == -1 {
		return ""
	}
	rest := raw[idx+6:]
	if amp := strings.Index(rest, "&"); amp != -1 {
		rest = rest[:amp]
	}
	if hash := strings.Index(rest, "#"); hash != -1 {
		rest = rest[:hash]
	}
	return strings.TrimSpace(rest)
}

func extractPortFromURL(raw string) string {
	withoutScheme := raw
	if idx := strings.Index(raw, "://"); idx != -1 {
		withoutScheme = raw[idx+3:]
	}
	if slash := strings.Index(withoutScheme, "/"); slash != -1 {
		withoutScheme = withoutScheme[:slash]
	}
	if colon := strings.LastIndex(withoutScheme, ":"); colon != -1 {
		return withoutScheme[colon+1:]
	}
	return ""
}
```

Both extractors are **pure string functions with no I/O** — the ideal table-driven targets.

#### Table-driven pattern (repo convention)

From `.planning/codebase/TESTING.md` (established pattern) and mirrored by
`beamsync/tls_test.go`:

```go
func TestExtractTokenFromURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "plain query", raw: "http://host:8080/?token=abc123", want: "abc123"},
		{name: "ampersand terminated", raw: "http://host/?token=abc&x=1", want: "abc"},
		{name: "hash terminated", raw: "http://host/?token=abc#frag", want: "abc"},
		{name: "missing token", raw: "http://host:8080/", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractTokenFromURL(tc.raw); got != tc.want {
				t.Fatalf("extractTokenFromURL(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}
```

> The repo also uses the subtest-in-a-loop form (`beamsync/tls_test.go:104-137` style, cited in
> TESTING.md as "Certificate host tests use subtests inside a loop"). Either form is acceptable;
> pick one and use it consistently.

#### `shareURL` — deterministic IP is required

`shareURL` falls back to `getLocalIP()` (`desktop/app.go:52`) when `a.currentIP` is empty.
`getLocalIP()` (`desktop/app.go:849-855`) calls `beamsync.GetLocalIP()` — a **live network
lookup**. A test that leaves `currentIP` empty would be non-deterministic (and can print the
`"⚠️ Could not detect LAN IP"` warning). Always set `currentIP` explicitly:

```go
app := &App{currentIP: "192.168.1.10"}
got := app.shareURL("8080", "tok")
want := fmt.Sprintf("%s://192.168.1.10:8080/?token=tok", beamsync.ServerScheme())
```

`beamsync.ServerScheme()` (`beamsync/tls.go:29-34`) depends on the `BEAMSYNC_ENABLE_TLS` env
var. For a deterministic `"http"` expectation, pin it:

```go
t.Setenv("BEAMSYNC_ENABLE_TLS", "")
```
`TLSEnabled()` (`beamsync/tls.go:24-27`) treats `""` as false → `ServerScheme() == "http"`.
Precedent: `beamsync/server_test.go:315` (`t.Setenv(tlsEnvVar, "")`) and
`beamsync/tls_test.go:14, 22`.

#### Config round-trip — `desktop/app.go:106-149`

```go
func configPath() string {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		cfgDir = filepath.Join(os.TempDir(), ".config")
	}
	return filepath.Join(cfgDir, "beamsync", "config.json")
}

func loadConfig() configData {
	var cfg configData
	data, err := os.ReadFile(configPath())
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, &cfg)

	if cfg.SavePath != "" {
		home, _ := os.UserHomeDir()
		cleanSave := filepath.Clean(cfg.SavePath)
		cleanHome := filepath.Clean(home)
		if stdruntime.GOOS == "windows" {
			// NTFS paths are case-insensitive; C:\Users\x must match c:\users\x.
			cleanSave = strings.ToLower(cleanSave)
			cleanHome = strings.ToLower(cleanHome)
		}
		if home != "" && !strings.HasPrefix(cleanSave, cleanHome) {
			cfg.SavePath = ""
		}
	}

	return cfg
}

func saveConfig(cfg configData) error {
	p := configPath()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}
```

**Critical: `configPath()` is rooted at the OS user-config dir, so the test must redirect it.**
`os.UserConfigDir()` resolves differently per platform:

| Platform | Resolution |
|---|---|
| linux/BSD | `$XDG_CONFIG_HOME` if set, else `$HOME/.config` |
| darwin | `$HOME/Library/Application Support` |
| windows | `%AppData%` |

Therefore the test must set **all three** env vars to the same temp root so the test is
platform-independent:

```go
home := t.TempDir()
t.Setenv("HOME", home)                               // unix + os.UserHomeDir()
t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config")) // linux
t.Setenv("AppData", home)                            // windows os.UserConfigDir()
```

Precedent for HOME redirection: `beamsync/tls_test.go:57-63`:

```go
func TestLoadOrCreateLocalCertificatePersistsSecureECDSAFiles(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	...
}
```

Also note `t.Setenv` **fails the test if the test uses `t.Parallel()`** — do not mark these
tests parallel.

**Second critical detail — the home-prefix guard.** `loadConfig` clears `SavePath` unless it is
under `os.UserHomeDir()`. With `HOME` redirected to the temp dir, a `SavePath` *inside* that
temp dir survives the round-trip, and one *outside* it (e.g. `/etc/passwd`) is cleared. Both
branches are testable and are the strongest assertions available:

```go
// survives: under the (redirected) home
cfg := configData{SavePath: filepath.Join(home, "Downloads", "BeamSync")}
if err := saveConfig(cfg); err != nil {
	t.Fatalf("saveConfig returned error: %v", err)
}
got := loadConfig()
if got.SavePath != cfg.SavePath {
	t.Fatalf("loadConfig().SavePath = %q, want %q", got.SavePath, cfg.SavePath)
}
```

> `configData.TransferSettings` is `beamsync.TransferSettings` (`desktop/app.go:103`) — the
> `desktop` module imports `beamsync` via `replace beamsync => ../beamsync` (`desktop/go.mod`),
> so it is usable in the test struct literal without extra setup.

#### `ensureSaveDir` — `desktop/app.go:159-175`

```go
func ensureSaveDir(path string) error {
	if path == "" {
		return fmt.Errorf("empty save path")
	}
	info, err := os.Stat(path)
	if err == nil && !info.IsDir() {
		backup := path + ".bin.bak"
		if err := os.Rename(path, backup); err != nil {
			// Rename failed (e.g. backup exists) — remove stray file and recreate
			_ = os.Remove(path)
			fmt.Printf("⚠️ Save path was a file, removed %s (backup %s exists)\n", path, backup)
		} else {
			fmt.Printf("⚠️ Save path was a file, moved to %s\n", backup)
		}
	}
	return os.MkdirAll(path, 0755)
}
```

Three testable branches: (1) empty path → error; (2) path is an existing **file** → renamed to
`path + ".bin.bak"` and replaced by a directory; (3) path does not exist → created as a
directory. Use `t.TempDir()` as the root (precedent: `beamsync/server_test.go:66`,
`beamsync/server_test.go:316`).

> `ensureSaveDir` prints to stdout on the file branch — do not assert on stdout.

---

### `desktop/frontend/dist/.gitkeep` (NEW) + `.gitignore` changes

**Analog:** none — the repo contains **zero** `.gitkeep` files today (verified via
`find . -name .gitkeep`). This is a new pattern for this repo; it follows the standard
"placeholder for a git-tracked empty directory" idiom.

#### The problem (D-11)

`desktop/main.go:11-12`:
```go
//go:embed all:frontend/dist
var assets embed.FS
```

`desktop/.gitignore:3` ignores `frontend/dist`, so a fresh clone has no `desktop/frontend/dist`
directory → `go build`/`go test` fails with
`pattern all:frontend/dist: no matching files found`.
The `all:` prefix includes dotfiles, so a single `.gitkeep` satisfies the embed pattern.

#### BOTH `.gitignore` files must change — verified empirically

The root `.gitignore:20` also excludes the directory:
```
desktop/frontend/dist/
```
Git cannot re-include a file whose **parent directory** is excluded, so a negation in
`desktop/.gitignore` alone is NOT sufficient.

Empirically verified (throwaway repo, `/tmp/opencode/gitcheck2`):

| Config | `git check-ignore -v sub/dist/.gitkeep` | `git add -A` result |
|---|---|---|
| root `sub/dist/` + sub `dist` (current shape) | **ignored** → `sub/.gitignore:1:dist` | `.gitkeep` not staged |
| root `sub/dist/*` + `!sub/dist/.gitkeep` + sub `dist/*` + `!dist/.gitkeep` | **not ignored** (negation wins) | `A sub/dist/.gitkeep` only — `index.html` still ignored |

**Required edit — root `.gitignore` line 20:**
```diff
-desktop/frontend/dist/
+desktop/frontend/dist/*
+!desktop/frontend/dist/.gitkeep
```

**Required edit — `desktop/.gitignore` line 3:**
```diff
-frontend/dist
+frontend/dist/*
+!frontend/dist/.gitkeep
```

**Required new file — `desktop/frontend/dist/.gitkeep`:** empty file.

> The `dist/*` form (rather than keeping `dist/` and adding a negation) is **mandatory** —
> with the directory form the parent stays excluded and git never descends into it.
> Verified: `git check-ignore -v` reports the `!dist/.gitkeep` negation and `git status`
> shows only `.gitkeep` staged; `dist/index.html` remains ignored.

#### Files that must remain ignored

`desktop/frontend/dist/assets/`, `dist/index.html`, `dist/favicon.png` etc. — the `dist/*` form
plus a single `!…/.gitkeep` negation keeps them ignored (verified above).

---

### `.github/workflows/ci.yml` (MODIFIED — config)

**Analog:** none — CI config has no in-repo analog beyond itself.

#### Current `test-go` job (`.github/workflows/ci.yml:35-46`)

```yaml
  test-go:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      - uses: actions/setup-go@v6
        with: { go-version: '1.25' }
      - uses: actions/setup-node@v5
        with: { node-version: '24' }
      - run: sudo apt-get update && sudo apt-get install -y libasound2-dev
      - run: cd desktop/frontend && npm ci && npm run build
      - run: (cd beamsync && go test ./...)
      - run: (cd desktop && go test ./...)
```

#### Required changes (D-12, D-13)

```yaml
      - run: (cd beamsync && go test -race ./...)
      - run: (cd desktop && go test -race .)
```

> `-race` on `beamsync` is **required** for D-05 to mean anything — a concurrency test without
> `-race` cannot detect a data race. D-13 states the current suite already passes under `-race`
> (verified 2026-11-12).

#### The `desktop/` scope problem (D-12) — verified locally

```
$ cd desktop && go list ./
desktop
$ cd desktop && go list ./...
desktop
desktop/frontend/node_modules/flatted/golang/pkg/flatted
```

`go list ./...` in `desktop/` picks up the Go package vendored inside npm dependencies
(`desktop/frontend/node_modules/flatted/golang/pkg/flatted/flatted.go`). Go does **not** skip
`node_modules` (only `.`-prefixed, `_`-prefixed, and `testdata` directories are excluded).
`go list .` returns only the root package. The same scoping is needed for the other
`desktop/` Go commands in the workflow:

| Workflow location | Current | Should become |
|---|---|---|
| `.github/workflows/ci.yml:46` (`test-go`) | `(cd desktop && go test ./...)` | `(cd desktop && go test -race .)` |
| `.github/workflows/ci.yml:68` (`build`) | `(cd desktop && go build ./...)` | `(cd desktop && go build .)` |
| `.github/workflows/ci.yml:23-24` (`lint-go`, golangci-lint action) | `args: ./...` + `working-directory: desktop` | `args: .` + `working-directory: desktop` |

> The `lint-go` job's `golangci-lint` invocation at `.github/workflows/ci.yml:23-24` also uses
> `./...` with `working-directory: desktop`, so it currently lints the `flatted` package too.
> D-12 says "Apply the same scoping to `go build`/`go vet` for `desktop/`" — the lint action is
> the de-facto `go vet` runner here, so it should be scoped to `.` as well. The `beamsync`
> lint step (`:17-18`) stays `./...` — that module has no nested stray packages.

#### Optional cleanup enabled by D-11

The `test-go` job's `- run: cd desktop/frontend && npm ci && npm run build` step
(`.github/workflows/ci.yml:44`) is no longer *required* for `go test` to compile, because
`desktop/frontend/dist/.gitkeep` satisfies the embed pattern. Removing it would make the
`test-go` job faster and independent of Node. This is **optional** — the plan may keep it. The
`lint-svelte` and `build` jobs still need their frontend build.

---

## Shared Patterns

### Assertion style — Go stdlib `testing`, no assertion library

**Source:** `beamsync/server_test.go`, `beamsync/rate_limiter_test.go`, `beamsync/history_test.go`
**Apply to:** all four new/modified test files

```go
if got != want {
	t.Fatalf("%s = %q, want %q", name, got, want)
}
```
```go
if len(records) != 2 {
	t.Fatalf("expected capped history length 2, got %d", len(records))
}
```
```go
if rec.Code != http.StatusForbidden {
	t.Fatalf("%s returned %d, want %d", target, rec.Code, http.StatusForbidden)
}
```

Rules observed repo-wide:
- `t.Fatalf` for assertion failures (stops the test); `t.Fatal` for unrecoverable setup errors.
- Message format: `<subject> = <got>, want <want>` — got first, want second.
- `%q` for strings, `%d` for ints, `%#v` / `%+v` for structs, `%v` for errors.
- Every helper starts with `t.Helper()`.

### Clock injection instead of `time.Sleep`

**Source:** `beamsync/rate_limiter_test.go:12-26` + `beamsync/server.go:115`
**Apply to:** `beamsync/progress_test.go`, `beamsync/event_dispatcher_test.go` (if any timing), `desktop/app_test.go`

```go
now := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
limiter.now = func() time.Time { return now }
...
now = now.Add(time.Minute)   // advance the injected clock; never sleep
```

Production side mirrors it with a `now func() time.Time` struct field defaulting to `time.Now`
(`beamsync/server.go:106, 115`; same pattern at `beamsync/auth_tokens.go:56, 63` — `now func() time.Time`
defaulting to `time.Now` — used in `beamsync/auth_tokens_test.go:50-53`).

### Async assertions — channel + timeout, never bare sleeps

**Source:** `beamsync/server_test.go:143-147`, `beamsync/server_test.go:464-477`
**Apply to:** `beamsync/event_dispatcher_test.go`, `beamsync/progress_test.go`

```go
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("safeEmit did not invoke callback")
	}
```

### Filesystem isolation — `t.TempDir()` and `t.Setenv()`

**Source:** `beamsync/server_test.go:66`, `beamsync/server_test.go:316`, `beamsync/tls_test.go:61`
**Apply to:** `desktop/app_test.go`, `beamsync/progress_test.go`

```go
dir := t.TempDir()
if err := os.WriteFile(filepath.Join(dir, "photo.png"), []byte("first"), 0600); err != nil { //nolint:gosec
	t.Fatalf("write existing file: %v", err)
}
```

Note the `//nolint:gosec` comment on `os.WriteFile` with a `0600` literal — `gosec` is enabled
in both `beamsync/.golangci.yml` and `desktop/.golangci.yml`, and the same annotation appears at
`beamsync/server_test.go:67` and `:317`.

### No mocking framework

**Source:** `.planning/codebase/TESTING.md`
**Apply to:** all new test files

Real implementations + `t.TempDir()` + `t.Setenv()` + injectable `now` funcs. No gomock/testify/mock.
No `httptest.NewServer` unless the test genuinely needs a live server —
`beamsync/server_test.go` uses `httptest.NewRecorder()` / `httptest.NewRequest()` for handler
tests (`beamsync/server_test.go:26-29`, `beamsync/rate_limiter_test.go:83-84`).

### Lint configuration

Both modules run golangci-lint v1.64 with `govet`, `staticcheck`, `errcheck`, `gosec`,
`errorlint` enabled (`beamsync/.golangci.yml`, `desktop/.golangci.yml`). Implications for new test code:

- `errcheck` — every returned error must be checked or explicitly ignored with `//nolint:errcheck`
  (e.g. `beamsync/server_test.go:328`: `defer func() { _ = server.Shutdown() }() //nolint:errcheck`).
- `gosec` — `os.WriteFile` with a literal permission needs `//nolint:gosec`.
- `errorlint` — compare errors with `errors.Is`, not `==`
  (precedent: `beamsync/auth_tokens_test.go:32`: `if err := store.validate(...); !errors.Is(err, errInvalidToken)`).

### Cross-boundary event-name contract

**Source:** `beamsync/server.go:377` (emit) ↔ `desktop/frontend/src/App.svelte:458-459` (listen)
**Apply to:** `beamsync/progress_test.go`, `beamsync/server.go`

| Event name | Go emitter | Svelte listener | Direction |
|---|---|---|---|
| `upload_progress` | `progressWriter` at `beamsync/server.go:957` (receive) + direct emit at `:995` | `App.svelte:458` → `handleProgressUpdate(data, "receive")` | desktop receives |
| `download_progress` | `progressWriter` at `beamsync/server.go:1275` and `:1341` (send) | `App.svelte:459` → `handleProgressUpdate(data, "send")` | desktop sends |

Any rename must move in lockstep across both files.

---

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `desktop/frontend/dist/.gitkeep` | config/placeholder | n/a | Repo contains **zero** `.gitkeep` files — this is a new pattern for this codebase. Follow the standard empty-dir-placeholder idiom (verified against git's ignore semantics above). |
| `.github/workflows/ci.yml` (modified) | config | n/a | No sibling CI-workflow analog for the `-race` flag or module scoping; the change is a targeted diff to the existing `test-go` / `build` / `lint-go` jobs. |

---

## Metadata

**Analog search scope:** `beamsync/*.go` (all 9 `*_test.go` files), `desktop/*.go` (2 files, 0 tests),
`.github/workflows/*.yml` (3 files), `.planning/codebase/TESTING.md`, root `CONTRIBUTING.md`.

**Files scanned:** 9 `beamsync` test files, 6 `beamsync` source files, 2 `desktop` source files,
1 CI workflow, 2 `.gitignore` files.

**Analogs read in full:** `beamsync/server_test.go` (547), `beamsync/rate_limiter_test.go` (224),
`beamsync/history_test.go` (231), `beamsync/auth_tokens_test.go` (91), `beamsync/tls_test.go` (137),
`beamsync/stats_test.go` (74), `beamsync/server.go` (targeted sections: 1-140, 190-229, 340-409,
640-699, 940-999, 1255-1364), `desktop/app.go` (1-300, 824-883), `desktop/main.go` (39),
`.github/workflows/ci.yml` (68).

**Verification performed:**
- `go list .` vs `go list ./...` in `desktop/` — confirmed the stray `flatted` package (D-12).
- `git check-ignore -v` + `git add -A` in a throwaway repo — confirmed the `dist/*` +
  `!dist/.gitkeep` form is required in **both** `.gitignore` files (D-11).
- `git ls-files desktop/frontend/dist` — confirmed no dist file is currently tracked.
- `find . -name .gitkeep` — confirmed no existing `.gitkeep` precedent.

**Pattern extraction date:** 2026-11-12
