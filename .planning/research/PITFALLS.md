# Refactoring Pitfalls — BeamSync Codebase

**Date:** 2026-07-20
**Context:** Quality overhaul for a working Go + Wails v2 + Svelte 3 desktop P2P file transfer app. The app works — the risk is breaking it during cleanup.

---

## 1. Splitting `server.go` (1726 lines) into Separate Files

### 1a. Package-level singletons break when files are extracted

**The trap:** `defaultEventDispatcher` (server.go:81) is a package-level `var` created at `init()`. All 12+ call sites reference it as `defaultEventDispatcher.Emit(...)`. Extracting the event dispatcher into `events.go` is safe _only_ if you move the variable along with the type. If you create the dispatcher as a local in `events.go`'s `init()`, and `server.go` still references the old one — you now have two dispatchers and silence drops.

**Warning signs:**
- Any file that references `defaultEventDispatcher` after extraction still works because `init()` runs first — but only if you check _every_ call site.
- Events stop reaching the frontend silently (no compile error).

**Prevention:**
- Move the `var` declaration with the type, not separately.
- After extraction: `rg "defaultEventDispatcher" beamsync/` — confirm exactly one declaration, N > 0 usages.
- Add a test that emits an event and verifies the callback fires.

**Phase:** Refactoring phase (server.go split)

### 1b. `chunkBufferPool` lives in server.go but is used across handler boundaries

**The trap:** `chunkBufferPool` (server.go:88) is a `sync.Pool` of 8MB byte slices. If you split upload handling into `upload_handler.go` and download handling into `download_handler.go`, both need access to this pool. A naive move puts it in one file — then the other file compiles but uses its own pool.

**Warning signs:**
- Double-pool created: each file declares its own pool, doubling memory in worst case.
- `go vet` won't catch this. No compile error.

**Prevention:**
- Declare `chunkBufferPool` in a shared file (e.g., `pool.go`) that both handlers import.
- Or: make it a field on `HTTPServer` instead of a package var (cleaner, but bigger diff).

**Phase:** Refactoring phase (server.go split)

### 1c. HTTP handler closures capture server state implicitly

**The trap:** The `StartServer()` function (server.go:771-1360) registers ~20 HTTP handler functions, most of which close over local variables (`server *HTTPServer`, `store *tokenStore`, `rateLimiter`, `settings`, `history`, `eventDispatcher`, `stats`, etc.). Extracting handlers into standalone functions means these were implicit parameters — now they must be explicit. Forgetting one creates a compile error (good) but adding them all changes every handler signature in one commit, making the diff unreviewable.

**Warning signs:**
- The extracted file has a function with 8+ parameters matching HTTPServer fields.
- Every extracted handler needs `server` as a parameter.

**Prevention:**
- Two-step extraction: (1) add methods to `HTTPServer` struct in the new file, capturing state via `server.someField` — zero parameter change. (2) Optionally slim later. This keeps the diff reviewable.
- Or: use a method set `func (s *HTTPServer) handleUpload(...)` — methods live in the new file, callers in server.go just reference `s.handleUpload`.

**Phase:** Refactoring phase (server.go split)

### 1d. `go:embed` directive lives in server.go — moving UI files breaks it

**The trap:** The `//go:embed ui/*.html ui/*.png` directive is near the top of server.go. If you split the file but keep the directive in what becomes `server.go`, it still works. But if someone moves it to a different package (like `beamsync/ui/`), the import path changes and all embedded references break.

**Warning signs:**
- Build error: `pattern ui/*.html: no matching files found`
- The `embed.FS` variable is declared in server.go and referenced only in upload HTML serving

**Prevention:**
- Keep the `embed` directive in the file that serves embedded content. Since the embedded web UI files are at `beamsync/ui/`, they naturally stay with server.go or a small `serve_ui.go`.
- Document: "embedded assets and their serving handler must remain in the same package as the `embed` directive."

**Phase:** Refactoring phase (server.go split)

### 1e. Rate limiter is defined partly at top (type) and partly at bottom (usage)

**The trap:** The `clientRateLimiter` type is defined at server.go:167, but the `allow()` method and bucket logic are at server.go:203-250. If you split `middleware.go` or `rate_limiter.go`, you must move the _entire_ type (struct + all methods). Partially moving methods to a different file in the same package is fine (Go permits this), but partial moves to a _different package_ break unexported field access.

**Warning signs:**
- `clientRateLimiter.allow()` undefined if moved to a new package without moving the struct
- Unexported fields in tests break

