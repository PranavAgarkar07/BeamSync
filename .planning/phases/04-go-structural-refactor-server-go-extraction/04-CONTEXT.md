# Phase 4: Go Structural Refactor — `server.go` Extraction - Context

**Gathered:** 2026-11-13
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase splits `beamsync/server.go` (1538 lines) into focused files **within
`package beamsync`**. It is a **pure structural move: no behavior changes.**

**In scope:**
- Moving each concept into its own file (`events.go`, `rate_limiter.go`, `progress.go`,
  `io_helpers.go`, `middleware.go`, `helpers.go`, `upload_handler.go`, `download_handler.go`)
- Converting the inline HTTP handler closures in `StartServerE` / `StartSenderE` into
  methods on `*HTTPServer` (PITFALLS.md 1c)
- Trimming `server.go` to orchestration: the `HTTPServer` struct, `serverState`,
  `StartServerE`, `StartSenderE`, route registration, and the `//go:embed ui/*` directive

**Out of scope:**
- Any behavior change — including making the event dispatcher per-server (D-02)
- Event-name constants (Phase 5's `SVE-08` owns the shared Go/Svelte event names)
- Deleting `build/linux/firewall_setup.sh` (that is `DOC-03`, Phase 8)
- Sub-packages, interfaces with one implementation, DI containers (ROADMAP "Out of Scope")
- Any change to the `beamsync` module's public API

</domain>

<decisions>
## Implementation Decisions

### Scope & correctness
- **D-01:** The extraction set is **re-derived from the current 1538-line file**, not patched
  from the roadmap's previous list. That list named nine symbols that no longer exist
  (`hashMatches`, `processManifest`, `uploadBufferInitialCapacity`, `writeJob`,
  `writeWorkerCount`, `largeFileThreshold`, `startWriteWorkers`, `writeFileToDisk`,
  `downloadProgressWriter`). Planning against a stale symbol list guarantees plans for code
  that is not there.
- **D-02:** The **global event dispatcher stays global.** The previous plan proposed killing
  `defaultEventDispatcher` and giving each `HTTPServer` its own — justified as *"12+ call
  sites"*. There is now exactly **one** call site (`safeEmit`), so the safety argument no
  longer holds, and this phase's charter is *no behavior changes*. A singleton's ownership
  change is a behavior change; it buys nothing here. It moves **with its type** to
  `events.go` (PITFALLS.md 1a), and the roadmap's own verification is kept:
  `rg "defaultEventDispatcher" beamsync/` must show exactly **1 declaration**, N > 0 usages.
  Per-server dispatch is deferred to a phase that can design it deliberately.
- **D-03:** The **duplicate firewall script is not touched in this phase.** The previous plan
  4.11 said to delete `beamsync/firewall_setup.sh` — but that file is the `//go:embed
  firewall_setup.sh` source read at compile time; deleting it breaks the build. Only
  `build/linux/firewall_setup.sh` is the true duplicate (byte-identical, referenced by
  nothing), and it belongs to `DOC-03` in Phase 8, whose wording has been corrected to name
  that file alone.
- **D-04:** The inline handler closures become **methods on `*HTTPServer`**, not standalone
  functions with long parameter lists (PITFALLS.md 1c). The upload handler needs `uploadDir`,
  which is a `StartServerE` parameter — it is added as an `HTTPServer` field so the method
  signature stays short. This is what makes `server.go` genuinely "orchestration only"
  (`STR-09`).

### Preserving Phase 3's hardening
- **D-05:** The extraction must carry Phase 3's concurrency work in `beamsync/` **intact**.
  These live in `beamsync/` and every one is pinned by a test that fails if it is dropped or
  weakened:
  - `eventDispatcher.stateMu` and `eventDispatcher.close()` (the dispatcher keeps its own
    lock so `close()` cannot race with `emit()`)
  - `progressWriter.eventName` and the nil-guarded `progressWriter.now`
  - `progressWriter.Write`'s zero-byte guard
  - `HTTPServer.settingsMu`, `Settings()` returning a **snapshot**, `UpdateSettings` swapping
    atomically, and `currentSettings()` — the live `TransferSettings` carries the device
    blocklist/trustlist, and a torn read could mis-apply it
  - `rateLimitMiddleware` taking a **settings provider**, not a captured pointer
  No extraction may "simplify" any of these — a dropped lock is a data race, and Phase 3's
  mutation campaign exists precisely to catch that.

  **Scope note (corrected 2026-11-13):** the App-level `endpointMu`, `serverMu` and
  `stateMu` live in `desktop/app.go`, **not** in `beamsync/`. This phase does not touch
  `desktop/app.go`, so they are out of scope here — but `beamsync` must not gain a
  dependency on them either.
- **D-06:** `//go:embed ui/*.html ui/*.png` **stays in `server.go`** (PITFALLS.md 1d) — it must
  remain in the package that serves the assets, and moving it breaks the build. Likewise
  `//go:embed firewall_setup.sh` stays in `firewall.go`. The `uiFS` declaration has **4 use
  sites, all in `server.go`**, so keeping it there leaves zero cross-file references.

### Filling gaps the roadmap and an earlier draft of this file left open
- **D-10:** The `/upload` and download handlers capture **`emit`**, which is **not** an
  `HTTPServer` field — it is a parameter of `StartServerE` / `StartSenderE`. Converting the
  closures into methods (D-04) therefore needs an **`emit EventCallback` field** on
  `HTTPServer` alongside `uploadDir`, so neither handler grows a long parameter list. One
  field serves both handler files.
- **D-11:** `startBootstrapRotator` and `getLocalIPForURL` were named by neither the
  roadmap nor the earlier draft of this file, so they had no assigned home. They belong in
  **`events.go`** (the rotator emits events) and **`helpers.go`** respectively.
- **D-12:** The two download handler bodies — `/download` and `/download/N` — are
  **behaviorally identical** apart from the name variable (`filename` vs `realName`, both
  `filepath.Base(filePath)`), a comment, and one extra level of nesting inside the loop.
  Verified by whitespace-insensitive diff of the two bodies. A single
  `handleDownload(filePath string, w http.ResponseWriter, r *http.Request)` method serves
  both call sites — one implementation, not two copies to keep in sync.
- **D-13:** Test **function names** are the keys of Phase 3's mutation table, so tests may
  move between `_test.go` files but must never be **renamed** — the table in `03-REVIEW.md`
  would stop matching.

### Sequencing
- **D-07:** Extraction is **risk-ascending**: the pure moves first (`events.go`,
  `rate_limiter.go`, `progress.go`, `io_helpers.go`, `middleware.go`, `helpers.go`), then the
  handler-body extractions that convert closures into methods (`upload_handler.go`,
  `download_handler.go`), then the final trim. Each step is verified on its own.
- **D-08:** After **every** extraction step: `go build ./...`, `go vet ./...`, and
  `go test -race ./...` must pass in both modules. This is Phase 3's safety net being used
  for its intended purpose — a regression should be caught at the step that introduced it,
  not at the end.
- **D-09:** Use `git mv` for moved code so git history is preserved, and keep each extraction
  as its own commit so a revert is surgical.

### Agent discretion
- Exact ordering within the low-risk group (D-07) and the split between `helpers.go` and
  `io_helpers.go` — provided the PITFALLS 1b single-declaration rule for `chunkBufferPool`
  holds.
- Whether `PendingTransfer` and the `PendingTransfer` bookkeeping move to `events.go` or
  stay in `server.go`.
- The exact `HTTPServer` field names for the stored upload directory and the emit
  callback (D-10).
- Whether the route-registration tables in `StartServerE` / `StartSenderE` are reformatted
  into a named helper — as long as the registered paths and middleware order are unchanged.
- Whether `serverState` moves out of `server.go` — it is used by the watchdog and both
  handlers, so it may fit better in `events.go` or stay.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope
- `.planning/ROADMAP.md` — Phase 4 section (re-derived plan list, dependencies, risks)
- `.planning/REQUIREMENTS.md` — Phase 4 block: `STR-01`, `STR-02`, `STR-04`–`STR-09`
- `.planning/PROJECT.md` — project context and the quality-only scope of this milestone

### Refactoring pitfalls (must-follow)
- `.planning/research/PITFALLS.md` §1a–1d — the `server.go` split gotchas: the package
  singleton must move with its type; `chunkBufferPool` must have exactly one declaration;
  handler closures must become methods, not 8-parameter functions; the `go:embed` directive
  must stay in the serving package
- `.planning/codebase/CONVENTIONS.md` — Go conventions this repo follows
- `.planning/codebase/ARCHITECTURE.md` — the layering this extraction must preserve
- `.planning/codebase/STRUCTURE.md` — target directory layout

### The safety net this phase depends on
- `.planning/phases/03-tests-before-refactoring/03-VERIFICATION.md` — 18/18 must-haves
  verified
- `.planning/phases/03-tests-before-refactoring/03-REVIEW.md` — the 33-mutant table; this
  is the checklist of behaviours that must stay green through every extraction
- `.planning/phases/03-tests-before-refactoring/03-CONTEXT.md` — the Phase 3 decisions
  (D-01…D-13) that D-05 carries forward
- `.planning/phases/03-tests-before-refactoring/03-PATTERNS.md` — existing test analogs and
  the file→analog map for new test files

### Production code being split
- `beamsync/server.go` — the file being split (1538 lines). Key regions: event dispatcher
  (28–113, 227), rate limiter (115–226), embed + pool (242–250), `serverState` (251–301),
  `HTTPServer` (302–426), `progressWriter` (428–459), I/O helpers (460–487), helpers
  (488–505, 638–653), middleware (506–637), watchdog/rotator/emit (654–749),
  `StartServerE` (750–1192), `StartSenderE` (1193–1538)
- `beamsync/server_test.go`, `beamsync/rate_limiter_test.go`, `beamsync/progress_test.go`,
  `beamsync/progress_integration_test.go`, `beamsync/progress_wiring_test.go`,
  `beamsync/event_dispatcher_test.go`, `beamsync/permissions_test.go` — the tests that must
  stay green; they move with their subject or keep referring to the same package symbols

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `beamsync/localip.go` — already extracted in an earlier phase and a model for a clean
  single-concept file (`GetLocalIP`, `firstUnicastIPv4`, `dialOutIP`). Note: it has **no**
  colocated test file — `port_manager.go` ↔ `port_manager_test.go` is the tightest
  source+test pair in the package.
- `beamsync/auth_tokens.go`, `beamsync/permissions.go`, `beamsync/history.go`,
  `beamsync/stats.go`, `beamsync/tls.go`, `beamsync/port_manager.go`,
  `beamsync/firewall.go` — the seven files Phase 1/2 already extracted from `server.go`.
  They show the established shape: one concept per file, same package, tests alongside.
- `beamsync/port_manager.go` + `beamsync/port_manager_test.go` — the canonical
  source-and-its-test pair to copy for the new files

### Established Patterns
- One concept per file, all in `package beamsync` — no sub-packages
- No file-level doc comment; doc comments on **exported** symbols only, plus
  invariant-explaining comments on unexported ones
- One stdlib-only `import ( … )` block per file
- Tests are co-located and named `{source}_test.go`; a file and its test move together
- `t.Fatalf("<subject> = <got>, want <want>")` — no assertion library
- Same-package white-box testing, so unexported symbols stay reachable
- `//go:embed` declarations sit in the file that owns the asset

### Integration Points
- `desktop/app.go` imports `beamsync` and calls `StartServerE` / `StartSenderE` /
  `RunFirewallSetup` / `ServerScheme` — the exported surface must not change
- `beamsync/server.go` is the only file that references `uiFS`, `defaultEventDispatcher`,
  `chunkBufferPool` and `serverState` today — after the split, each has exactly one home
- `desktop/app.go` also references `beamsync.GetLocalIP()` — that call must keep working
  after `localip.go` is untouched

</code_context>

<specifics>
## Specific Ideas

- **This is the payoff phase for Phase 3.** Every extraction step has an automated check
  that fails if the step breaks something. The phase should be planned to *use* that net
  continuously — verify after each step, not once at the end.
- **Risk-ascending order is deliberate.** The pure file moves cannot change behavior if done
  carefully; the closure-to-method conversions can. Sequencing the riskiest changes last
  means a problem is always close to its cause.
- **The `server.go` line count is a success metric, not the goal.** The goal (STR-09) is that
  `server.go` contains only orchestration. The count follows from that.

</specifics>

<deferred>
## Deferred Ideas

- **Per-server event dispatcher.** Move `defaultEventDispatcher` off the package global so
  each `HTTPServer` owns its own. A behavior change, not a move — it needs its own design
  (where does `safeEmit` get the server from?) and its own plan. Deferred.
- **Event-name constants** (`EventDeviceConnected`, `EventUploadProgress`, …) shared between Go
  and Svelte — Phase 5's `SVE-08`.
- **Delete `build/linux/firewall_setup.sh`** — `DOC-03`, Phase 8.
- **`uploadIntegrityHeader` / `sha256Hex`** are currently unused (`//nolint:unused // reserved`).
  Phase 3 deliberately left them alone: removing them is not a structural move, and the
  roadmap's "wire up integrity checking" idea is a capability, not a refactor. Left for a
  phase that owns it.

</deferred>

---

*Phase: 4-Go Structural Refactor — `server.go` Extraction*
*Context gathered: 2026-11-13*
