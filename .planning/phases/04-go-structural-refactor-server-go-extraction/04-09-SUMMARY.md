---
phase: 04-go-structural-refactor-server-go-extraction
plan: 09
subsystem: infra
tags: [go, refactor, verification, zero-delta-guard, orchestration-only, import-pruning]

# Dependency graph
requires:
  - phase: 03-tests-before-refactoring
    provides: the 33-mutant table whose test function names must not change, and the -race suite used as the per-step safety net
  - phase: 04-go-structural-refactor-server-go-extraction
    plan: 08
    provides: beamsync/server.go at 698 lines with the final 11 imports and no handler body left inline — the state this plan proves rather than produces
provides:
  - a verified orchestration-only beamsync/server.go (the STR-09 end state) — zero declarations across the 30 extracted symbols, the six retained anchors present at their exact lines, exactly the final 11 imports, and uiFS at 5 hits in server.go alone
  - the phase's closing two-module gate — beamsync go build/go vet/go test -race and desktop go build/go vet/go test -race, both green with -count=1
affects: []

# Actuals (#2632) — same chars/4 scale as the plan's estimate
actuals:
  tokens: 0
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Zero-delta guard as a first-class plan step (third use this phase, after 04-02 Task 2.2 and 04-03 Task 3.2): when the predecessor already deleted every declaration the trim plan was written to delete, the honest artifact is a proof plus an `--allow-empty` commit — not a cosmetic diff. Manufacturing a reformat to justify the commit would make the diff the deliverable and the proof the afterthought, which is exactly backwards for a verification plan"
    - "Two sweeps with opposite polarity: the declaration sweep must return ZERO while the retained-surface sweep must return EXACTLY ONE per anchor. A single 'did it work' grep can only assert one half of the requirement — pairing them catches a stranded declaration and an over-eager deletion with the same instrument"
    - "Call-site reconciliation as the anti-T-04-09-01 measurement: for each extracted symbol still referenced by server.go, the pre-phase call-site count was compared with the shipped count, and EVERY delta was reconciled against a specific line in the file the symbol moved to (`setCORSHeaders` 5→4 via middleware.go:77, `extractToken` 4→2 via middleware.go:33/:50, `clientIP` 2→1 via middleware.go:83). A bare 'counts match' claim would not have caught a silently dropped auth call — the reconciliation is what makes the number meaningful"

key-files:
  created: []
  modified: []

key-decisions:
  - "Task 9.1 was committed with `--allow-empty` and a body that explains the zero delta, matching the precedent set by 04-02's `37c156c` and 04-03's `ad0649a`. No reformat, no re-comment, no invented edit: `gofmt -l beamsync/server.go` is silent and `git status --porcelain` was empty before the commit, so there was genuinely nothing to stage"
  - "The sweeping assertion was run at both widths — the plan's own 10-pattern acceptance command and a 30-pattern expansion covering every symbol the eight extraction plans moved (`EventCallback`, `defaultEventDispatcher`, `newEventDispatcher`, `eventDispatchJob`, `startBootstrapRotator`, `safeEmit`, `logTransfer`, `rateLimitState`, `rateLimitDecision`, `newClientRateLimiter`, `setRateLimitHeaders`, `setCORSHeaders`, `extractToken`, `clientIP`, `tokenMiddleware`, `tokenMiddlewareAny`, `generateID`, `sha256Hex`, `autoRenamePath`, `getLocalIPForURL`, `uploadIntegrityHeader`). Both returned exit 1 with no output. The narrower command is the plan's contract; the wider one is what actually rules out a stranded declaration in a symbol the plan's regex forgot"
  - "The plan's briefing note that `getLocalIPForURL` is a route-registration call in server.go is wrong and was corrected rather than forced. Its only caller is `startBootstrapRotator`, which itself moved to `events.go` in 04-01, so the surviving call is `events.go:161` and server.go correctly has no reference at all. Nothing was added to server.go to satisfy the note"
  - "`desktop`'s suite was run with `-count=1` even though the plan's command omits it: the first invocation returned `ok desktop (cached)`, and a cache hit is not evidence that the final tree passes. Forcing the run produced `ok desktop 1.193s`. The beamsync suite already carried `-count=1`"
  - "`tokenMiddleware` (the single-scope variant) has no production call site — only its declaration at `middleware.go:27` and two test call sites in `server_test.go`. Verified pre-existing: the baseline `server.go` also had exactly zero call sites, so the extraction did not drop an auth check. Left alone: removing it is a behavior removal, not a structural move, and it would break two tests whose names must not change (D-13)"
  - "`golangci-lint` was not attempted. Same pre-existing environmental limitation recorded in 04-01 through 04-08: the local v1.64.8 cannot decode Go 1.27.4 export data and fails on untouched files too, so a run would produce noise, not signal. `go build`, `go vet`, `gofmt -l` and `go test -race` remain the authoritative gate, and this plan changes no code, so no lint posture could shift"