**Prevention:**
- Keep rate limiter in the same package initially. Extract type + all methods + test file as one unit.
- Don't move to a sub-package (`beamsync/auth/`) until after the within-package split compiles and tests pass.

**Phase:** Refactoring phase (server.go split)

---

## 2. Adding Tests to an Untested Codebase

### 2a. `package beamsync` uses internal (unexported) state — tests in `_test` package can't access it

**The trap:** Existing tests use `package beamsync` (not `package beamsync_test`), giving them access to unexported symbols. This is correct but means new test files _must_ also use `package beamsync`. If someone creates a test file with `package beamsync_test`, it won't compile against unexported types like `clientRateLimiter` or `progressWriter`.

**Warning signs:**
- Test file uses `package beamsync_test` but references `clientRateLimiter` → compile error
- Mixed package declarations in test files create confusion

**Prevention:**
- All existing tests use `package beamsync` — continue the pattern.
- If you later move code to sub-packages, only export what's needed for tests.

**Phase:** Testing phase

### 2b. What to test first: the event bridge, not the file I/O

**The trap:** The instinct is to test the HTTP upload handler first (it's the "real" functionality). But the upload handler has many I/O dependencies (temp files, multipart parsing, write workers). The highest-risk, lowest-test-coverage component is `eventDispatcher` (server.go:39-80) — it's the sole path for UI updates. A silent event drop makes the app look frozen.

**Priority order for this codebase:**
1. `eventDispatcher` — buffer-full behavior, panic recovery, concurrent safety
2. `progressWriter` / `downloadProgressWriter` — event format, throttling, both types (then merge them)
3. Wails `eventChan` bridge (`app.go:eventChan`, `makeCallback`) — drop behavior
4. HTTP handler auth-middleware interaction (partial test exists at server_test.go:24, extend it)
5. Frontend (Svelte): start with 1 component test for a design-system component (skeleton test infrastructure before tests)

**Warning signs:**
- eventDispatcher never tested; its behavior is emergent
- Frontend has zero test infrastructure (no `vitest`, no `@testing-library/svelte`, not even in devDependencies)

**Prevention:**
- Write eventDispatcher test before touching server.go extraction (catch regressions from the split)
- Add `vitest` + `@testing-library/svelte` to frontend devDependencies before writing any component test

**Phase:** Testing phase

### 2c. HTTP handler tests depend on `httptest` but server_test.go tests are sequential

**The trap:** Existing server_test.go tests create `newTokenStoreForTest(t)`, set up `httptest.NewRecorder`/`httptest.NewRequest`, call handlers directly. They don't start the real server. If you write new tests against the real server (with `net/http/httptest.NewServer`), port conflicts and goroutine leaks appear.

**Warning signs:**
- `httptest.NewServer` creates a real listener on a random port
- Tests that start servers leave goroutines running after completion

**Prevention:**
- Match existing test style: call middleware or handler functions directly, don't start real servers.
- If you must start a real server, use `t.Cleanup(server.Close)` and verify goroutine count.

**Phase:** Testing phase

### 2d. Test helper duplication — `newTokenStoreForTest` pattern

**The trap:** `newTokenStoreForTest(t)` is defined in `auth_tokens_test.go`. If you add test files for new extracted components, they'll need similar helpers. The temptation is to duplicate the helper or import from another test file (Go doesn't allow test-only imports from production files). 

**Prevention:**
- Put shared test helpers in `export_test.go` (the standard Go pattern: `export_test.go` re-exports unexported internals for use by external test packages).
- Or define all helpers in each test file that needs them — acceptable for small duplication.

**Phase:** Testing phase (early, when adding first new test file)

---

## 3. Restructuring App.svelte (2160 lines)

### 3a. Wails `window.runtime` is a global — extract into a store, not props

**The trap:** The entire Svelte app uses `window.runtime.SendEvent()` and `window.runtime.EventsOn()` for Wails IPC. If you split into `TransferProgress.svelte`, `SettingsPanel.svelte`, etc., each component needs access to runtime. Passing runtime as a prop to every component is fragile and couples every component to Wails.

**Warning signs:**
- Every new component has `export let runtime` in its props
- Renaming events requires updating N component files

**Prevention:**
- Create a Svelte store (`wails-store.js`) that wraps `window.runtime` and exposes reactive `$events`. Components subscribe to the store, not to `window.runtime` directly.
- Do this _before_ splitting the component — one commit to introduce the store, then split.

**Phase:** Frontend refactoring

### 3b. Event name strings must match Go side exactly — and there are many

