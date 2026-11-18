---
phase: 04-go-structural-refactor-server-go-extraction
plan: 08
subsystem: infra
tags: [go, refactor, download-handler, closure-to-method, de-duplication, http-streaming]

# Dependency graph
requires:
  - phase: 03-tests-before-refactoring
    provides: the 33-mutant table whose test names must not change, and TestMultiFileSenderPathEmitsCompletionProgress as the pin on the download terminal emit
  - phase: 04-go-structural-refactor-server-go-extraction
    plan: 07
    provides: beamsync/upload_handler.go as the phase's first method-based handler, HTTPServer.emit initialized in BOTH constructors (so s.emit is non-nil on the sender server), and an 827-line server.go still holding both download closures inline
provides:
  - beamsync/download_handler.go — func (s *HTTPServer) handleDownload(filePath string, w, r) as the single implementation behind both /download and /download/N (D-12)
  - beamsync/server.go at 698 lines; both download routes delegate to httpServer.handleDownload with the rate-limit → token-auth chain, token scopes and consume flags unchanged; bufio and mime pruned, leaving the final 11 imports
affects: [04-09]

# Actuals (#2632) — same chars/4 scale as the plan's estimate
actuals:
  tokens: 2199
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Two-way normalized diff (new this plan): the move was proved from BOTH source bodies rather than one — the multi-file body (which already declares its own base name) needed only the six receiver swaps, and the single-file body needed the base-name declaration prepended plus seven `filename` → `realName` identifier rewrites (the `filename=` inside the Content-Disposition format string deliberately preserved) — and each normalized text diffed EMPTY against the one new method. So the de-duplication is proved, not asserted: both call sites provably converge on the same 65-line body"
    - "Residual-audit anchor sweep: the 13 named behavior anchors were counted in one file and the other, and every surviving hit was then read in place. Three anchors legitimately remain in server.go — Cache-Control ×7 from other routes, beginUpload/endUpload at :48/:58 as the serverState method declarations — so 'the body moved' is established per anchor with a reason for each survivor, not by a zero-count alone"
    - "Bodies as data: the old text was cut from `git show HEAD:beamsync/server.go` by line range into /tmp/opencode/gsd0408/, normalized by a checked-in-paste Python script that reports its substitution counts per rule (emit-field=1 state=3 logTransfer=1 emit(=4 comment=1), then the filename×7 set), so 'I swapped the right things' is a reproducible artifact"

key-files:
  created:
    - beamsync/download_handler.go
  modified:
    - beamsync/server.go

key-decisions:
  - "ONE method, not two (CONTEXT D-12). The single-file and multi-file bodies differ only in their base-name local — `filename` derived outside the closure vs `realName` derived inside it — so `handleDownload` declares `realName := filepath.Base(filePath)` as its first statement and both routes pass their file path in. This is the plan's only non-mechanical edit, and it is proved behavior-preserving: previously `filename = filepath.Base(filePaths[0])`, now `realName = filepath.Base(filePath)` where `filePath` is an argument the call sites supply as `filePaths[0]` or the captured `path`"
  - "The unified body is the MULTI-FILE body, because it is the complete one — it already derives its own base name, so it needs no restructuring at all. The single-file body is the one that had to lose its outer dependency"
  - "Kept the FULLER completion comment (3 lines: \"Without it a fast transfer (finished inside the / 500ms throttle window) emits NO progress event at all…\") and dropped the multi-file cross-reference \"// Completion event — see the single-file send path for why.\" — after the merge there is no single-file send path for it to point at, so the reference would dangle"
  - "Added a short doc comment explaining WHY one method serves two routes (the D-12 invariant). upload_handler.go's handleUpload carries no comment, so this is the one intentional shape difference between the two handler files; an unexported symbol earns a comment here only because the merge would otherwise look arbitrary to the next reader"
  - "Deleted `filePath := filePaths[0]` and `filename := filepath.Base(filePath)` from the `len(filePaths) == 1` branch. They were declared OUTSIDE the closure and only the closure body read them; delegation makes them unused locals, which is a Go compile error rather than a warning, so the plan's own build gate forces the deletion"
  - "Kept `idx := i` and `filePath := path` in the multi-file loop even though `go.mod`'s Go 1.25 makes per-iteration loop variables the norm: they are existing code and removing them is an unrelated change (04-PATTERNS warns the same)"
  - "Kept `defer file.Close()` verbatim — no `//nolint` added or removed, so the lint posture is unchanged from the pre-move code"
  - "Pruned `bufio` and `mime` from server.go in the same commit as the rewiring: their last two uses each were inside the moved bodies (`bufio.NewReaderSize` ×2, `mime.TypeByExtension` ×2), so Go forces it and server.go lands on the final 11 imports that 04-09 asserts"

