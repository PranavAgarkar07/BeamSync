---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
current_phase: 3
current_phase_name: Tests Before Refactoring
status: complete
stopped_at: "Phase 3 complete — verification passed (18/18 must-haves)"
last_updated: "2026-11-12T12:30:00.000Z"
last_activity: 2026-11-12
last_activity_desc: Phase 3 complete — 24/50 requirements, verification passed 18/18
progress:
  total_phases: 8
  completed_phases: 3
  total_plans: 7
  completed_plans: 7
---

# Project State

**Last Updated:** 2026-11-12

Phase 1: complete — 6/6.
Phase 2: complete — 7/7.
Phase 3: complete — 6/6 (TST-03, TST-05, TST-06 landed with the initial test suite; TST-01 landed in plan 03-01, TST-04 landed in plan 03-03, TST-02 landed in plan 03-02).

## Current Position

Phase: 3 of 8 — Tests Before Refactoring — **COMPLETE, verification passed (18/18 must-haves)**
Plan: 3 of 3 in current phase — 03-01, 03-02, 03-03 all complete
Status: Phase complete — next is Phase 4 (Go Structural Refactor)
Last activity: 2026-11-12 — Phase 3 complete
Progress: [██████████] 100%

### Phase 3 outcome

Test safety net delivered: **24/50 requirements complete** (was 13/50 at the start of this
session). 85 test functions across both Go modules; `go test -race` green in both;
`golangci-lint` clean in both; Windows cross-build OK.

Live bugs found and fixed while building the net:
- **`download_progress` was emitted by nothing.** Phase 1 deleted `downloadProgressWriter` and
  pointed the sender at `progressWriter`, whose `Write` hardcoded `upload_progress` — so the
  desktop send-direction panel never moved. Now `progressWriter` carries an `eventName` field
  set per path at all three construction sites, and the send path emits a completion event.
- **A fast send emitted zero progress events.** The send path re-seeded `lastEmit`, so a
  transfer finishing inside the 500 ms throttle window emitted nothing at all.
- **Data race on the shared endpoint fields.** `currentIP`/`currentPort`/`currentToken` were
  written from three goroutines (processEvents, startIPMonitor, the Wails call handlers) and
  read from others. Now guarded by `endpointMu` with snapshot/set accessors.
- **`extractPortFromURL` returned `1]`** for a bracketed IPv6 host with no port.
- **The home-directory guard used a raw string prefix**, accepting `<home>-evil` as inside home.
- **CI was red** on `main` and `dev` — `errorlint` violations in the firewall-failover error
  path plus a `typecheck` failure on the `/v2`-suffixed wails import. Both fixed; CI lint is
  now clean.

### Safety net quality

Every production `eventName` literal is protected by an end-to-end test that drives the real
HTTP handler — mutating any one of the three sites fails the suite. The dispatcher tests use a
primer construction (a callback that signals entry from inside the blocking callback) so the
drop-on-full assertion is deterministic rather than a scheduler coin-flip (measured: 197/200
flips with a scheduler yield). Nine mutants were written and all are killed.

Deferred to later phases (logged in `.planning/phases/03-tests-before-refactoring/deferred-items.md`):
the `|`-in-filename payload ambiguity, and the Linux-only CI matrix (Phase 7) that leaves the
Windows-only branch of `desktop/app.go` unexercised.

## Milestone: Quality Overhaul

| Phase | Status | Progress |
|-------|--------|----------|
| 1 — Event System & Version | ● Complete | 6/6 reqs |
| 2 — DevEx Tooling & CI | ● Complete | 7/7 reqs |
| 3 — Tests Before Refactoring | ● Complete | 6/6 reqs |
| 4 — Go Structural Refactor | ○ Planned | 1/10 reqs |
| 5 — Svelte Component Split | ○ Planned | 0/9 reqs |
| 6 — Error Handling & Logging | ○ Planned | 1/4 reqs |
| 7 — CI/CD & Release | ○ Planned | 0/3 reqs |
| 8 — Documentation & Cleanup | ○ Planned | 2/5 reqs |

**Total:** 24/50 requirements complete

## Active Phase

Phase 3 — Tests Before Refactoring (6/6 done)

Outstanding in Phase 3:

- None — TST-01 (plan 03-01), TST-04 (plan 03-03), and TST-02 (plan 03-02) are covered, and
  TST-03/TST-05/TST-06 were already satisfied by the initial test suite.

## Planning Data Corrections (2026-11-12)

Codebase analysis found `REQUIREMENTS.md` had **all 50 checkboxes marked `[x]` since its
initial commit** (`6d2860f`) — the boxes were never updated as work landed, so the tracker
claimed 50/50 while the real figure was 13/50. Checkboxes were re-derived from the
implementation and corrected to the counts above.

Verified by inspection, not by trusting the tracker:

| Claim | Reality |
|-------|---------|
| Phase 2 complete | True — all 7 config artifacts exist and CI passes; only `02-01-SUMMARY.md` was missing (now written) |
| Phase 1 complete | True — but summaries were phase-level (`01-SUMMARY.md`) while GSD pairs one summary per *plan*. Split into `01-01/01-02/01-03-SUMMARY.md`; rollup renamed `01-OVERVIEW.md` so it stops double-counting (was reporting 5 summaries for 4 plans) |
| STR-03 merge duplicate writers | True — `downloadProgressWriter` was deleted in `d750b19`; only `progressWriter` remains |
| STR-10 `shareURL()` helper | True — added in `c11f2be` |
| ERR-01 sentinel errors | True — `errInvalidToken`/`errExpiredToken`/`errUsedToken` in `beamsync/auth_tokens.go` |
| DOC-02 CONTRIBUTING matches DevEx | True — documents `go test ./...`, `npm test`, ESLint/Prettier |
| DOC-05 README matches reality | True after fixing the port range (README said 3000–3098; code opens 3000–3100) |
| Phases 3–8 partially done | Corrected — see table above |

Also stale: `ROADMAP.md` sizes `server.go` at 1726 lines (written at `6d2860f`; it is now
1414 after Phase 1/2 extraction) and `App.svelte` at 2160 (now 2232).

## Recent Commits

- `415c6e2` — test(03-02): add progressWriter unit tests for receive path, throttle, and payload
- `d058ae1` — fix(03-02): emit download_progress on the send path via progressWriter.eventName
- `8dd231d` — test(03-02): add failing progressWriter send-path event-name test
- `8742a3e` — test(03-03): add desktop app.go pure-function test suite
- `e219ebb` — test(03-03): add desktop shareURL endpoint URL test
- `844f8dd` — chore(03-03): un-ignore desktop frontend dist embed placeholder
- `38edd92` — fix(nav): deterministic scrollspy with active state, a11y and docs
- `39aa414` — chore(installer): per-user NSIS install dir with offline WebView2 note
- `ecedbd7` — feat(desktop): toast server_error with manual fix steps on start failure
- `8624593` — fix(tls): use OS config dir for local certs and update API docs
- `9a3a894` — refactor(server): surface startup errors via StartServerE/StartSenderE with desktop failover
- `d82bc63` — feat(firewall): Windows Defender setup with manual steps and docs
- `833e298` — feat(beamsync): add offline LAN IP resolver with interface fallback
- `b2e7e0f` — docs(phase-02): discuss Phase 2 — Tooling & DevEx
- `3efba50` — docs(phase-02): add Phase 2 plan — Tooling & DevEx
- `d750b19` — Phase 1: Foundation — Event System & Version

## Artifacts

