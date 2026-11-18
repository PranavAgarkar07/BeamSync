---
phase: 04-go-structural-refactor-server-go-extraction
verified: 2026-11-18T22:54:12Z
status: human_needed
score: 23/25 must-haves verified
behavior_unverified: 2
overrides_applied: 0
behavior_unverified_items:
  - truth: "04-02 backstop — the limiter's map and counter are safe for concurrent use, confirmed by the race detector rather than by reading the fields"
    test: "Drive one clientRateLimiter from N goroutines with distinct keys while prune() and evictOldestIfFull() fire concurrently, then run under `go test -race`"
    expected: "No data race report; no lost or double-counted request in the per-window counter"
    why_human: "No test in beamsync/ exercises clientRateLimiter concurrently — rate_limiter_test.go and middleware_test.go contain no goroutines, WaitGroup, or sync primitives. The race detector can only prove the absence of a race on code it actually runs, so `go test -race ./...` passing does not confirm this backstop. The moved fields are byte-identical to the pre-phase baseline (pure file move), so this is a pre-existing coverage gap, not a phase-introduced regression."
  - truth: "04-08 — a download of a missing file still receives the 404 File not found response"
    test: "GET /download/<nonexistent-path> with a valid client-bound token; assert status 404 and body contains 'File not found'"
    expected: "HTTP 404 with the exact pre-phase body; the transfer_stats defer still fires"
    why_human: "No test asserts the 404 branch — grep for `File not found`, `StatusNotFound`, and `404` across beamsync/*_test.go and desktop/*_test.go returns nothing. The plan declared this truth inspection-verified. The code is present at beamsync/download_handler.go:30-33 and its body is byte-identical to the pre-phase baseline, so the risk is a missing test, not a changed behavior."
human_verification:
  - test: "Run the concurrent-limiter check: construct a clientRateLimiter, spawn ~16 goroutines hammering allow() with a mix of shared and distinct keys, and call prune()/evictOldestIfFull() from another goroutine; run with -race."
    expected: "No race detector output; no panic from concurrent map writes; final per-window counter consistent with the number of allowed requests."
    why_human: "No existing test exercises clientRateLimiter concurrently, so the phase's backstop truth cannot be evidenced by the suite. Code is a pure move and unchanged from baseline, so this is coverage, not correctness."
  - test: "GET /download/<path-that-does-not-exist> against a running StartServerE instance with a valid token; also confirm the missing-file path still emits transfer_stats."
    expected: "404 with body 'File not found' (prefix match; http.Error appends a newline), and one transfer_stats event observed by the callback."
    why_human: "No test pins the 404 branch. The handler body is byte-identical to the pre-phase baseline, so behavior is preserved by diff-identity rather than by execution."
---

# Phase 4: Go Structural Refactor — `server.go` Extraction — Verification Report

**Phase Goal:** Split `beamsync/server.go` (1538 lines) into focused files within
`package beamsync`. **No behavior changes.** Extract in risk-ascending order, verifying the full
suite after every step.

**Verified:** 2026-11-18T22:54:12Z (local date 2026-11-19)
**Status:** human_needed — goal achieved; 2 of 25 truths rest on diff-identity rather than a
behavioral test (see Human Verification).
**Re-verification:** No — initial verification (no prior `04-VERIFICATION.md` existed).
**Scope verified:** HEAD `110d6f6` on `dev`, clean working tree.
**Baseline:** `65c6a2d` — the parent of the first refactor commit `1f2c7f0`.

## Verification Method & Baseline Correction

The dispatch named `d0281c7` as the pre-phase baseline. That commit is **stale**: it carries a
1448-line `beamsync/server.go`, which is the size *after* the first extraction, not before it.
The true pre-phase baseline is `65c6a2d`, whose `beamsync/server.go` is exactly **1538 lines** —
the figure ROADMAP Phase 4 names as its input. All diffs, call-count parities, and
byte-comparisons below were computed against `65c6a2d` (materialised read-only at
`/tmp/pre/beamsync/` via `git archive`). Using `d0281c7` would have silently hidden the entire
`events.go` extraction from the audit.

Two independent evidence tracks were used:

1. **Audit tools** — `query verify.artifacts`, `query verify.key-links`, `query
   verify.phase-completeness`, `roadmap.validate` against all nine plans.