patterns-established:
  - "When two duplicate bodies collapse into one implementation, prove the convergence from BOTH originals: normalizing only the body you move leaves the other call site's equivalence untested, and the divergence would be invisible in the diff you actually looked at"

requirements-completed: [STR-08]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "The two download bodies collapsed into func (s *HTTPServer) handleDownload(filePath, w, r) in beamsync/download_handler.go, behaviorally identical to both originals"
    requirement: "STR-08"
    verification:
      - kind: other
        ref: "normalized diff A: multi-file body from 18465dc:beamsync/server.go:717-779 with six receiver swaps (emit-field=1, state=3, logTransfer=1, emit(=4) => diff -w -B vs the new body => no output (63 -> 65 lines, the +2 being the fuller comment)"
        status: pass
      - kind: other
        ref: "normalized diff B: single-file body from 18465dc:beamsync/server.go:647-710 with `realName := filepath.Base(filePath)` prepended and seven filename→realName rewrites (Content-Disposition `filename=` literal preserved) plus the same six receiver swaps => diff -w -B vs the same new body => no output"
        status: pass
    human_judgment: false
  - id: D2
    description: "The /download route delegates to httpServer.handleDownload(filePaths[0], w, r) and the two outer locals are gone"
    requirement: "STR-08"
    verification:
      - kind: other
        ref: "server.go:641-645 => if len(filePaths) == 1 { mux.HandleFunc(\"/download\", rateLimitMiddleware(httpServer.uploadLimiter, nil, tokenMiddlewareAny(tokens, []tokenScope{tokenScopeTransfer, tokenScopeBootstrap}, true, func(w,r){ httpServer.handleDownload(filePaths[0], w, r) }))) }; `grep -n 'filename :=' beamsync/server.go` => 0 matches"
        status: pass
    human_judgment: false
  - id: D3
    description: "Each /download/%d route keeps its idx and filePath captures and delegates with the captured filePath"
    requirement: "STR-08"
    verification:
      - kind: other
        ref: "server.go:646-652 => idx := i; filePath := path; mux.HandleFunc(fmt.Sprintf(\"/download/%d\", idx), …) calling httpServer.handleDownload(filePath, w, r)"
        status: pass
    human_judgment: false
  - id: D4
    description: "server.go imports pruned to the final 11; download_handler.go imports exactly the 7 the body needs"
    requirement: "STR-08"
    verification:
      - kind: other
        ref: "server.go:3-15 => context embed encoding/json fmt net/http os path/filepath runtime/debug strings sync time (11, no bufio, no mime); download_handler.go:3-11 => bufio fmt mime net/http os path/filepath time (7)"
        status: pass
      - kind: other
        ref: "gofmt -l . => no output; go build ./... && go vet ./... => exit 0"
        status: pass
    human_judgment: false
  - id: D5
    description: "must_have truth 1: a download of a missing file still receives 404 File not found (inspection-verified — no test pins this path)"
    requirement: "STR-08"
    verification:
      - kind: other
        ref: "download_handler.go:30-34 => file, err := os.Open(filePath); if err != nil { http.Error(w, \"File not found\", http.StatusNotFound); return }; also :40-43 the \"Failed to stat file\" 500 branch"
        status: pass
    human_judgment: false
  - id: D6
    description: "must_have truth 2: a completed download still emits the download_progress completion event and the transfer_stats event — the Phase 3 terminal-emit invariant survives the merge"
    requirement: "STR-08"
    verification:
      - kind: unit
        ref: "beamsync/server_test.go#TestMultiFileSenderPathEmitsCompletionProgress => PASS (0.46s, -race -count=1)"
        status: pass
      - kind: other
        ref: "download_handler.go:56 eventName: \"download_progress\"; :59 now: time.Now; :71 s.emit(pw.eventName, …) using pw.eventName NOT a literal; :72 recordSent; :73 transfer_stats — the mutants #2/#3/#18/#19 sites, all present"
        status: pass
    human_judgment: false
  - id: D7
    description: "Token scopes, consume flags and the rate-limit chain are unchanged on both routes"
    requirement: "STR-08"
    verification:
      - kind: other
        ref: "both routes still read rateLimitMiddleware(httpServer.uploadLimiter, nil, tokenMiddlewareAny(tokens, []tokenScope{tokenScopeTransfer, tokenScopeBootstrap}, true, …)); the nil settings provider on the download limiter is pre-existing and untouched"
        status: pass
    human_judgment: false
  - id: D8
    description: "No test function was renamed or removed (CONTEXT D-13), so the Phase 3 33-mutant table still keys on every name"
    requirement: "STR-08"
    verification:
      - kind: other
        ref: "git diff --name-only 18465dc..HEAD -- '*_test.go' => no files; grep -c '^func Test' beamsync/server_test.go => 13, unchanged; TestStartSenderIssuesClientBoundSingleUseDownloadToken => PASS (0.01s)"
        status: pass
    human_judgment: false
  - id: D9
    description: "Full Phase 4 gate green: beamsync build + vet + test -race, desktop build + vet + test -race"
    requirement: "STR-08"
    verification:
      - kind: other
        ref: "(cd beamsync && go build ./... && go vet ./... && go test -race -count=1 ./...) => ok beamsync 14.560s; audio [no test files]"
        status: pass
      - kind: other
        ref: "(cd desktop && go build . && go vet . && go test -race .) => ok desktop 1.193s"
        status: pass
    human_judgment: false
  - id: D10
    description: "T-04-08-01 (information disclosure while the response assembly moves): the header-building, 404/stat-error branches and the terminal log travelled with the body"
    requirement: "STR-08"
    verification:
      - kind: other
        ref: "anchor sweep — download_handler.go has File not found, download_progress, recordSent, copyChunked, progressWriter, Cache-Control, X-Filename ×2, Content-Length, bufio.NewReaderSize, mime.TypeByExtension, beginUpload, endUpload, logTransfer; server.go has 0 of the download-specific ones. The 3 survivors were read in place: beginUpload/endUpload at server.go:48/:58 are serverState's method declarations (D-06 keeps the type), Cache-Control ×7 belongs to /heartbeat, the page routes and /logo.png"
        status: pass
    human_judgment: false
  - id: D11
    description: "Prohibition: the extraction changed no runtime behavior, no event payload and no HTTP response body"
    verification: []
    human_judgment: true
    rationale: "A de-duplicating closure-to-method move is proven by the two empty normalized diffs (both source bodies converge on the one new body after a counted set of substitutions), the per-anchor residual audit, and the full -race suite in both modules; a reviewer confirms from the diffstat (83 added in download_handler.go, 131 removed from server.go) that no behavior surface was touched"

