---
phase: 04-go-structural-refactor-server-go-extraction
plan: 06
subsystem: infra
tags: [go, refactor, helpers, generateid, path-collision, reserved-integrity]

# Dependency graph
requires:
  - phase: 03-tests-before-refactoring
    provides: TestGenerateIDReturnsHexValue, TestGenerateIDDoesNotCollideInSmallSample and TestAutoRenamePathFindsNonCollidingName as the pins on the two moved helpers, plus the 33-mutant table whose test names must not change
  - phase: 04-go-structural-refactor-server-go-extraction
    plan: 05
    provides: beamsync/middleware.go extraction, server.go at 1036 lines with its middleware block gone and only the helper cluster still ahead of StartServerE
provides:
  - beamsync/helpers.go — uploadIntegrityHeader, sha256Hex, the two var _ blank assignments, generateID, autoRenamePath, getLocalIPForURL
  - beamsync/server.go trimmed 1036 → 995 lines, its diff purely deletions, and its import block now free of crypto/rand, crypto/sha256 and encoding/hex
affects: [04-07, 04-08, 04-09]

# Actuals (#2632) — same chars/4 scale as the plan's estimate
actuals:
  tokens: 740
  tasks: 2
  commits: 1

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Diff-before-delete, fifth repetition (04-02, 04-03, 04-04, 04-05, now 04-06): the origin range server.go:210-246 was captured to a temp file and md5'd (285027d913b41193348e73a95eefb325), helpers.go:14-50 was diffed against that capture, and HELPERS BLOCK IDENTICAL printed before the origin was deleted — the move is proven a copy, not asserted"
    - "Deletion-only diff as the proof that no call site moved: `git diff HEAD~1 HEAD -- beamsync/server.go | grep '^+[^+]'` returns nothing (0 additions against 41 removals), so the two live call sites cannot have been edited"
    - "Import pruning derived from the moved block, not guessed: `grep -n 'rand\\.\\|sha256\\.\\|hex\\.'` returned exactly four hits at :213, :214, :222 and :225 — all inside 210-246 — before the three imports were removed; fmt/os/filepath/strings/time were each re-counted (46/8/13/8/33 uses) and kept"

key-files:
  created:
    - beamsync/helpers.go
  modified:
    - beamsync/server.go

key-decisions:
  - "Moved the helper cluster as one contiguous block (server.go:210-246) rather than symbol by symbol: the seven declarations are already adjacent, so a single capture-and-diff is both smaller and stronger evidence than seven separate moves"
  - "helpers.go is header + the captured block verbatim, in the original source order — uploadIntegrityHeader → sha256Hex → var _ → var _ → generateID → autoRenamePath → getLocalIPForURL — so a reader comparing the two files sees the same sequence"
  - "Import block is the eight stdlib packages the block actually needs (crypto/rand, crypto/sha256, encoding/hex, fmt, os, path/filepath, strings, time) in gofmt order with no inline comments, matching the repo's zero-inline-comment-on-imports convention"
  - "Deleted crypto/rand, crypto/sha256 and encoding/hex from server.go in the same commit as the move rather than deferring: their last four uses were all inside the moved block, so Go's unused-import rule forces the deletion and deferring it would have left a file that cannot build (D-08/D-09)"
  - "Kept fmt, os, path/filepath, strings and time in server.go — all five still have live uses outside the moved block (46, 8, 13, 8 and 33 respectively), so pruning any of them would have broken 04-07/04-08's source file two plans early"
  - "Preserved both //nolint:unused // reserved markers and both var _ = blank assignments exactly: dropping either half changes the file's lint posture, which `go build` would not catch but CI's golangci-lint would"
  - "Did not touch //go:embed ui/*.html ui/*.png or var uiFS embed.FS (D-06 / PITFALLS 1d) — they remain at server.go:33-34, and a package-wide grep confirms all 5 uiFS hits are still in server.go"
  - "Task 6.2 produced no diff — it is the verification-only task — so it has no commit of its own; its gate output is recorded here instead, and the plan metadata commit is the second commit in the sequence"

