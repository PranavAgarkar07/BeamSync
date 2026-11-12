# Phase 3: Tests Before Refactoring - Context

**Gathered:** 2026-11-12
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase delivers a **test safety net for the components that Phase 4 (Go file-splitting)
and Phase 5 (Svelte component split) will restructure** — plus the one live behavior bug
that testing surfaces.

**In scope:**
- `TST-01` — unit tests for `eventDispatcher` (emit, drop-on-full, concurrency, panic recovery)
- `TST-02` — unit tests for `progressWriter`, **including the `download_progress` event-name fix** (see D-05)
- `TST-04` — unit tests for `desktop/app.go` pure functions
- CI hardening: add `-race` to the Go test job (D-04), and unblock `desktop/` test compilation (D-07, D-08)

**Out of scope:**
- `TST-03` (`copyChunked`), `TST-05` (httptest send-receive flow), `TST-06` (token auth) — **already
  satisfied** by existing tests (`beamsync/server_test.go`, `beamsync/auth_tokens_test.go`).
  Verified by inspection on 2026-11-12.
- Frontend (Svelte) component tests — Phase 5, requirement `SVE-09`. Phase 2 already scaffolded
  Vitest + `@testing-library/svelte`; only the tests themselves are deferred.
- Coverage of the new Windows/parity code (`GetLocalIP`, `firstUnicastIPv4`, `runWindowsFirewallSetup`) —
  zero coverage today, but out of scope for this phase (D-03).
- Any Phase 4/5 structural extraction itself.

</domain>

<decisions>
## Implementation Decisions

### Scope & Priority
- **D-01:** Phase 3 covers **TST-01, TST-02, TST-04 only** — the three genuinely outstanding
  requirements. The other three (TST-03/05/06) are already satisfied by existing tests.
- **D-02:** Do **not** add coverage for the newly-added Windows/parity surface (`GetLocalIP`,
  `firstUnicastIPv4`, `runWindowsFirewallSetup`) in this phase, despite them having zero coverage.
  They are not in the Phase 3 requirement set and adding them widens the diff. — **Reversibility:**
  reversible — they can be picked up in a later phase without touching these decisions.
- **D-03:** Do **not** add a frontend/Vitest smoke test in this phase. Frontend testing is
  `SVE-09` in Phase 5; Phase 2 already delivered the test infrastructure it needs.

### TST-01 — `eventDispatcher`
- **D-04:** The drop/overflow test asserts **black-box**: `emit()` returns `false` and
  `DroppedCount()` increments. Do **not** read `d.queue` internals — the field is a
  `chan eventDispatchJob` and Phase 4 will move the type into `beamsync/events.go`.
  Reading internals would couple the test to a representation that is about to change.
- **D-05:** The concurrency test asserts an **exact drop count** — the number of events dropped
  must equal the number the emitter failed to enqueue. A smoke-level "nothing panicked" assertion
  would pass even if the drop accounting were wrong.
- **D-06:** The panic-recovery test asserts the **dispatcher survives**: emit with a panicking
  callback, then emit again and assert the later event is still delivered. This exercises the
  `recover()` in `eventDispatcher.run()` (`beamsync/server.go:56-65`), which is the only thing
  standing between one bad callback and a dead dispatcher goroutine.
  — **Reversibility:** reversible — assertion strength can be loosened later without touching production code.

### TST-02 — `progressWriter` + the `download_progress` bug
- **D-07:** **A live bug was found during codebase scout and is fixed in this phase.**
  `download_progress` is emitted by **nothing** in the Go code. Phase 1 (`d750b19`) deleted
  `downloadProgressWriter` and pointed the sender path at `progressWriter`, but
  `progressWriter.Write` hardcodes the event name `"upload_progress"`
  (`beamsync/server.go:377`). The desktop send-direction handler
  (`desktop/frontend/src/App.svelte:459` — `EventsOn("download_progress", ...)`) therefore
  **never fires**, so download progress is silently broken in the shipped app today.
  Fix shape: add an **`eventName` field** to `progressWriter`; the receive path sets
  `"upload_progress"`, the send path sets `"download_progress"`.
  Test-first: write the failing test, then fix. — **Reversibility:** costly — the event name is a
  cross-boundary contract with the Svelte frontend (`App.svelte:458-459`); renaming it back would
  require reverting both the Go emit and the frontend listener together.
- **D-08:** The `progressWriter` test **asserts the event name per path** — `"upload_progress"` on
  the receive path, `"download_progress"` on the send path — not just the payload format.
  Asserting only the payload would have passed while the bug in D-07 was live.
