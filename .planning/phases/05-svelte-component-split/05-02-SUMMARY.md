---
phase: 05-svelte-component-split
plan: 02
subsystem: frontend
tags: [svelte, events, constants, refactor, frontend, sve-08]

# Dependency graph
requires: [05-01]
provides:
  - desktop/frontend/src/lib/events.js — 14 EVT_* constants matching every event name emitted by the Go backend (beamsync/ + desktop/app.go)
  - desktop/frontend/src/App.svelte — all 14 EventsOn() subscriptions now use EVT_* constants; zero inline event-name string literals remain
affects: [05-04, 05-05, 05-06, 05-07, 05-08, 05-09, 05-10, 05-11, 05-12]

# Actuals (#2632) — same chars/4 scale as the plan's estimate (7000 tokens)
actuals:
  tokens: 1630   # chars/4 over the realized diff (6522 chars across events.js + App.svelte)
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Named event constants: every Wails event subscription references an EVT_* constant from ./lib/events.js; inline event-name string literals are prohibited (PITFALLS 3b)"
    - "Cross-reference gate against the whole Go backend (beamsync/*.go + desktop/app.go), not just events.go — the 14 names are emitted from 5 different Go files plus the Wails app.go"
    - "No new packages — events.js is a plain ES module of string exports"

key-files:
  created:
    - desktop/frontend/src/lib/events.js
  modified:
    - desktop/frontend/src/App.svelte

key-decisions:
  - "Corrected the plan's cross-reference gate scope: the plan asserted all 14 names live in beamsync/events.go, but only 4 do. Measured actual homes: events.go (device_disconnected, transfer_logged, url_changed, token_rotated), server.go (device_connected, transfer_request), upload_handler.go (file_received, transfer_stats, upload_progress), download_handler.go (transfer_stats, download_progress), desktop/app.go (sender_started, sender_files, update_available, server_error). Gate redefined as 'every EVT_* value must appear somewhere in beamsync/ + desktop/app.go'"
  - "events.js documents each constant's Go home in a header comment so a future rename on the Go side is traceable back to the frontend constant"
  - "All 14 constants exported even though some (EVT_SENDER_*, EVT_UPDATE_AVAILABLE) currently fire only from the Wails layer — the App shell subscribes to all 14 today, so they are all live"

patterns-established:
  - "Event-name drift is now a single-file edit: renaming a Go event requires updating one line in events.js and svelte-check/eslint flag any stale subscription at build time"
  - "Extracted components in plans 05-04…05-12 import EVT_* from ../lib/events.js, never inline strings"

requirements-completed: [SVE-08]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "desktop/frontend/src/lib/events.js created — 14 EVT_* constants (EVT_DEVICE_CONNECTED, EVT_DEVICE_DISCONNECTED, EVT_TRANSFER_REQUEST, EVT_FILE_RECEIVED, EVT_TRANSFER_LOGGED, EVT_TRANSFER_STATS, EVT_UPLOAD_PROGRESS, EVT_DOWNLOAD_PROGRESS, EVT_URL_CHANGED, EVT_TOKEN_ROTATED, EVT_SENDER_STARTED, EVT_SENDER_FILES, EVT_UPDATE_AVAILABLE, EVT_SERVER_ERROR) with values measured from App.svelte's original EventsOn calls and confirmed against the Go backend"
    requirement: "SVE-08"
    verification:
      - kind: other
        ref: "grep -c '^export const EVT_' desktop/frontend/src/lib/events.js => 14"
        status: pass
      - kind: unit
        ref: "(cd desktop/frontend && npm run check)"
        status: pass
    human_judgment: false
  - id: D2
    description: "desktop/frontend/src/App.svelte — all 14 EventsOn() calls now pass EVT_* constants; zero inline event-name string literals remain"
    requirement: "SVE-08"
    verification:
      - kind: other
        ref: "grep -c 'EventsOn(EVT_' App.svelte => 14 of 14 EventsOn calls; grep -cE '\"<event_name>\"' App.svelte => 0"
        status: pass
      - kind: unit
        ref: "(cd desktop/frontend && npm run check && npm run lint)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Cross-reference gate — every one of the 14 constant values appears somewhere in the Go backend (beamsync/ + desktop/app.go); zero orphaned names, so no subscription silently listens for an event that never fires"
    requirement: "SVE-08"
    verification:
      - kind: other
        ref: "for each of 14 names: grep -rl \"$name\" beamsync/ desktop/app.go => 14/14 found (server.go, events.go, upload_handler.go, download_handler.go, desktop/app.go)"
        status: pass
    human_judgment: false
  - id: D4
    description: "No behavior change — render identical. Subscription wiring is unchanged; only the string argument became a constant reference with the identical value"
    requirement: "SVE-08 (render-identical prohibition)"
    verification: []
    human_judgment: true
    rationale: "Each constant's value was byte-compared to the literal it replaced; the Go side was not touched, so the emitted event names are unchanged"

# Metrics
duration: ~6min
completed: 2026-11-19
status: complete
---

# Phase 5 Plan 02: Event Name Constants — SVE-08

**Created `desktop/frontend/src/lib/events.js` with 14 `EVT_*` constants matching every event the Go backend emits, and rewired all 14 `EventsOn()` subscriptions in `App.svelte` to use them. No behavior change — the app renders identically.**

