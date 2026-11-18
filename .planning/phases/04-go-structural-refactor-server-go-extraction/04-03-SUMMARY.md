---
phase: 04-go-structural-refactor-server-go-extraction
plan: 03
subsystem: infra
tags: [go, refactor, progress-writer, streaming, throttle, clock-injection, package-internal-move]

# Dependency graph
requires:
  - phase: 03-tests-before-refactoring
    provides: the eight TestProgressWriter* tests that pin the eventName field, the nil-guarded now clock, the zero-byte guard and the filename|written|total payload, plus the 33-mutant table naming those tests as the killers of mutants #1-#5
  - phase: 04-go-structural-refactor-server-go-extraction
    plan: 02
    provides: beamsync/rate_limiter.go extraction, server.go already reduced to 1244 lines
provides:
  - beamsync/progress.go — progressWriter (w/total/written/filename/eventName/emit/lastEmit/minInterval/now) and its single Write method
  - beamsync/server.go trimmed 1244 → 1206 lines; the progress writer declarations are gone, only its three construction literals and one comment reference remain
  - the three Phase 3 invariants proven to have travelled intact (eventName, nil-guarded now, zero-byte early return) by a byte-identical diff plus the whole progressWriter suite
affects: [04-04, 04-05, 04-06, 04-07, 04-08, 04-09]

# Actuals (#2632) — same chars/4 scale as the plan's estimate
actuals:
  tokens: 721
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Diff-before-delete, repeated from 04-02: extract the origin line range to a temp file, write the new file, diff the new file's body against the origin range, and only then delete from the origin — the move is proven a copy, not asserted"
    - "Invariant-by-invariant grep gate: each of the three Phase 3 artifacts D-05 protects is located by name in the new file (eventName field, `if n == 0`, `if pw.now != nil`, `pw.emit(pw.eventName, …)`) so a silent simplification would show as a missing hit"
    - "Guard task with zero file delta committed --allow-empty, matching 04-02 Task 2.2 — one traceable commit per task without inventing a code change"

key-files:
  created:
    - beamsync/progress.go
  modified:
    - beamsync/server.go

key-decisions:
  - "Moved server.go:220-256 as one contiguous block — the doc comment, the struct with all nine fields, and Write — and proved it byte-identical with `diff` before deleting from the origin"
  - "progress.go imports only fmt, io and time (one parenthesized stdlib block, no file-level doc comment) per conventions C1/C2; the analog is beamsync/stats.go for the struct-plus-methods shape"
  - "io stays in server.go: copyChunked still needs io.Writer/io.ReadFull (it moves to io_helpers.go in 04-04), so the move required no import edits at all — the import block is untouched and still 22 entries"
  - "The three progressWriter construction literals stay in server.go for now (receive path at 776-equivalent, send paths at 1097/1168-equivalent); relocating them into upload_handler.go / download_handler.go is 04-07 and 04-08's job"
  - "progress_test.go needed no change: all eight TestProgressWriter* functions and the progressWriterFixture / newProgressWriterForTest / progressPayloadFields helpers already sat at the target path with the target names, so Task 3.2's delta is zero by design"
  - "The fixture's comment pointing at beamsync/server.go as the source of the production construction sites was left alone — it is still accurate, because the literals do remain in server.go after this plan"

patterns-established:
  - "A pure type-plus-method move needs no import edits when the origin file still uses every affected package — check the remaining consumers before touching the import block, and leave it alone when they still hold"

