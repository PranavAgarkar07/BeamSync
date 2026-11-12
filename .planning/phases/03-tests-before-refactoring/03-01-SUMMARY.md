---
phase: 03-tests-before-refactoring
plan: 01
subsystem: testing
tags: [go, event-dispatcher, testing, concurrency, race, ci]
requires:
  - phase: 01-foundation-event-system-version
    provides: eventDispatcher implementation
provides:
  - eventDispatcher unit test suite covering normal emit, drop-on-full, empty-buffer fallback, panic recovery, and concurrency exact drop accounting
affects:
  - 04-go-structural-refactor
tech-stack:
  added: []
  patterns:
    - Black-box eventDispatcher assertions via emit return value + DroppedCount() seam (no queue internals access)
    - Blocking-callback primer construction for deterministic drop-on-full tests
    - Channel-with-timeout async waits (never time.Sleep)
actuals:
  tokens: 2900
  tasks: 3
  commits: 3
key-files:
  created:
    - beamsync/event_dispatcher_test.go
  modified:
    - .github/workflows/ci.yml
key-decisions:
  - "D-04: assert drops black-box via emit return value plus DroppedCount() — no queue internals access"
requirements-completed:
  - TST-01
duration: 18min
completed: "2026-11-12"
status: complete
coverage:
  - id: D1
    description: eventDispatcher unit test suite (normal emit, drop-on-full, empty-buffer fallback, panic recovery, concurrency exact drop accounting)
    requirement: TST-01
    verification:
      - kind: unit
        ref: "beamsync/event_dispatcher_test.go#TestEventDispatcherConcurrentEmitsExactDropAccounting"
        status: pass
    human_judgment: false
---

# Phase 3 Plan 1: eventDispatcher Test Suite

**eventDispatcher unit tests landed before the Phase 4 file-splitting refactor**

## Performance

- **Duration:** 18 min
- **Started:** 2026-11-12T11:05:00Z
- **Completed:** 2026-11-12T11:22:46Z
- **Tasks:** 3
- **Files modified:** 2

## Accomplishments

- Added `beamsync/event_dispatcher_test.go` pinning the `eventDispatcher` contract as testable
  behavior before Phase 4 relocates it into `beamsync/events.go`:
  - `TestEventDispatcherDeliversNormalEmit` — single emit delivery (`device_connected|peer-a`),
    FIFO emission ordering, and duplicate emission delivering separately.
  - `TestEventDispatcherEmptyBufferFallsBack` — buffer sizes `0` and `-1` fall back to
    `eventDispatcherBufferSize` (1024); 1024 emits succeed and the 1025th returns false with
    `DroppedCount()` incremented by exactly 1.
  - `TestEventDispatcherDropsFromFullBuffer` — with a provably blocked worker (primer
    construction), capacities 1 and 2 accept exactly `capacity` emits and drop the
    `capacity+1`-th, with the drop counter incremented by exactly 1.
  - `TestEventDispatcherSurvivesCallbackPanic` — dispatcher survives a panicking callback and
    delivers the next event.
  - `TestEventDispatcherConcurrentEmitsExactDropAccounting` — 4 goroutines x 25 emits with a
    bounded wait until delivered + failed equals the total emitted, then asserts the exact drop
    count with no data race.
- Updated `.github/workflows/ci.yml`: `test-go` now runs `(cd beamsync && go test -race ./...)`
  and `(cd desktop && go test -race .)`; the desktop `lint-go` golangci-lint step now uses
  `args: .`; the desktop `build` step now runs `(cd desktop && go build .)`.

## Task Commits

Each task was committed atomically:

1. **Task 1: eventDispatcher normal emit delivers** - `e3560b3` (test)
2. **Task 2: eventDispatcher fallback, drop, and panic tests** - `9acfbcf` (test)
3. **Task 3: eventDispatcher concurrency exact drop accounting** - `afa9742` (test)

**Plan metadata:** final close-out commit `docs(03-01): complete event dispatcher test plan`

## Files Created/Modified

- `beamsync/event_dispatcher_test.go` - New eventDispatcher unit test suite (created)
- `.github/workflows/ci.yml` - CI Go test step now uses `-race`; desktop lint/build scoped to root package (modified)

## Decisions Made

- D-04: assert drop accounting black-box via the `emit` return value plus `DroppedCount()` seam; do not read the `queue` channel internals (they move to `beamsync/events.go` in Phase 4).

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plan 03-02 (progressWriter tests + the live `download_progress` event-name fix) is next in phase 03.
- Phase 4 structural refactor (events extraction) can now consume the dispatcher contract tests as its safety net.

## Self-Check: PASSED

- Created files exist: `beamsync/event_dispatcher_test.go` (FOUND), `.github/workflows/ci.yml` (FOUND).
- Commits exist: `e3560b3` (FOUND), `9acfbcf` (FOUND), `afa9742` (FOUND).
- All task acceptance criteria re-verified: normal emit/FIFO/duplicates, empty fallback, drop
  threshold capacity+1, panic recovery, concurrency exact accounting, CI `-race` updates — all pass.
- Plan-level verification re-run: `(cd beamsync && go test ./...)` pass; `(cd beamsync && go test -race ./...)` pass.

---

*Phase: 03-tests-before-refactoring*
*Completed: 2026-11-12*
