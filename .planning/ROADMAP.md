# BeamSync — Quality Overhaul Roadmap

**Date:** 2026-07-20
**Type:** Brownfield quality pass — no feature changes, no user-facing behavior changes.
**Constraint:** All Go refactoring stays within `package beamsync` — no sub-packages.

---

## Milestone Summary

| Phase | Name | Reqs | Depends On | Risk | Effort |
|-------|------|------|-----------|------|--------|
| 1 | Foundation — Event System & Version | EVT-01–03, VER-01–03 | None | Low | 2–3 files |
| 2 | Tooling & DevEx | TOL-01–07 | Phase 1 | Low | Config-only |
| 3 | Tests Before Refactoring | TST-01–06 | Phase 2 (linters + test infra) | Medium | 6 new test files |
| 4 | Go Structural Refactor | STR-01–10 | Phase 3 (tests catch regressions) | High | 9 extracted files |
| 5 | Svelte Component Split | SVE-01–09 | Phase 2 (ESLint, Vitest) | Medium | 7 components + store |
| 6 | Error Handling & Logging | ERR-01–04 | Phase 3 (test coverage safety net) | Low | 4 conventions |
| 7 | CI/CD & Release | CID-01–03 | Phase 1 (version), Phase 2 (CI working) | Low | 3 configs |
| 8 | Documentation & Cleanup | DOC-01–05 | All prior phases | Low | 5 tasks |

**Total:** 50 requirements across 8 phases.

---

## Phase 1: Foundation — Event System & Version

**Goal:** Eliminate silent event drops and unify the version string to a single source of truth before any structural work begins.

**Requirements:** EVT-01, EVT-02, EVT-03, VER-01, VER-02, VER-03

### Plans

1.1 **Resize `eventDispatcher` buffer** — `beamsync/server.go` line ~72: change `make(chan eventDispatchJob, 256)` → `1024`. Verify no other code depends on the 256 constant.

1.2 **Resize `eventChan` buffer** — `desktop/app.go` line ~70: change `eventChan: make(chan appEvent, 100)` → `512`. Make the capacity a named `const eventChanCapacity = 512` in `app.go`.

1.3 **Add drop counters to event dispatcher** — Add `atomic.Int64 dropped` field to `eventDispatcher` struct. Increment on the `default:` branch of the non-blocking send. Expose via `DroppedCount()` method. Log on emit and periodically in `processEvents`.

1.4 **Unify version to `wails.json`** — Remove `const currentVersion = "v2.4.0"` from `desktop/app.go:29`. Read version from `wails.json` at startup (embed + parse JSON). Expose via `App.GetVersion()` Wails-bound method.

1.5 **Pass version to Svelte frontend** — Remove hardcoded `appVersion="v2.2"` from `App.svelte:944`. Call `window.go.main.App.GetVersion()` on mount. Use the returned value in the About view.

1.6 **Add drop counter to `makeCallback`** — `desktop/app.go` `makeCallback`: add `atomic.Int64` counter for drops on the `default:` branch of the `eventChan` send.

### Dependencies

None. Phase 1 is fully standalone and must be done first.

### Risks

- **Silent double dispatcher** (PITFALLS.md 1a): When adding drop counters, ensure the same `eventDispatcher` instance is instrumented — not a new one created during extraction.
- **Version string disparity persists if path missed** (PITFALLS.md 4c): Verify all three locations (`app.go`, `App.svelte`, `wails.json`) converge after this phase.
- **Low risk** overall — no structural changes, just parameter tweaks and counters.

---

## Phase 2: Tooling & DevEx

**Goal:** Add linting, formatting, type checking, and test infrastructure so subsequent phases have safety nets. All config — zero logic changes.

**Requirements:** TOL-01, TOL-02, TOL-03, TOL-04, TOL-05, TOL-06, TOL-07

### Plans

2.1 **golangci-lint for `beamsync/`** — Create `.golangci.yml` in `beamsync/` with: `govet`, `staticcheck`, `errcheck`, `gosec`, `errorlint`. Run `(cd beamsync && golangci-lint run ./...)` and fix initial issues using `--new-from-rev` to scope to new code.

2.2 **golangci-lint for `desktop/`** — Same config in `desktop/`. Run separately per PITFALLS.md 5a.

2.3 **ESLint + `eslint-plugin-svelte3`** — Add `eslint`, `eslint-plugin-svelte3` to `desktop/frontend/package.json` devDependencies. Create `.eslintrc.cjs`. Add `"lint": "eslint src/"` script.