patterns-established:
  - "A trim plan that inherits an already-trimmed file should be executed as an assertion suite: pick the must-be-zero patterns, the must-be-exactly-one patterns, the exact allowed set for any counted thing (imports, uiFS hits, route registrations) and reconcile every surviving call site against its new home — then commit empty. The proof is the deliverable"

requirements-completed: [STR-09]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "Task 9.1 acceptance: the plan's declaration sweep returns no matches against beamsync/server.go"
    requirement: "STR-09"
    verification:
      - kind: other
        ref: "rg -n 'type eventDispatcher|func startWatchdog|var chunkBufferPool|func copyChunked|func autoRenamePath|type clientRateLimiter|func rateLimitMiddleware|type progressWriter|func handleUpload|func handleDownload' beamsync/server.go => no output, exit 1 (re-run for Task 9.2 acceptance: same result)"
        status: pass
    human_judgment: false
  - id: D2
    description: "The wider 30-symbol sweep also returns no matches — no extracted declaration is stranded under a name the plan's regex omits"
    requirement: "STR-09"
    verification:
      - kind: other
        ref: "the same command extended to all 30 symbols (incl. EventCallback, defaultEventDispatcher, newEventDispatcher, eventDispatchJob, startBootstrapRotator, safeEmit, logTransfer, rateLimitState, rateLimitDecision, newClientRateLimiter, setRateLimitHeaders, setCORSHeaders, extractToken, clientIP, tokenMiddleware, tokenMiddlewareAny, generateID, sha256Hex, autoRenamePath, getLocalIPForURL, uploadIntegrityHeader) => no output, exit 1"
        status: pass
    human_judgment: false
  - id: D3
    description: "The six retained anchors are present, one match each, at the exact lines the orchestrator reported"
    requirement: "STR-09"
    verification:
      - kind: other
        ref: "rg -c per pattern => 1 for each: //go:embed ui/*.html ui/*.png (:29), var uiFS embed.FS (:30), type serverState struct (:32), type HTTPServer struct (:83), func StartServerE (:212), func StartSenderE (:479); the plan's 4-pattern Task 9.2 variant also 1 each"
        status: pass
    human_judgment: false
  - id: D4
    description: "The import block is exactly the final 11 and none of the 11 pruned imports survives"
    requirement: "STR-09"
    verification:
      - kind: other
        ref: "sed -n '/^import (/,/^)/p' beamsync/server.go | grep -c '^\\s*\"' => 11; list => context embed encoding/json fmt net/http os path/filepath runtime/debug strings sync time; comm -23 baseline-vs-current => bufio crypto/rand crypto/sha256 encoding/hex io math mime mime/multipart net strconv sync/atomic (11 removed, matching 04-PATTERNS' predicted set exactly); go build ./... exit 0 proves the set is sufficient"
        status: pass
    human_judgment: false
  - id: D5
    description: "uiFS is exactly 5 hits, all in server.go — 1 declaration plus the 4 reads (D-06 keeps the embed in the serving file)"
    requirement: "STR-09"
    verification:
      - kind: other
        ref: "rg -c uiFS beamsync/ => beamsync/server.go:5 and no other file; rg -n => :30 var uiFS embed.FS, :291 ui/upload.html, :317 ui/logo.png, :608 ui/download.html, :633 ui/logo.png"
        status: pass
    human_judgment: false
  - id: D6
    description: "The guarded REFERENCES survive — only declarations moved. HTTPServer still fields clientRateLimiter x4 and EventCallback; both Start*E signatures still take EventCallback; both constructors still set emit: emit"
    requirement: "STR-09"
    verification:
      - kind: other
        ref: "server.go:97-100 pageLimiter/heartbeatLimiter/transferLimiter/uploadLimiter all *clientRateLimiter; :108-112 the emit field comment plus `emit EventCallback`; :212 and :479 `callback EventCallback` in the signatures; :257 and :508 `emit: emit`; :235 and :492 `emit := func(evt, data string) { safeEmit(callback, evt, data) }`"
        status: pass
    human_judgment: false
  - id: D7
    description: "All 11 route registrations survive with their middleware chains intact, and the registry is identical to the pre-phase baseline as a multiset of path expressions"
    requirement: "STR-09"
    verification:
      - kind: other
        ref: "route paths baseline vs current, sorted+counted, are identical: /\"/\" x2, /\"/heartbeat\" x2, /\"/logo.png\" x2, /\"/download\", /download/%d (fmt.Sprintf), /\"/request-transfer\", /\"/stats\", /\"/upload\" — 11 registrations each side. rateLimitMiddleware call sites 11 -> 11; tokenMiddlewareAny(tokens 7 -> 7; safeEmit 2 -> 2; startWatchdog 2 -> 2; startBootstrapRotator 2 -> 2; generateID 1 -> 1; newClientRateLimiter 8 -> 8"
        status: pass
      - kind: other
        ref: "server.go:427 /upload => tokenMiddlewareAny(tokens, [session,bootstrap], false, httpServer.handleUpload); :642-643 /download => [... transfer,bootstrap], true, httpServer.handleDownload(filePaths[0],...); :649-650 /download/%d => same scopes/flag, httpServer.handleDownload(filePath,...)"
        status: pass
    human_judgment: false
  - id: D8
    description: "Every call-site count that changed was reconciled to a specific line in the file that now owns the code — no auth call was silently dropped"
    requirement: "STR-09"
    verification:
      - kind: other
        ref: "setCORSHeaders 5 -> 4: the 4 route sets (:290/:314/:605/:630) survive unchanged, the 5th (baseline :569) is inside rateLimitMiddleware => middleware.go:77. extractToken 4 -> 2: the 2 bootstrap validates (:286/:601) survive, the 2 in tokenMiddleware/Any (baseline :525/:542) => middleware.go:33/:50. clientIP 2 -> 1: the /request-transfer use (:342) survives, the rate-limit use (baseline :575) => middleware.go:83"
        status: pass
      - kind: other
        ref: "tokenMiddleware (single-scope) has 0 production call sites both before and after: baseline server.go declares it at :519 with no caller; now middleware.go:27 with only server_test.go:21/:46 as callers. Pre-existing dead surface, not a dropped check"
        status: pass
    human_judgment: false
  - id: D9
    description: "T-04-09-01 (stale route or symbol left behind during the trim) is mitigated: the sweep is clean AND the auth/handler pins pass"
    requirement: "STR-09"
    verification:
      - kind: unit
        ref: "beamsync -race -count=1 targeted run => PASS TestTokenMiddlewareRejectsMissingOrInvalidToken (0.00s), TestTokenMiddlewareAllowsValidTokenAndOptions (0.00s), TestStartServerLifecycleRootAndHeartbeat (6.31s), TestStartSenderIssuesClientBoundSingleUseDownloadToken (5.82s), TestUploadWithoutFileReturnsBadRequest (0.01s, both subtests), TestUploadWithFileSavesToDiskAndEmitsEvents (0.13s), TestReceivePathEmitsCompletionProgress (0.02s), TestMultiFileSenderPathEmitsCompletionProgress (0.44s)"
        status: pass
    human_judgment: false
  - id: D10
    description: "No test function was renamed or removed (CONTEXT D-13), so Phase 3's 33-mutant table still keys on every name"
    requirement: "STR-09"
    verification:
      - kind: other
        ref: "git diff --name-only 99be273..HEAD -- '*_test.go' => no files; per-file '^func Test' counts unchanged (server_test.go 13, progress_test.go 8, progress_wiring_test.go 7, event_dispatcher_test.go 7, middleware_test.go 5, rate_limiter_test.go 3); 83 beamsync + 32 desktop = 115 test functions"
        status: pass
    human_judgment: false
  - id: D11
    description: "Full Phase 4 gate green in both modules, with -count=1 so no result is a cache hit"
    requirement: "STR-09"
    verification:
      - kind: other
        ref: "(cd beamsync && go build ./... && go vet ./...) => exit 0; go test -race -count=1 ./... => ok beamsync 14.463s, ? beamsync/audio [no test files]"
        status: pass
      - kind: other
        ref: "(cd desktop && go build . && go vet .) => exit 0; go test -race -count=1 . => ok desktop 1.193s (the plan's plain `go test -race .` reported (cached) first, so it was re-run forced)"
        status: pass
      - kind: other
        ref: "gofmt -l beamsync/server.go => no output"
        status: pass
    human_judgment: false
  - id: D12
    description: "Prohibition: the extraction changed no runtime behavior, no event payload and no HTTP response body"
    verification: []
    human_judgment: true
    rationale: "This plan changed no code at all — `git status --porcelain` was empty before its single commit and that commit is empty — so the prohibition holds for this plan by construction. Across the phase the evidence is the unchanged route registry (11 registrations, same path multiset as the 1538-line baseline), the reconciled call-site counts, the unchanged 115 test function names, and a fully green -race suite in both modules"

