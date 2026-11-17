---
phase: 04-go-structural-refactor-server-go-extraction
plan: 01
subsystem: infra
tags: [go, refactor, event-dispatcher, goroutine, sync-mutex, package-internal-move]

# Dependency graph
requires:
  - phase: 03-tests-before-refactoring
    provides: eventDispatcher test suite (emit/drop/panic/close-race) and the 33-mutant table that pins close() idempotence and the stateMu pairing
provides:
  - beamsync/events.go — EventCallback, eventDispatcherBufferSize, eventDispatchJob, eventDispatcher (+ methods), defaultEventDispatcher, startWatchdog, startBootstrapRotator, safeEmit, logTransfer
  - beamsync/server.go trimmed 1538 → 1356 lines, event subsystem removed, sync/atomic import dropped
  - defaultEventDispatcher now has exactly one declaration and one usage, both in events.go
  - the event dispatcher's test file owns its shutdown test, so events.go and its test pair 1:1
affects: [04-02, 04-03, 04-04, 04-05, 04-06, 04-07, 04-08, 04-09, phase-05-svelte-component-split]

# Actuals (#2632) — same chars/4 scale as the plan's estimate
actuals:
  tokens: 3674
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Package-internal extraction: move a whole type together with its package singleton, then run build+vet+race before touching the next concept"
    - "Singleton-with-its-type (PITFALLS 1a): defaultEventDispatcher sits in the same block as eventDispatcher so no second dispatcher can be created by a stranded declaration"
    - "Risk-ascending extraction with a per-step gate (CONTEXT D-07/D-08) — the Phase 3 safety net is used after every move, not once at the end"

key-files:
  created:
    - beamsync/events.go
  modified:
    - beamsync/server.go
    - beamsync/event_dispatcher_test.go
    - beamsync/progress_test.go

key-decisions:
  - "defaultEventDispatcher stays a package global (CONTEXT D-02) — per-server dispatch is a behavior change, not a move, and is deferred"
  - "The singleton was placed between the type and its constructor so declaration and type cannot drift apart"
  - "PendingTransfer stayed in server.go — it is orchestration state for HTTPServer.pendingTransfers, and plan 04-09 pins that home"
  - "TestEventDispatcherCloseStopsWorkerAndIsIdempotent moved without renaming (CONTEXT D-13 — the Phase 3 mutation table keys on test function names)"

patterns-established:
  - "Extracted file shape: package clause, one stdlib-only parenthesized import block, no file-level doc comment, doc comments only where they record a non-obvious invariant"
  - "Acceptance by grep gate: a package singleton is proven single-homed by counting its declaration and use sites, not by trusting the move"

requirements-completed: [STR-06]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "Event dispatcher (EventCallback, eventDispatchJob, eventDispatcher with stateMu/closed, constructor, run/emit/DroppedCount/close) moved into beamsync/events.go"
    requirement: "STR-06"
    verification:
      - kind: unit
        ref: "beamsync/event_dispatcher_test.go#TestEventDispatcherDeliversNormalEmit"
        status: pass
      - kind: unit
        ref: "beamsync/event_dispatcher_test.go#TestEventDispatcherCloseRacesEmitSafely"
        status: pass
      - kind: unit
        ref: "beamsync/event_dispatcher_test.go#TestEventDispatcherCloseStopsWorkerAndIsIdempotent"
        status: pass
    human_judgment: false
  - id: D2
    description: "Background event producers (startWatchdog, startBootstrapRotator, safeEmit, logTransfer) moved into events.go with bodies unchanged"
    requirement: "STR-06"
    verification:
      - kind: other
        ref: "(cd beamsync && go build ./... && go vet ./...) && (cd beamsync && go test -race ./...)"
        status: pass
      - kind: other
        ref: "grep -rn 'defaultEventDispatcher' beamsync/ => 2 hits, both events.go"
        status: pass
    human_judgment: false
  - id: D3
    description: "Misplaced dispatcher shutdown test relocated from progress_test.go to event_dispatcher_test.go, name unchanged, imports rebalanced"
    requirement: "STR-06"
    verification:
      - kind: unit
        ref: "beamsync/event_dispatcher_test.go#TestEventDispatcherCloseStopsWorkerAndIsIdempotent"
        status: pass
      - kind: other
        ref: "(cd beamsync && go build ./... && go vet ./...) && (cd desktop && go build . && go vet . && go test -race .)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Prohibition: the extraction changed no runtime behavior, no event payload, and no HTTP response body"
    verification: []
    human_judgment: true
    rationale: "Plan prohibition verified by judgment — each moved function body is byte-identical to its original, which a reviewer confirms by diff inspection; no automated assertion can prove absence of behavior change"

# Metrics
duration: 8min
completed: 2026-11-17
status: complete
---

# Phase 4 Plan 01: Event Dispatcher & Producer Extraction Summary

**The event subsystem — dispatcher type, its package singleton, and all four background producers — now lives in `beamsync/events.go`, leaving `server.go` 182 lines shorter with the same behavior and a single-homed singleton.**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-11-17T16:43:00Z
- **Completed:** 2026-11-17T16:51:00Z
- **Tasks:** 3
- **Files modified:** 4 (1 created, 3 modified)

## Accomplishments