# Metrics
duration: 5min
completed: 2026-11-18
status: complete
---

# Phase 4 Plan 08: `download_handler.go` Extraction Summary

**The two download handler bodies — `/download` and `/download/N`, behaviorally identical apart from their base-name local (CONTEXT D-12) — are now one method, `func (s *HTTPServer) handleDownload(filePath, w, r)` in `beamsync/download_handler.go`, with both routes reduced to a one-line delegation, `server.go` down to 698 lines and the final 11 imports, and the de-duplication proved by an empty normalized diff from *each* of the two originals.**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-11-18T21:37:00+05:30
- **Completed:** 2026-11-18T21:42:00+05:30
- **Tasks:** 3 (2 committed, 1 verification-only)
- **Files modified:** 2 (1 created, 1 modified)

## Accomplishments

- Created `beamsync/download_handler.go` (83 lines): the doc comment naming the D-12 invariant, then `func (s *HTTPServer) handleDownload(filePath string, w http.ResponseWriter, r *http.Request)` with the complete download body in source order — `realName := filepath.Base(filePath)` (`:18`), `s.state.beginUpload()` and the first `transfer_stats` emit with its deferred `endUpload` partner (`:19-24`), the `Cache-Control` / `Content-Disposition` / `X-Filename` / `Access-Control-Expose-Headers` headers (`:25-28`), `os.Open` with the `File not found` 404 (`:30-34`), `defer file.Close()` and `file.Stat()` with the 500 branch (`:35-42`), `Content-Length` and `mime.TypeByExtension` (`:43-49`), the `progressWriter` literal with `eventName: "download_progress"`, `emit: s.emit` and `now: time.Now` (`:52-60`), `copyChunked(pw, bufReader, 8*1024*1024)` (`:61`), the terminal `s.emit(pw.eventName, …)` + `recordSent` + `transfer_stats` block (`:71-73`), and `logTransfer(s.history, s.emit, …)` (`:75-82`). Seven stdlib imports.
- **Proved the merge from both directions.** Two independent normalized diffs, both empty on the first pass: (A) the multi-file body from `18465dc:beamsync/server.go:717-779` with six receiver substitutions (`emit:` field → `s.emit`, three `state.*` → `s.state.*`, `logTransfer(s.history, emit,` → `s.emit`, four bare `emit(` → `s.emit(`) and the one-line cross-reference comment expanded to the fuller three-line version; (B) the single-file body from `:647-710` with `realName := filepath.Base(filePath)` prepended, seven `filename` identifier rewrites (the `filename=` inside the Content-Disposition format string preserved, as is the `progressWriter` field name), and the same six receiver swaps. Each normalized text `diff -w -B`'d against the one new body printed nothing, so the claim "one method serves both" is a measurement rather than an assertion.
- Rewired both routes. `server.go:641-645` is now `if len(filePaths) == 1 { mux.HandleFunc("/download", rateLimitMiddleware(httpServer.uploadLimiter, nil, tokenMiddlewareAny(tokens, []tokenScope{tokenScopeTransfer, tokenScopeBootstrap}, true, func(w, r) { httpServer.handleDownload(filePaths[0], w, r) }))) }`; `:646-652` keeps `idx := i` and `filePath := path` and passes the captured `filePath`. Middleware order, token scopes, the `true` consume flag and the `nil` settings provider on the download limiter are all unchanged.
- Deleted the plan's named compile hazard: `filePath := filePaths[0]` and `filename := filepath.Base(filePath)` were declared outside the `/download` closure, so delegating would have left both unused — a hard Go error, not a warning. Both are gone, and `handleDownload` derives its own `realName` from its parameter instead.
- Trimmed `server.go` 827 → 698 lines (131 removed, 2 added) and pruned `bufio` and `mime`, whose last two uses each were inside the moved bodies. `server.go` now carries exactly the final 11 imports — `context embed encoding/json fmt net/http os path/filepath runtime/debug strings sync time`.
- Executed the anchor sweep as an audit, not a count. All 13 named anchors are present in `download_handler.go` (`X-Filename` twice, for the header and the expose-headers value). Ten are absent from `server.go`; the three survivors were read at their lines and are pre-existing non-download code — `beginUpload`/`endUpload` at `server.go:48`/`:58` are `serverState`'s method *declarations* (D-06 keeps the type in place) and the seven `Cache-Control` sets belong to `/heartbeat`, the HTML page routes and `/logo.png`.
- Full gate green in both modules: `beamsync` build + vet + `go test -race -count=1 ./...` → `ok beamsync 14.560s`; `desktop` build + vet + `go test -race .` → `ok desktop 1.193s`. The two download-named tests pass explicitly: `TestMultiFileSenderPathEmitsCompletionProgress` (0.46s) and `TestStartSenderIssuesClientBoundSingleUseDownloadToken` (0.01s).

