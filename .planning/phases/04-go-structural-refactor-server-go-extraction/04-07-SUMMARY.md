---
phase: 04-go-structural-refactor-server-go-extraction
plan: 07
subsystem: infra
tags: [go, refactor, upload-handler, closure-to-method, multipart, event-callback]

# Dependency graph
requires:
  - phase: 03-tests-before-refactoring
    provides: TestUploadWithoutFileReturnsBadRequest and TestUploadWithFileSavesToDiskAndEmitsEvents as the pins on the /upload path, plus the 33-mutant table whose test names must not change
  - phase: 04-go-structural-refactor-server-go-extraction
    plan: 06
    provides: beamsync/helpers.go (autoRenamePath), beamsync/io_helpers.go (copyChunked), beamsync/progress.go (progressWriter with the upload_progress invariants), and a 995-line server.go whose upload closure is still inline
provides:
  - beamsync/upload_handler.go — func (s *HTTPServer) handleUpload(w, r) as a method on the server
  - beamsync/server.go at 827 lines; /upload route delegates to httpServer.handleUpload with the rate-limit → token-auth chain unchanged; io and mime/multipart pruned
  - HTTPServer gains emit EventCallback (both constructors) and uploadDir string (StartServerE only)
affects: [04-08, 04-09]

# Actuals (#2632) — same chars/4 scale as the plan's estimate
actuals:
  tokens: 910
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Closure-to-method proof by normalized diff (first use): the old closure body was extracted from 4c0b484~2:beamsync/server.go with a Python range finder, the six receiver swaps were applied to the OLD text with sed, and `diff -w -B old_norm new` printed nothing — so every remaining difference is provably a captured-local substitution, not an edit"
    - "Receiver-swap inventory before the move: grep counted 11 s.* accesses in the new file and 0 residual `autoRenamePath(uploadDir` / bare `emit(` in server.go, which catches a missed capture that a compiler would silently resolve to a package-level symbol"
    - "Behavior-anchor sweep after the move: MaxBytesReader, the multipart 400, upload_progress, file_received, autoRenamePath, copyChunked, Accept-Ranges, markHeartbeat, recover() and transfer_stats were each grepped by name in the new file and confirmed absent from server.go, so 'the body moved' is proven per-feature rather than by line count"

key-files:
  created:
    - beamsync/upload_handler.go
  modified:
    - beamsync/server.go
    - beamsync/server_test.go

key-decisions:
  - "Moved the closure body verbatim with exactly six receiver substitutions (emit→s.emit ×3, state→s.state ×3, httpServer.history→s.history, httpServer.stats→s.stats, uploadDir→s.uploadDir) and no other change; the normalized diff is the evidence, not an assertion (D-07 risk-ascending order: first behavior-adjacent extraction)"
  - "Added `emit EventCallback` to HTTPServer and set it in BOTH constructors, not just StartServerE: StartSenderE registers /download and /download/N, and 04-08's handleDownload reads s.emit, so a sender server with a nil emit would panic on the first transfer_stats call — the field is set two plans before its second consumer exists, which is the only ordering that avoids a mid-phase nil"
  - "Deliberately did NOT set uploadDir in StartSenderE. The sender registers no upload route and s.uploadDir is read only by handleUpload, so an empty field there is correct rather than an omission (Task 7.1's acceptance criteria pin both halves)"
  - "Left the `uploadDir string` parameter of StartServerE in place alongside the new field: the parameter still feeds os.MkdirAll at :221 and the meta/QR payloads, so removing it would have been a signature change outside this plan's mandate (no public API changes)"
  - "Pruned `io` and `mime/multipart` from server.go in the same commit as the move — their last uses were inside the upload body, so Go forces it; `bufio` and `mime` were kept because the download closure still needs bufio.Reader and mime.TypeByExtension until 04-08"
  - "Split TestUploadWithoutFileReturnsBadRequest into two subtests rather than adding a new top-level function: D-13 requires the name to survive, and the existing case sends a valid multipart body so it never reaches the content-type branch — without the `non-multipart content type` subtest that http.Error at upload_handler.go:37 had zero coverage"
  - "Task 7.3 was the only task touching a test file, and it added 48 lines to server_test.go with no other edit — so the Phase 3 mutation table's key set is unchanged"

patterns-established:
  - "For a closure-to-method move, transform the OLD body with the receiver substitutions and diff against the NEW file: it turns 'I swapped the right six things' from a claim into a checkable artifact, and it catches both a missed swap and an accidental extra edit"