**The trap:** App.svelte handles events like `device_connected`, `transfer_request`, `upload_progress`, `download_progress`, `device_disconnected`, `transfer_complete`, `transfer_error`, `transfer_stats`, `heartbeat`, `server_ready`, `receiving_started`, onevent callbacks tied to these strings. If you rename an event in Go during refactoring, the Svelte side breaks silently (no handler fires).

**Warning signs:**
- Go side renames `upload_progress` to `upload_progress_v2` — Svelte handler never fires, no error
- Frontend shows stale progress

**Prevention:**
- Define event names as constants in Go (`beamsync/events.go`) and re-export to JS via a single source (or at least document the event protocol).
- In Svelte, centralize event name strings in a constants file, not inline in component handlers.

**Phase:** Frontend refactoring (coordinate with event system refactoring in server.go split)

### 3c. Drag-and-drop handler is fragile — both HTML5 and Wails paths fire

**The trap:** App.svelte has two drag-and-drop paths (HTML5 `window.ondrop` + Wails `OnFileDrop` binding) with a 500ms `dropGuard` to prevent double-fire (CONCERNS.md:160-165). Splitting the component means moving these handlers. If the drop guard logic ends up in different lifecycle contexts (e.g., one in `App.svelte` and one in a child), the timing assumption breaks.

**Warning signs:**
- `dropGuard` is set in one component's scope, checked in another — `setTimeout` closure mismatch
- Files are sent twice or not at all

**Prevention:**
- Keep all drag-and-drop logic in the parent component until you decide to use _only_ one path.
- If you remove the HTML5 handler (recommended), do it separately and test with actual file drops.

**Phase:** Frontend refactoring

### 3d. `_progressTimeout` (30s) is a magic number used once — extraction loses context

**The trap:** The 30-second progress timeout (App.svelte:422-440) combined with event names like `upload_progress` and `download_progress` must be kept together. If you split progress into `TransferProgress.svelte`, the timeout reset logic and event subscription must move together.

**Warning signs:**
- Progress timeout lives in parent, events emit in child — `onDestroy` doesn't clean up
- Stale progress bar shows "Idle" during active transfer

**Prevention:**
- Extract progress timeout + event subscription + UI as one cohesive unit.
- Verify `onDestroy` cleans up all event listeners.

**Phase:** Frontend refactoring

---

## 4. Wails Bindings During Refactoring

### 4a. Binding method names are strings, not symbols

**The trap:** Wails v2 exposes Go methods to JS via binding. The Go side uses `//export MethodName` or struct methods on `*App`. The JS side calls `window.go.main.App.MethodName()`. If you rename a Go method during refactoring, the JS call silently returns `undefined is not a function`. There is no compile-time check across the boundary.

**Warning signs:**
- No compile error in Go or JS when a method is renamed
- Runtime error: `window.go.main.App.StartReceiver is not a function`

**Prevention:**
- Rename Go methods in one commit, update all frontend callers in the same commit.
- Test the full flow (click button → method fires → event returns) after any rename.
- Keep a manifest of all bound methods and their frontend call sites.

**Phase:** Any phase touching `desktop/app.go`

### 4b. `processEvents` goroutine uses a buffered channel with silent drops

**The trap:** `eventChan` (app.go:70, cap=100) feeds into `processEvents()` goroutine (app.go:159). `makeCallback` (app.go:232-237) does a non-blocking send with `default:` — events are silently dropped when the channel is full. Refactoring the event system must preserve this behavior or explicitly change it. Adding new event types increases fill rate.

**Warning signs:**
- Adding a new event type (e.g., `file_selected`) pushes the channel closer to 100
- No unit test monitors drop rate

**Prevention:**
- Make `eventChan` capacity a named constant. Review it when adding new event types.
- Add a drop counter metric (atomic uint64, logged or exposed via a debug endpoint).
- If you change from non-blocking to blocking send, ensure the producer goroutine can't deadlock with the consumer.

**Phase:** Event system refactoring

### 4c. `wails.json` productVersion is disconnected from Go version strings

**The trap:** Three version strings exist: `wails.json` productVersion, `desktop/app.go:29` (`currentVersion = "v2.4.0"`), and `App.svelte:944` (`appVersion="v2.2"`). Refactoring the version display is a perfect opportunity to unify to one source, but if you forget any path, the version disparity persists.

**Warning signs:**
- `wails.json` says v2.4.0, About panel says v2.2
- User confusion in bug reports

**Prevention:**
- Derive all version strings from `wails.json` or a build-time injected `ldflags` value.
- Remove hardcoded version from `app.go` and `App.svelte`.
- Fix this in _one_ isolated PR, don't bury it in a larger refactoring.