2.4 **Prettier** — Add `prettier` + `prettier-plugin-svelte` devDependencies. Create `.prettierrc`. Add `"format": "prettier --write src/"` script.

2.5 **Scaffold Vitest** — Add `vitest`, `@testing-library/svelte`, `jsdom` devDependencies. Create `vitest.config.ts`. Add `"test": "vitest run"` script. Verify `npm run test` prints "No tests found" — zero logic yet.

2.6 **svelte-check** — Add `svelte-check` devDependency. Verify `npx svelte-check` passes.

2.7 **GitHub Actions CI** — Create `.github/workflows/ci.yml` with matrix:
   - `lint-go`: `(cd beamsync && golangci-lint run ./...) && (cd desktop && golangci-lint run ./...)`
   - `lint-svelte`: `npm run lint` in `desktop/frontend/`
   - `test-go`: `(cd beamsync && go test ./...) && (cd desktop && go test ./...)`
   - `test-frontend`: `npm run test` in `desktop/frontend/`
   - `build`: Wails build for ubuntu-latest (smoke test)

### Dependencies

- Phase 1 (version unified so CI build steps reference correct version)

### Risks

- **First lint run floods with warnings** (PITFALLS.md 5b): Use `--new-from-rev` initially. Do not fix all existing lint — flag them and defer to Phase 4 refactoring.
- **Go workspace lint fails** (PITFALLS.md 5a): CI must run per-module, not from workspace root.
- **go.work vs module-direct go vet** (PITFALLS.md 5d): CI scripts must `cd` into each module, not rely on `go.work` file.
- **Frontend infra is net-new** (PITFALLS.md 5c): Dedicated phase — no logic changes mixed in. The PR diff should be 100% config files.

---

## Phase 3: Tests Before Refactoring

**Goal:** Write tests for all untested components *before* any file-splitting or structural changes, so regressions from Phase 4-5 are caught immediately.

**Requirements:** TST-01, TST-02, TST-03, TST-04, TST-05, TST-06

### Plans

3.1 **Test `eventDispatcher`** — `beamsync/event_dispatcher_test.go`:
   - Normal emit fires callback
   - Buffer full: verify `emit()` returns false (drop behavior)
   - Concurrent emit from multiple goroutines — no data race
   - Panic recovery in callback goroutine
   - All in `package beamsync` per existing convention (PITFALLS.md 2a)

3.2 **Test `progressWriter`** — `beamsync/progress_test.go`:
   - Write bytes → progress callback fires with correct `written`/`total`
   - Throttling: consecutive writes within `minInterval` coalesce
   - Completion: final `progress` event + `transfer_complete`
   - Test both `progressWriter` signatures (pre-merge), then merge and retest

3.3 **Test `copyChunked`** — `beamsync/io_helpers_test.go`:
   - Small file: copied fully
   - Large file (>2 chunks): chunked correctly
   - Error mid-stream: returns partial write count + error
   - Uses pipes (`io.Pipe`) — no real files

3.4 **Test `desktop/app.go` logic** — `desktop/app_test.go`:
   - URL builder outputs correct format (before dedup, capture expected output)
   - Version source returns value from wails.json
   - `processEvents` goroutine dispatches events correctly
   - Create test helpers in `export_test.go` per PITFALLS.md 2d

3.5 **Integration test: full send-receive flow** — `beamsync/server_test.go`:
   - Start `httptest.NewServer` → init receiver → issue token → upload file → verify on disk
   - Same pattern as existing `server_test.go` — use `t.Cleanup(server.Close)`
   - Don't start real servers per PITFALLS.md 2c — use `httptest.NewRecorder`/`NewRequest` style

3.6 **Integration test: token auth** — `beamsync/auth_tokens_test.go`:
   - Valid token: passes middleware
   - Expired token: 401
   - Wrong scope: 403
   - Wrong IP binding: 403
   - Reuse consumed token: 401

### Dependencies

- Phase 2 (Vitest scaffold exists for 3.5–3.6 test runner, linters catch test code issues)

### Risks

