---
phase: 03-tests-before-refactoring
plan: 03
subsystem: testing
tags: [go, desktop, wails, testing, url-parsing, config-persistence, embed, gitignore]
requires:
  - phase: 02-tooling-devex
    provides: golangci-lint configs for both Go modules and the CI test-go job that runs the desktop module
provides:
  - desktop/app.go pure-function unit test suite (shareURL, token extraction, port extraction, config load/save round-trip, save-directory management)
  - committed desktop/frontend/dist/.gitkeep placeholder that unblocks go test compilation in the desktop module on a fresh clone
  - root .gitignore and desktop/.gitignore exceptions that un-ignore only the embed placeholder and keep the rest of dist/ ignored
  - TST-04 requirement satisfied
affects:
  - 04-go-structural-refactor
  - 06-error-handling-and-logging
actuals:
  tokens: 5554
  tasks: 3
  commits: 3
tech-stack:
  added: []
  patterns:
    - Environment-redirection fixture (HOME/XDG_CONFIG_HOME/APPDATA/LOCALAPPDATA/USERPROFILE) for hermetic user-config filesystem tests
    - Table-driven pure-function tests with t.Run subtests, stdlib testing only, no assertion library
    - go:embed placeholder idiom - committed .gitkeep satisfies an `all:` embed pattern so a module compiles without a frontend build
key-files:
  created:
    - desktop/app_test.go
    - desktop/frontend/dist/.gitkeep
  modified:
    - .gitignore
    - desktop/.gitignore
key-decisions:
  - "D-11: un-ignore the embed placeholder in BOTH .gitignore files using the dist-glob form plus a negation for the .gitkeep placeholder - git cannot re-include a file whose parent directory is excluded, so a negation in desktop/.gitignore alone is not sufficient"
  - "D-12: scope desktop Go commands to the root package (`go test .`) - `go test ./...` also compiles a stray Go package vendored inside desktop/frontend/node_modules"
  - "Fixture redirects %USERPROFILE% alongside HOME/XDG_CONFIG_HOME/APPDATA/LOCALAPPDATA so the config home-prefix guard is hermetic on Windows runners as well as Unix"
requirements-completed:
  - TST-04
duration: 14min
completed: "2026-11-12"
status: complete
coverage:
  - id: D1
    description: desktop/app.go pure-function unit test suite covering shareURL endpoint construction, token/port URL extraction, config load-save round-trip with the home-prefix guard, and the three ensureSaveDir branches
    requirement: TST-04
    verification:
      - kind: unit
        ref: "desktop/app_test.go#TestShareURLEmbedsStoredEndpoint"
        status: pass
      - kind: unit
        ref: "desktop/app_test.go#TestExtractTokenFromURL"
        status: pass
      - kind: unit
        ref: "desktop/app_test.go#TestExtractPortFromURL"
        status: pass
      - kind: unit
        ref: "desktop/app_test.go#TestLoadConfigRoundTripsSavePathInsideHome"
        status: pass
      - kind: unit
        ref: "desktop/app_test.go#TestLoadConfigClearsSavePathOutsideHome"
        status: pass
      - kind: unit
        ref: "desktop/app_test.go#TestEnsureSaveDirRejectsEmptyPath"
        status: pass
      - kind: unit
        ref: "desktop/app_test.go#TestEnsureSaveDirCreatesMissingDirectory"
        status: pass
      - kind: unit
        ref: "desktop/app_test.go#TestEnsureSaveDirReplacesFileWithDirectory"
        status: pass
    human_judgment: false
  - id: D2
    description: desktop module test compilation unblocked - the committed desktop/frontend/dist/.gitkeep satisfies the //go:embed all:frontend/dist pattern, so `go test` in desktop/ runs on a fresh clone with no frontend build
    verification:
      - kind: other
        ref: "git check-ignore desktop/frontend/dist/.gitkeep (not ignored) + (cd desktop && go test .) compiles and passes"
        status: pass
    human_judgment: false
---

# Phase 3 Plan 3: desktop/app.go Pure-Function Test Suite

**Nine unit tests now pin the desktop share-URL, URL-parsing, config round-trip, and save-directory contracts — and the desktop Go module compiles for tests on a fresh clone without a frontend build**

## Performance

- **Duration:** 14 min
- **Started:** 2026-11-12T11:27:00Z
- **Completed:** 2026-11-12T11:41:18Z
- **Tasks:** 3
- **Files modified:** 5

## Accomplishments

- **Desktop module test compilation unblocked (Task 1.1).** `desktop/main.go` embeds
  `all:frontend/dist`, but `desktop/frontend/dist` was gitignored — so `go test` in `desktop/`
  failed with `pattern all:frontend/dist: no matching files found` on any fresh clone.
  Committed an empty `desktop/frontend/dist/.gitkeep` (the `all:` prefix includes dotfiles) and
  un-ignored it in **both** ignore files using the `dist/*` glob plus a negation for the
  `.gitkeep` placeholder. The `dist/*`
  form is mandatory: git cannot re-include a file whose parent directory is excluded, so a
  negation in `desktop/.gitignore` alone would have been silently ineffective. Every other file
  under `dist/` (the built `index.html`, `assets/`, favicons) stays ignored.
