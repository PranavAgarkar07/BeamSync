# Phase 3: Tests Before Refactoring - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-11-12
**Phase:** 3-Tests Before Refactoring
**Areas discussed:** Scope & Priority, TST-01 eventDispatcher, TST-02 progressWriter, TST-04 desktop/app.go, CI -race

---

## Scope & Priority

| Option | Description | Selected |
|--------|-------------|----------|
| Roadmap TST-01/02/04 only | Matches the 3 genuinely-outstanding requirements | ✓ |
| Add untested new-code coverage | Also test GetLocalIP, firstUnicastIPv4, runWindowsFirewallSetup | |
| Add frontend smoke test | Also add a Vitest smoke test for one design-system component | |

**User's choice:** Roadmap TST-01/02/04 only
**Notes:** TST-03, TST-05, TST-06 verified as already satisfied by existing tests during
codebase analysis on 2026-11-12 (`beamsync/server_test.go`, `beamsync/auth_tokens_test.go`).
Frontend tests belong to Phase 5 (`SVE-09`).

---

## TST-01 eventDispatcher

| Option | Description | Selected |
|--------|-------------|----------|
| Black-box via emit() return + DroppedCount | Test the public seam: newEventDispatcher(1) + emit() → verify false returned + DroppedCount() == 1. No internals poked. | ✓ |
| Plus assert internal queue length | Also read d.queue directly (len(d.queue)) to assert queue depth — white-box, more brittle to Phase 4 refactor | |
| Test struct directly, no goroutine | Construct eventDispatcher{} struct literal directly with no goroutine, then drive emit() manually | |

**User's choice:** Black-box via emit() return + DroppedCount
**Notes:** `d.queue` is about to move into `beamsync/events.go` during Phase 4, so asserting on it
would couple the test to a representation that is changing.

---

### TST-01 concurrency test assertions

| Option | Description | Selected |
|--------|-------------|----------|
| Exact drop-count assertion | N goroutines × M emits under -race; assert no panic, no race, and DroppedCount() == actual drops (counted exactly) | ✓ |
| Smoke-level: no panic, no race | Assert only that nothing panics and the race detector is clean — no exact count | |
| Accounting invariant | Assert total emitted == delivered + dropped without pinning exact numbers | |

**User's choice:** Exact drop-count assertion
**Notes:** A smoke-level assertion would pass even if drop accounting were wrong.

---

### TST-01 panic recovery assertion depth

| Option | Description | Selected |
|--------|-------------|----------|
| Assert dispatcher survives | Emit with a callback that panics; assert the dispatcher goroutine survives and later emits still work (server.go:56-65 recover path) | ✓ |
| Assert no panic escapes | Just assert emit() returns true and no test-level panic escapes | |

**User's choice:** Assert dispatcher survives
**Notes:** The `recover()` in `eventDispatcher.run()` is the only thing between one bad callback
and a dead dispatcher goroutine.

---

## TST-02 progressWriter

| Option | Description | Selected |
|--------|-------------|----------|
| Add eventName field to progressWriter | Upload path sets "upload_progress", send path sets "download_progress". Matches the deleted downloadProgressWriter design | ✓ |
| Add direction field, map inside Write | Keep one type but infer name from a `direction` field ("upload"/"download") and map to the event name inside Write | |
| Restore separate downloadProgressWriter | Undo Phase 1's merge | |

**User's choice:** Add eventName field to progressWriter
**Notes:** This fixes a live bug found during codebase scout — `download_progress` is currently
emitted by nothing in the Go code (Phase 1 commit `d750b19` deleted `downloadProgressWriter` and
reused `progressWriter`, whose `Write` hardcodes `"upload_progress"` at `beamsync/server.go:377`).
The desktop send-direction handler `desktop/frontend/src/App.svelte:459` therefore never fires.

---

### TST-02 event assertion

| Option | Description | Selected |
|--------|-------------|----------|
| Assert event name per path | Assert the emitted event name is upload_progress on the receive path and download_progress on the send path | ✓ |
| Assert payload + throttle only | Only assert the payload format "filename|written|total" and the throttle interval, not the event name | |

**User's choice:** Assert event name per path
**Notes:** Payload-only assertions would have passed while the `download_progress` bug was live.

