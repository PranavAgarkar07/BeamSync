---
phase: 04-go-structural-refactor-server-go-extraction
plan: 02
subsystem: infra
tags: [go, refactor, rate-limiter, clock-injection, sync-mutex, package-internal-move]

# Dependency graph
requires:
  - phase: 03-tests-before-refactoring
    provides: clientRateLimiter unit tests (block-after-limit, window reset, oldest-client eviction) and the five TestRateLimitMiddleware* tests that pin mutant #33 (settings provider, not captured pointer)
  - phase: 04-go-structural-refactor-server-go-extraction
    plan: 01
    provides: beamsync/events.go extraction, server.go already reduced to 1356 lines
provides:
  - beamsync/rate_limiter.go — rateLimitState, rateLimitDecision, clientRateLimiter (mu/limit/window/maxClients/clients/now), newClientRateLimiter, allow, prune, evictOldestIfFull
  - beamsync/server.go trimmed 1356 → 1244 lines; limiter declarations removed, only references remain (four *clientRateLimiter fields, setRateLimitHeaders, rateLimitMiddleware signature)
  - the clock-injection seam (now func() time.Time field + now: time.Now constructor default) carried intact into the new file
affects: [04-03, 04-04, 04-05, 04-06, 04-07, 04-08, 04-09]

# Actuals (#2632) — same chars/4 scale as the plan's estimate
actuals:
  tokens: 1350
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Contiguous-block move: the whole limiter (state struct, decision struct, type, constructor, all three methods) moves as one unit, so window arithmetic and the map cannot be split across files (T-04-02-01)"
    - "Byte-identical move proof: diff the source line range against the new file body BEFORE deleting from the origin, then delete — the move is verified as a copy, not asserted"
    - "Guard task with zero file delta: verify the test split by counting test functions per name prefix across the package instead of trusting the file name"

key-files:
  created:
    - beamsync/rate_limiter.go
  modified:
    - beamsync/server.go

key-decisions:
  - "Moved server.go:27-137 as one contiguous block — the two structs, the type, the constructor and all three methods — and proved it byte-identical by diffing the original line range against the new file body before deleting from the origin"
  - "The now func() time.Time field and the now: time.Now constructor default travelled together, preserving the fake-clock seam the limiter tests replace (rate_limiter_test.go:15)"
  - "Reused TestClientRateLimiterBlocksAfterLimit as the tracer happy path instead of adding a duplicate test: it is exactly the path the plan describes (construct a limiter, first client allowed, third exceeds the limit with retryAfter > 0)"
  - "Task 2.2 is a guard with a designed zero file delta, so it was committed with --allow-empty to keep one traceable commit per task without inventing a code change"
  - "No import edits were needed in server.go: sync (serverState.mu, HTTPServer.settingsMu) and time are still used there"

patterns-established:
  - "Diff-before-delete: a pure move is verified as a byte-level copy of the origin range, which is the only mechanical proof available that the arithmetic inside the block did not shift"