- **`package beamsync` vs `package beamsync_test`** (PITFALLS.md 2a): All new test files must use `package beamsync` (not `_test` suffix) to access unexported types like `eventDispatcher`, `progressWriter`.
- **Test helper duplication** (PITFALLS.md 2d): Create `export_test.go` early — move shared helpers there so all test files can import them.
- **HTTP handler tests vs real server** (PITFALLS.md 2c): Match existing `httptest.NewRecorder`/`NewRequest` pattern. Add `httptest.NewServer` only for TST-05 integration test.
- **Sequential test execution** (PITFALLS.md 2c): Flag any test that starts a real listener — ensure `t.Cleanup` and no port conflicts.
- **Medium risk** — code is tested for the first time; bugs found now are expected, not alarming.

---

## Phase 4: Go Structural Refactor — `server.go` Extraction

**Goal:** Split `beamsync/server.go` (1726 lines) into ~10 focused files within `package beamsync`. No behavior changes. Extract in dependency order.

**Requirements:** STR-01, STR-02, STR-03, STR-04, STR-05, STR-06, STR-07, STR-08, STR-09, STR-10

### Plans

4.1 **Extract `helpers.go`** — Move `generateID`, `sha256Hex`, `hashMatches`, `clientIP`, `autoRenamePath`, `processManifest`, `uploadBufferInitialCapacity` from `server.go` -> `helpers.go`. No struct dependencies. **Risk: Low.**

4.2 **Extract `rate_limiter.go`** — Move `clientRateLimiter` type + `allow()` method + bucket logic + existing `rate_limiter_test.go`. Move the full type (struct + all methods) as one unit per PITFALLS.md 1e. **Risk: Low.**

4.3 **Extract `progress.go`** — Merge `progressWriter` + `downloadProgressWriter` into single `progressWriter` with `direction string` field ("upload" / "download"). Combine event name selection. Write test for merged type. **Risk: Low.**

4.4 **Extract `io_helpers.go`** — Move `copyChunked` + `chunkBufferPool` + related tests. **Risk: Low.** Note: `chunkBufferPool` must be declared in a shared location — put it here; upload_handler.go and download handlers reference `io_helpers.chunkBufferPool` (same package so just `chunkBufferPool`).

4.5 **Extract `middleware.go`** — Move `tokenMiddleware`, `rateLimitMiddleware`, `setCORSHeaders`, `setRateLimitHeaders`. Extract inline closures from `StartServer`/`StartSender` into methods on `*HTTPServer` to avoid 8-parameter functions (PITFALLS.md 1c). **Risk: Low.**

4.6 **Extract `events.go`** — Move `EventCallback`, `eventDispatchJob`, `eventDispatcher` struct + methods, `startWatchdog`, `safeEmit`, `logTransfer`. **Critical: move `var defaultEventDispatcher` with the type, not separately** (PITFALLS.md 1a). After move: `rg "defaultEventDispatcher" beamsync/` — exactly 1 declaration, N > 0 usages. Add event name constants here:
   ```go
   const (
       EventDeviceConnected    = "device_connected"
       EventDeviceDisconnected = "device_disconnected"
       EventUploadProgress     = "upload_progress"
       EventDownloadProgress   = "download_progress"
       EventTransferRequest    = "transfer_request"
       EventTransferLogged     = "transfer_logged"
       // ...
   )
   ```
   **Risk: Medium.** Must update all `safeEmit` callers if signature changes. Kill the global singleton — make `HTTPServer` hold its own `*eventDispatcher`.

4.7 **Extract `upload_handler.go`** — Move write pipeline: `writeJob`, `manifestEntry`, `writeWorkerCount`, `largeFileThreshold`, `startWriteWorkers`, `writeFileToDisk`. Extract inline upload closures into `*HTTPServer` methods. Depends on: `progress.go`, `io_helpers.go`, `helpers.go`, `events.go`. **Risk: Medium.** Test via Phase 3 integration tests.

4.8 **Extract `download_handler.go`** — Move download pipeline if separate from `StartSender` inline handlers. If download logic is tightly coupled to sender state, keep as `*HTTPServer` methods in `server.go` and extract later. **Risk: Medium.**

4.9 **Trim `server.go`** — After all extractions, `server.go` retains: `HTTPServer` struct, `StartServer()`, `StartSender()`, `serverState`, route setup. It imports nothing new (same `package beamsync`). Verify: `go build ./...` passes. **Risk: High** — last step, must compile against all extracted files.

4.10 **Deduplicate URL construction** — `desktop/app.go`: add `func (a *App) shareURL(port, token string) string` helper. Replace 6+ inline `fmt.Sprintf` calls with it. Also extract `gatherFileEntries` to eliminate duplicate file-info gathering (CONCERNS.md lines 392-407 and 444-459).