## Task Commits

Each task was committed atomically:

1. **Task 8.1: add the unified download handler method** — `e1d784b` (refactor)
2. **Task 8.2: delegate both download routes to handleDownload** — `99be273` (refactor)
3. **Task 8.3: full verification gate** — no code change, so no commit (gate only: `go build`/`go vet`/`go test -race` in both modules, all green)

**Plan metadata:** _(this commit — docs: complete plan)_

## Files Created/Modified

- `beamsync/download_handler.go` (created, 83 lines) — the `handleDownload` method plus a 7-package stdlib import block (`bufio`, `fmt`, `mime`, `net/http`, `os`, `path/filepath`, `time`). Narrower than `upload_handler.go` because the download body never touches multipart, `encoding/json`, `io`, `runtime/debug` or `strings`.
- `beamsync/server.go` (modified, 827 → 698) — the two download closures replaced by delegations; `filePath`/`filename` locals deleted from the `len(filePaths) == 1` branch; `bufio` and `mime` pruned. `PendingTransfer`, `//go:embed ui/*`, `uiFS`, `serverState`, both `Start*E` signatures and every other route are untouched.

## Decisions Made

- **The multi-file body is the template.** It is the complete program — it already derives its own base name — so adopting it verbatim needs zero restructuring; the single-file body is the one that had an outer dependency to lose. Adoption direction matters for reviewability: the diff you *don't* have to justify is the one from the body you kept whole.
- **`realName` as the first statement, not after the Cache-Control set.** `04-PATTERNS.md`'s illustrative snippet declares `filename := filepath.Base(filePath)` between the `Cache-Control` and `Content-Disposition` headers; the implemented method instead puts `realName := filepath.Base(filePath)` at the top, matching the multi-file body. Nothing reads the name before the `Content-Disposition` line, so the two placements are behaviorally identical — and the snippet's placement would have been a gratuitous edit to the body being proved a copy. The plan's action text ("the `realName := filepath.Base(filePath)` derivation" first in the moved set) is what was followed.
- **Drop the cross-reference comment, keep the explanation.** Multi-file said "Completion event — see the single-file send path for why."; single-file carried the three-line why. Keeping the reference would have left a dangling pointer once the "single-file send path" ceased to exist as a separate thing, so the unified method carries the explanation and not the pointer.
- **Substitute into the OLD text, never normalize the NEW one.** Both proofs transformed the pre-move body and diffed it against the shipped file. Normalizing the new file instead would have made any edit invisible — the direction of the proof is the whole point.
- **Record the import prune as forced, not chosen.** `bufio` and `mime` had exactly two uses each, all four inside the moved bodies. Go rejects an unused import, so the prune is the compiler's requirement rather than a tidy-up; the same commit carries it so no intermediate state has a dangling import.
- **Leave `idx`/`filePath` captures alone.** `go.mod` targets Go 1.25, where per-iteration loop variables make the copies redundant — but they are pre-existing code, and 04-PATTERNS explicitly flags removing them as an unrelated change. Phase 4's charter is structural movement, not modernization.