- **Share-URL contract pinned (Task 1.2).** `TestShareURLEmbedsStoredEndpoint` constructs
  `&App{currentIP: "192.168.1.10"}` directly — no Wails runtime context needed — pins
  `BEAMSYNC_ENABLE_TLS` so `beamsync.ServerScheme()` is deterministically `http`, and asserts
  `http://192.168.1.10:8080/?token=tok` plus the empty-port variant `http://192.168.1.10:/?token=tok`.
  Because the stored IP is set, no network lookup occurs.
- **Full pure-function suite added (Task 2.1).**
  - `TestExtractTokenFromURL` — 6 table-driven cases: plain query token, ampersand truncation
    (`?token=abc&x=1` -> `abc`), hash truncation (`?token=abc#frag` -> `abc`), whitespace trimming
    (`?token=  abc123  ` -> `abc123`), missing token -> `""`, empty token segment -> `""`.
  - `TestExtractPortFromURL` — 5 table-driven cases: scheme+path stripping
    (`http://192.168.1.10:8080/share?token=tok` -> `8080`), no port -> `""`, no colon at all
    (`192.168.1.10/share?token=tok` -> `""`), IPv6 literal with port
    (`http://[::1]:8080/share?token=tok` -> `8080`, last-colon semantics), and a bare
    `http://192.168.1.10:9090` -> `9090`.
  - `TestLoadConfigRoundTripsSavePathInsideHome` — with the user config home redirected to a
    temp dir, a `SavePath` under that home survives the `saveConfig` -> `loadConfig` round-trip.
  - `TestLoadConfigClearsSavePathOutsideHome` — a `SavePath` outside the redirected home is
    cleared on load, proving the persisted config cannot target an arbitrary filesystem location.
  - `TestEnsureSaveDirRejectsEmptyPath` — empty path returns an error.
  - `TestEnsureSaveDirCreatesMissingDirectory` — a missing path is created as a directory.
  - `TestEnsureSaveDirReplacesFileWithDirectory` — a file at the save path is renamed to
    `<path>.bin.bak` and replaced by a directory (the backup is asserted to still be a regular
    file, so the prior content is not silently destroyed).
- **Verification.** `(cd desktop && go test .)` and `(cd desktop && go test -race .)` both pass
  (9 test functions, 18 subtests). `(cd desktop && go vet .)` is clean and
  `gofmt -l desktop/app_test.go` outputs nothing.

## Task Commits

Each task was committed atomically:

1. **Task 1.1: un-ignore the desktop frontend embed placeholder** - `844f8dd` (chore)
2. **Task 1.2: desktop shareURL emits the stored endpoint URL** - `e219ebb` (test)
3. **Task 2.1: desktop app.go pure-function test suite** - `8742a3e` (test)

**Plan metadata:** final close-out commit `docs(03-03): complete desktop app test plan`

## Files Created/Modified

- `desktop/app_test.go` - New test file: the `desktop/` module's first test file. Covers
  `shareURL`, `extractTokenFromURL`, `extractPortFromURL`, the `loadConfig`/`saveConfig`
  round-trip with the home-prefix guard, and all three `ensureSaveDir` branches (created)
- `desktop/frontend/dist/.gitkeep` - Empty placeholder satisfying `//go:embed all:frontend/dist`
  so the desktop module compiles for tests without a frontend build (created)
- `.gitignore` - Root ignore entry changed from `desktop/frontend/dist/` to
  `desktop/frontend/dist/*` plus a negation for the `.gitkeep` placeholder (modified)
- `desktop/.gitignore` - Sub-module ignore entry changed from `frontend/dist` to
  `frontend/dist/*` plus a negation for the `.gitkeep` placeholder (modified)
- `.planning/phases/03-tests-before-refactoring/deferred-items.md` - Pre-existing, out-of-scope
  lint finding logged for a future plan (created)

## Decisions Made

- **D-11 — both ignore files must change, using the `dist/*` glob plus a `.gitkeep` negation.** The
  `dist/` form keeps the parent directory excluded, so git never descends into it and the
  negation never applies. The `dist/*` form lets git descend, applies the negation to
  `.gitkeep`, and leaves every built artifact ignored.
- **D-12 — desktop Go commands target the root package only.** `go test .` / `go build .` /
  `go vet .` in `desktop/`, not `./...`, which also compiles the stray Go package vendored
  inside `desktop/frontend/node_modules/flatted/golang/pkg/flatted`.
- **Redirect `%USERPROFILE%` alongside the other home variables.** `os.UserHomeDir()` reads
  `%USERPROFILE%` on Windows and `$HOME` elsewhere, so the config fixture must cover both for
  the home-prefix guard to be hermetic on every runner.