requirements-completed: [STR-01]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "clientRateLimiter type, its constructor and its three methods (allow, prune, evictOldestIfFull) — plus rateLimitState and rateLimitDecision — moved into beamsync/rate_limiter.go with bodies unchanged"
    requirement: "STR-01"
    verification:
      - kind: unit
        ref: "beamsync/rate_limiter_test.go#TestClientRateLimiterBlocksAfterLimit"
        status: pass
      - kind: unit
        ref: "beamsync/rate_limiter_test.go#TestClientRateLimiterResetsAfterWindow"
        status: pass
      - kind: unit
        ref: "beamsync/rate_limiter_test.go#TestClientRateLimiterEvictsOldestClientWhenFull"
        status: pass
      - kind: other
        ref: "diff of beamsync/server.go:27-137 (pre-move) against beamsync/rate_limiter.go:8-118 => no differences"
        status: pass
    human_judgment: false
  - id: D2
    description: "server.go trimmed of the limiter: no rateLimitState / rateLimitDecision / clientRateLimiter declarations remain, only same-package references from HTTPServer fields and middleware"
    requirement: "STR-01"
    verification:
      - kind: other
        ref: "grep -rn '^type clientRateLimiter struct|^func (l *clientRateLimiter)' beamsync/ => 7 hits, all beamsync/rate_limiter.go"
        status: pass
      - kind: other
        ref: "(cd beamsync && go build ./... && go vet ./...) && (cd beamsync && go test -race -count=1 ./...)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Rate limiter test split guarded: rate_limiter_test.go holds exactly the three TestClientRateLimiter* functions, the five TestRateLimitMiddleware* stay put for 04-05 Task 5.2, and no test function was renamed"
    requirement: "STR-01"
    verification:
      - kind: other
        ref: "grep -rn '^func TestClientRateLimiter' beamsync/ => 3 hits, all rate_limiter_test.go"
        status: pass
      - kind: other
        ref: "grep -rn '^func TestRateLimitMiddleware' beamsync/ => 5 hits, all rate_limiter_test.go (unmoved)"
        status: pass
      - kind: other
        ref: "(cd desktop && go build . && go vet . && go test -race .)"
        status: pass
    human_judgment: false
  - id: D4
    description: "The limiter's map and counter are safe for concurrent use — confirmed by the race detector, not by reading the fields"
    requirement: "STR-01"
    verification:
      - kind: other
        ref: "(cd beamsync && go test -race -count=1 ./...) => ok beamsync 7.894s"
        status: pass
    human_judgment: false
  - id: D5
    description: "Prohibition: the extraction changed no runtime behavior, no event payload and no HTTP response body"
    verification: []
    human_judgment: true
    rationale: "A pure move cannot be proven behavior-neutral by an automated assertion; a reviewer confirms it from the byte-identical diff and the fact that the four limiter call sites in server.go still compile against the same constructor/signature"

# Metrics
duration: 4min
completed: 2026-11-17
status: complete
---

# Phase 4 Plan 02: Rate Limiter Extraction Summary

**The client rate limiter — its state/decision structs, the mutex-guarded type, its constructor and all three methods — now lives in `beamsync/rate_limiter.go`, a byte-identical copy of the block it replaced, leaving `server.go` 112 lines lighter with the window-boundary arithmetic untouched.**

## Performance

- **Duration:** ~4 min
- **Started:** 2026-11-17T16:52:50Z
- **Completed:** 2026-11-17T16:56:02Z
- **Tasks:** 2
- **Files modified:** 2 (1 created, 1 modified; `rate_limiter_test.go` needed no change — it already sat at the target path)

## Accomplishments

- Created `beamsync/rate_limiter.go` (118 lines) holding `rateLimitState`, `rateLimitDecision`, `clientRateLimiter` (with `mu sync.Mutex`, `limit`, `window`, `maxClients`, `clients map[string]*rateLimitState`, `now func() time.Time`), `newClientRateLimiter` (including the `maxClients: 4096` and `now: time.Now` defaults), `allow`, `prune` and `evictOldestIfFull`.
- Trimmed `beamsync/server.go` from 1356 to 1244 lines (−112). What remains are references, not declarations: the four `*clientRateLimiter` fields on `HTTPServer`, `setRateLimitHeaders(…, decision rateLimitDecision)`, and `rateLimitMiddleware`'s `lim *clientRateLimiter` parameter — all legal same-package references.
- Proved the move is a copy, not a rewrite: `sed -n '27,137p' beamsync/server.go` was diffed against the body of the new file **before** the origin was deleted, and the two were identical. The window-boundary expression `if !ok || now.Sub(state.windowStart) >= l.window` is therefore the same characters as before, as is the empty-key map lookup and the tie-broken oldest-client eviction.
- Guarded the test split (Task 2.2): `beamsync/rate_limiter_test.go` defines exactly the three `TestClientRateLimiter*` functions for the moved type, while all five `TestRateLimitMiddleware*` functions stay where they are for 04-05 Task 5.2. No test function name changed (CONTEXT D-13 — the Phase 3 mutation table keys on them).
- Full gate green in both modules: `beamsync` build + vet, `go test -race -count=1 ./...` → `ok beamsync 7.894s`; `desktop` build + vet + `go test -race .` → `ok desktop 1.173s`; `gofmt -l beamsync/ desktop/` clean.

## Task Commits

Each task was committed atomically:

