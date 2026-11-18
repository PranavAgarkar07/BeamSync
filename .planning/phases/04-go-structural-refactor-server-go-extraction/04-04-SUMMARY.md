---
phase: 04-go-structural-refactor-server-go-extraction
plan: 04
subsystem: infra
tags: [go, refactor, io-helpers, chunked-copy, sync-pool, package-internal-move]

# Dependency graph
requires:
  - phase: 03-tests-before-refactoring
    provides: TestCopyChunkedCopiesDataAndReportsCount as the pre-existing pin on the chunked-copy contract, and the 33-mutant table whose test names must not change
  - phase: 04-go-structural-refactor-server-go-extraction
    plan: 03
    provides: beamsync/progress.go extraction, server.go already reduced to 1206 lines with io still imported for copyChunked
provides:
  - beamsync/io_helpers.go — chunkBufferPool (sync.Pool, 8 MiB *[]byte) and its single consumer copyChunked
  - beamsync/io_helpers_test.go — TestCopyChunkedCopiesDataAndReportsCount with empty, shorter-than-chunk and exactly-one-chunk subtests
  - beamsync/server.go trimmed 1206 → 1171 lines; both I/O declarations gone, the io and sync imports still needed and therefore untouched
affects: [04-05, 04-06, 04-07, 04-08, 04-09]

# Actuals (#2632) — same chars/4 scale as the plan's estimate
actuals:
  tokens: 1259
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Diff-before-delete, third repetition (04-02, 04-03, now 04-04): capture both origin ranges to temp files, write the new file, diff each block against its origin, and only then delete — the move is proven a copy, not asserted"
    - "Single-declaration gate for a package singleton: `rg -c \"var chunkBufferPool\" beamsync/*.go` must be exactly 1 hit; PITFALLS 1b's trap is a second declaration, not visibility, so the count is the check"
    - "Function-name fidelity for a test move: the test body is copied verbatim (the function's closing brace merely moves below the new subtests) and the name is untouched, preserving the Phase 3 mutant table keys (D-13)"
    - "Tracer verified end-to-end under auto mode: the happy path was re-run after the move (`-run TestCopyChunkedCopiesDataAndReportsCount -v`) and the suite was green before the expansion task began"

key-files:
  created:
    - beamsync/io_helpers.go
    - beamsync/io_helpers_test.go
  modified:
    - beamsync/server.go
    - beamsync/server_test.go

key-decisions:
  - "Moved the pool and the copy loop as two byte-identical blocks in one file — `sync.Pool.New` returning an 8 MiB *[]byte stays next to the only code that Gets and Puts it, so the acquire/release pairing cannot drift apart (T-04-04-01)"
  - "File layout is `package beamsync` + one parenthesized stdlib block (`io`, `sync`) + the pool + copyChunked; no file-level doc comment and no doc comment on either unexported symbol, matching conventions C1/C2/C3 and the port_manager.go file shape"
  - "No import edits in server.go: io is still used by the upload body (io.EOF / io.Copy(io.Discard, part) / io.Copy) until 04-07 moves it, and sync is still used by serverState.mu and the HTTPServer mutexes — removing either would have broken the build and forced it back in a later plan"
  - "Kept the original 9216-byte / chunkSize 17 assertions verbatim at the top of the moved test rather than wrapping them in a fourth subtest; the diff-before-delete discipline says the existing body is copied, not restructured, and the plan only asked for three additions"
  - "Subtest names are exactly `empty`, `shorter-than-chunk`, `exactly-one-chunk` as the plan specifies, so the must_have truths are greppable in the suite rather than implied"
  - "The `go test -race` gate was run with `-count=1` so the pass is a real execution rather than a cached result"

patterns-established:
  - "When a package-level var and its single consumer move together, prove each with its own diff against its own origin range — a combined diff would hide a swapped or dropped line between two non-adjacent blocks"

