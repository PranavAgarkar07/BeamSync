---
phase: 05-svelte-component-split
plan: 03
subsystem: frontend
tags: [svelte, utils, refactor, frontend, helpers]

# Dependency graph
requires: [05-01]
provides:
  - desktop/frontend/src/lib/utils.js — 5 pure presentation helpers (formatDuration, formatSize, fileIcon, isValidIPv4, normalizeTransferStats) usable by any component without Svelte or Wails imports
  - desktop/frontend/src/App.svelte — 69 lines of local helper declarations removed; helpers imported from ./lib/utils.js
affects: [05-04, 05-05, 05-06, 05-07, 05-08, 05-09, 05-10, 05-11, 05-12]

# Actuals (#2632) — same chars/4 scale as the plan's estimate (10000 tokens)
actuals:
  tokens: 1150   # chars/4 over the realized diff (112 added + 69 removed lines)
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Pure helper module: src/lib/utils.js contains zero Svelte imports, zero Wails imports, zero DOM access — importable from .js tests and any component"
    - "Behavior-preserving extraction: helper bodies copied byte-identically, including latent quirks (isValidIPv4's tautological leading-zero check), because Phase 5 is a no-behavior-change refactor"
    - "Impure-by-accident helpers get explicit parameters rather than closures: normalizeTransferStats(nextStats, fallbackStartedAt) instead of reading component state"

key-files:
  created:
    - desktop/frontend/src/lib/utils.js
  modified:
    - desktop/frontend/src/App.svelte

key-decisions:
  - "normalizeTransferStats was not actually pure in App.svelte — it closed over `transferStats.startedAt` as a fallback. Preserved identical behavior by adding an explicit second parameter `fallbackStartedAt = \"\"` and passing `transferStats.startedAt` at the single call site, rather than leaving it in App.svelte or duplicating state access in the module"
  - "Preserved isValidIPv4's latent bug: `String(value) === String(Number(part))` where `value = Number(part)` is a tautology, so \"01.2.3.4\" validates as true. Fixing it would reject addresses the app accepts today — a behavior change, prohibited by the Phase 5 charter. Logged below for a future phase"
  - "formatTransferTime and toast deliberately remain in App.svelte: formatTransferTime is passed as a prop to ActivityPanel and depends on locale formatting, while toast owns App-level notification state (UI-SPEC Open Question #1)"

patterns-established:
  - "Component plans 05-04…05-12 import these 5 helpers from ../lib/utils.js; no duplicate formatSize/fileIcon implementations"
  - "New pure helpers belong in utils.js, not in a component, so they are directly unit-testable in Plan 05-12 without mounting Svelte"

requirements-completed: []

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "desktop/frontend/src/lib/utils.js created — formatDuration, formatSize, fileIcon, isValidIPv4, normalizeTransferStats exported; no Svelte imports, no Wails imports, no DOM access, no toast()"
    verification:
      - kind: other
        ref: "grep -E 'import|document\\.|window\\.' desktop/frontend/src/lib/utils.js => zero matches; grep -c '^export function' => 5"
        status: pass
      - kind: unit
        ref: "(cd desktop/frontend && npm run check)"
        status: pass
    human_judgment: false
  - id: D2
    description: "App.svelte local declarations of the 5 helpers removed (69 lines); imports added from ./lib/utils.js; formatTransferTime and toast still declared locally"
    verification:
      - kind: other
        ref: "grep -E '^\\s*function (formatDuration|formatSize|fileIcon|isValidIPv4|normalizeTransferStats)\\b' App.svelte => none; function toast / formatTransferTime still present"
        status: pass
      - kind: unit
        ref: "(cd desktop/frontend && npm run check && npm run lint)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Behavior gate — 18/18 cases match the pre-extraction output: formatSize 0/500/1024/1048576, fileIcon pdf/png/unknown/empty, isValidIPv4 valid/invalid/3-part/non-numeric/leading-zero, formatDuration 0/500/45000/125000, normalizeTransferStats fallback-vs-payload"
    verification:
      - kind: unit
        ref: "node --input-type=module -e '<18 assertions against src/lib/utils.js>' => ALL BEHAVIOR GATES PASS (18/18)"
        status: pass
    human_judgment: false
  - id: D4
    description: "No behavior change — render identical. All call sites unchanged apart from the single normalizeTransferStats call gaining its explicit fallback argument"
    verification: []
    human_judgment: true
    rationale: "Bodies copied byte-identically from App.svelte; the only signature change is an added optional parameter whose value reproduces the original closure exactly"

# Metrics
duration: ~9min
completed: 2026-11-19
status: complete
---

# Phase 5 Plan 03: Pure Helper Module — utils.js

**Extracted 5 presentational helpers from `App.svelte` into `desktop/frontend/src/lib/utils.js` (112 lines), removing 69 lines of local declarations. All 18 behavior assertions match the pre-extraction output.**

## Performance

