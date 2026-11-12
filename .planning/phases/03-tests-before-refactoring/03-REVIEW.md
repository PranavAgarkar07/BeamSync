---
phase: 03-tests-before-refactoring
reviewed: 2026-11-12T18:40:00Z
depth: deep
files_reviewed: 7
files_reviewed_list:
  - beamsync/event_dispatcher_test.go
  - beamsync/progress_test.go
  - beamsync/server.go
  - desktop/app_test.go
  - desktop/main.go
  - desktop/app.go
  - .github/workflows/ci.yml
findings:
  critical: 1
  warning: 8
  info: 6
  total: 15
status: issues_found
---

# Phase 3: Code Review Report

**Reviewed:** 2026-11-12T18:40:00Z
**Depth:** deep (per-file + cross-module tracing + mutation testing)
**Files Reviewed:** 7
**Status:** issues_found

## Summary

The suite is well-built in style: injected clocks instead of `time.Sleep`, a blocking-primer trick that makes drop accounting deterministic, no `t.Parallel()` combined with `t.Setenv`, and no writes outside `t.TempDir()`. The production change in `beamsync/server.go` is correct and has no receive-path regression (verified below). CI is genuinely green: `go test -race ./...` (beamsync, 13.3s), `go test -race .` (desktop), and `golangci-lint run` in both modules all pass.

But the phase's stated purpose is a safety net, and I mutation-tested the net. **It has a hole exactly where the live bug was.** I changed `eventName` at all three construction sites in `server.go:972/1293/1361` to wrong values — first one at a time, then all three at once — and the entire beamsync suite stayed green. The unit tests build their own `progressWriter` literal (`progress_test.go:26-38`), so they can only ever prove "`Write` uses `pw.eventName`" (true) and never "the send path sets `download_progress`" (the actual defect). The Phase 4 refactor can silently reintroduce the shipped bug and CI will not notice.

The second theme is the fix being half-effective in production: the send path still has no completion emit, so for any transfer that finishes inside the 500 ms throttle window — which on LAN is every file below roughly tens of MB — **zero** `download_progress` events are emitted. The user-visible symptom the fix was written to remove persists for the common case (proven by probe, WR-01).

Verified non-issues (checked, not assumed):
- RED-by-design commit `8dd231d` was a genuine assertion failure (it built a literal with no `eventName` and expected `download_progress` from code that hardcoded `upload_progress`), not a compile error.
- Reverting `server.go:389` to the hardcoded string **does** fail two tests — the `Write`-level pin works.
- Receive path parity: `eventName: "upload_progress"` at 972 + unchanged `lastEmit` zero value reproduces the previous payload byte-for-byte. No regression.
- `desktop/frontend/dist/.gitkeep` + `//go:embed all:frontend/dist` builds and vets on a tree containing only `.gitkeep` — the fresh-clone claim holds.
- CI desktop scoping to `.` is justified, not lazy: `go list ./...` in `desktop/` really does pick up `desktop/frontend/node_modules/flatted/golang/pkg/flatted`.
- No test writes outside its own `t.TempDir()`; no test touches the real user config while the env redirection holds.

## Critical Issues

### CR-01: No test exercises the three production `progressWriter` construction sites — the exact regression this phase exists to block is undetected

**File:** `beamsync/progress_test.go:26-38`, `beamsync/progress_test.go:73-87`; production sites `beamsync/server.go:972`, `beamsync/server.go:1293`, `beamsync/server.go:1361`
**Severity:** BLOCKER

`newProgressWriterForTest` hardcodes the values the tests then assert:

```go
f.writer = &progressWriter{
    eventName:   eventName,   // supplied by the test
    ...
}
```

`TestProgressWriterSendPathEmitsDownloadProgress` therefore asserts `Write` emits `pw.eventName` — a property no one disputes — while the defect was that the *send path passed the wrong event name*. Proven by mutation on a copy of module `beamsync`:

| mutation | suite result |
|---|---|
| `server.go:1361` `download_progress` → `upload_progress` (multi-file send, the path used for any multi-select drag) | **all pass** |
| `server.go:972` → `wrong_receive_event`, `1293` → `wrong_send_event` | **all pass** |
| `server.go:389` `pw.eventName` → `"upload_progress"` (the original bug) | 2 failures ✔ |

So the suite pins the least likely regression (someone editing `Write`) and misses the three real ones. An empty/forgotten `eventName` is additionally silent at runtime: with no value the payload becomes `|name|written|total` and the frontend's `split("|")` yields `filename=""` (probe confirmed: `[[]|n.txt|3|100]`).

**Fix:** make the direction→event-name mapping exist in one place and cover it end-to-end, since a handler-level harness already exists in `beamsync/server_test.go:314-380`:

```go
// server.go — one source of truth, no per-site literal to forget
const (
    eventUploadProgress   = "upload_progress"
    eventDownloadProgress = "download_progress"
)

func newProgressWriter(w io.Writer, filename string, total int64, eventName string, emit func(string, string)) *progressWriter {
    return &progressWriter{w: w, filename: filename, total: total, eventName: eventName,
        emit: emit, minInterval: 500 * time.Millisecond, now: time.Now}
}
```

and an e2e assertion in the existing sender test (after WR-01's completion emit lands, this is deterministic):

```go
events := make(chan string, 64)
server, port, bootstrapToken, err := StartSenderE([]string{filePath}, func(name, data string) {
    events <- name + "|" + data
})
// ... download the file ...
if !waitForEvent(events, "download_progress", time.Second) {
    t.Fatal("send path never emitted download_progress")
}
```

Equivalent for the receive path in `TestUploadWithFileSavesToDiskAndEmitsEvents` (`server_test.go:443` already waits on events — add `waitForEvent(events, "upload_progress", ...)`, which works today because the receive path leaves `lastEmit` at zero).

## Warnings

### WR-01: Send path never emits a completion progress event — the fix is invisible for most real transfers

**File:** `beamsync/server.go:1289-1310` (single-file), `beamsync/server.go:1357-1384` (multi-file); contrast `beamsync/server.go:1009`
**Severity:** WARNING

Both send sites construct the writer with `lastEmit: time.Now()` (1295, 1363) and `minInterval: 500ms`, so the first emit is suppressed and no emit occurs until 500 ms of copying has elapsed. Unlike the receive path — which has an explicit final emit at line 1009 (`emit("upload_progress", ...written, written)`) — the send path emits nothing after `copyChunked` returns.

Probe result (13 × 8-byte writes through a send-shaped writer with the production `lastEmit`/`minInterval`):

```
download_progress events emitted during a fast 104-byte send: []
```

Consequence: a download completing in under 500 ms (every small file, and anything up to tens of MB on a gigabit LAN) produces **zero** `download_progress` events. `App.svelte:459` never fires, so `E.send`/the floating progress bar stay idle — the same "send-direction progress is dead" symptom `d058ae1` set out to fix, now narrowed to large files only.

**Fix:** mirror the receive path — after `copyChunked` succeeds at 1299/1367, emit the terminal event:

```go
written, copyErr := copyChunked(pw, bufReader, 8*1024*1024)
if copyErr == nil {
    emit("download_progress", fmt.Sprintf("%s|%d|%d", filename, written, written))
}
```

(Alternatively drop `lastEmit: time.Now()` so the first write always emits — but the explicit completion emit is what the frontend needs for a clean 100 %.)

### WR-02: `TestProgressWriterZeroByteWriteEmitsNothing` asserts something the code does not do

**File:** `beamsync/progress_test.go:197-218` (comment 197-199, `lastEmit` pin at 203)
**Severity:** WARNING

The comment claims "a zero-byte write is a no-op". `Write` has no `n == 0` branch — the only reason no event appears is that the test pins `lastEmit` inside the throttle window. Probe against the real type with `minInterval: 0`:

```
events after zero-byte unthrottled write: [upload_progress|n.txt|0|100]
```

So the test passes for a reason it does not state, and `written`/`sink` assertions would pass even if `Write` gained a bogus `n == 0` emit path. A safety net whose name teaches the wrong invariant is worse than no test.

**Fix:** either state the real invariant, or make the code match the name.

```go
// if the no-op is intended, make it explicit in Write:
func (pw *progressWriter) Write(p []byte) (int, error) {
    n, err := pw.w.Write(p)
    pw.written += int64(n)
    if n == 0 {
        return n, err
    }
    ...
```
and keep the test as-is; otherwise rename to `...ZeroByteWriteInsideThrottleWindow...` and drop the "no-op" claim, plus add an unthrottled case that pins the actual (zero-byte emit) behaviour.

### WR-03: `loadConfig`'s home-containment guard is a prefix match; the new test enshrines the weakness as correct

**File:** `desktop/app.go:131`; test at `desktop/app_test.go:115-127`
**Severity:** WARNING

```go
if home != "" && !strings.HasPrefix(cleanSave, cleanHome) {
    cfg.SavePath = ""
}
```

`/home/user-evil/Downloads/BeamSync` **passes** the "inside home" check for home `/home/user`. Probe against the real functions (temp home `…/user`, stored path `…/user-evil/Downloads/BeamSync`):

```
loadConfig().SavePath = "/tmp/TestProbe.../001/user-evil/Downloads/BeamSync"   // NOT cleared
```

`TestLoadConfigClearsSavePathOutsideHome` uses `/outside/BeamSync`, which fails the prefix test for *every* possible home, so it can never detect the boundary bug — the guard's only real failure mode is untested. Also untested: `home == ""` (when `os.UserHomeDir()` errors the guard is skipped entirely and any path is trusted).

**Fix:** containment by path boundary, plus the missing cases:

```go
rel, err := filepath.Rel(cleanHome, cleanSave)
if home == "" || err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
    cfg.SavePath = ""
}
```
```go
{name: "sibling of home is rejected", save: filepath.Join(base, "user-evil", "x")},
{name: "home itself is accepted",     save: home},
```

### WR-04: Every dispatcher leaks its worker goroutine; no cleanup path exists and `release` is not deferred

**File:** `beamsync/server.go:43-52` (`go dispatcher.run()`, queue never closed), `beamsync/event_dispatcher_test.go:78-97`, `:123`, `:152`
**Severity:** WARNING

`run()` ranges over `d.queue`, and nothing ever closes it — there is no `Close`/`stop` on `eventDispatcher`. Probe: creating 10 dispatchers leaves 10 goroutines behind.

```
goroutines before=3 after creating 10 dispatchers=13 (leak=10)
```

The new tests create 9 dispatchers per run, so the suite permanently leaks 9 goroutines and would fail the moment `goleak.VerifyTestMain` is added (a standard next step for a refactor safety net). Separately, `close(release)` at lines 123 and 152 is not deferred, so any assertion failure before it leaves the worker blocked on `<-release` with undrained jobs.

**Fix:**
```go
type eventDispatcher struct {
    queue   chan eventDispatchJob
    closeOnce sync.Once
    done    chan struct{}
}

func (d *eventDispatcher) Close() {
    d.closeOnce.Do(func() { close(d.queue) })
    <-d.done
}
// run(): defer close(d.done)
```
then in tests `d := newEventDispatcher(n); t.Cleanup(d.Close)` and `defer close(release)` in `startBlockingEventDispatcher` (return the closer, or `t.Cleanup(func(){ close(release) })` — with an idempotent guard, since two tests close it explicitly).

### WR-05: `redirectUserConfigHome` never verifies the redirection took effect — a silent miss overwrites the developer's real config

**File:** `desktop/app_test.go:91-98`, used at `:102`, `:117`
**Severity:** WARNING

The suite's safety ("config round-trip tests never touch the developer's real config file") rests on five env vars being honoured by `os.UserConfigDir()`/`os.UserHomeDir()` on every platform. Nothing asserts it. If the redirection silently stops matching — a platform where `os.UserConfigDir()` ignores all of them, a wrapper, a future Go change — `saveConfig` at `app_test.go:105` writes the developer's actual `~/.config/beamsync/config.json` (and `TestLoadConfigClearsSavePathOutsideHome` truncates it with `/outside/BeamSync`). The failure would be invisible: the test still passes, because it reads back the same wrong file.

**Fix:** make the assumption an assertion inside the helper:

```go
func redirectUserConfigHome(t *testing.T, dir string) {
    t.Helper()
    t.Setenv("HOME", dir)
    ...
    if p := configPath(); !strings.HasPrefix(p, filepath.Clean(dir)+string(filepath.Separator)) {
        t.Fatalf("config redirection failed: configPath()=%q not under %q", p, dir)
    }
}
```

### WR-06: Requirement traceability overstates coverage (TST-02 verification is a `grep`; TST-04 names untested surfaces)

**File:** `.planning/phases/03-tests-before-refactoring/03-02-SUMMARY.md` (D2 `verification.kind: other`), `.planning/REQUIREMENTS.md:24`, `.planning/REQUIREMENTS.md:26`, `desktop/app_test.go`
**Severity:** WARNING

TST-02's bug-fix verification is:

```
grep -c 'eventName:' beamsync/server.go == 3 && grep -c 'progressWriter{' beamsync/server.go == 3
```

That checks the *field exists three times*, not that any value is correct — the same mutation that leaves the suite green (CR-01) also leaves this verification green. And `REQUIREMENTS.md:26` marks TST-04 `[x]` for "URL builder, **version source, event channel**", but `desktop/app_test.go` contains no test for the version source (`startup`, `app.go:181-192`) or the event channel (`processEvents`, `app.go:227-256`) — neither is listed in `deferred-items.md` either.

**Fix:** replace the grep with the e2e assertion from CR-01; either add tests for the version/event-channel paths (they need a small extraction: `func parseVersion(data []byte) string`) or move them to `deferred-items.md` and leave TST-04 unchecked for that part.

### WR-07: `extractPortFromURL` returns garbage for an IPv6 URL without a port — untested, while the IPv6 case gives false confidence

**File:** `desktop/app.go:280-294`; test at `desktop/app_test.go:75`
**Severity:** WARNING

```
extractPortFromURL("http://[::1]/share?token=tok") = "1]"
```

`LastIndex(":")` inside a bracket literal returns `1]`, which the caller stores as `a.currentPort` (`app.go:252`) and feeds into `shareURL`. The suite tests IPv6 *with* a port only, so the case that breaks is the one omitted. Impact is bounded (callers pass URLs produced by `beamsync`, which always include a port), but this is precisely the class of bug a pure-function safety net is supposed to surface.

**Fix:**
```go
u, err := url.Parse(raw)
if err != nil || u.Port() == "" {
    return ""
}
return u.Port()
```
with cases `http://[::1]/share`, `http://[::1]` (no path), `192.168.1.10:8080/x` (no scheme).

### WR-08: CI still validates only Linux, while the new tests carry Windows-specific logic and claim Windows hermeticity

**File:** `.github/workflows/ci.yml:35-46`; claims in `03-03-SUMMARY.md` (`key-decisions`: "hermetic on Windows runners as well as Unix")
**Severity:** WARNING

`test-go` runs `runs-on: ubuntu-latest` only. `desktop/app.go:126-130` is a `GOOS == "windows"` branch (case-insensitive path comparison) that no test anywhere executes, yet `TestLoadConfigRoundTripsSavePathInsideHome` is the test meant to protect it. The "Windows runners" the summary refers to do not exist in this workflow.

**Fix:** add a matrix for the desktop module only (cheap — one module, no port binding):
```yaml
  test-desktop-cross:
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, windows-latest, macos-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v5
      - uses: actions/setup-go@v6
        with: { go-version: '1.25' }
      - run: (cd desktop && go test -race .)
```
If that is out of scope, remove the Windows claims from the summary and log the untested branch in `deferred-items.md`.

## Info

### IN-01: The `now == nil` fallback branch is untested, and redundant at every production site

**File:** `beamsync/server.go:383-386`; sites `:975`, `:1297`, `:1365`
**Issue:** Every test supplies a clock, so `if pw.now != nil` never takes the false path. Meanwhile all three production sites pass `now: time.Now`, which is exactly what the fallback would do — so the three added field assignments buy nothing at runtime. Coverage of `Write` reads 100 % while the guard's nil branch is dead in tests.
**Fix:** add one case with `now: nil` that asserts the writer still emits and does not panic; optionally drop `now: time.Now` from the three sites so the guard is the single default.

### IN-02: The "empty port" case pins a malformed URL as correct

**File:** `desktop/app_test.go:26-30`
**Issue:** `shareURL("")` is asserted to equal `http://192.168.1.10:/?token=tok` — an invalid URL. Any future fix that omits the colon (`net.JoinHostPort`) breaks this test, i.e. the net blocks an improvement.
**Fix:** make it a negative/characterization case (`// known limitation: no guard against an empty port`) or have `shareURL` skip the `:` when the port is empty and assert the valid URL.

### IN-03: `extractTokenFromURL` prefix-matches "token=" — `refresh_token=` returns a value

**File:** `desktop/app.go:265-278`
**Issue:** `strings.Index(raw, "token=")` also matches inside `refresh_token=`, `csrf_token=`, etc. Probe: `extractTokenFromURL("http://h/?refresh_token=abc")` → `"abc"`. No test covers it; the six cases cover only well-formed input.
**Fix:** parse with `net/url` and read `u.Query().Get("token")`; add the `refresh_token` and `?a=1&token=x` cases.

### IN-04: The delimiter test locks the buggy wire format in as passing behaviour

**File:** `beamsync/progress_test.go:290-315`
**Issue:** The subtest asserts the payload splits into 5 parts — i.e. it asserts the bug. It is documented (§`deferred-items.md`) so this is acceptable as a characterization test, but it will fail (by design) on the eventual fix, and `deferred-items.md` does not say "this test must be updated".
**Fix:** add the test name to the deferred item's "Suggested home" line so the payload-format change is not mistaken for a regression.

### IN-05: `TestProgressWriterSingleWriterPerTransferPath` claims a contract it cannot test

**File:** `beamsync/progress_test.go:318-414`
**Issue:** The name and doc comment say the test "pins the single-writer contract", but it drives one goroutine sequentially — no test can assert non-thread-safety. What it actually pins is `copyChunked` ↔ `progressWriter` byte accounting, which is valuable.
**Fix:** rename to `TestCopyChunkedThroughProgressWriterByteAccounting` and keep the single-writer note in the type doc comment only.

### IN-06: POSIX-only path literal in a cross-platform config test

**File:** `desktop/app_test.go:119`
**Issue:** `/outside/BeamSync` works on Windows only by accident (`filepath.Clean` → `\outside\BeamSync`, which fails the prefix check for a `C:\…` home). Harmless today, misleading as documentation of intent.
**Fix:** `filepath.Join(string(filepath.Separator), "outside", "BeamSync")`.

---

_Reviewed: 2026-11-12T18:40:00Z_
_Reviewer: gsd-code-reviewer (adversarial pass; findings validated by mutation testing and targeted probes on throwaway copies — the working tree was not modified)_
_Depth: deep_

_Verification commands run:_
- `cd beamsync && go test -race -count=1 ./...` → `ok beamsync 13.350s`
- `cd desktop && go test -race -count=1 .` → `ok desktop 1.016s`
- `cd beamsync && GOTOOLCHAIN=go1.25.5 golangci-lint run ./...` → clean (exit 0)
- `cd desktop && GOTOOLCHAIN=go1.25.5 golangci-lint run .` → clean (exit 0)
- `gofmt -l beamsync/ desktop/*.go` → no output; `go vet` clean in both modules

_Environment note:_ without `GOTOOLCHAIN=go1.25.5`, local `golangci-lint v1.64.8` fails with `could not import sync/atomic (… export data version 5 is greater than maximum supported version 2) (typecheck)` against the local Go 1.27.3 toolchain. That is a toolchain/version mismatch on the reviewer machine, not a repo defect — CI's pinned `go 1.25` + `install-mode: goinstall` avoids it.
