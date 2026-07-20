# Research Summary — Quality Overhaul

**Date:** 2026-07-20
**Scope:** BeamSync Go + Wails v2 + Svelte 3 desktop app

---

## Key Findings

1. **Three monoliths drive the work**: `beamsync/server.go` (1726 lines), `desktop/app.go` (734 lines), and `App.svelte` (2160 lines) all need file-level splitting—no sub-packages, no new abstractions, just reorganize into focused files within the same package.

2. **Event system has silent drops at two levels** — `eventDispatcher` (cap 256) and `eventChan` (cap 100) both use non-blocking sends with `default:`. Under burst load (50 files), progress events and `device_connected` can vanish silently. Resize both buffers before any splitting work.

3. **Version source is triplicated** — `wails.json`, `desktop/app.go:29`, and `App.svelte:944` each define a version string independently. Unify to a single build-time source before touching anything else.

4. **The event name protocol between Go and Svelte is all magic strings** — no compile-time check across the Wails boundary. Define event name constants in Go and centralize matching constants in a Svelte file.

5. **Testing strategy follows priority**: Go unit tests first (pure logic, no deps), then Go integration tests via `httptest.Server`, then Svelte component tests via Vitest. Event dispatcher and progress writer are the highest-risk untested code. Desktop module has zero coverage.

6. **`golangci-lint` must run per-module** — the Go workspace has separate `go.mod` files per module with a `replace` directive. Running from the repo root won't work. CI must lint `beamsync/` and `desktop/` independently.

7. **Wails binding methods are string-referenced on the JS side** — renaming a Go method compiles fine but breaks `window.go.main.App.MethodName()` silently. Any rename must update all frontend callers in one commit and be tested end-to-end.

---

## Implications for Roadmap

| Order | Phase | Rationale |
|-------|-------|-----------|
| 1 | **Event system buffers** — resize `eventDispatcher` (256→1024) and `eventChan` (100→512), add drop counters | Must be done before any splitting to avoid introducing event drops as a new bug mid-refactoring |
| 2 | **Version unification** — single source from `wails.json`, remove hardcoded strings from Go and Svelte | Quick standalone change; unblocking for about view and release automation |
| 3 | **CI/DevEx tooling** — `golangci-lint`, ESLint+Prettier, Vitest scaffold, GitHub Actions CI | Run linting on existing code first (using `--new-from-rev` to avoid overwhelming diffs), then tests become enforceable |
| 4 | **Go unit tests for untested code** — `eventDispatcher`, `progressWriter`, `copyChunked`, `desktop/app.go` logic | Write tests before refactoring to catch regressions from the split |
| 5 | **`server.go` extraction** — 9 focused files in `package beamsync`, same package | Dependency order: helpers → rate_limiter → progress → io_helpers → middleware → events → upload_handler → trim server.go |
| 6 | **`App.svelte` component split** — 7 components (ReceiveView, SendView, SettingsView, AboutView, ToastManager, ProgressOverlay, UpdateBanner) + wails store | Must create Wails store before splitting to avoid passing `window.runtime` as props everywhere |
| 7 | **Error handling conventions** — sentinel errors, `fmt.Errorf("%w")`, remove ad-hoc string errors | Lightweight, can run in parallel with Go structural work |
| 8 | **CD / releases** — GitHub Actions release workflow with Wails build matrix | Only after CI is solid and version source is unified |

**Key constraint**: Do not create sub-packages. Keep `package beamsync` for the Go backend. All restructuring is file-level only. This avoids breaking `desktop/` imports and keeps the diff reviewable.

**Firewall**: Clean up duplicate firewall scripts (embedded, on-disk, build/) before or alongside any firewall.go changes.

---

## Sources

- `.planning/research/STACK.md` — Go HTTP structure, testing patterns, Svelte component strategy, CI/CD, error handling, linting
- `.planning/research/FEATURES.md` — Table stakes, differentiators, anti-features, dependency graph between features
- `.planning/research/ARCHITECTURE.md` — Extraction order for server.go (8 steps), App.svelte split (7 components), event flow analysis, Go↔Svelte state patterns
- `.planning/research/PITFALLS.md` — 18 specific traps across splitting, testing, Wails bindings, CI, and module restructuring with prevention for each