4.11 **Delete duplicate firewall scripts** — Remove `beamsync/firewall_setup.sh` and `build/linux/firewall_setup.sh`. Keep only the embedded script in `beamsync/firewall.go`.

### Dependencies

- Phase 3 (tests exist to catch regressions from every extraction step)
- Extract in numbered order within this phase — each step depends on prior extractions

### Risks

- **Package-level singleton split across files** (PITFALLS.md 1a): Move `defaultEventDispatcher` with `events.go`, not before it.
- **`chunkBufferPool` double-declaration** (PITFALLS.md 1b): One declaration in `io_helpers.go`, reference it from `upload_handler.go` and download handlers.
- **Handler closure captures become 8-parameter functions** (PITFALLS.md 1c): Use `*HTTPServer` methods instead.
- **`go:embed` breaks** (PITFALLS.md 1d): Keep `//go:embed ui/*.html ui/*.png` in `server.go` — don't move to another file or package.
- **go vet / build after each extraction** — run `go vet ./...` and `go test ./...` after every extraction step (ARCHITECTURE.md precondition 5).
- **`git mv` for continuity** — use `git mv` for renamed/moved files to preserve git history.
- **High risk for step 4.9** (final trim) — verify every reference in `server.go` is accounted for.

---

## Phase 5: Svelte Component Split

**Goal:** Split `desktop/frontend/src/App.svelte` (2160 lines) into focused components. Create shared Wails store first to avoid passing `window.runtime` as props everywhere.

**Requirements:** SVE-01, SVE-02, SVE-03, SVE-04, SVE-05, SVE-06, SVE-07, SVE-08, SVE-09

### Plans

5.1 **Create Wails store** — `desktop/frontend/src/lib/wails.js`: Svelte writable store wrapping `window.runtime.*`. Exposes:
   - `runtime` — the global for direct calls
   - Reactive stores for common event streams
   - Components import the store, not `window.runtime` directly
   **Must be done before any component extraction** (PITFALLS.md 3a).

5.2 **Define event name constants** — `desktop/frontend/src/lib/events.js`: centralize all event name strings matching `beamsync/events.go` constants. Components import event names from here, not inline strings (PITFALLS.md 3b).

5.3 **Extract `lib/utils.js`** — Move `formatDuration`, `formatSize`, `fileIcon`, `isValidIPv4`, `normalizeTransferStats`, `toast` function from `App.svelte` into `src/lib/utils.js`.

5.4 **Extract `ReceiveView.svelte`** — `src/views/ReceiveView.svelte`: QR display, connection state, file list, receive controls. Props for state; events for `changeSavePath`, `disconnectReset`, `openFile`. **Risk: Low.**

5.5 **Extract `SendView.svelte`** — `src/views/SendView.svelte`: Sender dialog, QR, file list, send controls. Props for sender state. **Risk: Low.**

5.6 **Extract `SettingsView.svelte`** — `src/views/SettingsView.svelte`: Transfer mode, devices, blocked extensions, sounds, save path. **Risk: Medium** — many handler callbacks pass back up; clear event interface needed.

5.7 **Extract `AboutView.svelte`** — `src/views/AboutView.svelte`: Version display, credits, links. **Risk: Low** — static.

5.8 **Extract `ToastManager.svelte`** — `src/components/ToastManager.svelte`: Toast notification rendering. Subscribes to toast state from App shell. **Risk: Low.**

5.9 **Extract `ProgressOverlay.svelte`** — `src/components/ProgressOverlay.svelte`: Floating progress bars for active transfers. Includes progress timeout reset logic (30s → 120s, PITFALLS.md 3d). Keep timeout + event subscription + UI as one cohesive unit. **Risk: Low.**

5.10 **Extract `TransferRequestModal.svelte`** — `src/components/TransferRequestModal.svelte`: Incoming transfer approval modal. **Risk: Low.**

5.11 **Trim `App.svelte`** — After all extractions, `App.svelte` is a thin shell: global drag-and-drop, event binding orchestration, state ownership, mode routing, `svelte:window` handlers. Keep drag-and-drop in App shell (both HTML5 and Wails paths) per PITFALLS.md 3c.

5.12 **Add Vitest component tests** — Test each extracted component:
   - `ReceiveView.svelte`: renders QR when connection established
   - `SettingsView.svelte`: toggles settings, emits change events
   - `AboutView.svelte`: displays version string
   - `ToastManager.svelte`: shows/hides toasts

