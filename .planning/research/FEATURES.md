# Features Research — Quality & Maintainability

**Context:** BeamSync is a personal/open-source Wails v2 desktop app (Go 1.25.5 + Svelte 3). Brownfield quality overhaul — no new user features.

## Table Stakes (Must Have)

These are the minimum for a maintainable open-source desktop app. Without them, contributors can't safely change code.

| Feature | Complexity | Notes |
|---------|-----------|-------|
| **Go unit tests for core logic** | Low | `beamsync/` already has some (`server_test.go`, `auth_tokens_test.go`, etc.). Fill gaps: `eventDispatcher`, `progressWriter`, `tokenStore` edge cases, rate limiter contention. `go test ./...` |
| **Go unit tests for desktop module** | Low | `desktop/` has zero tests. Start with `app.go` — URL builder, version source, event channel behavior. Pure logic only (no Wails mocking). |
| **Frontend linting** | Low | Add ESLint + Svelte plugin to `desktop/frontend/`. Catches unused vars, missing reactivity declarations, a11y issues. One config file + one `package.json` script. |
| **Frontend formatting** | Low | Add Prettier. One config file. Auto-format saves arguments. |
| **Go linting** | Low | `golangci-lint` in the `beamsync/` module. Catches errcheck, ineffassign, govet, staticcheck in one command. |
| **Consistent Go error handling** | Medium | CONCERNS.md flags this. Current code mixes `fmt.Errorf`, `errors.New`, and bare string returns. Adopt: wrapped errors with `fmt.Errorf %w` for sentinel errors, no `errors.Wrap` (stdlib covers it as of Go 1.20). |
| **Unified version source** | Low | `wails.json` productVersion as single source, injected at build time. Removes mismatch between Go hardcode and Svelte hardcode. |
| **Deduplicate URL construction** | Low | Extract `shareURL()` helper. 6 callers, one change. |
| **Merge duplicate progressWriter** | Low | One struct, one `direction` field. |
| **CI — lint + test + build** | Medium | GitHub Actions workflow: `golangci-lint`, `go test ./...`, `vite build`, `go build`. Runs on PR and push to main. Without this, every other quality gate is unenforceable. |
| **Basic CONTRIBUTING.md** | Low | Already exists. Verify it matches reality after changes. |
| **Remove on-disk firewall script copies** | Low | Keep embedded version only. Deletes `beamsync/firewall_setup.sh` and `build/linux/firewall_setup.sh`. |

## Differentiators (Nice to Have)

These raise the bar from "works on my machine" to "someone else could confidently ship this."

| Feature | Complexity | Notes |
|---------|-----------|-------|
| **Integration tests (Go)** | Medium | Test the full send-receive flow using `httptest.Server`. Start sender, start receiver, upload file, download file, verify contents. No Wails involved — just HTTP. |
| **Frontend component tests** | Medium | Add Vitest + `@testing-library/svelte`. Start with the simplest components first (TransferProgress, DevicePanel) — pure rendering with props. Avoid testing App.svelte directly until it's split. |
| **Structured logging** | Medium | Replace `fmt.Printf` and `log.Println` scatter with `slog` (stdlib since Go 1.21). Log lines become grep-able, level-filterable. No need for a framework. |
| **Transfer history persistence** | Low | JSON file in XDG config dir. ~50 lines. Already identified in CONCERNS.md. Low effort, high UX value. |
| **Automated releases** | Medium | GoReleaser or `goreleaser` config + GitHub Actions. Builds for Windows/macOS/Linux, attaches to GitHub Releases. Removes manual build invocation. |
| **Benchmark regression detection** | Low | `go test -bench=. -benchmem` in CI. Store baseline, compare on PR. Catches perf regressions before merge. ~20 lines of CI config. |
| **Svelte type checking** | Low | Add `svelte-check` to frontend CI. Catches prop mismatches, unused stores, missing exports. One npm script. |
| **Dependabot / Renovate** | Zero | One config file. Auto-PRs for dependency updates. Reduces "works on my machine from 2024" drift. |
| **Event channel metrics** | Low | Counter in `eventDispatcher` for dropped events. Surfaces silent failure. |
| **Pre-commit hooks** | Low | `pre-commit` config or husky. Run `go vet`, linter, formatter before every commit. Keeps main green without CI being the gate. |