- Created `beamsync/events.go` (191 lines) holding `EventCallback`, `eventDispatcherBufferSize`, `eventDispatchJob`, the `eventDispatcher` type with its `stateMu`/`closed` fields, `newEventDispatcher`, `run`, `emit`, `DroppedCount`, `close`, the `defaultEventDispatcher` singleton, and the four background producers.
- Trimmed `beamsync/server.go` from 1538 to 1356 lines and dropped its now-unused `sync/atomic` import — `dropped atomic.Int64` was that file's only use.
- Moved `startWatchdog`, `startBootstrapRotator`, `safeEmit` and `logTransfer` verbatim: panic recovery, the nil-callback early return in `safeEmit`, and the nil-history early return in `logTransfer` are all carried as written.
- Hauled `TestEventDispatcherCloseStopsWorkerAndIsIdempotent` out of `progress_test.go` into `event_dispatcher_test.go` without renaming it, so the dispatcher's source file and its test file pair 1:1 and the Phase 3 mutation table still resolves.
- The grep gate holds: `defaultEventDispatcher` has exactly two occurrences in the package, both in `events.go` — one declaration, one usage inside `safeEmit`.

## Task Commits

Each task was committed atomically:

1. **Task 1.1 (tracer): move the event dispatcher into events.go** — `1f2c7f0` (refactor)
2. **Task 1.2: move the background event producers into events.go** — `50a6485` (refactor)
3. **Task 1.3: relocate the misplaced dispatcher test + full gate** — `ce6143c` (test)

**Plan metadata:** _(this commit — docs: complete plan)_

## Files Created/Modified

- `beamsync/events.go` (created, 191 lines) — the whole event subsystem: dispatcher type + singleton + background producers.
- `beamsync/server.go` (modified, −182 lines) — event symbols removed, `sync/atomic` import dropped; `PendingTransfer`, `uiFS`, `serverState`, `HTTPServer` and the two `Start*E` orchestration functions stay put.
- `beamsync/event_dispatcher_test.go` (modified, +50 lines) — gained `TestEventDispatcherCloseStopsWorkerAndIsIdempotent` and the `runtime` import it needs.
- `beamsync/progress_test.go` (modified, −50 lines) — lost that test and its now-unused `runtime` import.

## Decisions Made

- **Singleton placement:** `var defaultEventDispatcher` sits between the `eventDispatcher` type and `newEventDispatcher`, inside the same declaration block as the type (PITFALLS 1a / T-04-01-01). A stranded declaration would silently create a second dispatcher with no compile or vet error, so the two are kept adjacent rather than separated by the constructor.
- **`PendingTransfer` stayed in `server.go`:** CONTEXT left its home to agent discretion and 04-09 pins it there as orchestration state for `HTTPServer.pendingTransfers`. Moving it now would have made 04-09's trim read as a re-move.
- **Tracer happy path came from the existing suite:** the plan's tracer action — emit one job through `newEventDispatcher` and assert the callback receives `<event>|<data>` within 1s — is exactly `TestEventDispatcherDeliversNormalEmit`'s first subtest, which already existed. It was run explicitly (`go test -race -run TestEventDispatcherDeliversNormalEmit -v`) rather than duplicated as a new test.
- **Test relocation, not renaming:** CONTEXT D-13 makes test function names the keys of Phase 3's 33-mutant table, so the move preserved the name exactly.

## Verification

Every task ran the plan's `<verify>` chain; Task 1.3 ran the full Phase 4 gate from CONTEXT D-08:

```
(cd beamsync && go build ./... && go vet ./...)     → exit 0
(cd beamsync && go test -race ./...)                → ok beamsync 7.9s
(cd desktop  && go build . && go vet . && go test -race .) → ok desktop 1.2s
grep -rn "defaultEventDispatcher" beamsync/         → 2 hits, both events.go
gofmt -l beamsync/                                  → clean
```

`go test -race ./...` also covers the moved code paths: `safeEmit` is exercised by `server_test.go`, and `logTransfer` / `startBootstrapRotator` / `startWatchdog` are driven indirectly through the receive and send path integration tests.

## Deviations from Plan

None - plan executed exactly as written.

The only non-conformance is environmental, not a code change: the locally installed `golangci-lint` v1.64.8 cannot decode Go 1.27.4 export data (`could not import sync/atomic … export data version 5`), and reports the same class of error on untouched files (e.g. `stats.go`). It is a pre-existing linter/toolchain mismatch, so `go build` / `go vet` / `go test -race` remain the authoritative gate here. No lint-driven change was made.

## Issues Encountered

- None affecting the work. The expected intermediate state after Task 1.1 — `safeEmit` still in `server.go` calling `defaultEventDispatcher` — was confirmed rather than treated as a failure, since `safeEmit` itself moves in Task 1.2.

## Threat Flags

None — no new network surface, auth path, file-access pattern, or trust-boundary change. The move is package-internal and the two register entries (T-04-01-01 singleton split, T-04-01-02 close/emit race) are both mitigated as planned.

## Known Stubs

None.

## Next Phase Readiness

- `beamsync/events.go` is now the home for anything event-shaped. 04-02 (`rate_limiter.go`) is next in the risk-ascending order and starts from a `server.go` that is 182 lines lighter.
- `server.go` still carries cross-file references into the new file (`startWatchdog`, `startBootstrapRotator`, `safeEmit`, `logTransfer`) — legal within one package and expected; later plans do not need to touch them.
- The event-name literals (`url_changed`, `token_rotated`, `transfer_logged`, `device_disconnected`) moved with their producers and are untouched, so Phase 5's `SVE-08` work starts from the same string set it expects.

---

*Phase: 04-go-structural-refactor-server-go-extraction*
*Completed: 2026-11-17*

## Self-Check: PASSED

All four created/modified files exist on disk and all three task commits (`1f2c7f0`, `50a6485`, `ce6143c`) are present in the log.