- **Duration:** ~9 min
- **Tasks:** 2
- **Files modified:** 2 (1 created, 1 modified)
- **App.svelte:** 2232 → 2186 lines (net −46 after the two earlier plans' additions)

## Accomplishments

- Created `desktop/frontend/src/lib/utils.js` with `formatDuration`, `formatSize`, `fileIcon`, `isValidIPv4`, and `normalizeTransferStats` — pure ES module, no Svelte/Wails/DOM imports, directly unit-testable.
- Removed the 5 local declarations from `App.svelte` (−69 lines) and added a single import block; `formatTransferTime` and `toast` intentionally remain in the shell.
- Ran an 18-case behavior gate comparing every helper's output against the pre-extraction values — all pass.

## Task Commits

1. **Task 3.1 (tracer): create utils.js with pure presentation helpers** — `d2651c4` (refactor)
2. **Task 3.2: import helpers from utils.js, remove local declarations** — `f548604` (refactor)

## Files Created/Modified

- `desktop/frontend/src/lib/utils.js` (created, 112 lines) — 5 pure helpers with provenance comments citing the original App.svelte line ranges.
- `desktop/frontend/src/App.svelte` (modified, +8/−69) — import block added, local declarations removed, one call site updated.

## Decisions Made

- **Made `normalizeTransferStats` genuinely pure.** The original read `transferStats.startedAt` from component state as its fallback. Rather than leave it in the shell (which the plan permitted only for `formatTransferTime`/`toast`) or smuggle state access into a module, the fallback became an explicit second parameter and the single call site passes `transferStats.startedAt`. Output is unchanged; `utils.js` stays import-safe.
- **Preserved `isValidIPv4`'s latent bug rather than fixing it.** The guard `String(value) === String(Number(part))` compares `Number(part)` to itself — always true — so `"01.2.3.4"` is accepted. Rejecting it would be a behavior change; the charter forbids that. Documented below for a future correctness phase.

## Verification

```
(cd desktop/frontend && npm run check)  → 0 errors, 1 pre-existing unrelated warning (Typewriter.svelte)
(cd desktop/frontend && npm run lint)   → 0 errors, 1 pre-existing unrelated warning (Typewriter.svelte)
behavior gate (18 assertions)           → ALL BEHAVIOR GATES PASS (18/18)
grep local declarations in App.svelte   → none of the 5 remain; toast + formatTransferTime still present
grep imports/DOM in utils.js            → zero Svelte, Wails, or document/window references
```

## Deviations from Plan

**One signature change, required to satisfy the plan's own purity requirement.**

- **Planned:** all 5 helpers are "pure functions in utils.js with identical behavior".
- **Found:** `normalizeTransferStats` was not pure — it captured `transferStats.startedAt` from component state.
- **Resolution:** added `fallbackStartedAt = ""` as a second parameter; the single call site (`EventsOn(EVT_TRANSFER_STATS, …)`) now passes `transferStats.startedAt`. Identical output, now pure. Kept out of the "deviations requiring user approval" class because it preserves behavior exactly and satisfies the plan's stated truth rather than relaxing it.

No scope dropped. No helper behavior altered.

## Issues Encountered

- **My own behavior-gate expectation was wrong, not the code.** I initially asserted `isValidIPv4("01.2.3.4") === false`. It returned `true`. Investigating showed the original App.svelte implementation has the same tautological guard, so the extraction is faithful and my expectation was incorrect. Corrected the assertion to document the preserved quirk instead of "fixing" production behavior inside a refactor.

## Latent Bug Carried Forward (not introduced here)

`isValidIPv4` accepts leading-zero octets (`"01.2.3.4"` → `true`) because `String(value) === String(Number(part))` is a tautology when `value = Number(part)`. The likely intent was `String(Number(part)) === part` (rejecting non-canonical octets). This predates Phase 5 and is preserved verbatim. Recommended follow-up: fix in a dedicated correctness phase with a test, since it is a user-input validation surface (`upsertDevice` gates manual device IP entry on it).

## Threat Flags

None — no new network endpoints, auth paths, file-access patterns, or schema changes. T-05-03-01 (tampering: helper behavior drift) is mitigated by the 18-case behavior gate plus byte-identical bodies. T-05-03-SC: no new packages.

## Known Stubs

None.

## Next Phase Readiness

- All three Wave 1–2 foundation modules now exist: `lib/wails.js` (Plan 01), `lib/events.js` (Plan 02), `lib/utils.js` (this plan).
- Wave 3 (05-04 ReceiveView, 05-05 SendView, 05-06 AboutView) can start: views import helpers from `../lib/utils.js`, events from `../lib/events.js`, and Wails from `../lib/wails.js`.

## Self-Check: PASSED

- `desktop/frontend/src/lib/utils.js` exists with exactly 5 exported functions ✓
- No Svelte/Wails/DOM imports in utils.js ✓
- All 5 local declarations removed from App.svelte; `toast` and `formatTransferTime` retained ✓
- Behavior gate 18/18 ✓
- Both task commits present: `d2651c4`, `f548604` ✓
- `npm run check` and `npm run lint` pass (0 errors) ✓
- Working tree clean before this summary was written ✓