patterns-established:
  - "When a helper set is already contiguous in the source, move it as one block and prove it with one diff — seven tiny moves would multiply the chances of a transcription slip without adding any evidence"

requirements-completed: [STR-09]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "The seven helper declarations (uploadIntegrityHeader, sha256Hex, both var _ blank assignments, generateID, autoRenamePath, getLocalIPForURL) moved into beamsync/helpers.go as a byte-identical copy of server.go:210-246"
    requirement: "STR-09"
    verification:
      - kind: other
        ref: "diff of beamsync/server.go:210-246 (pre-move, md5 285027d913b41193348e73a95eefb325) against beamsync/helpers.go:14-50 => no differences"
        status: pass
      - kind: other
        ref: "grep -n 'const uploadIntegrityHeader\\|^func sha256Hex\\|^func generateID\\|^func autoRenamePath\\|^func getLocalIPForURL\\|^var _ =' beamsync/helpers.go => exactly 7 hits at lines 14, 16, 21, 22, 24, 32, 48"
        status: pass
    human_judgment: false
  - id: D2
    description: "server.go no longer holds any of the moved helper declarations and no longer imports crypto/rand, crypto/sha256 or encoding/hex; its change is deletions only"
    requirement: "STR-09"
    verification:
      - kind: other
        ref: "grep -nE 'const uploadIntegrityHeader|^func generateID|^func sha256Hex|^func autoRenamePath|^func getLocalIPForURL' beamsync/server.go => no matches"
        status: pass
      - kind: other
        ref: "git diff HEAD~1 HEAD -- beamsync/server.go | grep '^+[^+]' => empty; 41 lines removed, 0 added; 1036 -> 995 lines"
        status: pass
    human_judgment: false
  - id: D3
    description: "generateID still returns a hexadecimal identifier of the same length and shape after the move — must_have truth 2"
    requirement: "STR-09"
    verification:
      - kind: unit
        ref: "beamsync/server_test.go#TestGenerateIDReturnsHexValue"
        status: pass
      - kind: unit
        ref: "beamsync/server_test.go#TestGenerateIDDoesNotCollideInSmallSample"
        status: pass
    human_judgment: false
  - id: D4
    description: "autoRenamePath still returns a non-colliding path for an existing file — must_have truth 3"
    requirement: "STR-09"
    verification:
      - kind: unit
        ref: "beamsync/server_test.go#TestAutoRenamePathFindsNonCollidingName"
        status: pass
    human_judgment: false
  - id: D5
    description: "The reserved-integrity symbols keep their unused status and lint posture — both //nolint:unused // reserved markers and both var _ blank assignments survived the move"
    requirement: "STR-09"
    verification:
      - kind: other
        ref: "grep -c 'nolint:unused // reserved' beamsync/helpers.go => 2; grep -c '^var _ =' beamsync/helpers.go => 2; uploadIntegrityHeader and sha256Hex each appear exactly twice in the package, both in helpers.go"
        status: pass
      - kind: other
        ref: "(cd beamsync && go vet ./...) => clean; gofmt -l beamsync/ => no output"
        status: pass
    human_judgment: false
  - id: D6
    description: "The //go:embed ui/*.html ui/*.png directive and var uiFS embed.FS stayed in server.go (D-06 / PITFALLS 1d) — zero cross-file uiFS references"
    requirement: "STR-09"
    verification:
      - kind: other
        ref: "grep -n 'go:embed ui/' beamsync/server.go => :33; grep -n 'var uiFS embed.FS' => :34; grep -c uiFS beamsync/server.go => 5; grep -rn uiFS beamsync/ excluding server.go => nothing"
        status: pass
    human_judgment: false
  - id: D7
    description: "Every helper still has a live caller after the move — no symbol was orphaned and no call site was lost"
    requirement: "STR-09"
    verification:
      - kind: other
        ref: "grep -rn: generateID => server.go:382 + server_test.go:80,94; autoRenamePath => server.go:499 + server_test.go:71; getLocalIPForURL => events.go:161; uploadIntegrityHeader/sha256Hex => helpers.go:21,22 only (intentionally unused, as before)"
        status: pass
    human_judgment: false
  - id: D8
    description: "No test function was renamed or moved (CONTEXT D-13), so the Phase 3 33-mutant table still keys on every name"
    requirement: "STR-09"
    verification:
      - kind: other
        ref: "the commit touches no _test.go file at all — git diff --stat HEAD~1 HEAD lists only beamsync/helpers.go and beamsync/server.go"
        status: pass
    human_judgment: false
  - id: D9
    description: "Full Phase 4 gate green: beamsync build + vet + test -race, desktop build + vet + test -race"
    requirement: "STR-09"
    verification:
      - kind: other
        ref: "(cd beamsync && go build ./... && go vet ./... && go test -race -count=1 ./...) => ok beamsync 2.968s"
        status: pass
      - kind: other
        ref: "(cd desktop && go build . && go vet . && go test -race -count=1 .) => ok desktop 1.191s; gofmt -l beamsync/ desktop/ => no output"
        status: pass
    human_judgment: false
  - id: D10
    description: "Prohibition: the extraction changed no runtime behavior, no event payload and no HTTP response body"
    verification: []
    human_judgment: true
    rationale: "A pure move is proven by the byte-identical block diff plus server.go's deletion-only diff plus the full -race suite in both modules; a reviewer confirms from the diffstat (50 added in helpers.go, 41 removed from server.go, no other file edited) that no behavior surface was touched"