requirements-completed: [STR-04]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "chunkBufferPool and copyChunked moved into beamsync/io_helpers.go as byte-identical copies of their server.go ranges, with exactly one pool declaration left in the package"
    requirement: "STR-04"
    verification:
      - kind: other
        ref: "diff of beamsync/server.go:42-47 (pre-move) against beamsync/io_helpers.go:8-13 => no differences"
        status: pass
      - kind: other
        ref: "diff of beamsync/server.go:220-246 (pre-move) against beamsync/io_helpers.go:15-41 => no differences"
        status: pass
      - kind: other
        ref: "rg -c 'var chunkBufferPool' beamsync/*.go => beamsync/io_helpers.go:1"
        status: pass
    human_judgment: false
  - id: D2
    description: "server.go no longer holds the pool or the helper — only same-package references remain (none for the pool; copyChunked call sites in the receive and both send paths)"
    requirement: "STR-04"
    verification:
      - kind: other
        ref: "rg -n '^func copyChunked|var chunkBufferPool' beamsync/ => both hits in io_helpers.go only; server.go 1206 -> 1171 lines"
        status: pass
      - kind: other
        ref: "(cd beamsync && go build ./... && go vet ./...) && (cd beamsync && go test -race -count=1 ./...) => ok beamsync 14.648s"
        status: pass
    human_judgment: false
  - id: D3
    description: "copyChunked still copies zero bytes and reports zero on an empty reader — must_have truth 1"
    requirement: "STR-04"
    verification:
      - kind: unit
        ref: "beamsync/io_helpers_test.go#TestCopyChunkedCopiesDataAndReportsCount/empty"
        status: pass
    human_judgment: false
  - id: D4
    description: "copyChunked still treats a source shorter than the chunk size and a source exactly one chunk long identically (both copied whole with n == len(payload)) — must_have truth 2"
    requirement: "STR-04"
    verification:
      - kind: unit
        ref: "beamsync/io_helpers_test.go#TestCopyChunkedCopiesDataAndReportsCount/shorter-than-chunk"
        status: pass
      - kind: unit
        ref: "beamsync/io_helpers_test.go#TestCopyChunkedCopiesDataAndReportsCount/exactly-one-chunk"
        status: pass
    human_judgment: false
  - id: D5
    description: "Test function name unchanged across the move, so the Phase 3 33-mutant table still keys on it (D-13)"
    requirement: "STR-04"
    verification:
      - kind: other
        ref: "git show 9b23a41 -- beamsync/io_helpers_test.go beamsync/server_test.go => only one definition of TestCopyChunkedCopiesDataAndReportsCount, now in io_helpers_test.go"
        status: pass
    human_judgment: false
  - id: D6
    description: "Prohibition: the extraction changed no runtime behavior, no event payload and no HTTP response body"
    verification: []
    human_judgment: true
    rationale: "A pure move is proven by the two byte-identical diffs plus the full -race suite in both modules; a reviewer confirms no behavior surface was touched from the diffstat (41 added lines in io_helpers.go, 35 removed from server.go, no other production file edited)"

# Metrics
duration: 3min
completed: 2026-11-18
status: complete
---

# Phase 4 Plan 04: `io_helpers.go` Extraction Summary

**`chunkBufferPool` and its only consumer `copyChunked` now live in `beamsync/io_helpers.go` as byte-identical copies of the blocks they replaced, leaving `server.go` at 1171 lines with exactly one pool declaration in the package and the `io` import still in place for the upload body that 04-07 will move.**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-11-18T12:22:44Z
- **Completed:** 2026-11-18T12:25:15Z
- **Tasks:** 2
- **Files modified:** 4 (2 created, 2 modified)

## Accomplishments