### Dependencies

- Phase 2 (Vitest scaffold from TOL-05, ESLint from TOL-03)

### Risks

- **`window.runtime` passed as props everywhere** (PITFALLS.md 3a): Create Wails store (5.1) before any component extraction. If extracted first, you'll be retrofitting after the fact.
- **Event name strings mismatch Go side** (PITFALLS.md 3b): Centralize in `events.js` (5.2) before component extraction. Cross-reference with `beamsync/events.go` constants.
- **Drag-and-drop double-fire** (PITFALLS.md 3c): Keep all drag-drop in App.svelte shell. Do not split into child components.
- **Progress timeout ends up in wrong component** (PITFALLS.md 3d): Extract `ProgressOverlay` with timeout + event subscription + UI as one unit.
- **`onDestroy` doesn't clean up event listeners**: Verify every `EventsOn` in extracted components has a corresponding `EventsOff` in `onDestroy`.

---

## Phase 6: Error Handling & Logging

**Goal:** Establish Go error conventions — sentinel errors, `fmt.Errorf("%w")` wrapping, structured `slog` logging. Lightweight enough to run in parallel with Phase 4 but sequenced after Phase 3 for test safety.

**Requirements:** ERR-01, ERR-02, ERR-03, ERR-04

### Plans

6.1 **Define sentinel errors** — `beamsync/errors.go` in `package beamsync`:
   ```go
   var (
       ErrTokenExpired        = errors.New("token expired")
       ErrTokenInvalid        = errors.New("token invalid")
       ErrTokenWrongScope     = errors.New("token scope mismatch")
       ErrTokenIPMismatch     = errors.New("token IP mismatch")
       ErrFileTooLarge        = errors.New("file exceeds size limit")
       ErrTransferNotFound    = errors.New("transfer not found")
       ErrUploadInProgress    = errors.New("upload already in progress")
   )
   ```

6.2 **Wrap errors** — Replace ad-hoc string errors like `fmt.Errorf("invalid token")` with `fmt.Errorf("%w: ...", ErrTokenInvalid, detail)` throughout `beamsync/`.

6.3 **Replace `fmt.Printf` / `log.Println` with `slog`** — `beamsync/` and `desktop/`:
   - `slog.Info("listening", "addr", addr)` — structured fields
   - `slog.Error("upload failed", "file", name, "err", err)` — with stack-like context
   - Remove all `fmt.Printf("ERROR: ...")` patterns
   - Remove all `log.Println(...)` calls

6.4 **Add structured fields** — Every log line includes:
   - Request ID (UUID per HTTP request, passed through context)
   - Session ID (from token, if available)
   - Component (e.g., `upload_handler`, `token_auth`, `rate_limiter`)

6.5 **Audit `errorlint` findings** — Run `golangci-lint` with `errorlint` enabled. Fix all findings: use `errors.Is()` / `errors.As()` instead of `==` comparisons on wrapped errors.

### Dependencies