1. **Task 2.1 (tracer): move the rate limiter into rate_limiter.go** — `24cab26` (refactor)
2. **Task 2.2: guard the rate limiter test split + full gate** — `37c156c` (test; `--allow-empty` — the guard's designed delta is zero)

**Plan metadata:** _(this commit — docs: complete plan)_

## Files Created/Modified

- `beamsync/rate_limiter.go` (created, 118 lines) — `rateLimitState`, `rateLimitDecision`, `clientRateLimiter` + `newClientRateLimiter` + `allow` / `prune` / `evictOldestIfFull`. Imports `sync` and `time` only, one parenthesized block, no file-level doc comment (conventions C1/C2).
- `beamsync/server.go` (modified, −112 lines) — limiter declarations deleted; `PendingTransfer`, `//go:embed ui/*` + `uiFS`, `serverState`, `HTTPServer`, the middleware block and both `Start*E` functions untouched.
- `beamsync/rate_limiter_test.go` (verified, unchanged) — already at its target path with the right eight functions.

## Decisions Made

- **Whole block, one unit.** `server.go:27-137` was moved as a single contiguous range (state struct → decision struct → type → constructor → `allow` → `prune` → `evictOldestIfFull`). Splitting it would have distributed the window arithmetic and the map across files — the exact shape of threat T-04-02-01 (denial of service from limiter state loss during the move).
- **Diff-before-delete.** The moved range was compared against the new file's body with `diff` and came back identical before anything was removed from `server.go`. This converts "I moved it carefully" into a mechanical fact for every reviewer, and it is why the boundary condition, empty-key behaviour and eviction order can be stated as unchanged rather than assumed.
- **Clock injection travels with the type.** The `now func() time.Time` field and the `now: time.Now` constructor default moved together. `rate_limiter_test.go:15` replaces that field with a fake clock, and `TestClientRateLimiterResetsAfterWindow` depends on the window resetting at exactly `windowStart + window`; a stranded default would silently make the tests time-dependent.
- **Tracer happy path reused, not duplicated.** The plan asked for "construct a limiter with a limit and window and assert the first client is allowed and the next exceeds the limit" — that is verbatim `TestClientRateLimiterBlocksAfterLimit`, so it was run explicitly (`go test -race -run TestClientRateLimiterBlocksAfterLimit -v`) rather than copied into a second, weaker test. Same choice 04-01 made for its tracer.
- **`--allow-empty` for the guard task.** Task 2.2 is explicitly a guard ("`rate_limiter_test.go` already sits at the target path, so no file move is needed"). Its delta is zero by design, so an empty commit records the verification outcome and keeps one commit per task without inventing a code change.
- **No import edits in `server.go`.** `sync` (now only `serverState.mu` and `HTTPServer.settingsMu`) and `time` are both still used, so the limiter's departure required no import-block change. The final 11-import shape is still 04-09's job.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- None. The intermediate state is worth naming though: after Task 2.1, `server.go` still *references* `clientRateLimiter`/`rateLimitDecision` (fields, `setRateLimitHeaders`, `rateLimitMiddleware`) while the declarations live in the new file. That is the intended end state of this plan — the middleware function itself moves in 04-05 — and it compiles because both files are in `package beamsync`.
- Not re-run here: `golangci-lint`. 04-01 documented that the locally installed v1.64.8 cannot decode Go 1.27.4 export data and fails the same way on untouched files, so `go build` / `go vet` / `go test -race` remain the authoritative gate.

## User Setup Required

None - no external service configuration required.

## Threat Flags

None — no new network surface, auth path, file-access pattern, or trust-boundary change. The move is package-internal and T-04-02-01 is mitigated as planned: the type, its map, its counters and its window arithmetic moved as one unit, and the suite ran under `-race` afterwards.

## Known Stubs

None.

## Next Phase Readiness

- `beamsync/rate_limiter.go` is the home for anything limiter-shaped. 04-03 (`progress.go`) is next in the risk-ascending order and starts from a 1244-line `server.go`.
- 04-05 Task 5.2 can now move the five `TestRateLimitMiddleware*` functions into a new `middleware_test.go` beside the `middleware.go` extraction — they are untouched and still named as the Phase 3 mutation table expects (mutant #33 in particular).
- `server.go` still owns the four limiter fields and the middleware block; those are 04-05's trim targets, not regressions.

---

*Phase: 04-go-structural-refactor-server-go-extraction*
*Completed: 2026-11-17*

## Self-Check: PASSED

- `beamsync/rate_limiter.go` exists (118 lines) and holds all seven moved declarations.
- `beamsync/server.go` exists (1244 lines) with no limiter declarations remaining.
- Commits `24cab26` and `37c156c` are both present in the log.