- Created `beamsync/io_helpers.go` (41 lines) holding the `sync.Pool` whose `New` returns an 8 MiB `*[]byte`, followed by `copyChunked`. Imports `io` and `sync` in one parenthesized stdlib block; no file-level doc comment; no doc comment on either unexported symbol — the `port_manager.go` file shape.
- Trimmed `beamsync/server.go` from 1206 to 1171 lines (−35). Both I/O declarations are gone; what remains are the `copyChunked` call sites in the receive and send paths, which are legal same-package references.
- Proved both moves are copies, not rewrites, by the 04-02/04-03 diff-before-delete discipline: `server.go:42-47` (pool) and `server.go:220-246` (copy loop) were captured to temp files, diffed against `io_helpers.go:8-13` and `:15-41` — `POOL IDENTICAL` / `COPY IDENTICAL` — and only then deleted from the origin.
- PITFALLS 1b gate holds: `rg -c "var chunkBufferPool" beamsync/*.go` returns exactly `beamsync/io_helpers.go:1`. The pool travelled with its only consumer as one unit, so there is no stranded declaration and no second 8 MiB pool.
- Created `beamsync/io_helpers_test.go` (70 lines) carrying `TestCopyChunkedCopiesDataAndReportsCount` onto the file that matches convention C8 — the `io_helpers_test.go` name ROADMAP Phase 3 planned but never created. The original 9216-byte / `chunkSize 17` assertions are preserved verbatim; only the function's closing brace moved below the new subtests.
- Added the three subtests the plan names: `empty` (zero-length reader → `n == 0`, `err == nil`, empty destination), `shorter-than-chunk` (`"beam"` with `chunkSize 17` copied whole), `exactly-one-chunk` (17 bytes with `chunkSize 17` → `n == len(payload)`). Both must_have truths are now pinned by the suite rather than by inspection, and all three pass under `-race`.
- Full gate green: `beamsync` build + vet, `go test -race -count=1 ./...` → `ok beamsync 14.648s`; `desktop` build + vet + `go test -race -count=1 .` → `ok desktop 1.194s`; `gofmt -l beamsync/ desktop/` clean.
- Tracer feedback gate (auto mode): the happy path was re-run from its new home before the expansion task — `go test -race -run TestCopyChunkedCopiesDataAndReportsCount -v` → PASS — so the extraction layer was proven before the test file was split.

## Task Commits

Each task was committed atomically:

1. **Task 4.1 (tracer): move the chunk buffer pool and copyChunked into io_helpers.go** — `28370c3` (refactor)
2. **Task 4.2: move the copyChunked test into io_helpers_test.go and run the full gate** — `9b23a41` (test)

**Plan metadata:** _(this commit — docs: complete plan)_

## Files Created/Modified

- `beamsync/io_helpers.go` (created, 41 lines) — `chunkBufferPool` at 8-13 and `copyChunked` at 15-41, both byte-identical to the `server.go` ranges they replace. Imports `io`, `sync`.
- `beamsync/server.go` (modified, −35 lines) — pool and `copyChunked` declarations deleted; the `io` and `sync` imports deliberately untouched; every other block (`PendingTransfer`, `//go:embed ui/*` + `uiFS`, `serverState`, `HTTPServer`, the reserved-integrity helpers, the middleware block, both `Start*E` functions) untouched.
- `beamsync/io_helpers_test.go` (created, 70 lines) — `TestCopyChunkedCopiesDataAndReportsCount` plus its three subtests. Imports `bytes`, `strings`, `testing`.
- `beamsync/server_test.go` (modified, −16 lines) — the `copyChunked` test function removed; `bytes` and `strings` are still used elsewhere in the file (3 and 8 references), so its import block needed no edit.

## Decisions Made

