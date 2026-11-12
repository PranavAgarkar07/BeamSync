---
phase: 03-tests-before-refactoring
plan: 02
subsystem: testing
tags: [go, progress-writer, testing, events, download-progress, clock-injection]
requires:
  - phase: 03-tests-before-refactoring
    provides: eventDispatcher unit test suite and the beamsync test conventions established by plan 03-01
provides:
  - progressWriter unit test suite covering per-path event names, throttle coalescing, zero-byte writes, payload format, and the single-writer contract
  - download_progress event-name fix — the send path now emits the event the desktop frontend listens for
  - injectable progressWriter clock (now func() time.Time) with a nil-guard fallback to time.Now
affects:
  - 04-go-structural-refactor
  - desktop/frontend/src/App.svelte
tech-stack:
  added: []
  patterns:
    - Test-first proof of a live bug (RED test committed before the fix) — D-07
    - Injectable clock (now func() time.Time) mirroring clientRateLimiter.now, never real time.Sleep — D-09
    - Per-path event name set as a compile-time literal at each construction site — D-08
    - Test fixture helper that mirrors the production struct literal instead of hand-rolled setup
actuals:
  tokens: 4830
  tasks: 3
  commits: 3
key-files:
  created:
    - beamsync/progress_test.go
  modified:
    - beamsync/server.go
key-decisions:
  - "D-07: the send-path event-name test was committed RED before the fix, so the fix is provably what turned it green"
  - "D-08: the test asserts the event name per path (upload_progress on receive, download_progress on send), not just the payload"
  - "D-09: throttle behavior is split into a minInterval=0 emit-every-write case and an injected-clock coalescing case — no real time.Sleep"
  - "Left lastEmit unset on the receive-path construction site, preserving the existing asymmetry where the first write always emits"
requirements-completed:
  - TST-02
duration: 5min
completed: "2026-11-12"
status: complete
coverage:
  - id: D1
    description: "progressWriter unit test suite — per-path event names, throttle coalescing, zero-byte write, payload format, single-writer contract"
    requirement: TST-02
    verification:
      - kind: unit
        ref: "beamsync/progress_test.go#TestProgressWriterSendPathEmitsDownloadProgress"
        status: pass
      - kind: unit
        ref: "beamsync/progress_test.go#TestProgressWriterReceivePathEmitsUploadProgress"
        status: pass
      - kind: unit
        ref: "beamsync/progress_test.go#TestProgressWriterEmitsEveryWriteWithoutThrottle"
        status: pass
      - kind: unit
        ref: "beamsync/progress_test.go#TestProgressWriterCoalescesWritesWithinInterval"
        status: pass
      - kind: unit
        ref: "beamsync/progress_test.go#TestProgressWriterZeroByteWriteEmitsNothing"
        status: pass
      - kind: unit
        ref: "beamsync/progress_test.go#TestProgressWriterPayloadFormat"
        status: pass
      - kind: unit
        ref: "beamsync/progress_test.go#TestProgressWriterSingleWriterPerTransferPath"
        status: pass
    human_judgment: false
  - id: D2
    description: "download_progress bug fix — the send path emits the event name the desktop send-direction listener subscribes to"
    requirement: TST-02
    verification:
      - kind: unit
        ref: "beamsync/progress_test.go#TestProgressWriterSendPathEmitsDownloadProgress"
        status: pass
      - kind: other
        ref: "grep -c 'eventName:' beamsync/server.go == 3 && grep -c 'progressWriter{' beamsync/server.go == 3"
        status: pass
    human_judgment: false
---

# Phase 3 Plan 2: progressWriter Test Suite + download_progress Fix

**`progressWriter` unit tests plus the live fix that makes the desktop send-direction progress listener fire for the first time**

## Performance

- **Duration:** 5 min
- **Started:** 2026-11-12T11:54:08Z
- **Completed:** 2026-11-12T11:59:10Z
- **Tasks:** 3
- **Files modified:** 2

## Accomplishments

- **Fixed a shipped-app defect (D-07).** `progressWriter.Write` hardcoded `"upload_progress"` for
  every path, so `download_progress` was emitted by nothing in the Go code and the desktop
  send-direction listener (`App.svelte:459`) never fired — send-direction progress was dead in the
  shipped app. `progressWriter` now carries an `eventName` field set per path at all three
  construction sites: the receive path emits `upload_progress`, the single-file and multi-file send
  paths emit `download_progress`.
- **Proved the bug test-first.** The send-path test was committed RED (`8dd231d`) — it asserted
  `download_progress|report.pdf|5|100` and failed with
  `emitted event = "upload_progress|report.pdf|5|100"` — before the fix landed, so the fix is
  provably what turned it green.
- **Added an injectable clock (D-09).** `progressWriter` gained `now func() time.Time`, mirroring the
  existing `clientRateLimiter.now` precedent (`beamsync/server.go:115`). The throttle is tested with
  an advanced fake clock, never `time.Sleep`. `Write` nil-guards the field and falls back to
  `time.Now`, so a struct literal that forgets `now` cannot panic a live transfer.