# Metrics
duration: 10min
completed: 2026-11-18
status: complete
---

# Phase 4 Plan 09: Final `server.go` Trim Summary

**`beamsync/server.go` is proved — not merely assumed — to be orchestration-only: a 30-symbol declaration sweep returns zero matches, the six retained anchors sit at their exact lines from `//go:embed` (:29) to `StartSenderE` (:479), the import block is exactly the final 11, `uiFS` is 5 hits in `server.go` alone, all 11 route registrations match the 1538-line baseline as a multiset, every changed call-site count reconciles to a line in the file that now owns the code, and both Go modules pass build, vet and `-race` — committed as a designed `--allow-empty` zero-delta guard because there was nothing left to trim.**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-11-18T21:36:00+05:30
- **Completed:** 2026-11-18T21:46:00+05:30
- **Tasks:** 2 (1 committed as an empty guard, 1 verification-only)
- **Files modified:** 0 — zero-delta by design

## Accomplishments

- **Ran the declaration sweep at both widths and got zero from both.** The plan's 10-pattern acceptance command returned exit 1 with no output; the 30-symbol expansion — adding every name the eight extraction plans own, including `EventCallback`, `defaultEventDispatcher`, `newEventDispatcher`, `eventDispatchJob`, `startBootstrapRotator`, `rateLimitState`, `rateLimitDecision`, `newClientRateLimiter`, `setRateLimitHeaders`, `tokenMiddleware`, `sha256Hex`, `autoRenamePath`, `getLocalIPForURL`, `uploadIntegrityHeader` — returned the same. Nothing is stranded under a name the plan's regex forgot.
- **Confirmed the retained surface at the orchestrator's exact line numbers**, one match per pattern: `//go:embed ui/*.html ui/*.png` at `:29`, `var uiFS embed.FS` at `:30`, `type serverState struct` at `:32`, `type HTTPServer struct` at `:83`, `func StartServerE(...)` at `:212`, `func StartSenderE(...)` at `:479`. `type PendingTransfer struct` is also still at `:18`, and the nine `*HTTPServer` methods — `RespondToTransfer` `:115` through `RotateBootstrapToken` `:197` — are untouched.
- **Verified the import block is exactly the final 11**: `context embed encoding/json fmt net/http os path/filepath runtime/debug strings sync time`. Diffing the baseline's 22-entry block against it with `comm -23` returns precisely `bufio crypto/rand crypto/sha256 encoding/hex io math mime mime/multipart net strconv sync/atomic` — the same 11 that `04-PATTERNS.md` predicted would leave, and `go build ./...` proves nothing further is needed.
- **Counted `uiFS` package-wide**: `rg -c uiFS beamsync/` produces one line, `beamsync/server.go:5` (1 declaration at `:30`, 4 reads at `:291` upload.html, `:317` logo.png, `:608` download.html, `:633` logo.png). No other file references it, so D-06's "keep the embed in the serving file" left zero cross-file references.
- **Audited the route registry rather than counting it.** Extracting `mux.HandleFunc` path expressions from the pre-phase baseline (`65c6a2d:beamsync/server.go`) and from the shipped file and comparing them sorted-and-counted gives an identical multiset: `"/"` ×2, `"/heartbeat"` ×2, `"/logo.png"` ×2, `"/download"`, `fmt.Sprintf("/download/%d", …)`, `"/request-transfer"`, `"/stats"`, `"/upload"` — 11 registrations on both sides. `rateLimitMiddleware` call sites are 11 → 11 and `tokenMiddlewareAny(tokens` call sites 7 → 7 (the naive `rg -c` reads 8 → 7 only because the declaration line moved out with the function).
- **Reconciled every call-site count that *did* change against its new home.** `setCORSHeaders` 5 → 4: the four route sites (`:290`, `:314`, `:605`, `:630`) survive verbatim and the fifth was inside `rateLimitMiddleware`, now `middleware.go:77`. `extractToken` 4 → 2: the two bootstrap validations (`:286`, `:601`) survive and the two in `tokenMiddleware`/`tokenMiddlewareAny` are now `middleware.go:33`/`:50`. `clientIP` 2 → 1: `/request-transfer` at `:342` survives and the rate-limit use is now `middleware.go:83`. This is the evidence T-04-09-01 actually needs — a dropped auth call would have compiled cleanly.
- **Confirmed the one dead symbol is pre-existing.** `tokenMiddleware` (single-scope) has zero production call sites both before and after: it is declared at `middleware.go:27` now as it was at baseline `server.go:519`, with only two test callers (`server_test.go:21`, `:46`). Left in place — deleting it is a behavior removal, not a structural move, and would break two tests whose names must not change.
- **Ran the full two-module gate.** `beamsync`: `go build ./...` + `go vet ./...` exit 0, `go test -race -count=1 ./...` → `ok beamsync 14.463s`, `? beamsync/audio [no test files]`. `desktop`: `go build .` + `go vet .` exit 0, `go test -race -count=1 .` → `ok desktop 1.193s`. A targeted run of the nine auth/handler pins passes individually, including `TestStartServerLifecycleRootAndHeartbeat` (6.31s) and `TestStartSenderIssuesClientBoundSingleUseDownloadToken` (5.82s).
- **Committed the guard empty.** `git status --porcelain` was empty and `gofmt -l beamsync/server.go` silent before the commit, so Task 9.1 landed as `76828ca` with `--allow-empty` — the same shape as 04-02's `37c156c` and 04-03's `ad0649a` — rather than a cosmetic edit.

