# Requirements — BeamSync Quality Overhaul

## v1 Requirements (this milestone)

### Phase 1: Foundation — Event System & Version
- [x] EVT-01 — Resize `eventDispatcher` buffer from 256 to 1024
- [x] EVT-02 — Resize `eventChan` buffer from 100 to 512
- [x] EVT-03 — Add drop counters to event dispatcher
- [x] VER-01 — Unify version source to `wails.json` alone
- [x] VER-02 — Remove hardcoded version strings from `desktop/app.go`
- [x] VER-03 — Pass version to Svelte frontend via Wails bridge

### Phase 2: Tooling & DevEx
- [x] TOL-01 — Configure `golangci-lint` for `beamsync/` module (govet, staticcheck, errcheck, gosec, errorlint)
- [x] TOL-02 — Configure `golangci-lint` for `desktop/` module
- [x] TOL-03 — Add ESLint + `eslint-plugin-svelte3` to `desktop/frontend/`
- [x] TOL-04 — Add Prettier config to `desktop/frontend/`
- [x] TOL-05 — Scaffold Vitest in `desktop/frontend/`
- [x] TOL-06 — Add `svelte-check` to frontend tooling
- [x] TOL-07 — Configure GitHub Actions CI: lint → test → build (per-module)

### Phase 3: Tests Before Refactoring
- [x] TST-01 — Add unit tests for `eventDispatcher` (normal send, full buffer drop, concurrent access)
- [x] TST-02 — Add unit tests for `progressWriter` / `downloadProgressWriter` (write, progress events, completion)
- [x] TST-03 — Add unit tests for `copyChunked` I/O helper
- [x] TST-04 — Add unit tests for `desktop/app.go` URL builder, version source, event channel
- [x] TST-05 — Add integration test for full send-receive flow via `httptest.Server`
- [x] TST-06 — Add integration test for token auth (valid, expired, wrong scope, wrong IP)

### Phase 4: Go Structural Refactor
- [x] STR-01 — Extract rate limiter from `server.go` into `rate_limiter.go`
- [x] STR-02 — Extract progress tracking types into `progress.go`
- [x] STR-03 — Merge duplicate `progressWriter` / `downloadProgressWriter` into single type
- [x] STR-04 — Extract I/O helpers into `io_helpers.go`
- [x] STR-05 — Extract HTTP middleware into `middleware.go`
- [x] STR-06 — Extract event dispatcher into `events.go`
- [x] STR-07 — Extract upload handler into `upload_handler.go`
- [x] STR-08 — Extract download handler into `download_handler.go`
- [x] STR-09 — Trim `server.go` to orchestration only (StartServer, StartSender, route setup)
- [x] STR-10 — Deduplicate URL construction pattern — add `shareURL()` helper on App struct

### Phase 5: Svelte Component Split
- [x] SVE-01 — Create Wails store module (`wails.js`) to centralize `window.runtime.*` access
- [x] SVE-02 — Extract `ReceiveView.svelte` (QR display, receive controls)
- [x] SVE-03 — Extract `SendView.svelte` (file drop area, send controls)
- [x] SVE-04 — Extract `SettingsView.svelte` (app settings panel)
- [x] SVE-05 — Extract `AboutView.svelte` (version, credits)
- [x] SVE-06 — Extract `ToastManager.svelte` (notification toasts)
- [x] SVE-07 — Extract `ProgressOverlay.svelte` (transfer progress bars)
- [x] SVE-08 — Define event name constants in shared Go/Svelte file
- [x] SVE-09 — Add Vitest component tests for extracted components

### Phase 6: Error Handling & Logging
- [x] ERR-01 — Adopt Go sentinel errors for expected failure cases
- [x] ERR-02 — Convert ad-hoc string errors to `fmt.Errorf("%w: ...")` wrapping
- [x] ERR-03 — Replace `fmt.Printf` and `log.Println` scatter with `slog` (stdlib)
- [x] ERR-04 — Add structured fields (request ID, session, component) to log lines

### Phase 7: CI/CD & Release
- [x] CID-01 — GitHub Actions release workflow (tag → build matrix → GitHub Release)
- [x] CID-02 — Dependabot config for Go modules and npm
- [x] CID-03 — Add benchmark comparison to CI (`go test -bench=. -benchmem`)

### Phase 8: Documentation & Cleanup
- [x] DOC-01 — Update ARCHITECTURE.md to reflect new structure
- [x] DOC-02 — Update CONTRIBUTING.md to match current DevEx
- [x] DOC-03 — Remove duplicate on-disk firewall scripts (`beamsync/firewall_setup.sh`, `build/linux/firewall_setup.sh`)
- [x] DOC-04 — Add transfer history persistence (JSON file in XDG config dir)
- [x] DOC-05 — Verify README.md matches reality

## Out of Scope

- E2E testing (Playwright/Cypress) — too fragile for Wails desktop
- Sharded rate limiter — profile first, optimize second
- Sub-package splitting in `beamsync/` — keep `package beamsync`
- Interface-only abstractions — no interfaces with one implementation
- DI containers or factory patterns
- Coverage gates in CI — information, not enforcement
- Feature flags / toggles

## REQ-ID Index

| Prefix | Category | Count |
|--------|----------|-------|
| EVT | Event System | 3 |
| VER | Version | 3 |
| TOL | Tooling | 7 |
| TST | Testing | 6 |
| STR | Structural | 10 |
| SVE | Svelte | 9 |
| ERR | Error Handling | 4 |
| CID | CI/CD | 3 |
| DOC | Documentation | 5 |

---
*Last updated: 2026-07-20 after research*