2. **Independent re-derivation** — declaration sweeps, signature diffs, per-commit diff
   analysis, and call-count parity computed directly from git and the working tree. Every
   claim in the nine SUMMARY.md files was treated as unproven until reproduced here.

## Goal Achievement

### Observable Truths

25 truths: 4 derived from the ROADMAP goal text (there is no explicit Success Criteria list in
the Phase 4 section) plus 22 from the nine plans' `must_haves.truths`, minus one dedupe
(04-09's "no extracted declarations remain" *is* the roadmap's "split happened" claim).

| # | Truth | Source | Status | Evidence |
|---|-------|--------|--------|----------|
| 1 | `server.go` is split: every subsystem symbol the phase set out to extract is declared in a focused file, not in `server.go` | ROADMAP (dedupes 04-09 #2) | ✓ VERIFIED | A 29-pattern declaration sweep over `server.go` returns **0 matches**. Remaining declarations are exactly the roadmap's retained surface: `PendingTransfer`:18, `uiFS`:30, `serverState`:32 + 5 methods, `HTTPServer`:83 + 9 methods (the 2 extracted handler methods live in their own files), `StartServerE`:212, `StartSenderE`:479. Package-wide each extracted symbol has exactly one declaration, in its expected file. |
| 2 | All new files are `package beamsync`; no new sub-packages; the public Go API is unchanged | ROADMAP | ✓ VERIFIED | All 8 new production files + 2 new test files declare `package beamsync`. No new directory under `beamsync/`. Exported surface: 31 declarations, `diff` of pre vs post identical. `desktop/` had zero file changes during the phase and still builds (`go build .` OK, `go vet .` OK, `go test -race .` OK). |
| 3 | No behavior change — extracted bodies are byte-identical modulo package/receiver scaffolding; the two non-pure deltas are semantically inert | ROADMAP | ✓ VERIFIED | For waves 1–6, each commit's `server.go` diff is **deletion-only** and the deleted line multiset is identical to the added block (modulo `package`/`import`). Two declared deltas analysed below and found inert. All 80 pre-existing func/method signatures identical pre→head. |
| 4 | Extraction proceeded in risk-ascending order with the full suite run after every step | ROADMAP | ✓ VERIFIED | `server.go` line counts per commit: 1538 → 1448 → 1356 → 1244 → 1206 → 1171 → 1036 → 995 → 1007 → 827 → 698. Waves 1–6 (events, rate limiter, progress, io helpers, middleware, helpers) are pure moves; 7 (upload) and 8 (download) are closure→method conversions; 9 is the trim. Each SUMMARY records a green build/vet/race gate. |
| 5 | 04-01 — the dispatcher's `close()` cannot race with `emit()`; the `stateMu` guard travels with the type | 04-01 (backstop) | ✓ VERIFIED | `TestEventDispatcherCloseRacesEmitSafely` exists and passes under `-race`. `eventDispatcher.stateMu` and `close()` are present in `events.go` (stateMu :29, close :91–99), moved verbatim. |
| 6 | 04-02 — rate limiter window boundary unchanged: allow starts a fresh window when `now.Sub(state.windowStart) >= l.window` | 04-02 | ✓ VERIFIED | `allow` body byte-identical to baseline (only receiver/`package` lines differ). `TestClientRateLimiterResetsAfterWindow` and the boundary cases in `rate_limiter_test.go` pass. |
| 7 | 04-02 — an empty client key gets the same decision as before | 04-02 | ✓ VERIFIED | Map lookup path unchanged; `rate_limiter_test.go` covers the empty-key case and passes. |
| 8 | 04-02 — eviction of the oldest client is unchanged | 04-02 | ✓ VERIFIED | `prune`/`evictOldestIfFull` moved verbatim; `TestClientRateLimiterEvictsOldestClientWhenFull` passes. |
| 9 | 04-02 — the limiter's map and counter are safe for concurrent use, **confirmed by the race detector** | 04-02 (backstop) | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | No test exercises `clientRateLimiter` concurrently: `rate_limiter_test.go` and `middleware_test.go` contain no `go func`, `WaitGroup`, or `sync.` usage. `-race` cannot prove a race absent from code it never runs concurrently. Code is a byte-identical pure move (pre-existing gap). |
| 10 | 04-03 — progress throttle boundary unchanged (minInterval 0 emits every write; large minInterval coalesces) | 04-03 | ✓ VERIFIED | `Write` body byte-identical; `TestProgressWriterCoalescesWritesWithinInterval` / `TestProgressWriterEmitsEveryWriteWithoutThrottle` pass. |
| 11 | 04-03 — a zero-byte write still emits no progress event | 04-03 | ✓ VERIFIED | Zero-byte guard present at `progress.go:29–33`; `TestProgressWriterZeroByteWriteEmitsNothing` passes. |
| 12 | 04-03 — payload is still `filename\|written\|total`, event name carried by the separate `eventName` field | 04-03 | ✓ VERIFIED | `pw.emit(pw.eventName, fmt.Sprintf("%s\|%d\|%d", ...))` moved, not rewritten. `TestProgressWriterPayloadFormat` passes. |
| 13 | 04-03 — `written` and `total` remain `int64` byte counts | 04-03 (backstop) | ✓ VERIFIED | Struct fields declared `int64` in `progress.go`; `TestProgressWriterCountsBytesActuallyWritten` passes and asserts byte-count semantics. |
| 14 | 04-04 — `copyChunked` copies zero bytes and reports zero | 04-04 | ✓ VERIFIED | `TestCopyChunkedCopiesDataAndReportsCount` (moved to `io_helpers_test.go`) passes. |
| 15 | 04-04 — `copyChunked` handles a source shorter than one chunk and exactly one chunk identically | 04-04 | ✓ VERIFIED | Same test's table covers sub-chunk and exact-chunk sizes; passes. |
| 16 | 04-05 — a request with no token is still rejected with 403 | 04-05 | ✓ VERIFIED | Empty-token path in `extractToken`/`tokenMiddlewareAny` byte-identical; `TestTokenMiddlewareRejectsMissingOrInvalidToken` passes. |
| 17 | 04-05 — middleware ORDER unchanged: rate limiting still wraps token validation | 04-05 | ✓ VERIFIED | All 11 route registrations compared pre vs post: identical nesting (`rateLimitMiddleware(..., tokenMiddlewareAny(...))`), identical limiters, identical token scopes and `consume` flags. Call-site counts: `rateLimitMiddleware` 11→11, `tokenMiddlewareAny` 7→7. |
| 18 | 04-06 — the package boundary is unchanged; no new sub-packages | 04-06 | ✓ VERIFIED | All files `package beamsync`; `find beamsync -type d` unchanged. |
| 19 | 04-06 — `generateID` still returns a hexadecimal identifier of the same length and shape | 04-06 | ✓ VERIFIED | Body moved verbatim; `TestGenerateIDReturnsHexValue` and `TestGenerateIDDoesNotCollideInSmallSample` pass. |
| 20 | 04-06 — `autoRenamePath` still returns a non-colliding path for an existing file | 04-06 | ✓ VERIFIED | Body moved verbatim; `TestAutoRenamePathFindsNonCollidingName` passes. |
| 21 | 04-07 — a non-multipart upload still receives 400 `Expected multipart/form-data` | 04-07 | ✓ VERIFIED | `TestUploadWithoutFileReturnsBadRequest/non-multipart_content_type` passes. |
| 22 | 04-07 — a valid multipart upload still lands in the upload directory via `autoRenamePath` and emits `upload_progress` | 04-07 | ✓ VERIFIED | `TestUploadWithFileSavesToDiskAndEmitsEvents` passes; `handleUpload` calls `autoRenamePath` and `s.emit("upload_progress", ...)`. |
| 23 | 04-08 — a download of a missing file still receives 404 `File not found` (inspection-verified; no test pins it) | 04-08 | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | Code present at `download_handler.go:30–33`, byte-identical to baseline, but grep for `File not found`/`StatusNotFound`/`404` across all `*_test.go` returns nothing. Plan itself declared this inspection-only. |
| 24 | 04-08 — a completed download still emits the `download_progress` completion event and `transfer_stats` | 04-08 | ✓ VERIFIED | `TestSenderPathEmitsDownloadProgress`, `TestMultiFileSenderPathEmitsDownloadProgress`, `...CompletionProgress` all pass. |
| 25 | 04-09 — `server.go` retains `HTTPServer`, `serverState`, `StartServerE`, `StartSenderE`, the route registration, and the `//go:embed` directive | 04-09 | ✓ VERIFIED | Declarations at :18, :29–30, :32, :83, :212, :479; 11 route registrations present. See "Preserved Invariants → D-06" below for the exact line map. |

**Score:** 23/25 truths verified (2 present, behavior-unverified)
**Overrides applied:** 0 — no `overrides:` block existed and none was needed.

### Roadmap Success Criteria

The Phase 4 ROADMAP section states its contract in the goal sentence and the 4.1–4.9 plan
outlines rather than as a numbered SC list. The four derivable criteria are rows 1–4 above;
all four are ✓ VERIFIED. STR-03 and STR-10 are explicitly excluded by ROADMAP.

## Required Artifacts

All ten artifacts pass exists → substantive → wired. No artifact is a stub: every file contains
real declarations with call sites, and no file is a forwarding shell.

| Artifact | Expected | Lines | Status | Wiring evidence |
|----------|----------|-------|--------|-----------------|
| `beamsync/events.go` | dispatcher type, singleton, background producers | 191 | ✓ VERIFIED | `defaultEventDispatcher` 1 decl (:33) + 1 use (:175); `startWatchdog`/`startBootstrapRotator` called from both constructors (2 call sites each) |
| `beamsync/rate_limiter.go` | `clientRateLimiter` + methods | 118 | ✓ VERIFIED | 11 rate-limited route registrations, all constructed through `rateLimitMiddleware` |
| `beamsync/progress.go` | `progressWriter` + `Write` | 45 | ✓ VERIFIED | Constructed in both send/receive paths; `eventName` field (:20), nil-guarded clock (:36–39), zero-byte guard (:29–33) |
| `beamsync/io_helpers.go` | `chunkBufferPool`, `copyChunked` | 41 | ✓ VERIFIED | `copyChunked` 2 call sites (upload + unified download) |
| `beamsync/middleware.go` | 7 middleware funcs | 144 | ✓ VERIFIED | `rateLimitMiddleware(..., settings func() *TransferSettings, ...)` at :75; auth call parity below |
| `beamsync/helpers.go` | `generateID`, `sha256Hex`, `autoRenamePath`, `getLocalIPForURL`, `uploadIntegrityHeader` | 50 | ✓ VERIFIED | `generateID` 1→1, `autoRenamePath` 1→1, `getLocalIPForURL` 1→1 |
| `beamsync/upload_handler.go` | `POST /upload` as `*HTTPServer` method | 196 | ✓ VERIFIED | `handleUpload` declared once; route delegates at `server.go:427` |
| `beamsync/download_handler.go` | unified `handleDownload` | 83 | ✓ VERIFIED | Declared once at :17; both routes delegate (`server.go:643`, `:650`) |
| `beamsync/io_helpers_test.go` | moved `copyChunked` test | 70 | ✓ VERIFIED | Contains `TestCopyChunkedCopiesDataAndReportsCount`; `server_test.go` no longer does |
| `beamsync/middleware_test.go` | rate-limit middleware tests | 208 | ✓ VERIFIED | Contains `TestRateLimitMiddlewareReadsCurrentSettings` + `TestClientRateLimiter*` |

Production lines: `beamsync/*.go` **2356 → 2384** (+28). That delta is exactly the added
`package` + `import` scaffolding across eight new files; **zero logic lines were added**.

## Key Link Verification

All 12 plan-declared key links wired (`query verify.key-links` passes for all nine plans),
each re-checked by hand.

| From | To | Via | Status | Evidence |
|------|----|-----|--------|----------|
| `server.go` | `events.go` | `startWatchdog` from both constructors | ✓ WIRED | 2 call sites, both present |
| `rate_limiter.go` | `middleware.go` | `rateLimitMiddleware` consuming `clientRateLimiter.allow` | ✓ WIRED | `allow` call inside `rateLimitMiddleware` |
| `server.go` | `progress.go` | `progressWriter` construct sites | ✓ WIRED | receive path (StartServerE) + each send path (StartSenderE) |
| `io_helpers_test.go` | `io_helpers.go` | `TestCopyChunked...` exercises `copyChunked` | ✓ WIRED | test present and passing |
| `middleware.go` | `rate_limiter.go` | `rateLimitMiddleware` → `allow` | ✓ WIRED | same |
| `upload_handler.go` | `helpers.go` | `autoRenamePath` | ✓ WIRED | call present in upload body |
| `server.go` | `helpers.go` | `generateID`, `getLocalIPForURL` | ✓ WIRED | 1 call site each, in `StartServerE` |
| `server.go` | `upload_handler.go` | route → `httpServer.handleUpload` | ✓ WIRED | `server.go:427` |
| `upload_handler.go` | `io_helpers.go` | `copyChunked` | ✓ WIRED | call present |
| `server.go` | `download_handler.go` | route → `httpServer.handleDownload` | ✓ WIRED | `server.go:643` (`filePaths[0]`), `:650` (`filePath`) |
| `download_handler.go` | `progress.go` | `progressWriter` with `eventName: "download_progress"` | ✓ WIRED | constructor + event name present |
| `download_handler.go` | `io_helpers.go` | `copyChunked` | ✓ WIRED | call present |

### Authentication call-site integrity (package-wide, non-test)

Moved middleware must not lose a call site — deleting one compiles cleanly and silently
disables auth. Counts pre→post:

`setCORSHeaders` 5→5 · `extractToken` 4→4 · `clientIP` 2→2 · `tokenMiddlewareAny` 7→7 ·
`rateLimitMiddleware` 11→11 · `safeEmit` 2→2 · `startWatchdog` 2→2 · `generateID` 1→1 ·
`autoRenamePath` 1→1 · `getLocalIPForURL` 1→1.

Only two call counts drop, both fully explained by the deliberate D-12 unification of the two
identical download bodies into one method: `copyChunked` 3→2 and `logTransfer` 4→3. Runtime
executions are unchanged — each route still executes the body exactly once.

## Data-Flow Trace (Level 4)

No component renders dynamic data in this phase, but the extracted handlers must still reach
real state rather than a static literal. Traced each newly extracted path:

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `upload_handler.go` | uploaded file | `r.MultipartReader()` → `copyChunked` → `os.Create(realPath)` | Yes | ✓ FLOWING |
| `upload_handler.go` | progress events | `progressWriter{emit: s.emit}` ← `HTTPServer.emit` field set in constructor | Yes | ✓ FLOWING |
| `download_handler.go` | file bytes | `os.Open(filePath)` → `file.Stat()` → `copyChunked` | Yes | ✓ FLOWING |
| `download_handler.go` | transfer stats | `s.stats.snapshotDownloads(...)` via deferred `transfer_stats` emit | Yes | ✓ FLOWING |
| `rate_limiter.go` | decisions | `map[string]*rateLimitState` mutated per request | Yes | ✓ FLOWING |
| `progress.go` | byte counters | `written`, `total` from `io.Writer` return value | Yes | ✓ FLOWING |
| `events.go` | event delivery | buffered channel drained by a goroutine started in `newEventDispatcher` | Yes | ✓ FLOWING |

No `return null`/`return []`/hardcoded-empty substitution found in any extracted path.

## Behavioral Spot-Checks

Full gate re-run by the verifier at HEAD `110d6f6`, after all evidence gathering:

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| beamsync builds, vets, passes race suite | `cd beamsync && go build ./... && go vet ./... && go test -race -count=1 ./...` | `BUILD_OK`, `VET_OK`, `ok beamsync 14.976s` | ✓ PASS |
| desktop builds, vets, passes race suite | `cd desktop && go build . && go vet . && go test -race -count=1 .` | `BUILD_OK`, `VET_OK`, `ok desktop 1.190s` | ✓ PASS |
| No unformatted Go | `gofmt -l beamsync/ desktop/` | no output | ✓ PASS |
| Dispatcher close/emit race guard | `go test -race -run TestEventDispatcherCloseRacesEmitSafely ./...` | pass | ✓ PASS |
| Chunked copy semantics | `-run TestCopyChunkedCopiesDataAndReportsCount` | pass | ✓ PASS |
| Middleware reads current settings snapshot | `-run TestRateLimitMiddlewareReadsCurrentSettings` | pass | ✓ PASS |
| Sender path emits download progress + completion | `-run 'TestSenderPathEmitsDownloadProgress\|TestMultiFileSenderPathEmitsDownloadProgress\|TestMultiFileSenderPathEmitsCompletionProgress'` | pass | ✓ PASS |
| Progress zero-byte guard + payload format + byte counts | `-run 'TestProgressWriter'` | pass | ✓ PASS |
| Client-bound single-use download token | `-run TestStartSenderIssuesClientBoundSingleUseDownloadToken` | pass | ✓ PASS |
| Upload 400 branches + happy path | `-run 'TestUploadWithoutFileReturnsBadRequest\|TestUploadWithFileSavesToDiskAndEmitsEvents'` | pass | ✓ PASS |
| Token rejection | `-run TestTokenMiddlewareRejectsMissingOrInvalidToken` | pass | ✓ PASS |
| Helper equivalences | `-run 'TestAutoRenamePathFindsNonCollidingName\|TestGenerateIDReturnsHexValue'` | pass | ✓ PASS |

Test inventory: **84** test/bench functions in `beamsync` (identical name set pre vs post),
**32** in `desktop`, **116** distinct. No test function was renamed; `TestCopyChunked...` moved
file only. `server_test.go` held **14** top-level tests before and **13** after — the dispatch's
"13 before and after" was off by one; the moved test is accounted for in
`io_helpers_test.go`. All 31 test names referenced by Phase 3's 33-mutant table still exist.

### Probe Execution

| Probe | Command | Result | Status |
|-------|---------|--------|--------|
| — | `find scripts -path '*/tests/probe-*.sh'` | no `scripts/` directory exists in this repo | SKIPPED |

No probes are declared in any Phase 4 plan or SUMMARY, and the project has no probe harness.
Step 7c is not applicable to this phase.

## Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| STR-01 | 04-02 | Extract rate limiter into `rate_limiter.go` | ✓ SATISFIED | 118-line file, one declaration set, 11 rate-limited registrations |
| STR-02 | 04-03 | Extract progress tracking types into `progress.go` | ✓ SATISFIED | 45-line file; Phase 3 `eventName`/`now`/guards intact |
| STR-04 | 04-04 | Extract I/O helpers into `io_helpers.go` | ✓ SATISFIED | `chunkBufferPool` single declaration; 2 call sites |
| STR-05 | 04-05 | Extract HTTP middleware into `middleware.go` | ✓ SATISFIED | 144-line file; auth call counts unchanged |
| STR-06 | 04-01 | Extract event dispatcher into `events.go` | ✓ SATISFIED | 191-line file; singleton moved with its type |
| STR-07 | 04-07 | Extract upload handler into `upload_handler.go` | ✓ SATISFIED | `handleUpload` method; route delegates |
| STR-08 | 04-08 | Extract download handler into `download_handler.go` | ✓ SATISFIED | Single `handleDownload`; both routes delegate |
| STR-09 | 04-06, 04-09 | Trim `server.go` to orchestration only | ✓ SATISFIED (with INFO note) | 1538 → 698 lines; only orchestration + retained state remains. See INFO-1 for why 698, not ~200. |
| STR-03 | — | Merge duplicate progress writers | EXCLUDED | Delivered in an earlier phase per ROADMAP |
| STR-10 | — | `shareURL()` helper | EXCLUDED | Delivered in an earlier phase per ROADMAP |

No orphaned requirements: `grep "Phase 4" .planning/REQUIREMENTS.md` maps only the eight
in-scope STR IDs, and every one is claimed by at least one plan's `requirements` field.
`query verify.phase-completeness 04` → complete; `roadmap.validate` → 0 warnings.

## Declaration, Signature & Order Audit

This is the strongest evidence in the report — it is what makes "no behavior change" more than
an assertion.

**Declaration sweep.** 29 extracted-symbol patterns searched in `server.go`: **0 declarations**,
and package-wide every symbol has exactly one declaration in the expected file. Only `_` blank
vars duplicate at package scope. (The 04-09 commit body describes its own 30-pattern sweep;
both return zero.)

**Signature preservation.** All **80** pre-existing func/method signatures are identical pre→head.
The only additions are `func (s *HTTPServer) handleUpload(w, r)` and
`func (s *HTTPServer) handleDownload(filePath string, w, r)`. `StartServerE` and `StartSenderE`
signatures unchanged, so `desktop/app.go` is untouched and still compiles.

**Additions to `server.go` across the whole phase: 15 lines.**
2 struct fields (`emit`, `uploadDir`) + 7 comment lines documenting them, 3 constructor assignments,
3 delegation lines (2 routes + the `upload` route). Nothing else was added — no logic, no
reordering.

**Wave order, risk-ascending — confirmed from per-commit diffs:**

| Wave | Commit | File | `server.go` diff character |
|------|--------|------|----------------------------|
| 1 | `1f2c7f0`, `50a6485` | `events.go` | pure move, deletion-only |
| 2 | `24cab26` | `rate_limiter.go` | pure move, deletion-only |
| 3 | `f356a77` | `progress.go` | pure move, deletion-only |
| 4 | `28370c3` | `io_helpers.go` | pure move, deletion-only |
| 5 | `5777d98` | `middleware.go` | pure move, deletion-only |
| 6 | `ae0429f` | `helpers.go` | pure move, deletion-only |
| 7 | `2212b23`, `ad8beaa` | `upload_handler.go` | closure → method (fields added first) |
| 8 | `e1d784b`, `99be273` | `download_handler.go` | handler added, then routes swapped |
| 9 | `76828ca` | — | assertion-only, `--allow-empty` by design |

For waves 1–6 the deleted lines' multiset is identical to the added block's (modulo `package`
and `import`), so all six are provably pure moves.

**One ordering delta, analysed:** in `1f2c7f0`, `var defaultEventDispatcher =
newEventDispatcher(...)` changed position *within* `events.go` (now line 33, before the
function declarations). This is safe: no other package-level var initialiser depends on it, and
`newEventDispatcher`'s goroutine touches no globals. Go's cross-file init order changed
(`server.go` → `events.go`) with no observable effect — nothing that runs at init reads or
writes the dispatcher.

## The Two Non-Pure Deltas (declared by the planner)

Both were verified inert, not merely accepted.

**(a) Unified `handleDownload`.** The single-file body used the outer closure variable
`filename := filepath.Base(filePath)`, while the multi-file body used `realName`. The unified
method computes `realName := filepath.Base(filePath)` per request from the same parameter. This
is a pure function of the argument, so the value is identical by construction; a byte-diff of
the single-file body against `handleDownload` shows only this addition, the receiver
qualification, and the closing-brace collapse (`})))` → `}`). No route lost or gained a header,
an emit, or an error branch.

**(b) Expanded completion comment.** `// Completion event — see the single-file send path for
why.` became a three-line comment. Comment text only; the two pre-phase bodies were otherwise
byte-identical, so the terminal-emit invariant is unchanged.

## Preserved Invariants

**D-06 — `//go:embed` stays in `server.go`**

| Symbol | Line | Notes |
|--------|------|-------|
| `PendingTransfer` | :18 | referenced only in `server.go` |
| `//go:embed ui/*.html ui/*.png` | :29 | directive, in place |
| `var uiFS embed.FS` | :30 | 5 hits total: declaration + reads at :291, :317, :608, :633 — all in `server.go`, zero cross-file references |
| `serverState` | :32 | methods :39–:73 |
| `HTTPServer` | :83 | struct + methods :115–:210 |
| `StartServerE` | :212 | unchanged signature |
| `StartSenderE` | :479 | unchanged signature |

**D-02 — global dispatcher stays global.** `defaultEventDispatcher`: 1 declaration (:33) + 1
usage (:175), both in `events.go`. No per-server dispatcher was introduced (that is a deliberate
later-phase deferral, not a miss).

**D-12 — one `handleDownload`.** Single declaration at `download_handler.go:17`; both routes
delegate (`server.go:643`, `:650`). No duplicate body remains.

**D-05 — Phase 3 concurrency work intact.** `eventDispatcher.stateMu` + `close()`
(`events.go:29`, :91–99); `progressWriter.eventName` + nil-guarded `now` + zero-byte guard
(`progress.go:20`, :36–39, :29–33); `HTTPServer.settingsMu` / `Settings()` snapshot /
`UpdateSettings` atomic swap / `currentSettings()` (`server.go:88–155`); `rateLimitMiddleware`
still takes a settings **provider** (`middleware.go:75`).

**No post-construction mutation** of `HTTPServer.emit` or `.uploadDir`: both are set only in
struct literals (`server.go:256–257`, `:508`), so the fields are effectively immutable after
construction.

## Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| — | — | — | — | **None.** |

- No `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` markers in any phase-touched file, so the
  debt-marker gate passes with no blocker.
- No empty implementations, no `return nil`/`return []` stubs, no hardcoded-empty props, no
  console-log-only bodies (Go project).
- Pre-existing `ponytail:` and `//nolint` annotations were moved verbatim with their code; none
  were added or removed.
- No stubs: every extracted file has at least one real declaration and at least one wired
  consumer (see Required Artifacts / Key Links).
- `golangci-lint` was **not** run: the locally installed v1.64.8 cannot decode Go 1.27.4 export
  data (toolchain mismatch). This is an environmental limitation, not a phase result; CI owns
  that check.

## Notes (INFO — no action required)

**INFO-1 — `server.go` is 698 lines, not ~200.** The roadmap's 4.9 defines the retained surface
as the struct, `serverState`, `StartServerE`, `StartSenderE`, the route registration and the
embed directive. Those are what remain. The bulk is `StartServerE` + `StartSenderE` (**487
lines**) which still contain **10 inline route closures** — 5 per constructor: heartbeat, `/`,
`/logo.png`, `/stats` and `/request-transfer` in `StartServerE`; heartbeat, `/`, `/logo.png`,
`/download` and `/download/N` in `StartSenderE`. Those closures were never in the phase's
extraction set (D-01 re-derived the set from the file; these were not named), so their presence
is within the ROADMAP definition of "route registration". Stated plainly so a reviewer expecting
a ~200-line file is not surprised: the -840-line reduction is real, but the two constructors
remain the dominant block.

**INFO-2 — 04-09's commit is deliberately empty.** `76828ca` is `--allow-empty` and says so in
its body: each of the eight extraction plans had already deleted its own declarations and
pruned its own orphaned imports, so by wave 9 there was nothing left to trim. Plan 04-09
delivered the 30-pattern assertion + retained-surface line map instead of a cosmetic edit that
would have dirtied the diff. Recorded as INFO, not a gap: the truth it asserts ("no extracted
declarations remain") is independently VERIFIED by the sweep in this report.

**INFO-3 — baseline correction.** See "Verification Method" above: `d0281c7` is post-wave-1
(1448 lines) and would have masked the `events.go` extraction. `65c6a2d` is the correct
baseline.

**INFO-4 — dispatch test-count claim corrected.** The dispatch stated `server_test.go` had "13
tests before and after"; the measured counts are 14 before and 13 after, the difference being
`TestCopyChunkedCopiesDataAndReportsCount`, which moved to `io_helpers_test.go` rather than
being deleted or renamed. No coverage was lost.

## Human Verification Required

### 1. Concurrent `clientRateLimiter` safety under `-race`

**Test:** Construct a `clientRateLimiter`, run ~16 goroutines calling `allow()` with a mix of
shared and distinct keys, and call `prune()` / `evictOldestIfFull()` from another goroutine.
Run with `-race`.
**Expected:** No race-detector output, no concurrent-map panic, and a per-window counter
consistent with the number of allowed requests.
**Why human:** No test in the repo exercises the limiter concurrently — `-race` cannot prove the
absence of a race in code it never runs in parallel. The code is a byte-identical pure move, so
this is a coverage gap inherited from before the phase.

### 2. Missing-file download returns 404 `File not found`

**Test:** Against a running `StartServerE` instance with a valid client-bound token, `GET
/download/<path-that-does-not-exist>`; observe the callback for `transfer_stats`.
**Expected:** HTTP 404 with body `File not found` (plus the newline `http.Error` appends), and
one `transfer_stats` event from the deferred emit.
**Why human:** No test asserts this branch. The handler body is byte-identical to the pre-phase
baseline, so behavior is preserved by diff-identity rather than by execution — acceptable, but
only a human (or a new test) can close it.

Both items are **pre-existing coverage gaps, not phase-introduced regressions**. If the reviewer
accepts diff-identity as sufficient proof for a pure move, both can be closed as accepted risk
and this phase's status becomes `passed`; no code change is required either way.

## Gaps Summary

**No gaps. The phase goal is achieved.**

`beamsync/server.go` went from 1538 to 698 lines (−840, −54.6%) across eight focused files, all
within `package beamsync`, with the public API and all 80 pre-existing signatures byte-identical,
the two non-pure deltas proven inert, all auth call sites intact, and the full build/vet/race
gate green in both modules. The only reason this report is not `passed` is that two of the 25
truths — one planner-declared backstop (concurrent limiter safety) and one inspection-only truth
(missing-file 404) — have no behavioral test, which is a pre-existing coverage gap rather than a
defect introduced here.

---

_Verified: 2026-11-18T22:54:12Z_
_Verifier: gsd-verifier_