- **D-09:** The throttle test is **split into two**: (a) a payload/behavior test with
  `minInterval = 0` so every write emits, and (b) a coalescing test using an **injected clock**.
  `progressWriter` currently calls `time.Now()` directly and is not injectable, so part (b)
  requires adding a `now func() time.Time` field — following the existing precedent of
  `clientRateLimiter.now` (`beamsync/server.go:115`). Real `time.Sleep` between writes is
  rejected: it is slow and flaky under CI load.

### TST-04 — `desktop/app.go`
- **D-10:** Test **pure functions only** — `shareURL`, `extractTokenFromURL`, `extractPortFromURL`,
  `loadConfig`/`saveConfig` round-trip, `ensureSaveDir`. These need no Wails runtime context.
  `processEvents` / `makeCallback` drop behavior is explicitly **not** in scope.
- **D-11:** **Unblock `desktop/` test compilation.** `desktop/frontend/dist` is gitignored
  (`desktop/.gitignore:3`), so `go test ./...` in `desktop/` fails to compile with
  `pattern all:frontend/dist: no matching files found` unless the frontend was built first.
  Fix: commit a `desktop/frontend/dist/.gitkeep` placeholder and un-ignore that single path.
  Verified: the `all:` embed prefix includes dotfiles, so a `.gitkeep` satisfies the pattern and
  `go build ./...` succeeds with no frontend build.
- **D-12:** **Use `go test .` (root package) instead of `go test ./...` in `desktop/`.**
  `desktop/` is a single `package main` (`app.go` + `main.go`), and `./...` also compiles a stray
  Go package vendored inside npm dependencies — `desktop/frontend/node_modules/flatted/golang/pkg/flatted`
  — which Go does not skip (`node_modules` is not in Go's excluded-directory set; only `.`-prefixed,
  `_`-prefixed, and `testdata` are). `go list .` returns only the root package.
  Apply the same scoping to `go build`/`go vet` for `desktop/`.

### CI
- **D-13:** Add **`-race`** to the Go test CI job. `TST-01` requires a concurrency test, but
  `go test ./...` without `-race` will not detect a data race — the test would look meaningful
  while proving nothing. The current suite passes under `-race` (verified 2026-11-12), so this
  is a safe tightening. — **Reversibility:** reversible — removing the flag is a one-line CI change.

### The agent's Discretion
- Exact test function names, subtest structure, and helper placement (per-file vs. a shared
  `export_test.go`). Note the roadmap mentions `export_test.go` (PITFALLS.md 2d), but the existing
  convention already places helpers in the test file that uses them (e.g. `newTokenStoreForTest`
  in `auth_tokens_test.go`, `freePort`/`startRawServerForTest` in `server_test.go`) — and Go allows
  any test file in the same package to use them, so no consolidation is required.
- Whether the `desktop/` tests live in `desktop/app_test.go` or are split by concern.
- Whether the `progressWriter` `now` field is `now func() time.Time` or a `timeSource` interface.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope & requirements
- `.planning/ROADMAP.md` — Phase 3 section (goal, plans 3.1–3.6, dependencies, risks)
- `.planning/REQUIREMENTS.md` — Phase 3 block: `TST-01`–`TST-06`
- `.planning/STATE.md` — current position; Phase 3 is 3/6 done as of 2026-11-12

### Testing constraints (must-follow)
- `.planning/research/PITFALLS.md` §2a — `package beamsync` (not `_test`) is required to reach
  unexported types; §2b — priority order for what to test first; §2c — don't start real servers,
  use `httptest.NewRecorder`/`NewRequest`; §2d — test helper duplication
- `.planning/codebase/TESTING.md` — established patterns: Go stdlib `testing`, no assertion library,
  `t.Fatalf` style, no mocking framework, `t.TempDir()` / `t.Setenv()` / injectable `now` funcs
  instead of mocks, co-located `*_test.go` in the same package

### Prior phase decisions
- `.planning/phases/01-foundation-event-system-version/CONTEXT.md` — event/version decisions
  (drop counters live on the dispatcher; version sourced from `wails.json`)
- `.planning/phases/02-tooling-devex/CONTEXT.md` — tooling decisions (Vitest/jsdom scaffold,
  per-module golangci-lint configs, CI job layout)

### Production code under test
- `beamsync/server.go` — `eventDispatcher` + `emit()` + `DroppedCount()` (lines 28–84),
  `progressWriter` (lines 362–381), `copyChunked` (line 383), `clientRateLimiter.now` (line 115)
- `beamsync/server_test.go` — existing integration-test patterns and shared helpers to reuse
  (`waitForEvent`, `startServerForTest`, `startRawServerForTest`, `freePort`, `extractInjectedToken`)