# Metrics
duration: 2min
completed: 2026-11-18
status: complete
---

# Phase 4 Plan 06: `helpers.go` Extraction Summary

**The seven-declaration helper cluster — including the two `//nolint:unused // reserved` integrity symbols and their blank-assignment guards — now lives in `beamsync/helpers.go` as a byte-identical copy of `server.go:210-246`, leaving `server.go` at 995 lines with a purely deletion-only diff, `crypto/rand`/`crypto/sha256`/`encoding/hex` pruned from its imports, and the `uiFS` embed untouched where D-06 requires it.**

## Performance

- **Duration:** ~2 min
- **Started:** 2026-11-18T12:31:43Z
- **Completed:** 2026-11-18T12:34:05Z
- **Tasks:** 2 (1 committed + Task 6.2, verification-only)
- **Files modified:** 2 (1 created, 1 modified)

## Accomplishments

- Created `beamsync/helpers.go` (50 lines): `uploadIntegrityHeader` (14), `sha256Hex` (16), `var _ = uploadIntegrityHeader` (21), `var _ = sha256Hex` (22), `generateID` (24), `autoRenamePath` (32), `getLocalIPForURL` (48). One parenthesized stdlib import block of the eight packages the block actually needs, no file-level doc comment — the `localip.go` file shape PATTERNS.md names as the primary analog.
- Proved the move is a copy: `server.go:210-246` was captured to a temp file and md5'd as `285027d913b41193348e73a95eefb325`; `helpers.go:14-50` was diffed against that capture and `HELPERS BLOCK IDENTICAL` printed before the origin was deleted. Fifth run of the 04-02/04-03/04-04/04-05 discipline. (The first range guess, `12,48`, was wrong by two lines and the diff said so — same failure mode 04-04 and 04-05 both recorded; re-deriving from `wc -l` fixed it without touching any code.)
- Trimmed `beamsync/server.go` from 1036 to 995 lines (−41: the 37-line block, its trailing blank separator, and the three dead imports) with **zero added lines**. `git diff HEAD~1 HEAD -- beamsync/server.go | grep '^+[^+]'` is empty, so neither the `generateID()` call at `:382` nor the `autoRenamePath()` call at `:499` was touched.
- Pruned `crypto/rand`, `crypto/sha256` and `encoding/hex` from `server.go`. The deletions are forced, not cosmetic: those three packages had exactly four uses, at `:213`, `:214`, `:222` and `:225` — all inside the moved range — and Go rejects an unused import, so the tree does not build without the prune.
- Kept `fmt`, `os`, `path/filepath`, `strings` and `time` deliberately: they still have 46, 8, 13, 8 and 33 uses in `server.go` outside the helper block, so removing them would have broken 04-07/04-08's source file two plans early. `server.go` now carries 15 imports, down from 18.
- Preserved the lint posture exactly. Both `//nolint:unused // reserved` markers and both `var _ = …` blank assignments travelled verbatim; `uploadIntegrityHeader` and `sha256Hex` each still appear exactly twice in the package, and both appearances are in `helpers.go`. Removing either half would have left `go build` green but turned CI's `golangci-lint` red.
- Left the embed alone (D-06 / PITFALLS 1d): `//go:embed ui/*.html ui/*.png` sits at `server.go:33` and `var uiFS embed.FS` at `:34`; `grep -c uiFS beamsync/server.go` reports 5 (1 declaration + 4 uses) and no other file in the package references it.
- Every moved symbol retains a live caller: `generateID` → `server.go:382` (and `server_test.go:80,94`), `autoRenamePath` → `server.go:499` (and `server_test.go:71`), `getLocalIPForURL` → `events.go:161`. Nothing was orphaned by the move.
- Full gate green in both modules: `beamsync` build + vet, `go test -race -count=1 ./...` → `ok beamsync 2.968s`; `desktop` build + vet + `go test -race -count=1 .` → `ok desktop 1.191s`; `gofmt -l beamsync/ desktop/` clean.