- **`&App{}` constructed directly, no Wails runtime.** The pure functions under test never
  touch the Wails context, so `desktop/app_test.go` stays `package main` and needs no runtime
  bootstrap (D-10).
- **The share-URL test pins `BEAMSYNC_ENABLE_TLS`.** `beamsync.ServerScheme()` reads that env
  var; without pinning it the expected URL would depend on the developer's shell environment.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Redirect `%USERPROFILE%` in the config round-trip fixture**
- **Found during:** Task 2.1 (desktop app.go pure-function test suite)
- **Issue:** the `redirectUserConfigHome` helper set `HOME`, `XDG_CONFIG_HOME`, `APPDATA`, and
  `LOCALAPPDATA`, and its doc comment claimed it covered the "USERPROFILE fallbacks" — but it
  never set `USERPROFILE`. `os.UserHomeDir()` reads `%USERPROFILE%` on Windows, so on a Windows
  runner the `loadConfig` home-prefix guard would have compared the persisted save path against
  the *developer's real profile* instead of the fixture. The
  "out-of-home save path is cleared" assertion would have passed for the wrong reason (or
  failed outright), and the test would have read or written the real user profile.
- **Fix:** added `t.Setenv("USERPROFILE", dir)` to the helper, matching the comment's stated
  intent. Harmless on Unix (the variable is not read there).
- **Files modified:** `desktop/app_test.go`
- **Verification:** `(cd desktop && go test -race .)` pass; `(cd desktop && go vet .)` clean;
  `gofmt -l desktop/app_test.go` empty.
- **Committed in:** `8742a3e` (part of task commit)

---

**Total deviations:** 1 auto-fixed (1 missing critical)
**Impact on plan:** The fix makes the existing config fixture match its documented intent and
keeps the test hermetic on all three target platforms. No scope creep — no production code was
touched.

## Issues Encountered

- **`golangci-lint run .` fails in `desktop/` with `main.go:19:9: undefined: wails (typecheck)`.**
  This is **pre-existing and unrelated to this plan** — `main.go` imports
  `github.com/wailsapp/wails/v2`, whose import path's last element (`v2`) differs from the
  package name (`wails`), and golangci-lint v1.64's `typecheck` analyzer does not resolve the
  package name for `/v2`-suffixed paths. `go build`, `go test`, and `go vet` are unaffected.
  Verified pre-existing by reproducing the same error in a clean copy of the `desktop/` module
  with `desktop/app_test.go` removed. Logged to
  `.planning/phases/03-tests-before-refactoring/deferred-items.md` rather than fixed, because
  `desktop/main.go` is outside this plan's `files_modified` list and the fix (an explicit import
  alias) is a production-file change that deserves its own plan and review.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- **TST-04 is satisfied.** The `desktop/` module now has a real test file and a green
  `go test -race .` — it was the last Go module in the repo with zero tests.
- **Phase 4 (Go Structural Refactor) can proceed.** The desktop pure-function contract
  (share URL, URL parsing, config persistence, save-directory management) is now pinned as
  executable behavior, so the Phase 4/5 restructuring has a safety net on the desktop side in
  addition to the `beamsync/` side delivered by plans 03-01 and 03-02.
- **A fresh clone can now run the desktop Go tests** with no frontend build step — the CI
  `test-go` job's `npm ci && npm run build` prerequisite is no longer needed for `go test`.
- **Plan 03-02 (`progressWriter` + the live `download_progress` fix) is the remaining Phase 3
  work.** Note the `download_progress` event-name contract is unaffected by this plan — this
  plan adds no emitters and touches no event names.

## Self-Check: PASSED

- Created files exist: `desktop/app_test.go` (FOUND), `desktop/frontend/dist/.gitkeep` (FOUND),
  `.planning/phases/03-tests-before-refactoring/deferred-items.md` (FOUND).
- Modified files exist: `.gitignore` (FOUND), `desktop/.gitignore` (FOUND).
- Commits exist: `844f8dd` (FOUND), `e219ebb` (FOUND), `8742a3e` (FOUND).
- Task 1.1 verify re-run: `git check-ignore` reports the placeholder as not ignored and
  `git ls-files --error-unmatch desktop/frontend/dist/.gitkeep` resolves — PASS.
- Task 1.2 / 2.1 verify re-run: `(cd desktop && go test .)` PASS; `(cd desktop && go test -race .)`
  PASS (9 functions, 18 subtests); `(cd desktop && go vet .)` PASS; `gofmt -l desktop/app_test.go`
  outputs nothing — PASS.
- All acceptance criteria re-verified against the plan: token ampersand/hash truncation and
  whitespace trim; port scheme/path stripping and empty-without-colon; in-home save path
  preserved and out-of-home save path cleared; empty/create/replace save-directory branches — all
  pass.

---

*Phase: 03-tests-before-refactoring*
*Completed: 2026-11-12*