## Anti-Features (Deliberately Don't Build)

These would add maintenance surface for no user benefit in a personal LAN tool.

| Feature | Reason |
|---------|--------|
| **Interface with one implementation** | Every `interface` before there's a second implementation is speculative. Go conventions tolerate concrete types. |
| **Factory / Provider / DI container** | The app has one server, one app struct, one frontend. Wails provides the binding. A DI framework adds import cost, hides wiring. |
| **Abstracted transport layer** | Currently HTTP. Could wrap in `Transport` interface "for when we add WebRTC." Don't. Add when the second transport is being written. |
| **Feature flags / toggles** | Single-user app. Feature flags are a team coordination tool, not a technical one. |
| **Full mock suite for Wails** | Mocking the Wails runtime is extremely fragile and version-coupled. Test pure Go logic, integration-test HTTP. Accept that Wails shell is manually tested. |
| **E2E framework (Playwright/Cypress)** | Cross-platform desktop E2E for a Wails app means headless Chromium + compiled Go binary + OS window system interaction. Huge CI cost, high flake rate, near-zero ROI for a personal tool. |
| **Code generation** | No protobuf, no OpenAPI codegen, no ent/Prisma-style generators. The schema is simple (transfer records, tokens). Handled types are clearer and faster. |
| **Coverage threshold in CI (e.g., 80% gate)** | Coverage gates cause gamification (write tests for coverage, not quality). Instead: CI runs all tests, flags new uncovered code in PR diff. Go's `-coverprofile` for information, not enforcement. |
| **Separate `beamsync/auth` sub-package splitting** | CONCERNS.md mentions this. Premature — the auth package would be one file with one consumer. Split when there are multiple auth-related files or multiple consumers. |
| **Sharded rate limiter** | Currently single mutex. CONCERNS.md flags it as a performance concern. For a LAN tool handling 1-2 simultaneous transfers, this is not a bottleneck. Profile first, optimize second. |

## Testing Strategy for Wails Go + Svelte

```
Priority order (highest value per line of test):
  1. Go unit tests (beamsync core + desktop app.go)
     — Pure logic. Fast. No deps. Catches most bugs.
  2. Go integration tests (HTTP handlers via httptest.Server)
     — Full request-response cycle. No Wails needed.
  3. Frontend component tests (Vitest + testing-library/svelte)
     — Component rendering, prop reactivity, event emission.
  4. Manual acceptance (no automation)
     — Wails shell, QR scan, drag-drop, sound. Too fragile to automate.
```

**What NOT to test (ponytail):**
- Wails binding layer (mocked integration, high churn with Wails releases)
- Audio playback correctness
- The QR code generation library (trust the dep)
- CSS/visual rendering (no visual regression tests — overkill for a personal tool)

## Dependencies Between Features

```
Tests must come first:
  Add Go unit tests  ──────►  Refactor server.go
  Split App.svelte    ──────►  Add component tests

CI must come after tests:
  Go unit tests exist  ─────►  CI runs go test
  Linting configured   ─────►  CI runs linter

Releases depend on CI:
  CI green on main    ──────►  GoReleaser can tag
  Version unified     ──────►  Release uses single source of truth
```

## Recommendations for This Milestone

**Do first (prerequisites):**
- Go unit tests for `desktop/app.go` (URL builder, version source)
- Fill gap tests in `beamsync/` (eventDispatcher, tokenStore edge cases)
- ESLint + Prettier config for frontend
- `golangci-lint` config for Go
- GitHub Actions CI (lint + test + build)

**Do second (structural):**
- Unify version source
- Deduplicate URL builder, progressWriter
- Consistent Go error handling
- Transfer history persistence

**Skip for this milestone:**
- E2E tests
- Sharded rate limiter
- Sub-package splitting
- GoReleaser (add after CI is solid)
- Benchmark regression detection