requirements-completed: [STR-02]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "progressWriter and its Write method moved into beamsync/progress.go, carrying the eventName field, the nil-guarded now clock and the zero-byte guard verbatim"
    requirement: "STR-02"
    verification:
      - kind: unit
        ref: "beamsync/progress_test.go#TestProgressWriterReceivePathEmitsUploadProgress"
        status: pass
      - kind: unit
        ref: "beamsync/progress_test.go#TestProgressWriterSendPathEmitsDownloadProgress"
        status: pass
      - kind: unit
        ref: "beamsync/progress_test.go#TestProgressWriterZeroByteWriteEmitsNothing"
        status: pass
      - kind: unit
        ref: "beamsync/progress_test.go#TestProgressWriterPayloadFormat"
        status: pass
      - kind: other
        ref: "diff of beamsync/server.go:220-256 (pre-move) against beamsync/progress.go:9-45 => no differences"
        status: pass
    human_judgment: false
  - id: D2
    description: "The throttle boundary is unchanged: minInterval 0 still emits on every write, and a large minInterval with an injected clock still coalesces writes inside the window"
    requirement: "STR-02"
    verification:
      - kind: unit
        ref: "beamsync/progress_test.go#TestProgressWriterEmitsEveryWriteWithoutThrottle"
        status: pass
      - kind: unit
        ref: "beamsync/progress_test.go#TestProgressWriterCoalescesWritesWithinInterval"
        status: pass
    human_judgment: false
  - id: D3
    description: "server.go trimmed of the progress writer — no progressWriter declaration remains there, only the three construction literals and one explanatory comment"
    requirement: "STR-02"
    verification:
      - kind: other
        ref: "grep -rn '^type progressWriter struct|^func (pw *progressWriter) Write' beamsync/ => 2 hits, both beamsync/progress.go"
        status: pass
      - kind: other
        ref: "(cd beamsync && go build ./... && go vet ./...) && (cd beamsync && go test -race -count=1 ./...)"
        status: pass
    human_judgment: false
  - id: D4
    description: "The per-path eventName emit survives the move — the receive path still emits upload_progress and both send paths still emit download_progress end-to-end (T-04-03-01)"
    requirement: "STR-02"
    verification:
      - kind: integration
        ref: "beamsync/progress_wiring_test.go#TestReceivePathEmitsUploadProgress"
        status: pass
      - kind: integration
        ref: "beamsync/progress_integration_test.go#TestSenderPathEmitsDownloadProgress"
        status: pass
      - kind: integration
        ref: "beamsync/progress_wiring_test.go#TestMultiFileSenderPathEmitsDownloadProgress"
        status: pass
    human_judgment: false
  - id: D5
    description: "written and total remain int64 byte counts — confirmed by the race/behavior tests rather than by inspection of the field types"
    requirement: "STR-02"
    verification:
      - kind: unit
        ref: "beamsync/progress_test.go#TestProgressWriterCountsBytesActuallyWritten"
        status: pass
      - kind: other
        ref: "(cd beamsync && go test -race -count=1 ./...) => ok beamsync 8.238s"
        status: pass
    human_judgment: false
  - id: D6
    description: "Prohibition: the extraction changed no runtime behavior, no event payload and no HTTP response body"
    verification: []
    human_judgment: true
    rationale: "A pure move cannot be proven behavior-neutral by assertion; a reviewer confirms it from the byte-identical diff of server.go:220-256 against progress.go:9-45, and from the fact that the three construction literals in server.go still compile against the same nine-field struct"

# Metrics
duration: 2min
completed: 2026-11-18
status: complete
---

# Phase 4 Plan 03: progressWriter Extraction Summary

**`progressWriter` — its nine-field single-writer struct, its invariant-explaining doc comment and its one `Write` method — now lives in `beamsync/progress.go` as a byte-identical copy of the block it replaced, leaving `server.go` 38 lines lighter with the throttle boundary, the zero-byte guard and the `<eventName>|filename|written|total` payload untouched.**

## Performance

- **Duration:** ~2 min
- **Started:** 2026-11-18T12:16:44Z
- **Completed:** 2026-11-18T12:18:47Z
- **Tasks:** 2
- **Files modified:** 2 (1 created, 1 modified; `progress_test.go` verified unchanged — it already sat at the target path)