- **Two blocks, two diffs.** The pool and the copy loop are non-adjacent in `server.go`, so each was captured and diffed separately against its own range in the new file. A single combined diff would have been blind to a line swapped between them.
- **Pool first, function second.** The plan phrase is "the `chunkBufferPool` declaration plus the `copyChunked` function … as one unit", so the file opens with the pool it is about, then the consumer. The declaration order inside the file is cosmetic; the one-declaration invariant is what carries the risk.
- **No import edits.** `server.go` still uses `io` (the `/upload` body's `io.EOF` / `io.Copy(io.Discard, part)` until 04-07) and `sync` (`serverState.mu`, `HTTPServer`'s mutexes). The plan's repo note warned about exactly this; leaving the import block alone kept the diff to the two removed blocks and avoided a build break that would have had to be reverted two plans later.
- **Original test body kept verbatim.** Rather than converting the 9216-byte assertion into a `multi-chunk` subtest, it stays as the function's opening body and the three required subtests follow it. That keeps the moved characters identical to the origin and matches the minimal-change posture of the phase.
- **Subtest names taken literally from the plan** (`empty`, `shorter-than-chunk`, `exactly-one-chunk`) even though the surrounding repo style uses sentence-like names — the plan's acceptance criteria name these three strings, and greppability is the point.
- **`-count=1` on every test run** so neither the moved code nor the new subtests can pass from cache.

## Deviations from Plan

None - plan executed exactly as written.

The only non-conformance is environmental and pre-existing, unchanged from 04-01 through 04-03: `golangci-lint` was not run because the locally installed v1.64.8 cannot decode Go 1.27.4 export data (`export data version 5`) and fails identically on untouched files. `go build`, `go vet`, `gofmt -l` and `go test -race` remain the authoritative gate. No lint-driven change was made.

## Issues Encountered

- None. The intermediate state is the intended one: after Task 4.1, `server.go` still *calls* `copyChunked` from the receive and send paths while the declaration lives in `io_helpers.go`; that is legal same-package Go and is exactly what 04-07/04-08 will relocate when the handler bodies move.
- The first diff attempt used hand-guessed line offsets and reported a spurious mismatch; re-deriving the ranges from the written file (`grep -n`) produced `POOL IDENTICAL` / `COPY IDENTICAL`. Worth recording because a wrong range can look like a real difference.

## User Setup Required

None - no external service configuration required.

## Threat Flags

None — no new network surface, auth path, file-access pattern, or trust-boundary change. The move is package-internal and touches no exported symbol.

T-04-04-01 (denial of service through pooled-buffer reuse during chunked copy) is mitigated as planned: the pool moved with its single consumer, the `chunkBufferPool.Get()` / `defer chunkBufferPool.Put(bufPtr)` pairing is byte-identical to its previous form, the PITFALLS 1b single-declaration gate confirms no second pool exists, and the whole suite — including the handler-level wiring tests that exercise the 8 MiB pool path — runs under `-race`.

## Known Stubs

None.

## Next Phase Readiness

- `beamsync/io_helpers.go` is now the home for chunked I/O. 04-05 (`middleware.go`) is next in the risk-ascending order and starts from a 1171-line `server.go`; it moves the contiguous middleware block (`extractToken` through `clientIP`), whose `rateLimitMiddleware` settings-provider signature must not change (mutant #33).
- 04-07 and 04-08 will find `copyChunked` callable from either new handler file without any import work — it is an unexported same-package function, and the `io` import in `server.go` still exists only because the upload body has not moved yet.
- The container path (`chunkSize == 8*1024*1024` → pooled buffer; anything else → `make`) is unchanged, so 04-07's upload body keeps its pooled 8 MiB reads and 04-08's download body keeps its pooled 8 MiB writes with no adjustment.

---

*Phase: 04-go-structural-refactor-server-go-extraction*
*Completed: 2026-11-18*

## Self-Check: PASSED

- `beamsync/io_helpers.go` exists (41 lines) and holds `chunkBufferPool` and `copyChunked`.
- `beamsync/io_helpers_test.go` exists (70 lines) and defines `TestCopyChunkedCopiesDataAndReportsCount` with the three named subtests.
- `beamsync/server.go` exists (1171 lines) with neither declaration remaining.
- `rg -c "var chunkBufferPool" beamsync/*.go` → exactly 1 hit, in `beamsync/io_helpers.go`.
- Commits `28370c3` and `9b23a41` are both present in the log.