requirements-completed: [STR-07]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "The /upload handler body moved out of the StartServerE closure into func (s *HTTPServer) handleUpload(w, r) in beamsync/upload_handler.go, verbatim apart from receiver access"
    requirement: "STR-07"
    verification:
      - kind: other
        ref: "normalized diff: old closure body from 4c0b484~2:beamsync/server.go with the six receiver swaps applied vs beamsync/upload_handler.go => no output (177 lines each)"
        status: pass
      - kind: other
        ref: "grep 'Expected multipart/form-data|upload_progress|file_received' beamsync/server.go => 0 matches; the same three anchors present at upload_handler.go:37, :129, :180"
        status: pass
    human_judgment: false
  - id: D2
    description: "The /upload route delegates to httpServer.handleUpload with the rate-limit-wraps-token-auth chain unchanged"
    requirement: "STR-07"
    verification:
      - kind: other
        ref: "server.go:429 => mux.HandleFunc(\"/upload\", rateLimitMiddleware(httpServer.uploadLimiter, httpServer.currentSettings, tokenMiddlewareAny(tokens, []tokenScope{tokenScopeSession, tokenScopeBootstrap}, false, httpServer.handleUpload)))"
        status: pass
    human_judgment: false
  - id: D3
    description: "HTTPServer carries emit EventCallback and uploadDir string; emit is set in both constructors and uploadDir only in StartServerE — must_have foundation for 04-08"
    requirement: "STR-07"
    verification:
      - kind: other
        ref: "server.go:258 uploadDir: uploadDir; :259 emit: emit (StartServerE); :510 emit: emit (StartSenderE); grep uploadDir in the StartSenderE literal => none"
        status: pass
      - kind: other
        ref: "grep -n 'emit: *emit|uploadDir: *uploadDir' beamsync/server.go => exactly 3 hits (:258, :259, :510); :685 and :756 are inner emit: fields on other struct literals"
        status: pass
    human_judgment: false
  - id: D4
    description: "A non-multipart upload still receives 400 Expected multipart/form-data — must_have truth 1, newly pinned"
    requirement: "STR-07"
    verification:
      - kind: unit
        ref: "beamsync/server_test.go#TestUploadWithoutFileReturnsBadRequest/non-multipart_content_type (POSTs Content-Type: application/json, asserts 400)"
        status: pass
    human_judgment: false
  - id: D5
    description: "The empty-multipart 400 path is still pinned under its original test name — must_have truth 1, preserved half"
    requirement: "STR-07"
    verification:
      - kind: unit
        ref: "beamsync/server_test.go#TestUploadWithoutFileReturnsBadRequest/empty_multipart_body (asserts 400 for a well-formed multipart body with zero parts)"
        status: pass
    human_judgment: false
  - id: D6
    description: "A valid multipart upload still lands in the upload directory via autoRenamePath and still emits upload_progress through the server emit callback — must_have truth 2"
    requirement: "STR-07"
    verification:
      - kind: unit
        ref: "beamsync/server_test.go#TestUploadWithFileSavesToDiskAndEmitsEvents (passes through the method; upload_handler.go:97 autoRenamePath(s.uploadDir, rawName), :129 eventName upload_progress, :139 copyChunked)"
        status: pass
      - kind: other
        ref: "grep -c 's\\.emit|s\\.state|s\\.history|s\\.stats|s\\.uploadDir' beamsync/upload_handler.go => 11 receiver accesses; 0 bare captures left in server.go"
        status: pass
    human_judgment: false
  - id: D7
    description: "The 100 GiB MaxBytesReader cap and the multipart validation moved with the body (T-04-07-01)"
    requirement: "STR-07"
    verification:
      - kind: other
        ref: "upload_handler.go:32 => r.Body = http.MaxBytesReader(w, r.Body, 100*1024*1024*1024); :30 Accept-Ranges; :31 s.state.markHeartbeat(); :20 recover() — all present in the method and absent from server.go"
        status: pass
    human_judgment: false
  - id: D8
    description: "No test function was renamed or removed (CONTEXT D-13), so the Phase 3 33-mutant table still keys on every name"
    requirement: "STR-07"
    verification:
      - kind: other
        ref: "grep -c '^func Test' beamsync/server_test.go => 13, unchanged; TestUploadWithoutFileReturnsBadRequest still at :366 with 1 declaration; its body gained two t.Run subtests only"
        status: pass
    human_judgment: false
  - id: D9
    description: "Full Phase 4 gate green: beamsync build + vet + test -race, desktop build + vet + test -race"
    requirement: "STR-07"
    verification:
      - kind: other
        ref: "(cd beamsync && go build ./... && go vet ./... && go test -race -count=1 ./...) => ok beamsync 2.901s; audio [no test files]"
        status: pass
      - kind: other
        ref: "(cd desktop && go build . && go vet . && go test -race .) => ok desktop 1.201s"
        status: pass
    human_judgment: false
  - id: D10
    description: "Prohibition: the extraction changed no runtime behavior, no event payload and no HTTP response body"
    verification: []
    human_judgment: true
    rationale: "A closure-to-method move is proven by the normalized body diff (177 lines, no differences after the six receiver swaps), the per-feature anchor sweep, and the full -race suite in both modules; a reviewer confirms from the diffstat (196 added in upload_handler.go, 194 removed from server.go, 48 test lines) that no behavior surface was touched"