- Phase 3 (tests confirm error handling changes don't break logic)

### Risks

- **Slog changes alter test assertions** — Update tests that assert on error strings. Use `errors.Is()` in test assertions rather than string matching.
- **Low risk** — conventions-only; no structural or behavioral changes.

---

## Phase 7: CI/CD & Release

**Goal:** Automated build matrix, GitHub Releases, dependency updates, and benchmark tracking.

**Requirements:** CID-01, CID-02, CID-03

### Plans

7.1 **Release workflow** — `.github/workflows/release.yml`:
   - Trigger: tag push matching `v*`
   - Build matrix: ubuntu-latest, macos-latest, windows-latest
   - Steps: `wails build` per platform → `gh release create` with artifacts
   - Version from tag, validated against `wails.json` productVersion (Phase 1)

7.2 **Dependabot config** — `.github/dependabot.yml`:
   - Go modules (`beamsync/go.mod`, `desktop/go.mod`) — weekly
   - npm (`desktop/frontend/`) — weekly
   - Group minor/patch updates, alert on major

7.3 **Benchmark CI** — Add to `ci.yml`:
   - `go test -bench=. -benchmem ./...` in `beamsync/`
   - Output to a PR comment or workflow summary
   - Compare against previous run (store baseline in CI artifacts)

### Dependencies

- Phase 1 (version unified from `wails.json` — release workflow reads same source)
- Phase 2 (CI from TOL-07 exists to extend)

### Risks

- **Wails cross-platform build issues** — Wails requires platform-specific system deps (GCC, WebKit2GTK on Linux, Xcode on macOS). CI runner setup must install these.
- **Benchmark baseline drift** — GitHub Actions runners have variable CPU performance. Only flag >10% regressions to avoid noise.

---

## Phase 8: Documentation & Cleanup

**Goal:** Update all documentation to reflect the new structure, clean up stale artifacts, and add transfer history persistence.

**Requirements:** DOC-01, DOC-02, DOC-03, DOC-04, DOC-05

### Plans

8.1 **Update `ARCHITECTURE.md`** — Reflect new file structure:
   - List all extracted files in `beamsync/` and their responsibilities
   - Document the component tree in `desktop/frontend/src/`
   - Update the architecture diagram
   - Event flow diagram with buffer sizes and drop counters

8.2 **Update `CONTRIBUTING.md`** — Match current DevEx:
   - Pre-commit: `golangci-lint run`, `go vet`, `go test`
   - Frontend: `npm run lint`, `npm run test`, `npm run format`
   - Branch naming: `phase-N-description`
   - Describe the test-first approach: write test → extract/refactor → verify test passes

8.3 **Remove duplicate firewall scripts** — Delete `beamsync/firewall_setup.sh`, `build/linux/firewall_setup.sh`. Verify `beamsync/firewall.go` is the single source of truth.

8.4 **Transfer history persistence** — `beamsync/history.go`:
   - On `logTransfer()`: append to JSON file in XDG config dir (`os.UserConfigDir()`/beamsync/history.json)
   - On startup: load existing history from disk, merge with in-memory ring buffer
   - Max file size: 1 MB (prevent unbounded growth)
   - Rotate on file corruption (delete and start fresh)

8.5 **Verify `README.md`** — Check:
   - Setup instructions match reality
   - Screenshots current (or flag for update)
   - Build/install instructions work with new structure
   - No references to removed files (old firewall scripts, monolith file names)

### Dependencies

- All prior phases (docs must reflect final state)

### Risks

- **Low risk** — documentation and config file management.
- **Transfer history persistence** is the only code change: test that on-disk format is backward-compatible with empty (fresh install) state.

---

## Integrity Checks

### Build after every phase

```bash
(cd beamsync && go build ./... && go vet ./... && golangci-lint run ./...)
(cd desktop && go build ./... && go vet ./... && golangci-lint run ./...)
(cd desktop/frontend && npm run lint && npm run test && npm run check)
```

### Test after every extraction (Phase 4)

```bash
go test ./...     # in both beamsync/ and desktop/
```

### Cross-reference event names

After Phase 4.6 (events.go) and Phase 5.2 (events.js):

```bash
rg "device_connected|device_disconnected|upload_progress|download_progress" beamsync/ desktop/
```

Every event name should appear in both `events.go` and `events.js` — no orphans.

### Wails binding audit

After any rename in `desktop/app.go`:

```bash
rg "window\.go\.main\.App\." desktop/frontend/
```

Every frontend call must match an exported Go method name. No silent `undefined is not a function` (PITFALLS.md 4a).

---

## Out of Scope (enforced)

| Item | Rationale |
|------|-----------|
| E2E testing (Playwright/Cypress) | Too fragile for Wails desktop |
| Sharded rate limiter | Profile first, optimize second |
| Sub-package splitting | Keep `package beamsync` |
| Interface-only abstractions | No interfaces with one implementation |
| DI containers / factory patterns | Overhead without benefit |
| Coverage gates in CI | Information, not enforcement |
| Feature flags / toggles | Not needed for quality pass |
| New features or UI changes | Quality-only milestone per agreement |

---

## Quick Reference

| Prefix | Category | Count | Phase |
|--------|----------|-------|-------|
| EVT | Event System | 3 | 1 |
| VER | Version | 3 | 1 |
| TOL | Tooling | 7 | 2 |
| TST | Testing | 6 | 3 |
| STR | Structural | 10 | 4 |
| SVE | Svelte | 9 | 5 |
| ERR | Error Handling | 4 | 6 |
| CID | CI/CD | 3 | 7 |
| DOC | Documentation | 5 | 8 |

---

*Generated from: PROJECT.md, REQUIREMENTS.md, research/SUMMARY.md, research/ARCHITECTURE.md, research/PITFALLS.md, codebase/CONCERNS.md*
*Last updated: 2026-07-20*