## Deviations from Plan

None of substance — the plan executed as written. Three recorded factual notes:

- **Stale line numbers in the plan text.** The plan cites `server.go:1354-1355` for the two outer locals and `:1390/:1461/:1383/:1454` for the `bufio`/`mime` uses. After 04-01…04-07 the real positions are `:644-645` for the locals, and the import uses are inside the closures at `:679/:750` (`bufio`) and `:672/:743` (`mime`). Every anchor was re-derived from the live file before cutting, exactly as 04-04 through 04-07 each recorded after a hand-guessed range failed.
- **Task 8.1's `<files>` lists `beamsync/server.go` as well.** No `server.go` edit was needed or made in that task: the method is purely additive and the file still built and tested green with both inline closures present (an uncalled method is not a Go error). The rewiring landed entirely in Task 8.2, so the commits split cleanly along "add" then "delegate".
- **`golangci-lint` was not run.** Same pre-existing environmental limitation recorded in 04-01 through 04-07: the local v1.64.8 cannot decode Go 1.27.4 export data and fails on untouched files too. The unmodified `defer file.Close()` carries no `//nolint` before or after, so the lint posture is unchanged by construction. `go build`, `go vet` and `go test -race` remain the authoritative gate.

## Issues Encountered

- **Nothing survived normalization on either pass.** Both diffs came back empty on the first attempt — the first time in this phase that has happened (04-07 needed a second pass for six hunks). The difference is attributable to enumerating the substitution set from the code rather than from an estimate: the multi-file body yielded exactly six receiver captures and one comment line, and the single-file body exactly seven `filename` identifiers (one of which shares a line with a string literal that must *not* change) plus the same six. The one trap was the Content-Disposition format string, `attachment; filename="%s"`, whose `filename=` is HTTP header syntax, not a Go identifier — a naive `\bfilename\b` rewrite would have silently produced a broken header with no compiler error. The rewrite set was therefore written as literal patterns per occurrence.
- **A duplicate-looking `if len(filePaths) == 1` remains in `server.go` at `:566`.** Checked and unrelated: that is the HTML file-card builder in `StartServerE`, which derives its own `name := filepath.Base(filePaths[0])` for the rendered page. It never referenced the deleted local, so it compiled untouched — worth noting only because a future grep for the deleted pattern will hit it.