## Task Commits

Each task was committed atomically:

1. **Task 6.1: move the shared helpers into helpers.go** — `ae0429f` (refactor)

**Task 6.2** (full verification gate) is verification-only and produced no working-tree change, so it has no commit of its own; its output is recorded above.

**Plan metadata:** _(this commit — docs: complete plan)_

## Files Created/Modified

- `beamsync/helpers.go` (created, 50 lines) — the seven-declaration helper cluster, byte-identical to the `server.go` range it replaces. Imports `crypto/rand`, `crypto/sha256`, `encoding/hex`, `fmt`, `os`, `path/filepath`, `strings`, `time`.
- `beamsync/server.go` (modified, −41 lines, 1036 → 995) — the helper block deleted; `crypto/rand`, `crypto/sha256`, `encoding/hex` removed from the import block; nothing added, nothing else touched (`PendingTransfer`, `//go:embed ui/*` + `uiFS`, `serverState`, `HTTPServer`, the middleware call sites, both `Start*E` functions with every route registration intact).

## Decisions Made

- **One contiguous block, original order.** The seven declarations were already adjacent at `server.go:210-246`, so they moved as a single unit in source order rather than as seven separate moves. One capture-and-diff is both smaller and stronger evidence than seven of them.
- **Prune the imports in the same commit as the move.** Deferring would leave a `server.go` that does not compile between commits, contradicting "each extraction is its own commit so a revert is surgical" (D-09).
- **Keep `fmt`/`os`/`path/filepath`/`strings`/`time`.** They are shared with the still-in-place upload and download bodies; each was re-counted before the import edit rather than assumed.
- **No test file touched.** Unlike 04-04 and 04-05, this plan has no test co-location step — the three tests that pin the moved helpers (`TestAutoRenamePathFindsNonCollidingName`, `TestGenerateIDReturnsHexValue`, `TestGenerateIDDoesNotCollideInSmallSample`) already live in `server_test.go` and reach the symbols through the package scope, so leaving them there is a no-op for D-13.
- **Task 6.2 gets no commit.** It is the gate, not a change; inventing an empty commit would make the commit list look uniform at the cost of truth.