- **Added `beamsync/progress_test.go`** — 7 test functions, 14 subtests, covering: the per-path
  event name on both paths; the receive path's first-write-always-emits asymmetry (`lastEmit` left
  unset, zero-value `time.Time`); `minInterval = 0` emitting on every write; injected-clock
  coalescing with a re-emit after the window; a zero-byte write emitting nothing and leaving
  `written` unchanged; the `filename|written|total` payload format including byte-not-code-point
  counting (`"héllo"` → 6), `written == total` on completion, exact int64 counts at 4 GiB, and the
  known `|`-in-filename delimiter limitation; and the single-writer contract (one writer per
  transfer path, exact byte accounting, monotonic emitted counts).

## Task Commits

Each task was committed atomically:

1. **Task 1.1: send-path progressWriter emits download_progress** - `8dd231d` (test) — RED by design
2. **Task 2.1: add progressWriter eventName and now fields, wire per-path event names** - `d058ae1` (fix)
3. **Task 2.2: add remaining progressWriter unit tests and go green** - `415c6e2` (test)

**Plan metadata:** final close-out commit `docs(03-02): complete progressWriter test plan`

## Files Created/Modified

- `beamsync/progress_test.go` - New progressWriter unit test suite (created)
- `beamsync/server.go` - `progressWriter` gains `eventName` and `now` fields, `Write` uses the
  injected clock with a nil-guard and the per-path event name, and all three construction sites set
  the per-path event name and clock (modified)
- `.planning/phases/03-tests-before-refactoring/deferred-items.md` - Logged the payload-delimiter
  limitation and the receive-path `total = 0` observation; marked the previously logged
  `desktop/main.go` lint item as resolved (modified)

## Decisions Made

- **D-07 (test-first):** the failing send-path test was committed before the production change, so
  the fix is provably the thing that turned it green.
- **D-08 (assert the event name per path):** the tests assert `upload_progress` on the receive path
  and `download_progress` on the send path. Asserting only the payload would have passed while the
  bug was live.
- **D-09 (split throttle tests, injected clock):** part (a) is the `minInterval = 0`
  emit-every-write case; part (b) is the coalescing case driven by an injected clock. No real
  `time.Sleep` anywhere.
- **Preserved the receive-path asymmetry:** `lastEmit` is left unset at the receive-path
  construction site, so the zero-value `time.Time` keeps the "first write always emits" behavior.
  This is now pinned by a test instead of being an accident.
- **Nil-guard over a constructor:** `progressWriter` has no constructor and is built by struct
  literal at three sites, so `Write` falls back to `time.Now` when `pw.now` is nil rather than
  adding a constructor that the plan's scope guardrails exclude.
- **Fixture helper instead of repeated literals:** `newProgressWriterForTest` mirrors the production
  struct literal once, so each test states only what it varies (event name, filename, total, throttle
  window, clock).

## Deviations from Plan

None - plan executed exactly as written.

The one place the plan and `03-PATTERNS.md` disagreed was the receive-path `lastEmit`. The pattern
map's "becomes" snippet adds `lastEmit: time.Now()` to the receive path; the plan explicitly
requires leaving it unset, and the plan + scope guardrails win. The asymmetry is preserved and
pinned by `TestProgressWriterReceivePathEmitsUploadProgress`.

## Issues Encountered

- The `|`-delimiter limitation subtest initially asserted 4 split parts for the payload
  `upload_progress|a|b.txt|5|100`; the correct count is 5 (the filename contributes the extra
  field). Corrected in the same commit — the assertion now documents that the frontend's
  `[filename, written, total]` destructuring mis-reads the byte counts.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- **Phase 3 is complete:** TST-01 (plan 03-01), TST-02 (this plan), and TST-04 (plan 03-03) are
  all covered. The phase now has a test safety net for `eventDispatcher`, `progressWriter`, and the
  `desktop/app.go` pure functions ahead of the Phase 4 file-splitting refactor.
- **Phase 4 can split `beamsync/server.go` safely:** `progressWriter` now has a pinned event-name
  contract and a tested throttle, and `eventDispatcher` has a black-box contract suite.
- **No blockers.** Two observations were logged to `deferred-items.md` rather than fixed here: the
  `|`-in-filename payload ambiguity (a cross-boundary wire-format change) and the receive path's
  `total = 0` during streaming (cosmetic — the frontend already renders "Unknown" until the final
  emit).

## Self-Check: PASSED

- Created files exist: `beamsync/progress_test.go` (FOUND).
- Modified files exist: `beamsync/server.go` (FOUND).
- Commits exist: `8dd231d` (FOUND), `d058ae1` (FOUND), `415c6e2` (FOUND).
- Task 1.1 RED proof re-verified before the fix: the test failed with
  `emitted event = "upload_progress|report.pdf|5|100"`; after Task 2.1 the same test passes.
- Task 2.1 verify re-run: `(cd beamsync && go test ./...)` pass;
  `grep -c 'eventName:' beamsync/server.go` = 3; `grep -c 'progressWriter{' beamsync/server.go` = 3.
- Task 2.2 acceptance criteria re-verified: receive-path event name + first-write emits;
  `minInterval = 0` emits every write; injected-clock coalescing and re-emit; zero-byte write is a
  no-op; completion payload reports `written == total` — all pass.
- Plan-level verification re-run: `(cd beamsync && go test ./...)` pass; `(cd beamsync && go test
  -race ./...)` pass; `go vet ./...` clean; `golangci-lint run ./...` clean (with
  `GOTOOLCHAIN=go1.25.5`); `gofmt -l beamsync/` clean; `(cd desktop && go test . && go build . &&
  go vet .)` pass.

---
*Phase: 03-tests-before-refactoring*
*Completed: 2026-11-12*