- `.planning/PROJECT.md` — project definition
- `.planning/REQUIREMENTS.md` — 50 requirements (checkboxes corrected 2026-11-12)
- `.planning/ROADMAP.md` — 8-phase roadmap
- `.planning/config.json` — YOLO mode, fine granularity, parallel
- `.planning/codebase/` — 7 codebase map documents (refresh skipped 2026-11-12)
- `.planning/research/` — 5 research documents
- `.planning/phases/01-foundation-event-system-version/CONTEXT.md` — Phase 1 context
- `.planning/phases/01-foundation-event-system-version/01-RESEARCH.md` — Phase 1 research
- `.planning/phases/01-foundation-event-system-version/01-01-PLAN.md` — Plan: dispatcher hardening
- `.planning/phases/01-foundation-event-system-version/01-02-PLAN.md` — Plan: desktop channel + version
- `.planning/phases/01-foundation-event-system-version/01-03-PLAN.md` — Plan: frontend version bridge
- `.planning/phases/01-foundation-event-system-version/01-01-SUMMARY.md` — Plan 1.1 summary (dispatcher hardening)
- `.planning/phases/01-foundation-event-system-version/01-02-SUMMARY.md` — Plan 1.2 summary (desktop channel + version)
- `.planning/phases/01-foundation-event-system-version/01-03-SUMMARY.md` — Plan 1.3 summary (frontend version bridge)
- `.planning/phases/01-foundation-event-system-version/01-OVERVIEW.md` — Phase 1 rollup + decisions (renamed from `01-SUMMARY.md` so it does not double-count against the 3 plans)
- `.planning/phases/02-tooling-devex/CONTEXT.md` — Phase 2 context
- `.planning/phases/02-tooling-devex/02-01-PLAN.md` — Plan: all tooling configs
- `.planning/phases/02-tooling-devex/02-01-SUMMARY.md` — Plan 2.1 summary (written 2026-11-12)
- `.planning/phases/03-tests-before-refactoring/03-CONTEXT.md` — Phase 3 context (D-01…D-13)
- `.planning/phases/03-tests-before-refactoring/03-PATTERNS.md` — Phase 3 pattern map
- `.planning/phases/03-tests-before-refactoring/03-01-PLAN.md` — Plan: eventDispatcher tests
- `.planning/phases/03-tests-before-refactoring/03-01-SUMMARY.md` — Plan 3.1 summary (eventDispatcher tests)
- `.planning/phases/03-tests-before-refactoring/03-02-PLAN.md` — Plan: progressWriter tests + `download_progress` fix
- `.planning/phases/03-tests-before-refactoring/03-02-SUMMARY.md` — Plan 3.2 summary (progressWriter tests + `download_progress` fix)
- `.planning/phases/03-tests-before-refactoring/03-03-PLAN.md` — Plan: desktop/app.go pure-function tests
- `.planning/phases/03-tests-before-refactoring/03-03-SUMMARY.md` — Plan 3.3 summary (desktop/app.go pure-function tests)
- `.planning/phases/03-tests-before-refactoring/deferred-items.md` — Out-of-scope findings logged during Phase 3

## Session

**Last session:** 2026-11-12T12:00:12.038Z
**Stopped at:** Completed 03-02-PLAN.md
**Resume file:** None

## Decisions

- [Phase 3]: Added eventDispatcher unit test suite (normal emit, drop-on-full, empty-buffer fallback, panic recovery, concurrency exact drop accounting) before Phase 4 refactor — Phase 4 will move the dispatcher into beamsync/events.go; black-box emit/DroppedCount seam keeps tests representation-agnostic (D-04)
- [Phase 3]: Added desktop/app.go pure-function unit test suite (shareURL, token/port URL extraction, config load-save round-trip with the home-prefix guard, all three ensureSaveDir branches) before Phase 4/5 restructuring — the desktop module had zero test files until now (D-10)
- [Phase 3]: Committed desktop/frontend/dist/.gitkeep and un-ignored it in BOTH the root and desktop .gitignore files using the dist/* + !dist/.gitkeep form — git cannot re-include a file whose parent directory is excluded, so a negation in desktop/.gitignore alone is silently ineffective (D-11)
- [Phase 3]: Scoped desktop Go commands to the root package (go test . / go build . / go vet .) — go test ./... also compiles a stray Go package vendored inside desktop/frontend/node_modules/flatted (D-12)
- [Phase 3]: D-07/D-08: committed the send-path event-name test RED before the fix, so the download_progress fix is provably what turned it green; the test asserts the event name per path (upload_progress on receive, download_progress on send), not just the payload
- [Phase 3]: D-09: progressWriter gained an injectable now func() time.Time clock (mirroring clientRateLimiter.now) with a nil-guard fallback to time.Now, so the throttle is tested with a fake clock and never time.Sleep
- [Phase 3]: Left lastEmit unset at the receive-path progressWriter construction site, preserving the asymmetry where the first write always emits — now pinned by a test instead of left as an accident

## Performance Metrics

| Plan | Duration | Tasks | Files |
|------|----------|-------|-------|
| Phase 03 P03 | 14min | 3 tasks | 5 files |
| Phase 03 P02 | 5min | 3 tasks | 2 files |