## User Setup Required

None - no external service configuration required.

## Threat Flags

None — no new network surface was added. `beamsync/download_handler.go` contains no new route, no new parameter parsing and no new file access: the one trust boundary the plan names (`client request -> handleDownload -> filePath`) is crossed by the same `os.Open(filePath)` that existed before, reachable only behind the same `rateLimitMiddleware(httpServer.uploadLimiter, nil, …)` wrapper and the same `tokenMiddlewareAny(tokens, []tokenScope{tokenScopeTransfer, tokenScopeBootstrap}, true, …)` check with the same consume flag. T-04-08-01 (information disclosure during the method move) is mitigated as planned — the four header sets, the `File not found` / `Failed to stat file` branches and the terminal `logTransfer` are present in the new file and absent from `server.go`, and the full suite runs under `-race` in both modules.

The one *pre-existing* failure mode this plan inherited is the nil `s.emit` that `handleDownload` would hit on the first `transfer_stats` emit; it was already closed in 04-07 Task 7.1, which initialized `emit: emit` in **both** constructors (`server.go:257`, `:508`). Verified at those two lines before running the gate.

## Known Stubs

None.

## Next Phase Readiness

- `beamsync/server.go` is at **698 lines** and no longer holds any handler body: the upload and download bodies both live in their own files, and every remaining route registration is a short closure or a bare method value. `bufio` and `mime` are gone, leaving exactly the final 11 imports — `context embed encoding/json fmt net/http os path/filepath runtime/debug strings sync time` — which is the import set 04-09 should assert.
- 04-09 (the final trim) inherits no compile hazard from this plan: no unused locals, no dangling imports, and `handleDownload`/`handleUpload` are both reached only through registered routes, so neither is an unused-method candidate.
- **Still resident in `server.go` and required to stay (D-06):** `PendingTransfer`, `//go:embed ui/*.html ui/*.png` + `uiFS`, `serverState` with its five methods, and both `Start*E` signatures. A package-wide `rg uiFS` must keep reporting its hits in `server.go` alone.
- The diffusion risk that motivated D-12 is now structurally impossible: there is one download implementation, so the two routes cannot drift apart again.

---

*Phase: 04-go-structural-refactor-server-go-extraction*
*Completed: 2026-11-18*

## Self-Check: PASSED

- `beamsync/download_handler.go` exists (83 lines) and declares `func (s *HTTPServer) handleDownload(filePath string, w http.ResponseWriter, r *http.Request)` at line 17.
- `server.go:641-645` delegates `/download` to `httpServer.handleDownload(filePaths[0], w, r)`; `:646-652` delegates each `/download/%d` to `httpServer.handleDownload(filePath, w, r)` with `idx := i` / `filePath := path` retained.
- Normalized diff A (multi-file old body + six receiver swaps + fuller comment) vs the new body → no output.
- Normalized diff B (single-file old body + prepended `realName` + seven `filename`→`realName` rewrites + the same six swaps) vs the same new body → no output.
- `grep -n 'filename :=' beamsync/server.go` → 0 matches; `gofmt -l .` → no output; `go build ./... && go vet ./...` → exit 0.
- Anchor sweep: all 13 anchors present in `download_handler.go`; the 3 that remain in `server.go` (`beginUpload`, `endUpload`, `Cache-Control`) were read in place and are `serverState` declarations and other routes.
- No `_test.go` file changed; 13 top-level tests, unchanged.
- Full gate green: `beamsync` build/vet/test-race `ok 14.560s`, `desktop` build/vet/test-race `ok 1.193s`.
- Commits `e1d784b` and `99be273` are in the log and touch only `download_handler.go` (created 83) and `server.go` (−131/+2).