## Accomplishments

- Created `beamsync/progress.go` (45 lines) holding the `progressWriter` doc comment, the struct with all nine fields (`w io.Writer`, `total`/`written int64`, `filename`, `eventName`, `emit func(string,string)`, `lastEmit time.Time`, `minInterval time.Duration`, `now func() time.Time`) and `Write`. Imports `fmt`, `io`, `time` only — one parenthesized stdlib block, no file-level doc comment (conventions C1/C2).
- Trimmed `beamsync/server.go` from 1244 to 1206 lines (−38). What remains are references, not declarations: the three `&progressWriter{…}` literals (receive path and the two send paths) and one explanatory comment about `progressWriter.total`. All legal same-package references.
- Proved the move is a copy, not a rewrite: `sed -n '220,256p' beamsync/server.go` was captured to a temp file and diffed against `progress.go:9-45` **before** the origin was deleted — identical. The throttle expression `if now.Sub(pw.lastEmit) >= pw.minInterval`, the emit call and the byte accounting are therefore the same characters as before.
- All three Phase 3 artifacts CONTEXT D-05 protects are present by name in the new file: `eventName string` (line 20), the zero-byte `if n == 0` early return (line 29), the nil-guarded `if pw.now != nil` clock (line 36), and `pw.emit(pw.eventName, fmt.Sprintf("%s|%d|%d", …))` (line 41). The emit call was moved, not renamed.
- No import edits were needed anywhere. `server.go` still uses `fmt` (payload construction in its literals… and throughout), `io` (`copyChunked`, which moves in 04-04) and `time` — so its 22-entry import block is unchanged by this plan.
- Full gate green in both modules: `beamsync` build + vet, `go test -race -count=1 ./...` → `ok beamsync 8.238s`; `desktop` build + vet + `go test -race .` → `ok desktop 1.186s`; `gofmt -l beamsync/ desktop/` clean.

## Task Commits

Each task was committed atomically:

1. **Task 3.1 (tracer): move the progressWriter into progress.go and verify the happy path** — `f356a77` (refactor)
2. **Task 3.2: run the full verification gate for the progress extraction** — `ad0649a` (test; `--allow-empty` — the guard's designed delta is zero)

**Plan metadata:** _(this commit — docs: complete plan)_

## Files Created/Modified

- `beamsync/progress.go` (created, 45 lines) — `progressWriter` + `Write`, plus the doc comment recording the single-writer invariant. Imports `fmt`, `io`, `time`.
- `beamsync/server.go` (modified, −38 lines) — progress writer declarations deleted; `PendingTransfer`, `//go:embed ui/*` + `uiFS`, `serverState`, `HTTPServer`, `copyChunked`, `chunkBufferPool`, the middleware block and both `Start*E` functions untouched.
- `beamsync/progress_test.go` (verified, unchanged) — already at its target path with the eight `TestProgressWriter*` functions and the three fixture helpers the plan expected.

## Decisions Made

- **Whole block, one unit.** `server.go:220-256` moved as a single contiguous range — doc comment, struct, `Write`. Splitting the struct from its method would have been legal Go but would have separated the field list from the only code that reads it, which is exactly where a "simplification" of the `eventName`/`now`/zero-byte invariants could hide.
- **Diff-before-delete, as in 04-02.** The origin range was captured, the new file written, the two diffed, and only then was the origin deleted. This turns "moved carefully" into a mechanical fact and is why the throttle boundary can be stated as unchanged rather than assumed. It also caught nothing — which is the point.
- **Import block left alone in both files.** The plan expected `progress.go` to need `fmt`/`io`/`time` and said nothing about trimming `server.go`; checking the remaining consumers confirmed `server.go` still needs all three (`copyChunked` for `io`, payload literals for `fmt`, `time.Now` for `time`). Removing `io` here would have broken the build and forced it back in 04-04.
- **Tracer happy path reused, not duplicated.** The plan asked to "write bytes through `Write` and assert the progress callback fires with the `eventName|filename|written|total` payload" — that is verbatim `TestProgressWriterReceivePathEmitsUploadProgress` and `TestProgressWriterSendPathEmitsDownloadProgress`, so both were run explicitly (`go test -race -run '…ReceivePath…|…SendPath…' -v`) rather than copied into a second, weaker test. Same choice 04-01 and 04-02 made for their tracers.
- **`--allow-empty` for the guard task.** Task 3.2's files list includes `progress_test.go`, but PATTERNS established that the eight tests already sit at the target path — the delta is zero by design. An empty commit records the verification outcome and keeps one commit per task without inventing a code change (matching 04-02 Task 2.2).
- **The fixture comment stayed as written.** `progress_test.go:23` says the literal "mirrors the production construction sites in beamsync/server.go". After the move the *type* lives in `progress.go` but the *construction sites* are still in `server.go`, so the comment remains accurate. Rewriting it would have been churn in a file this plan deliberately did not touch.
- **Estimate overshoot recorded honestly.** The plan estimated 12000 tokens; the realized diff is 2887 characters ≈ 721 tokens (chars/4 over `git diff 6cc450d..HEAD`), a ~17× overshoot. A 45-line pure move with zero test churn is far cheaper than the estimate assumed — recorded as measured rather than rounded toward 12000.

## Deviations from Plan

None - plan executed exactly as written.

The only non-conformance is environmental and pre-existing: `golangci-lint` was not re-run. 04-01 documented that the locally installed v1.64.8 cannot decode Go 1.27.4 export data (`export data version 5`) and fails identically on untouched files such as `stats.go`, so `go build` / `go vet` / `go test -race` remain the authoritative gate here. No lint-driven change was made.

## Issues Encountered

- None. Worth naming the intermediate state though: after Task 3.1, `server.go` still *constructs* `progressWriter` in three places while the declaration lives in `progress.go`. That is the intended end state of this plan — relocating those literals is 04-07 (upload handler) and 04-08 (download handler) — and it compiles because both files are in `package beamsync`.
- `go test -race` was run with `-count=1` to bypass the test cache, so the pass is a real execution of the moved code rather than a cached result.

## User Setup Required

None - no external service configuration required.

## Threat Flags

None — no new network surface, auth path, file-access pattern, or trust-boundary change. The move is package-internal. T-04-03-01 (information disclosure through the progress event payload) is mitigated as planned: the payload shape is unchanged because the code is byte-identical, the event name is still the separate `eventName` field rather than a literal, and the per-path tests (`TestReceivePathEmitsUploadProgress`, `TestSenderPathEmitsDownloadProgress`, `TestMultiFileSenderPathEmitsDownloadProgress`) all pass unchanged.

## Known Stubs

None.

## Next Phase Readiness

- `beamsync/progress.go` is now the home for anything progress-shaped. 04-04 (`io_helpers.go`) is next in the risk-ascending order and starts from a 1206-line `server.go`; it will move `copyChunked` and the `chunkBufferPool` declaration, which is the first plan that will actually trim `server.go`'s `io` import usage.
- 04-07 and 04-08 can now construct `progressWriter` from either new handler file without touching `progress.go`; the three literals currently in `server.go` are those plans' move targets, not regressions.
- Phase 5's `SVE-08` (event-name constants) starts from the same string set: `upload_progress` and `download_progress` are still literals at their original construction sites and were not renamed or centralised here.

---

*Phase: 04-go-structural-refactor-server-go-extraction*
*Completed: 2026-11-18*

## Self-Check: PASSED

- `beamsync/progress.go` exists (45 lines) and holds the `progressWriter` type and `Write`.
- `beamsync/server.go` exists (1206 lines) with no `progressWriter` declaration remaining.
- Commits `f356a77` and `ad0649a` are both present in the log.