# Metrics
duration: 3min
completed: 2026-11-18
status: complete
---

# Phase 4 Plan 07: `upload_handler.go` Extraction Summary

**The POST `/upload` body — the largest and most behavior-dense closure in the package — now lives in `beamsync/upload_handler.go` as `func (s *HTTPServer) handleUpload(w, r)`, a 177-line verbatim move with exactly six receiver substitutions, leaving `server.go` at 827 lines with a one-line route delegation, `io`/`mime/multipart` pruned, and `HTTPServer` carrying the `emit`/`uploadDir` state that 04-08 will reuse.**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-11-18T18:07:25+05:30
- **Completed:** 2026-11-18T18:10:16+05:30
- **Tasks:** 3 (all committed)
- **Files modified:** 3 (1 created, 2 modified)

## Accomplishments

- Created `beamsync/upload_handler.go` (196 lines): `func (s *HTTPServer) handleUpload(w http.ResponseWriter, r *http.Request)` holding the entire upload body in source order — the `recover()` defer (`:20`), the method check, `Accept-Ranges` (`:30`), `s.state.markHeartbeat()` (`:31`), the 100 GiB `http.MaxBytesReader` cap (`:32`), the multipart content-type 400 (`:37`), the boundary parse, the manifest loop, `autoRenamePath(s.uploadDir, rawName)` (`:97`), the `progressWriter` with `eventName: "upload_progress"` (`:129`), `copyChunked(pw, part, 8*1024*1024)` (`:139`), the flush/close sequence, the transfer log, `s.emit("transfer_stats", …)` (`:168`), the `s.emit("file_received", …)` goroutine (`:180`) and the final 200 (`:194`). Twelve stdlib imports, no file-level doc comment.
- **Proved the body is a move, not a rewrite.** The closure was cut from `4c0b484~2:beamsync/server.go` by a Python range finder (177 lines), the six receiver swaps were applied to the **old** text with `sed`, and `diff -w -B` against the new method printed nothing. Six hunks survived the first pass — `emit: emit` → `emit: s.emit`, three `state.*` → `s.state.*`, two `logTransfer(s.history, emit, …)` → `s.emit` — and normalizing exactly those six produced an empty diff. Every difference is therefore a captured-local substitution, which is what the plan asked for and nothing more.
- Added the two fields the method reads to `HTTPServer`: `emit EventCallback` and `uploadDir string`, both with explanatory doc comments. `StartServerE` initializes both (`server.go:258` `uploadDir: uploadDir`, `:259` `emit: emit`); `StartSenderE` initializes `emit` only (`:510`). The asymmetry is deliberate and pinned by Task 7.1's acceptance criteria — the sender registers no upload route, so `s.uploadDir` is never read there.
- Set `emit` in `StartSenderE` **two plans before its consumer exists**, because 04-08's `handleDownload` reads `s.emit` and `/download` + `/download/N` are registered inside `StartSenderE`. Deferring the assignment to 04-08 would have left a buildable tree whose first `transfer_stats` emit panics at runtime on a nil callback — a failure the compiler cannot catch.
- Rewrote the route as a delegation: `server.go:429` now reads `mux.HandleFunc("/upload", rateLimitMiddleware(httpServer.uploadLimiter, httpServer.currentSettings, tokenMiddlewareAny(tokens, []tokenScope{tokenScopeSession, tokenScopeBootstrap}, false, httpServer.handleUpload)))`. Same middleware order, same token scope set, same `consume` flag — only the inline closure became a bound method value.
- Trimmed `server.go` from 995 to 827 lines (194 removed, 2 added) and pruned `io` + `mime/multipart`, whose last uses were inside the upload body. `bufio` and `mime` were **kept**: the download closure still needs `bufio.Reader` and `mime.TypeByExtension` until 04-08. `server.go` now carries 13 imports.
- Pinned the uncovered 400 branch. `TestUploadWithoutFileReturnsBadRequest` keeps its name (D-13) and its position (`server_test.go:366`), and now has two subtests: `empty multipart body` (the original case — a well-formed multipart body with zero parts, which reaches the *later* `"No files uploaded"` 400 and never touches the content-type check) and `non-multipart content type` (POSTs `Content-Type: application/json`, asserts 400, exercising `upload_handler.go:37` for the first time). Without the second subtest, the branch the plan calls out as must_have truth 1 had no coverage at all.
- Full gate green in both modules: `beamsync` build + vet + `go test -race -count=1 ./...` → `ok beamsync 2.901s`; `desktop` build + vet + `go test -race .` → `ok desktop 1.201s`.

