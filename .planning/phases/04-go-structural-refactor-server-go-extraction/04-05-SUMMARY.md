---
phase: 04-go-structural-refactor-server-go-extraction
plan: 05
subsystem: infra
tags: [go, refactor, middleware, auth-chain, rate-limiting, settings-provider]

# Dependency graph
requires:
  - phase: 03-tests-before-refactoring
    provides: TestRateLimitMiddlewareReadsCurrentSettings as the pin on the settings-provider contract (mutant #33) and TestTokenMiddlewareRejectsMissingOrInvalidToken as the pin on the empty-token 403 path, plus the 33-mutant table whose test names must not change
  - phase: 04-go-structural-refactor-server-go-extraction
    plan: 04
    provides: beamsync/io_helpers.go extraction, server.go already reduced to 1171 lines with math, strconv and net still imported only for the middleware block
provides:
  - beamsync/middleware.go — extractToken, tokenMiddleware, tokenMiddlewareAny, setRateLimitHeaders, rateLimitMiddleware, setCORSHeaders, clientIP
  - beamsync/middleware_test.go — the five TestRateLimitMiddleware* functions
  - beamsync/server.go trimmed 1171 → 1036 lines, its diff purely deletions and its import block now free of math, strconv and net
affects: [04-06, 04-07, 04-08, 04-09]

# Actuals (#2632) — same chars/4 scale as the plan's estimate
actuals:
  tokens: 6096
  tasks: 3
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Diff-before-delete, fourth repetition (04-02, 04-03, 04-04, now 04-05): the origin range is captured to a temp file, the new file is composed as header + captured block, each block is diffed against its origin, and only then is the origin deleted — the move is proven a copy, not asserted"
    - "Deletion-only diff as the route-order proof: `git diff HEAD~2 HEAD -- beamsync/server.go | grep '^+[^+]'` returns nothing, so not a single line of server.go was added — the 11 rateLimitMiddleware and 7 tokenMiddleware/tokenMiddlewareAny call sites cannot have been reordered (T-04-05-01)"
    - "Import pruning derived from the moved block, not guessed: `grep -n '\\bmath\\.\\|\\bstrconv\\.\\|\\bnet\\.'` on the trimmed server.go returns zero hits before the import edit, and `json`/`strings` were kept because they still have 3 and 11 uses in the upload/download bodies"
    - "Tracer verified end-to-end under auto mode: the empty-token 403 path was re-run after the move (`-run 'TestTokenMiddlewareRejectsMissingOrInvalidToken|TestRateLimitMiddleware' -v`) and the full suite was green after the task commit, before the test file was split"

key-files:
  created:
    - beamsync/middleware.go
    - beamsync/middleware_test.go
  modified:
    - beamsync/server.go
    - beamsync/rate_limiter_test.go

key-decisions:
  - "Moved the seven declarations as one contiguous block (server.go:231-361) exactly as PATTERNS.md maps them, keeping the in-file order extractToken → tokenMiddleware → tokenMiddlewareAny → setRateLimitHeaders → rateLimitMiddleware → setCORSHeaders → clientIP so the chain reads top-down in the order a request traverses it"
  - "Import block is the eight stdlib packages PATTERNS.md specifies (encoding/json, fmt, math, net, net/http, strconv, strings, time) with no inline comments — the repo has zero inline comments on imports in any existing file, and `gofmt`/vet are clean without them"
  - "Deleted math, strconv and net from server.go in the same commit as the move rather than deferring: their last remaining uses were inside the moved block (math.Ceil, strconv.Itoa/FormatInt, net.ParseIP/SplitHostPort), so the deletion is forced by Go's unused-import rule and deferring it would have left a file that cannot build"
  - "Kept encoding/json and strings in server.go — json still has 3 uses (the settings decode at :517, the pending-transfer marshal at :576, the manifest decode at :655) and strings 11, so removing them would have broken 04-07/04-08's source file two plans early"
  - "Did not edit a single route registration: the middleware ORDER must not change (T-04-05-01), and the only way to guarantee that is to never touch the call sites — proven by server.go's diff containing zero added lines"
  - "The five TestRateLimitMiddleware* functions were copied as the single contiguous tail of rate_limiter_test.go (:74-270, including the doc comment above TestRateLimitMiddlewareReadsCurrentSettings), then the origin was pruned to :12-72; both halves diffed identical against their origins"
  - "Task 5.3 produced no diff — it is the verification-only task — so it has no commit of its own; its gate output is recorded here instead, and the plan metadata commit is the third commit in the sequence"
  - "The `go test -race` gate was run with `-count=1` so the pass is a real execution rather than a cached result"

patterns-established:
  - "When a deletion is the whole point, assert it: `grep '^+[^+]'` on the trimmed file's diff proves zero additions, which is a stronger statement about untouched call sites than a review of the removed lines"

requirements-completed: [STR-05]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "The seven middleware declarations moved into beamsync/middleware.go as byte-identical copies of server.go:231-361"
    requirement: "STR-05"
    verification:
      - kind: other
        ref: "diff of beamsync/server.go:231-361 (pre-move) against beamsync/middleware.go:14-144 => no differences"
        status: pass
      - kind: other
        ref: "grep -n '^func ' beamsync/middleware.go => exactly the seven functions, in the original order"
        status: pass
    human_judgment: false
  - id: D2
    description: "server.go no longer holds any middleware declaration and no longer imports math, strconv or net; its change is deletions only"
    requirement: "STR-05"
    verification:
      - kind: other
        ref: "grep -n '^func \\(extractToken\\|tokenMiddleware\\|tokenMiddlewareAny\\|setRateLimitHeaders\\|rateLimitMiddleware\\|setCORSHeaders\\|clientIP\\)' beamsync/server.go => no matches"
        status: pass
      - kind: other
        ref: "git diff HEAD~2 HEAD -- beamsync/server.go | grep '^+[^+]' => empty; 135 lines removed, 0 added; 1171 -> 1036 lines"
        status: pass
    human_judgment: false
  - id: D3
    description: "A request with no token is still rejected with 403 — must_have truth 1"
    requirement: "STR-05"
    verification:
      - kind: unit
        ref: "beamsync/server_test.go#TestTokenMiddlewareRejectsMissingOrInvalidToken"
        status: pass
    human_judgment: false
  - id: D4
    description: "The per-route middleware ORDER is unchanged: rate limiting still wraps token validation — must_have truth 2 (T-04-05-01)"
    requirement: "STR-05"
    verification:
      - kind: other
        ref: "server.go's plan diff is deletion-only, so none of the 11 rateLimitMiddleware and 7 tokenMiddleware/tokenMiddlewareAny call sites was edited or reordered"
        status: pass
      - kind: unit
        ref: "beamsync/server_test.go#TestTokenMiddlewareRejectsMissingOrInvalidToken still exercises the token layer directly and the handler-level wiring tests exercise the wrapper order"
        status: pass
    human_judgment: false
  - id: D5
    description: "The rateLimitMiddleware settings-provider signature is preserved, so the provider is still read per request (mutant #33)"
    requirement: "STR-05"
    verification:
      - kind: unit
        ref: "beamsync/middleware_test.go#TestRateLimitMiddlewareReadsCurrentSettings"
        status: pass
      - kind: other
        ref: "grep -n 'func rateLimitMiddleware' beamsync/middleware.go => (lim *clientRateLimiter, settings func() *TransferSettings, next http.HandlerFunc)"
        status: pass
    human_judgment: false
  - id: D6
    description: "middleware_test.go holds exactly the five TestRateLimitMiddleware* functions and exactly the six planned imports; rate_limiter_test.go holds exactly the three TestClientRateLimiter* functions and two imports"
    requirement: "STR-05"
    verification:
      - kind: other
        ref: "grep -c 'func TestRateLimitMiddleware' beamsync/*.go => 5, all in beamsync/middleware_test.go; sed -n '1,11p' shows encoding/json, net/http, net/http/httptest, strconv, testing, time"
        status: pass
      - kind: other
        ref: "grep -n '^func Test' beamsync/rate_limiter_test.go => the three TestClientRateLimiter* tests; its import block is testing + time"
        status: pass
    human_judgment: false
  - id: D7
    description: "No test function was renamed (CONTEXT D-13), so the Phase 3 33-mutant table still keys on every name"
    requirement: "STR-05"
    verification:
      - kind: other
        ref: "both moved blocks diff identical against their origin ranges — a rename would have shown up in the diff"
        status: pass
    human_judgment: false
  - id: D8
    description: "Full Phase 4 gate green: beamsync build + vet + test -race, desktop build + vet + test -race"
    requirement: "STR-05"
    verification:
      - kind: other
        ref: "(cd beamsync && go build ./... && go vet ./...) && (cd beamsync && go test -race -count=1 ./...) => ok beamsync 8.327s"
        status: pass
      - kind: other
        ref: "(cd desktop && go build . && go vet . && go test -race -count=1 .) => ok desktop 1.185s; gofmt -l beamsync/ desktop/ clean"
        status: pass
    human_judgment: false
  - id: D9
    description: "Prohibition: the extraction changed no runtime behavior, no event payload and no HTTP response body"
    verification: []
    human_judgment: true
    rationale: "A pure move is proven by the two byte-identical diffs plus the deletion-only server.go diff plus the full -race suite in both modules; a reviewer confirms from the diffstat (144 added in middleware.go, 135 removed from server.go, no other production file edited) that no behavior surface was touched"

# Metrics
duration: 3min
completed: 2026-11-18
status: complete
---

# Phase 4 Plan 05: `middleware.go` Extraction Summary

**The seven-symbol HTTP middleware chain now lives in `beamsync/middleware.go` as a byte-identical copy of `server.go:231-361`, leaving `server.go` at 1036 lines with a purely deletion-only diff, `math`/`strconv`/`net` pruned from its imports, and the settings-provider signature — mutant #33's target — untouched at line 75 of the new file.**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-11-18T12:27:47Z
- **Completed:** 2026-11-18T12:30:04Z
- **Tasks:** 3 (2 committed + Task 5.3, verification-only)
- **Files modified:** 4 (2 created, 2 modified)

## Accomplishments

- Created `beamsync/middleware.go` (144 lines): `extractToken` (14), `tokenMiddleware` (27), `tokenMiddlewareAny` (44), `setRateLimitHeaders` (61), `rateLimitMiddleware` (75), `setCORSHeaders` (116), `clientIP` (122). One parenthesized stdlib import block of the eight packages PATTERNS.md specifies, no file-level doc comment — the `auth_tokens.go` file shape its analog calls for.
- Proved the move is a copy: `server.go:231-361` was captured to a temp file, `middleware.go:14-144` was diffed against it, and `MIDDLEWARE BLOCK IDENTICAL` printed before the origin was deleted. Fourth run of the 04-02/04-03/04-04 discipline.
- Trimmed `beamsync/server.go` from 1171 to 1036 lines (−135) and pruned `math`, `strconv` and `net` from its import block. Those three deletions are forced, not cosmetic: `grep -n '\bmath\.\|\bstrconv\.\|\bnet\.'` returns zero hits once the block is gone, and Go rejects an unused import.
- Kept `encoding/json` and `strings` in `server.go` deliberately — they are still live in the upload/download bodies (3 and 11 uses) that 04-07/04-08 relocate, so pruning them would have broken the build.
- Created `beamsync/middleware_test.go` (208 lines) carrying all five `TestRateLimitMiddleware*` functions with the six imports they need. The contiguous tail of `rate_limiter_test.go` (`:74-270`, including the doc comment above `TestRateLimitMiddlewareReadsCurrentSettings`) was copied verbatim and diffed `MW TEST BLOCK IDENTICAL`.
- Pruned `beamsync/rate_limiter_test.go` from 270 to 68 lines: the three `TestClientRateLimiter*` tests remain with `testing` + `time` only. No test function was renamed (D-13), so every key in Phase 3's 33-mutant table still resolves.
- T-04-05-01 (elevation of privilege through a changed auth-chain order) is mitigated by construction and then verified: `git diff HEAD~2 HEAD -- beamsync/server.go` contains **zero added lines**, so none of the 11 `rateLimitMiddleware` and 7 `tokenMiddleware`/`tokenMiddlewareAny` call sites in `StartServerE`/`StartSenderE` was edited — the chain still reads rate-limit → token → handler.
- Full gate green in both modules: `beamsync` build + vet, `go test -race -count=1 ./...` → `ok beamsync 8.327s`; `desktop` build + vet + `go test -race -count=1 .` → `ok desktop 1.185s`; `gofmt -l beamsync/ desktop/` clean.
- Tracer feedback gate (auto mode): the empty-token 403 path and all five middleware tests were re-run after the task commit — `TestTokenMiddlewareRejectsMissingOrInvalidToken` printed both rejections and passed — before the test file was split.

## Task Commits

Each task was committed atomically:

1. **Task 5.1 (tracer): move the middleware chain into middleware.go** — `5777d98` (refactor)
2. **Task 5.2: move the rate-limit middleware tests into middleware_test.go** — `319a6d5` (test)

**Task 5.3** (full verification gate) is verification-only and produced no working-tree change, so it has no commit of its own; its output is recorded above.

**Plan metadata:** _(this commit — docs: complete plan)_

## Files Created/Modified

- `beamsync/middleware.go` (created, 144 lines) — the seven middleware declarations, byte-identical to the `server.go` range they replace. Imports `encoding/json`, `fmt`, `math`, `net`, `net/http`, `strconv`, `strings`, `time`.
- `beamsync/server.go` (modified, −135 lines, 1171 → 1036) — the middleware block deleted; `math`, `strconv`, `net` removed from the import block; nothing added, nothing else touched (`PendingTransfer`, `//go:embed ui/*` + `uiFS`, `serverState`, `HTTPServer`, `generateID`, the reserved-integrity helpers, `autoRenamePath`, `getLocalIPForURL`, both `Start*E` functions with every route registration intact).
- `beamsync/middleware_test.go` (created, 208 lines) — `TestRateLimitMiddlewareReturns429`, `TestRateLimitMiddlewareUsesXForwardedFor`, `TestRateLimitMiddlewareBypassesTrustedDevice`, `TestRateLimitMiddlewareRoundsRetryAfterUp`, `TestRateLimitMiddlewareReadsCurrentSettings`. Imports `encoding/json`, `net/http`, `net/http/httptest`, `strconv`, `testing`, `time`.
- `beamsync/rate_limiter_test.go` (modified, 270 → 68 lines) — the three `TestClientRateLimiter*` tests; import block pruned from six packages to `testing` + `time`.

## Decisions Made

- **One contiguous block, original order.** The seven declarations are adjacent in `server.go` and are moved as one unit in the order a request traverses them, so `middleware.go` reads top-down as the chain rather than as a symbol dump.
- **No inline comments on imports.** PATTERNS.md's import block carries per-line comments, but every existing file in `beamsync/` has none; the block is written plain and `gofmt` is clean. The comments' content (which symbol uses which package) is documented here instead.
- **Prune the imports in the same commit as the move.** Deferring the `math`/`strconv`/`net` removal to a later task would leave a `server.go` that does not compile between commits, which contradicts "each extraction is its own commit so a revert is surgical" (D-09).
- **Leave route registrations untouched.** The plan forbids changing the per-route chain order; the strongest available evidence is not a review of the call sites but a diff with no additions, which is why the zero-added-lines check is recorded as a coverage item.
- **Task 5.3 gets no commit.** It is the gate, not a change. Inventing an empty commit for it would have made the plan's commit list look uniform at the cost of truth.

## Deviations from Plan

None - plan executed exactly as written.

The only non-conformance is environmental and pre-existing, unchanged from 04-01 through 04-04: `golangci-lint` was not run because the locally installed v1.64.8 cannot decode Go 1.27.4 export data (`export data version 5`) and fails identically on untouched files. `go build`, `go vet`, `gofmt -l` and `go test -race` remain the authoritative gate. No lint-driven change was made.

## Issues Encountered

- The first attempt to diff the moved test block used a 197-line range when the file is 208 lines (the 11-line header plus the 197-line block), so `diff` reported a spurious trailing mismatch. Re-deriving the range from `wc -l` produced `MW TEST BLOCK IDENTICAL`. Same failure mode 04-04 recorded: a hand-guessed range looks like a real difference. Worth noting because both times the code was already correct.

## User Setup Required

None - no external service configuration required.

## Threat Flags

None — no new network surface, auth path, file-access pattern, or trust-boundary change. The move is package-internal and touches no exported symbol; `desktop/app.go`'s `beamsync` surface is unaffected.

T-04-05-01 (elevation of privilege via a changed auth middleware order) is mitigated as planned: the plan never edits route registration, the `server.go` diff is deletion-only (0 additions against 135 removals), `rateLimitMiddleware` keeps `settings func() *TransferSettings` at `middleware.go:75` and still reads the provider per request, and the whole suite — including `TestTokenMiddlewareRejectsMissingOrInvalidToken` (403 on empty token) and `TestRateLimitMiddlewareReadsCurrentSettings` (mutant #33) — runs under `-race` in both modules.

## Known Stubs

None.

## Next Phase Readiness

- `beamsync/middleware.go` is now the home for the HTTP chain. 04-06 (`helpers.go`) is next and starts from a 1036-line `server.go`; it moves `generateID`, `sha256Hex`, `uploadIntegrityHeader`, the two `var _ =` blank assignments and `autoRenamePath`, while `//go:embed ui/*` + `uiFS` stay put (D-06).
- 04-07 and 04-08 will register `httpServer.handleUpload` / `httpServer.handleDownload` inside the existing `rateLimitMiddleware(…, tokenMiddleware…(…))` wrappers without touching `middleware.go` — the wrappers are unchanged and still callable from `server.go`.
- `encoding/json` and `strings` remain imported in `server.go` solely because the upload and download bodies are still there; both imports follow those bodies out in 04-07/04-08, which will be the last chance to prune them.

---

*Phase: 04-go-structural-refactor-server-go-extraction*
*Completed: 2026-11-18*

## Self-Check: PASSED

- `beamsync/middleware.go` exists (144 lines) and defines exactly the seven middleware functions.
- `beamsync/middleware_test.go` exists (208 lines) and defines the five `TestRateLimitMiddleware*` functions.
- `beamsync/rate_limiter_test.go` exists (68 lines) with the three `TestClientRateLimiter*` tests and a two-import block.
- `beamsync/server.go` exists (1036 lines) with no middleware declaration and no `math`/`strconv`/`net` import.
- `grep -c 'func TestRateLimitMiddleware' beamsync/*.go` → exactly 5 hits, all in `beamsync/middleware_test.go`.
- Commits `5777d98` and `319a6d5` are both present in the log.