## Deviations from Plan

None - plan executed exactly as written.

Two factual notes recorded without any code impact:

- The plan's `key_links` says `generateID` "and `getLocalIPForURL` stay called from `StartServerE`". `generateID` does (`server.go:382`), but `getLocalIPForURL`'s only caller is `events.go:161` — inside `startBootstrapRotator`, which 04-02 moved into `events.go`. Both symbols are package-scope, so the move compiles and behaves identically; only the plan's description of the call site is out of date.
- `golangci-lint` was not run. This is the same pre-existing environmental limitation recorded in 04-01 through 04-05: the locally installed v1.64.8 cannot decode Go 1.27.4 export data (`export data version 5`) and fails identically on untouched files. `go build`, `go vet`, `gofmt -l` and `go test -race` remain the authoritative gate, and no lint-driven change was made.

## Issues Encountered

- The first attempt to diff the moved block used `helpers.go:12-48` when the file is 50 lines (the 13-line header plus the 37-line block), so `diff` reported a two-line mismatch at both ends. Re-deriving the range as `14,50` produced `HELPERS BLOCK IDENTICAL`. Third occurrence of the same hand-guessed-range mistake across 04-04, 04-05 and 04-06 — in all three cases the code was already correct and only the verification range was wrong.

## User Setup Required

None - no external service configuration required.

## Threat Flags

None — no new network surface, auth path, file-access pattern, or trust-boundary change. The move is package-internal and touches no exported symbol; `desktop/app.go`'s `beamsync` surface is unaffected.

T-04-06-01 (tampering through a helper move dropping a call site) is mitigated as planned: the moved block is a byte-identical copy of its origin, `server.go`'s diff contains zero added lines so neither surviving call site (`generateID` at `:382`, `autoRenamePath` at `:499`) was edited, `getLocalIPForURL`'s single caller in `events.go:161` is unchanged, and the three tests that pin the helpers — plus the whole `-race` suite in both modules — are green.

## Known Stubs

None.

## Next Phase Readiness

- `beamsync/helpers.go` is now the home for the shared helpers. 04-07 (`upload_handler.go`) is next and starts from a 995-line `server.go`; it converts the `/upload` closure into an `*HTTPServer` method, calling `autoRenamePath` in `helpers.go` and adding the `uploadDir`/`emit` fields D-04/D-10 require.
- `server.go` still imports `bufio`, `encoding/json`, `io`, `mime`, `mime/multipart` and `strings` solely because the upload and download bodies are still inline; 04-07 and 04-08 are the last chance to prune them, and neither `fmt`/`os`/`path/filepath`/`time` can follow those bodies out.
- `//go:embed ui/*` + `uiFS` remain at `server.go:33-34` and must stay there through the final trim (04-09) — a package-wide `rg uiFS` must keep reporting exactly 5 hits, all in `server.go`.

---

*Phase: 04-go-structural-refactor-server-go-extraction*
*Completed: 2026-11-18*

## Self-Check: PASSED

- `beamsync/helpers.go` exists (50 lines) and defines all seven moved declarations at lines 14, 16, 21, 22, 24, 32 and 48.
- `beamsync/server.go` exists (995 lines) with no helper declaration and no `crypto/rand`/`crypto/sha256`/`encoding/hex` import.
- `grep -n 'go:embed ui/' beamsync/server.go` → `:33`; `grep -c uiFS beamsync/server.go` → 5; no other file references `uiFS`.
- `grep -c 'nolint:unused // reserved' beamsync/helpers.go` → 2; `grep -c '^var _ =' beamsync/helpers.go` → 2.
- Full gate green: `beamsync` build/vet/test-race `ok 2.968s`, `desktop` build/vet/test-race `ok 1.191s`, `gofmt -l` clean.
- Commit `ae0429f` is present in the log and contains only `beamsync/helpers.go` and `beamsync/server.go`.