## Task Commits

Each task was committed atomically:

1. **Task 7.1: add the emit callback and uploadDir fields to HTTPServer** — `2212b23` (refactor)
2. **Task 7.2: move the /upload handler into upload_handler.go** — `ad8beaa` (refactor)
3. **Task 7.3: pin the non-multipart 400 branch and run the gate** — `4c0b484` (test)

**Plan metadata:** _(this commit — docs: complete plan)_

## Files Created/Modified

- `beamsync/upload_handler.go` (created, 196 lines) — the `handleUpload` method plus a 12-package stdlib import block (`bufio`, `encoding/json`, `fmt`, `io`, `mime`, `mime/multipart`, `net/http`, `os`, `path/filepath`, `runtime/debug`, `strings`, `time`). This is the first file in the phase whose imports are *wider* than `server.go`'s — it inherited every package the upload body used.
- `beamsync/server.go` (modified, 995 → 827) — the upload closure replaced by `httpServer.handleUpload`; `io` and `mime/multipart` pruned; `HTTPServer` gained `uploadDir` and `emit`; both constructor literals updated. `PendingTransfer`, `//go:embed ui/*` + `uiFS`, `serverState`, the download closure and both `Start*E` signatures are untouched.
- `beamsync/server_test.go` (modified, +48/−~10) — `TestUploadWithoutFileReturnsBadRequest` split into `empty multipart body` and `non-multipart content type`; no other test added, renamed or removed (13 `^func Test` before and after).

## Decisions Made

- **`emit` in both constructors, `uploadDir` in one.** The `emit` field is read by handlers registered in both servers, so a nil there is a latent panic; `uploadDir` is read by exactly one handler registered in exactly one server, so a nil/empty there is correct by construction. Writing the distinction into the struct's doc comments (`server.go:106-109`) makes the asymmetry readable instead of suspicious.
- **Keep the `uploadDir` parameter.** `StartServerE`'s `uploadDir string` still feeds `os.MkdirAll` (`:221`) and the meta/QR payloads, so the new field shadows nothing that can be removed. Deleting the parameter would change the public signature, which the phase charter forbids.
- **Six substitutions, proven.** The receiver swaps were enumerated before the move (11 `s.*` accesses, 0 bare captures left) and then verified after it by diffing the transformed old body against the new file. This is the first phase-4 extraction where a *wrong* swap would compile — `emit` and `state` are also package-level names in some scopes — so the compiler alone was not sufficient evidence.
- **Split the test, do not add one.** D-13 freezes test names, and adding `TestUploadNonMultipartReturnsBadRequest` would have grown the Phase 3 mutation table's key set. Two `t.Run` subtests inside the existing function keep the table valid while still covering the newly-reachable branch.
- **Leave `footer`-level behavior alone.** The `progressWriter` construction, the flush/close ordering and the `file_received` goroutine were carried across without reordering; the plan's prohibition on behavior change outranks any readability improvement inside the moved body.

## Deviations from Plan

**The executor was interrupted before writing this summary.** Tasks 7.1–7.3 all completed and their commits landed (`2212b23`, `ad8beaa`, `4c0b484`), but the session ended between Task 7.3's gate and the summary write. The orchestrator verified every acceptance criterion independently (struct fields, constructor sites, route line, import set, test names, both modules' gates) and then wrote this document. No code was re-executed and no additional commit was needed to make the tree correct — only the tracking update follows.

Two factual notes, recorded without code impact:

- Task 7.1's plan text describes the `StartSenderE` literal as being "at `server.go:1213`". After 04-01 through 04-06 trims it is at `:510`. The executor located it by the literal's assignment rather than by the stale line number, which is what the plan's `read_first` intended.
- `golangci-lint` was not run — the same pre-existing environmental limitation recorded in 04-01 through 04-06 (local v1.64.8 cannot decode Go 1.27.4 export data; it fails on untouched files too). `go build`, `go vet` and `go test -race` remain the authoritative gate.

## Issues Encountered

- The normalized diff needed a **second** pass. The first `sed` normalized only line-initial `emit(` and `state.` occurrences, so six hunks remained where the captures sat inside a struct literal (`emit: emit`), inside a goroutine (`state.beginUpload()`), or as the second argument to `logTransfer`. Extending the substitution to those shapes produced an empty diff. Worth recording because the first pass's five survivors look exactly like "the move introduced a change" — the distinction between a missed normalization and a real edit has to be made deliberately.
- Line-count drift from earlier plans' summaries: `server.go` was predicted at 1036 after 04-06 in 04-06's own "Next Phase Readiness" but is 995, and the upload closure was predicted at `:963-1141` but in fact started later. Both are cosmetic — every anchor was re-derived from the live file before the move, which is the discipline 04-04 through 04-06 each recorded after a hand-guessed range failed.

## User Setup Required

None - no external service configuration required.

## Threat Flags

None — no new network surface was added. The one trust boundary the plan names (`client request -> handleUpload -> uploadDir`) is the same boundary that existed before, crossed by the same code: the 100 GiB `http.MaxBytesReader` cap (`upload_handler.go:32`), the multipart content-type rejection (`:37`) and the boundary parse all moved verbatim. T-04-07-01 (DoS during the method move) is mitigated as planned — the cap and validation are present in the new file and absent from `server.go`, and the full suite runs under `-race` in both modules.

The one *new* failure mode this plan could have introduced is a nil `s.emit` on the sender server; it is closed at `server.go:510` rather than deferred to 04-08, which is why Task 7.1 required both constructors.

## Known Stubs

None.

## Next Phase Readiness

- `beamsync/upload_handler.go` is the upload handler's home, and `HTTPServer` now has the `emit` callback both handler methods need. 04-08 (`download_handler.go`) starts from an 827-line `server.go` and only has to add `handleDownload(filePath, w, r)` and replace the two closures — the struct work is already done.
- `server.go` still imports `bufio` and `mime`, and those two are **exactly** what 04-08 must prune: `bufio` serves the download's `bufio.NewReader` and `mime` its `mime.TypeByExtension`. After that prune the import set is the final 11 — `context embed encoding/json fmt net/http os path/filepath runtime/debug strings sync time` — which 04-09 asserts.
- **04-08's deletion hazard, unresolved and now closer:** the download closure still contains `filePath := filePaths[0]` and `filename := filepath.Base(filePath)` before the copy loop. Once the closure is replaced by a method taking `filePath` as a parameter, those two lines become unused variables and the build fails until they are deleted. 04-09 does not touch them, so 04-08 is the only place they can be removed.
- `PendingTransfer`, `serverState`, `//go:embed ui/*` and `uiFS` remain in `server.go` (D-06) and must stay there through 04-09; a package-wide `rg uiFS` must keep reporting exactly 5 hits, all in `server.go`.

---

*Phase: 04-go-structural-refactor-server-go-extraction*
*Completed: 2026-11-18*

## Self-Check: PASSED

- `beamsync/upload_handler.go` exists (196 lines) and declares `func (s *HTTPServer) handleUpload(w http.ResponseWriter, r *http.Request)` at line 18.
- `server.go:429` delegates to `httpServer.handleUpload` with `rateLimitMiddleware` wrapping `tokenMiddlewareAny` unchanged.
- `HTTPServer` declares `uploadDir string` (`:109`) and `emit EventCallback`; `emit: emit` appears at `:259` and `:510`, `uploadDir: uploadDir` at `:258`.
- Normalized diff of the old closure body (with the six receiver swaps applied) vs `upload_handler.go` → no output.
- `grep 'Expected multipart/form-data|upload_progress|file_received' beamsync/server.go` → 0 matches.
- `TestUploadWithoutFileReturnsBadRequest` still at `server_test.go:366` with two subtests; `go test` tree has 13 top-level tests, unchanged.
- Full gate green: `beamsync` build/vet/test-race `ok 2.901s`, `desktop` build/vet/test-race `ok 1.201s`.
- Commits `2212b23`, `ad8beaa`, `4c0b484` are in the log and touch only `server.go`, `upload_handler.go` and `server_test.go`.