## Task Commits

Each task was handled atomically:

1. **Task 9.1: remove leftover extracted symbols and unused imports from server.go** — `76828ca` (refactor, `--allow-empty`: the sweeps proved the removal had already happened in waves 1–8, so the task's file delta is zero)
2. **Task 9.2: full verification gate** — no code change, so no commit (gate only: `go build`/`go vet`/`go test -race -count=1` in both modules, all green)

**Plan metadata:** _(this commit — docs: complete plan)_

## Files Created/Modified

- **None.** This plan's designed output is a proof, not a diff. `beamsync/server.go` was already at its target state when the plan started: 698 lines, 11 imports, no extracted declarations, six retained anchors, `uiFS` at 5 hits, 11 routes.

## Decisions Made

- **`--allow-empty` over an invented edit.** The plan's Task 9.1 is named "remove leftover extracted symbols and unused imports", but waves 1–8 each removed their own residue — including the last orphaned imports (`io`, `mime/multipart` at 04-07; `bufio`, `mime` at 04-08). Reformatting a comment or re-wrapping a struct field to manufacture a diff would have made the commit message a fiction. The empty commit is the accurate record of "verified nothing was left".
- **Sweep wide, then sweep the plan's exact command.** The plan's 10-pattern regex covers the headline symbols but not the secondary ones (`EventCallback`, the dispatcher internals, the rate-limit state types). Running the plan's command satisfies the contract; running the 30-symbol version is what actually establishes the claim. Both were run and both were clean, so the summary can state the stronger fact without weakening the acceptance criterion.
- **Corrected the briefing on `getLocalIPForURL` instead of satisfying it.** The execution note listed it among the route-registration calls that must keep referencing extracted code, but it was never a route call: its only caller is `startBootstrapRotator`, which moved to `events.go` in 04-01, so the surviving call is `events.go:161`. Adding a reference to `server.go` to match the note would have been a behavior-adjacent edit to a file whose whole point is that it no longer needs one.
- **Forced `desktop`'s suite past the cache.** The plan's command is `go test -race .`, which returned `ok desktop (cached)` — a cache hit proves the tree matches a previous run, not that it passes now. `-count=1` produced a real `ok desktop 1.193s`.
- **Left `tokenMiddleware` dead surface alone.** It is unreachable from any route and was already so at baseline. Phase 4's charter is structural movement; a deletion here would be a behavior change keyed to two tests the mutation table names.
- **Did not touch `golangci-lint`.** The local v1.64.8 cannot decode Go 1.27.4 export data and fails on files this phase never edited. Since this plan changed zero code, no lint posture could shift; the gate remains `go build` + `go vet` + `gofmt -l` + `go test -race`.

## Deviations from Plan

### Auto-fixed Issues

None — no code was written, so no deviation rule could fire. Three recorded factual notes:

- **`desktop`'s gate was run with `-count=1`.** The plan specifies `go test -race .`; the first invocation hit the test cache (`ok desktop (cached)`), so it was re-run forced to get real evidence. Recorded here because the command in the plan's `<verify>` block is not literally what was executed.
- **The execution briefing's `getLocalIPForURL` claim was wrong and was not followed.** It named `getLocalIPForURL` among the route-registration references that must survive in `server.go`. Verified: its only caller is `startBootstrapRotator`, which lives in `events.go`, so `rg -n 'getLocalIPForURL' beamsync/server.go` returns nothing — correctly. No edit was made to force the reference into `server.go`.
- **`golangci-lint` was not run.** The same pre-existing environmental limitation recorded in 04-01 through 04-08. No code changed in this plan, so nothing could regress.

## Issues Encountered

- **Two call-site counts looked like dropped code and had to be reconciled line by line.** `rg`'s raw `-c` output disagreed with the baseline for `setCORSHeaders` (5 vs 4), `extractToken` (4 vs 2) and `clientIP` (2 vs 1) — and `tokenMiddlewareAny(tokens` read 8 vs 7. Every one of those deltas is code that moved into `middleware.go`: the declaration line for `tokenMiddlewareAny` (8 → 7), one use inside `rateLimitMiddleware` (`setCORSHeaders`, `clientIP`) and two inside `tokenMiddleware`/`tokenMiddlewareAny` (`extractToken`). Resolving them required reading each hit in place in both the baseline and the current tree. This mattering is the point of T-04-09-01: deleting a middleware call would compile, vet clean, and only fail a test that exercises the specific route.
- **The orchestrator's line numbers for the retained anchors were exactly right**, which is unusual in this phase — 04-04 through 04-08 each had to re-derive hand-guessed ranges. Every one of the six matched on the first check, so the sweeps needed no re-basing.

## User Setup Required

None - no external service configuration required.

## Threat Flags

None — this plan adds no surface of any kind; it created no file and edited no file. T-04-09-01 (tampering: a stale route or symbol left behind during the trim) is mitigated by measurement rather than by a change: the declaration sweep is clean at both 10 and 30 patterns, the retained-surface sweep finds all six anchors, the 11 route registrations match the pre-phase baseline as a multiset of path expressions, and the four call-site counts that differ are each reconciled to a specific line in the file that now owns the code. The nine auth/handler pin tests pass individually under `-race`, and the full suite passes in both modules with `-count=1`. The one pre-existing oddity — `tokenMiddleware` having no production caller — was confirmed present at baseline, so it is not this phase's doing.

## Known Stubs

None.

## Next Phase Readiness

- **STR-09 is met and verified.** `beamsync/server.go` is 698 lines of orchestration and nothing else: `PendingTransfer` (`:18`), the `//go:embed ui/*.html ui/*.png` directive (`:29`) with `var uiFS embed.FS` (`:30`), `serverState` (`:32`) with its five methods, `HTTPServer` (`:83`) with its nine methods and its `emit EventCallback` / `uploadDir string` fields, `StartServerE` (`:212`) and `StartSenderE` (`:479`). Every declaration the phase set out to move has left; every reference it needs has stayed.
- **Phase 4 in aggregate.** `server.go` went from **1538 lines to 698** — **−840 lines, −55%** — with the import block shrinking from **22 to 11**. Eight focused source files now carry the extracted concepts: `upload_handler.go` (196), `events.go` (191), `middleware.go` (144), `rate_limiter.go` (118), `download_handler.go` (83), `helpers.go` (50), `progress.go` (45) and `io_helpers.go` (41) — 868 lines in total — plus two new test files (`io_helpers_test.go` at 04-04, `middleware_test.go` at 04-05). No file was deleted. The eleven imports that left `server.go` are `bufio`, `crypto/rand`, `crypto/sha256`, `encoding/hex`, `io`, `math`, `mime`, `mime/multipart`, `net`, `strconv` and `sync/atomic`.
- **Phase 3's safety net held all the way through.** All 115 test functions (83 in `beamsync`, 32 in `desktop`) are still named what the 33-mutant table expects — no `_test.go` file was modified in this plan, and the phase moved tests between files without renaming one. The full `-race` suite is green in both modules.
- **The next plan inherits a clean, conventional tree.** No unused imports, no unused locals, no dangling references, `gofmt -l` silent, `go vet` clean in both modules. Two Go modules with no `go.work`, so any later gate must keep running `go vet ./...` inside `beamsync/` and `go vet .` inside `desktop/` rather than at the repository root.
- **Carried forward, deliberately untouched:** per-server event dispatch (deferred by D-02 — `defaultEventDispatcher` is still a package singleton, now in `events.go`), event-name constants (Phase 5's `SVE-08`), `build/linux/firewall_setup.sh` deletion (`DOC-03`, Phase 8), and the `uploadIntegrityHeader`/`sha256Hex` reserved-but-unused pair (`helpers.go`).

---

*Phase: 04-go-structural-refactor-server-go-extraction*
*Completed: 2026-11-18*

## Self-Check: PASSED

- `beamsync/server.go` is 698 lines; `git status --porcelain` was empty before Task 9.1's commit and that commit (`76828ca`) is empty.
- Declaration sweep, plan's 10-pattern form: no output, exit 1. Extended 30-symbol form: no output, exit 1.
- Retained-surface sweep: 1 match for each of `//go:embed` (`:29`), `var uiFS embed.FS` (`:30`), `type serverState struct` (`:32`), `type HTTPServer struct` (`:83`), `func StartServerE` (`:212`), `func StartSenderE` (`:479`).
- Import block: exactly 11 — `context embed encoding/json fmt net/http os path/filepath runtime/debug strings sync time`; `comm -23` against the 22-entry baseline yields the expected 11 removed.
- `rg -c uiFS beamsync/` → `beamsync/server.go:5` and no other file.
- Route registry: 11 registrations, identical path multiset to baseline `65c6a2d:beamsync/server.go`; `rateLimitMiddleware` 11 → 11, `tokenMiddlewareAny(tokens` 7 → 7 call sites.
- Call-site deltas reconciled: `setCORSHeaders` → `middleware.go:77`, `extractToken` → `middleware.go:33`/`:50`, `clientIP` → `middleware.go:83`.
- No `_test.go` file changed (`git diff --name-only 99be273..HEAD -- '*_test.go'` empty); 115 test functions, unchanged.
- Gate green: `beamsync` build+vet exit 0, `go test -race -count=1 ./...` → `ok beamsync 14.463s`; `desktop` build+vet exit 0, `go test -race -count=1 .` → `ok desktop 1.193s`; `gofmt -l beamsync/server.go` → no output.
- Commit `76828ca` is in the log and is empty by design.