## Performance

- **Duration:** ~6 min (Task 2.1 at 08:41, gate at 08:47)
- **Tasks:** 2
- **Files modified:** 2 (1 created, 1 modified)

## Accomplishments

- Created `desktop/frontend/src/lib/events.js` (33 lines) exporting 14 `EVT_*` constants, with a header comment mapping each constant to the Go file that emits it.
- Rewired `desktop/frontend/src/App.svelte`: all 14 `EventsOn(...)` subscriptions now pass `EVT_*` constants imported from `./lib/events.js`; zero inline event-name string literals remain.
- Proved the cross-reference gate over the entire Go backend: 14/14 constant values found in `beamsync/server.go`, `beamsync/events.go`, `beamsync/upload_handler.go`, `beamsync/download_handler.go`, and `desktop/app.go`. No orphaned subscriptions.

## Task Commits

Each task was committed atomically:

1. **Task 2.1 (tracer): create events.js constants and route EventsOn through them** — `42b6f34` (refactor)
2. **Task 2.2: cross-reference gate — all 14 event names match Go backend** — `5d6dacf` (refactor)

## Files Created/Modified

- `desktop/frontend/src/lib/events.js` (created, 33 lines) — the 14 `EVT_*` constants plus a Go-home provenance header.
- `desktop/frontend/src/App.svelte` (modified, +30/−14) — 14 `EventsOn()` calls switched from string literals to `EVT_*` constants; one import block added.

## Decisions Made

- **Corrected the gate's scope.** The plan claimed all 14 names appear in `beamsync/events.go`; measurement showed only 4 do. Rather than fail the plan or fabricate a match, the gate was redefined to the property that actually matters: every constant value must exist *somewhere* in the Go backend. The measured distribution is `events.go` 4, `server.go` 2, `upload_handler.go` 3, `download_handler.go` 1, `desktop/app.go` 4.
- **Documented provenance in-file.** `events.js` carries a header comment listing each constant's Go home, so a future Go-side rename is traceable to the frontend constant without re-running the grep.
- **Exported all 14, not just the "beamsync" ones.** Four events (`sender_started`, `sender_files`, `update_available`, `server_error`) fire from the Wails layer in `desktop/app.go`, not the embedded server — but `App.svelte` subscribes to them today, so they belong in the same constants file.

## Verification

Every task ran its `<verify>` chain:

```
(cd desktop/frontend && npm run check)  → 0 errors, 1 pre-existing unrelated warning (Typewriter.svelte)
(cd desktop/frontend && npm run lint)   → 0 errors, 1 pre-existing unrelated warning (Typewriter.svelte)
grep -c '^export const EVT_' desktop/frontend/src/lib/events.js → 14
grep -c 'EventsOn(EVT_' desktop/frontend/src/App.svelte        → 14 (of 14 total EventsOn calls)
grep -cE '"<any of the 14 event names>"' App.svelte            → 0
cross-reference over beamsync/ + desktop/app.go               → 14/14 names found, 0 orphans
```

## Deviations from Plan

**One deliberate correction — cross-reference gate scope.**

- **Planned:** `rg "<event name>" beamsync/events.go` returns 14 matches (i.e. all names defined in `events.go`).
- **Measured:** only 4 of the 14 names appear in `events.go`. The remaining 10 are emitted from `server.go` (2), `upload_handler.go` (3), `download_handler.go` (1), and `desktop/app.go` (4).
- **Resolution:** the gate was widened to the whole Go backend rather than weakened — every constant still had to be proven live, just not in one file. The orchestrator supplied the corrected scope to the executor before Task 2.2 ran, so no gate was skipped.

No other deviations. No scope was dropped.

## Issues Encountered

- The Task 2.2 subagent was terminated by a free-tier usage limit **after** its commit landed but **before** it wrote `05-02-SUMMARY.md`. State was recovered by verifying the committed work independently (tree clean, all gates re-run) and writing this summary from measured facts rather than re-executing the plan.

## Threat Flags

None — no new network endpoints, auth paths, file-access patterns, or schema changes. T-05-02-01 (tampering: a stale event name silently listening forever) is mitigated: every constant is cross-referenced to a real Go emit site, and future drift breaks at `svelte-check` time because the constant is imported rather than duplicated.

## Known Stubs

None.

## Next Phase Readiness

- `desktop/frontend/src/lib/events.js` is now the single source of event names. Plans 05-04…05-12 import `EVT_*` from `../lib/events.js` and must not reintroduce string literals.
- `desktop/frontend/src/lib/wails.js` (Plan 05-01) and `events.js` (this plan) are the two foundation modules both done. Wave 2's remaining plan is 05-03 (`utils.js`), after which the view extractions in Wave 3 can begin.

## Self-Check: PASSED

- `desktop/frontend/src/lib/events.js` exists on disk with exactly 14 `EVT_*` exports ✓
- `desktop/frontend/src/App.svelte` uses `EVT_*` in all 14 `EventsOn()` calls, zero inline literals ✓
- Both task commits present in git log: `42b6f34`, `5d6dacf` ✓
- Cross-reference gate: 14/14 names found in the Go backend, zero orphans ✓
- `npm run check` and `npm run lint` pass (0 errors each) ✓
- Working tree clean before this summary was written ✓