- `beamsync/auth_tokens_test.go` — `newTokenStoreForTest` helper
- `desktop/app.go` — pure functions to test: `shareURL` (49), `configPath` (106), `loadConfig` (114),
  `saveConfig` (133), `ensureSaveDir` (153), `extractTokenFromURL` (265), `extractPortFromURL` (274)
- `desktop/frontend/src/App.svelte` — frontend event listeners that must match the Go event names
  (`upload_progress` line 458, `download_progress` line 459)
- `desktop/main.go` — `//go:embed all:frontend/dist` (line 11), the constraint behind D-11
- `desktop/.gitignore` — line 3 ignores `frontend/dist`, the constraint behind D-11
- `.github/workflows/ci.yml` — `test-go` job (the `-race` change target, D-13)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `beamsync/server_test.go` helpers: `waitForEvent(events, want, timeout)` for async event
  assertions, `startRawServerForTest` / `startServerForTest` for real-server tests, `freePort` for
  port allocation, `extractInjectedToken` for HTML token extraction.
- `beamsync/auth_tokens_test.go` → `newTokenStoreForTest(t)` — token store fixture.
- `clientRateLimiter.now func() time.Time` (`beamsync/server.go:115`) — the established
  clock-injection pattern that D-09 reuses for `progressWriter`.
- `eventDispatcher.emit()` returns `bool` (false = dropped) and `DroppedCount()` exposes the
  counter — together these give TST-01 a clean black-box seam with no internals access.
- `desktop/app.go` pure functions need no Wails runtime context, so `desktop/app_test.go` can
  be `package main` and construct `&App{}` directly.

### Established Patterns
- Go stdlib `testing` only — no assertion library; failures use `t.Fatalf("... = %v, want %v", got, want)`.
- No mocking: real implementations + `t.TempDir()`, `t.Setenv()`, injectable `now` funcs.
- Table-driven tests with `t.Run` subtests for multi-case behavior.
- White-box testing: all test files use `package beamsync` to reach unexported types.
- Vitest + jsdom + `@testing-library/svelte` scaffolded in `desktop/frontend/` (Phase 2) — unused
  in this phase, reserved for Phase 5.

### Integration Points
- `progressWriter` is constructed in three places in `beamsync/server.go` — receive path (line 957),
  single-file send path (line 1275), multi-file send path (line 1341). Adding `eventName` means
  all three must set it; a missed one reproduces D-07.
- The Go→Svelte event contract spans `beamsync/server.go` and
  `desktop/frontend/src/App.svelte:458-459`. Any event-name change must move in lockstep.
- CI `test-go` job runs `go test ./...` in both modules; D-12 and D-13 change those commands.

</code_context>

<specifics>
## Specific Ideas

- The `download_progress` bug (D-07) is a **shipped-app defect**, not a hypothetical: send-direction
  progress in the desktop UI is currently dead. The Phase 3 test suite should fail on this before
  the fix lands, so the fix is provably the thing that turned it green.
- `desktop/frontend/dist` being gitignored means a fresh clone cannot run `go test ./...` in
  `desktop/` at all until the frontend is built. The `.gitkeep` placeholder (D-11) removes that
  footgun permanently.

</specifics>

<deferred>
## Deferred Ideas

- **Zero-coverage Windows/parity code** — `GetLocalIP`, `firstUnicastIPv4`, `dialOutIP`,
  `runWindowsFirewallSetup` in `beamsync/localip.go` and `beamsync/firewall.go` have no tests.
  Noted during codebase analysis on 2026-11-12; explicitly deferred from Phase 3 by D-02.
  A natural home is a future phase covering the new platform work, or Phase 4 if `localip.go`
  gets folded into the extraction.
- **Frontend component tests** — `SVE-09` in Phase 5. Infrastructure already exists (Phase 2).
- **`beamsync/export_test.go`** — the roadmap mentions it for shared helpers, but the existing
  same-package convention makes it unnecessary. Revisit only if helper duplication becomes real.
- **Token IP-binding** — `tokenStore.validate(value, _ string, ...)` ignores the client-IP
  parameter entirely, so `API.md`'s "IP binding" claim and roadmap TST-06's "wrong IP binding: 403"
  case describe behavior that does not exist. This is a **specification/implementation mismatch,
  not a test gap** — it needs a decision on whether to implement IP binding or correct the docs.
  Out of scope for Phase 3 (which tests what exists).

</deferred>

---

*Phase: 3-Tests Before Refactoring*
*Context gathered: 2026-11-12*