---

### TST-02 throttle test flakiness avoidance

| Option | Description | Selected |
|--------|-------------|----------|
| Split: minInterval=0 + injected clock | Use minInterval=0 for payload assertions (every write emits); separate test with a large minInterval + injected clock to assert coalescing | ✓ |
| Real sleeps between writes | Simple but slow and flaky under load | |

**User's choice:** Split: minInterval=0 + injected clock
**Notes:** `progressWriter` calls `time.Now()` directly today, so the injected-clock test requires
adding a `now func() time.Time` field — following the existing `clientRateLimiter.now` precedent
at `beamsync/server.go:115`.

---

## TST-04 desktop/app.go

| Option | Description | Selected |
|--------|-------------|----------|
| Commit a .gitkeep placeholder | Commit `desktop/frontend/dist/.gitkeep` (the `all:` embed prefix includes dotfiles) and un-ignore that path — verified: `go build` succeeds with no frontend build | ✓ |
| Require a frontend build first | Leave dist gitignored; desktop/ tests need `npm run build` first | |
| Build-tag split for the embed | Move the embed to a separate file with a build tag so tests compile without dist | |

**User's choice:** Commit a .gitkeep placeholder
**Notes:** Verified in a scratch copy — with `frontend/dist/.gitkeep` present and no real frontend
build, `go build ./...` succeeds and `go list -f '{{.EmbedFiles}}'` shows `frontend/dist/.gitkeep`.

---

### TST-04 stray node_modules package

| Option | Description | Selected |
|--------|-------------|----------|
| Use `go test .` in desktop/ | desktop/ is one package main — test the root package only, avoiding node_modules entirely | ✓ |
| Nested go.mod to exclude it | Add a go.mod inside `desktop/frontend/node_modules/flatted/golang/` so Go skips it as a nested module | |
| Leave as-is | It's harmless today, just noise in test output | |

**User's choice:** Use `go test .` in desktop/
**Notes:** Go does not skip `node_modules` (only `.`-prefixed, `_`-prefixed, and `testdata` are
excluded). `go list ./...` in `desktop/` returns both `desktop` and
`desktop/frontend/node_modules/flatted/golang/pkg/flatted`; `go list .` returns only the root package.

---

### TST-04 test targets

| Option | Description | Selected |
|--------|-------------|----------|
| Pure functions only | shareURL, extractTokenFromURL, extractPortFromURL, loadConfig/saveConfig round-trip, ensureSaveDir — all pure, no Wails ctx needed | ✓ |
| Also cover the event bridge | Also test processEvents + makeCallback drop behavior | |

**User's choice:** Pure functions only
**Notes:** `processEvents`/`makeCallback` would need `eventsClosed` channel wiring and add risk
without covering the Phase 4/5 extraction surface.

---

## CI -race

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, add -race to CI | Add -race to the test-go CI job so the concurrency test has teeth | ✓ |
| No, keep CI unchanged | Keep CI as-is; run -race locally only when writing the concurrency test | |

**User's choice:** Yes, add -race to CI
**Notes:** Verified the existing suite passes under `go test -race ./...` in `beamsync/`, so this
is a safe tightening rather than a new failure source.

---

## the agent's Discretion

- Exact test function names, subtest structure, and helper placement (per-file vs. a shared
  `export_test.go`).
- Whether the `desktop/` tests live in `desktop/app_test.go` or are split by concern.
- Whether the `progressWriter` clock field is `now func() time.Time` or a `timeSource` interface.

## Deferred Ideas

- **Zero-coverage Windows/parity code** — `GetLocalIP`, `firstUnicastIPv4`, `dialOutIP`,
  `runWindowsFirewallSetup` have no tests. Deferred from Phase 3 by scope decision.
- **Frontend component tests** — `SVE-09`, Phase 5.
- **`beamsync/export_test.go`** — roadmap mentions it, but the existing same-package convention
  makes it unnecessary.
- **Token IP-binding mismatch** — `tokenStore.validate(value, _ string, ...)` ignores the client-IP
  parameter, so `API.md`'s "IP binding" claim and TST-06's "wrong IP binding: 403" describe behavior
  that does not exist. Needs a separate decision: implement IP binding, or correct the docs.