**Phase:** Frontend refactoring, or a dedicated cleanup PR before anything else

---

## 5. Introducing Linting/CI to an Existing Project

### 5a. `golangci-lint` on a Go workspace with `replace` directive

**The trap:** The project is a Go workspace: `beamsync/go.mod` + `desktop/go.mod` with `replace beamsync => ../beamsync`. Most CI linters assume a single module root. Running `golangci-lint` from the workspace root may not analyze `beamsync/` correctly.

**Warning signs:**
- Linter says "no go files to analyze" or skips `beamsync/`
- `replace` directive causes "module not found" in CI

**Prevention:**
- Configure `golangci-lint` to run separately on each module directory, or use Go workspaces (Go 1.18+) with `go.work`.
- CI pipeline: lint `beamsync/` first, then `desktop/`.
- Script: `(cd beamsync && golangci-lint run ./...) && (cd desktop && golangci-lint run ./...)`

**Phase:** CI/DevEx phase

### 5b. First lint run will produce hundreds of warnings — suppress nothing silently

**The trap:** When you first run `golangci-lint` on this codebase, it will flag hundreds of issues (unused parameters, missing comments, `fmt.Printf` instead of structured logging, naming conventions). The temptation is to add `//nolint` everywhere or run `--fix` blindly.

**Warning signs:**
- The PR diff is 80% whitespace / formatting changes — impossible to review
- `//nolint` comments outnumber lines of logic

**Prevention:**
- Start with a `.golangci-lint.yml` that enables only _one_ linter at a time (e.g., `govet` first, then `staticcheck`).
- Two-phase CI introduction: (1) `--new-from-rev=HEAD~1` to only lint new code, (2) turn on full linting after existing issues are addressed in a separate cleanup PR.
- Formatting (gofmt, goimports) should be a separate PR done before any logic change.

**Phase:** CI/DevEx phase

### 5c. Frontend has zero lint/test infrastructure — adding it is a setup project

**The trap:** `desktop/frontend/package.json` has `dev`, `build`, `preview` scripts only — no `lint`, `test`, `typecheck`, `format`. Adding `eslint`, `vitest`, `svelte-check`, `prettier` means: adding devDependencies, creating config files (`.eslintrc.cjs`, `vitest.config.ts`, `svelte.config.js`), and possibly dealing with Svelte 3's pre-processor setup. If you add testing here, you're building the foundation _and_ writing tests in the same PR — too much.

**Warning signs:**
- The "add tests for frontend" PR suddenly grows config files, CI changes, and test infrastructure setup
- 80% of the diff is configuration, 20% is tests

**Prevention:**
- Dedicated "frontend tooling" PR: add vitest, eslint, prettier, svelte-check. Zero logic changes. Verify `npm run test` prints "No tests found" successfully.
- Then a second PR with actual tests.

**Phase:** CI/DevEx phase

### 5d. `go vet` passes but `go vet ./...` on workspace may not

**The trap:** `beamsync/go.mod` declares `go 1.25.5`. `go vet` on individual modules works, but `go vet ./...` from a workspace root with a `replace` directive can produce "inconsistent vendoring" or "missing module" errors if `go.work` is missing or stale.

**Warning signs:**
- Developer runs `go vet ./...` from repo root → gets module errors
- CI runs `go vet` from workspace root → fails
- Fine locally because developer has an active `go.work` file

**Prevention:**
- Add a `Makefile` or script that explicitly runs vet on each module: `go vet ./...` in `beamsync/` and `desktop/` separately.
- CI should not rely on `go.work` — run from each module directory.

**Phase:** CI/DevEx phase

---

## 6. Go Module Restructuring

### 6a. Moving from `package beamsync` to sub-packages breaks all `desktop/` imports

**The trap:** Currently everything in `beamsync/` is `package beamsync`. `desktop/app.go` imports `beamsync` and calls `beamsync.StartServer()`, `beamsync.NewTokenStore()`, `beamsync.ServerScheme()`, etc. If you introduce sub-packages (`beamsync/auth`, `beamsync/transfer`), every reference in `desktop/app.go` must change. The `replace beamsync => ../beamsync` directive keeps the module import working, but `beamsync.StartServer` becomes `transfer.StartServer` — a breaking rename.

**Warning signs:**
- `desktop/` doesn't compile after sub-package creation
- Import paths change, method receivers change, `beamsync.X` references must be updated

**Prevention:**
- Do _not_ create sub-packages unless the final API is settled. The current monolithic package is a valid Go pattern (the standard library's `package net` is 8000+ lines).
- If you must split: (1) create a compatibility layer — `package beamsync` re-exports symbols from sub-packages so `desktop/` doesn't need to change. (2) Then migrate callers one by one.
- Better: keep `package beamsync` and split into multiple files within the same package. This is zero-risk and achieves the readability goal without breaking imports.

**Phase:** Module restructuring phase (if undertaken)

### 6b. Renaming functions during extraction breaks Goroutine callers

**The trap:** `StartServer()` in server.go is called from `desktop/app.go` in a goroutine: `go func() { beamsync.StartServer(...) }()`. If you rename `StartServer` to `StartReceiverServer` during extraction, the goroutine call breaks silently at compile time (Go catches this). But if you refactor the function signature (e.g., add a `config` parameter), the Go compiler catches it — but only if you update the caller in the same commit. If the goroutine setup and the function are in different repos/modules, very bad.

**Warning signs:**
- Cross-module compile errors
- `desktop/app.go` references `beamsync.StartServer` but it's now `beamsync.StartReceiverServer`

**Prevention:**
- Extract-within-package first (same `package beamsync`, new files) — zero import changes.
- If you later change API surface, update all callers in `desktop/` in the same commit.
- `rg "beamsync\.\w+" desktop/` after any rename.

**Phase:** Module restructuring phase

### 6c. Test files in `package beamsync` access unexported functions — sub-package move breaks tests

**The trap:** `auth_tokens_test.go` calls `newTokenStoreForTest(t)` (unexported helper). `server_test.go` accesses `tokenStore` internals directly. Moving token logic to `beamsync/auth/` means these tests can no longer access unexported fields. Either the helpers must be exported, or the tests must be rewritten.

**Warning signs:**
- `tokenStore` becomes `auth.TokenStore` (exported) — but `maxClientsPerToken` (unexported) is now inaccessible
- Tests that exercise unexported behavior now compile but test wrong things

**Prevention:**
- Before moving to sub-packages, audit which unexported functions need test access. Export them (they were always part of the tested contract) or move them with the tests.
- Use `export_test.go` pattern: a file in `package auth` that exports internals only during `_test` builds.

**Phase:** Module restructuring phase (jointly with testing)

### 6d. Name collision when `beamsync` becomes both a module path and a package name

**The trap:** Currently the module is `module beamsync` and the package is `package beamsync`. If you create `beamsync/auth/auth.go`, the import path is `beamsync/auth` and the package name is `auth`. Works fine. But if you later create `beamsync/beamsync/` (common migration pattern for library extraction), you get `beamsync/beamsync` as the import path — confusing and can cause shadowing.

**Warning signs:**
- Import aliases needed everywhere: `import beamsync "beamsync/beamsync"`
- Go tooling shows ambiguous references

**Prevention:**
- Never create a package named the same as the module path.
- If you must have a top-level package, keep `package beamsync` at the module root and add sub-packages for new concerns.

**Phase:** Module restructuring phase

---

## Cross-Cutting Risks

### Silent event drops worsen during refactoring

When you split server.go and App.svelte, the number of event types can increase (each extractor adds "I'm alive" signals). The `eventDispatcher` (256 cap) and `eventChan` (100 cap) both drop events silently. More event types + same buffer capacity = higher drop probability.

**Mitigation:** Resize buffers _before_ splitting. 256 → 1024 for `eventDispatcher`. 100 → 512 for `eventChan`. Or add drop counters.

### `wails.json` and Wails v2 build config must stay valid

Wails v2 (`v2.12.0`) has specific requirements: `-tags desktop,production` on build, specific `ldflags`. If your refactoring adds new Go build tags or changes module paths, the Wails build command may need updating. Test with `wails build` after any module restructuring.

### Firewall script in three locations (embedded, on-disk, build/)

CONCERNS.md:56-61 flags this. If you touch `beamsync/firewall.go`, update or remove the on-disk copies. A refactoring that changes firewall behavior must propagate to all three copies, or delete the duplicates first.

---

## Summary: Which Phase Should Address What

| Pitfall | Phase |
|---------|-------|
| 1a-1e: server.go split gotchas | Refactoring (server.go split) |
| 2a-2d: testing traps | Testing |
| 3a-3d: Svelte split gotchas | Frontend refactoring |
| 4a-4c: Wails binding traps | Any phase touching app.go |
| 5a-5d: Linting/CI traps | CI/DevEx |
| 6a-6d: Module restructure traps | Module restructuring (if done) |
| Buffer sizing | Event system refactoring (before split) |
| Version string unification | Dedicated cleanup PR (before 3.0 work) |
| Firewall deduplication | Any phase touching firewall.go |
