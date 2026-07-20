# Stack Research — Quality Overhaul

**Research Date:** 2026-07-20
**Target:** Go 1.25.5 + Wails v2.12.0 + Svelte 3.x desktop app

---

## 1. Go HTTP Server Structure

### Current state
All handlers, middleware, rate limiter, progress tracking, watchdog, and event dispatch live in `beamsync/server.go` (1726 lines). Uses raw `net/http` with manual handler functions wrapped by middleware closures. No router library.

### Recommended: stdlib-first layered architecture

Keep `net/http` — the server has ~7 routes. Adding `chi` or `gorilla/mux` buys nothing for this route count and adds a dependency. Go 1.22+ `http.ServeMux` with method-based routing (`GET /upload`, `POST /upload`) would be nice but Go 1.25.5 already supports it.

```
beamsync/
  server.go           # HTTPServer struct, StartServer/StartSender, route setup
  upload_handler.go   # uploadFile handler, processManifest, chunked upload logic
  download_handler.go # downloadFile handler, file serving
  middleware.go       # tokenMiddleware, rateLimitMiddleware, setCORSHeaders
  progress.go         # progressWriter (merged), copyChunked
  rate_limiter.go     # clientRateLimiter (extract from server.go)
  event.go            # eventDispatcher (extract from server.go)
  watchdog.go         # startWatchdog (extract from server.go)
```

**Patterns per layer:**
- **Handlers** — func(w http.ResponseWriter, r *http.Request). Extract request parsing into helper funcs, keep handler body lean.
- **Middleware** — standard `func(next http.HandlerFunc) http.HandlerFunc` pattern. Already done correctly.
- **State** — `HTTPServer` struct holds server-wide state. Keep as-is. Avoid "Manager" / "Service" interfaces — single implementation does not need an interface.
- **Extraction approach** — Move files one at a time. Tests pass after each move. No behavioral changes.

**Confidence:** High (9/10) — follows existing patterns, minimal friction.
**Effort:** Small (2-4 hours) — pure mechanical extraction.

### What NOT to use

| Library | Why Not |
|---------|---------|
| `chi`, `gorilla/mux` | 7 routes, stdlib does it. Extra dep, no benefit. |
| `echo`, `gin`, `fiber` | Heavy frameworks. Echo is already a transitive dep via Wails but not used directly. Adding it as a direct dep couples us to its routing model. |
| Dependency injection frameworks (`wire`, `dig`, `fx`) | Massive overkill for a two-module project with one `HTTPServer` struct. |
| `context.Context` for per-request state | Middleware already passes state via closures. Context values are opaque and untestable. |

---

## 2. Testing Go net/http Handlers

### Current state
`beamsync/` has good tests (`server_test.go` 902 lines, `rate_limiter_test.go`, `auth_tokens_test.go`, etc.). Uses `testing` stdlib, `httptest`, table-driven subtests. No test framework. `desktop/` has zero tests.

### Recommendations

**Keep using:**
- `testing` stdlib — no testify, no ginkgo. The existing table-driven subtest pattern (`TestTokenMiddlewareRejectsMissingOrInvalidToken`) is idiomatic and sufficient.
- `httptest.NewRecorder()` / `httptest.NewRequest()` — already used, correct.
- `t.Fatalf`/`t.Errorf` over `assert`/`require` — zero-dependency approach works fine.

**Add for `desktop/`:**
- `httptest.NewServer` for integration-style tests that spin up a real server.
- Test the `App` struct methods by constructing it with minimal setup (no real audio engine, no real server).
- Use `io.Discard` / `bytes.Buffer` to capture event output instead of real event dispatch.

**Table-driven test pattern (already used, codify):**

```go
func TestHandler(t *testing.T) {
    tests := []struct {
        name       string
        method     string
        path       string
        body       io.Reader
        wantStatus int
    }{
        {name: "valid upload", method: http.MethodPost, path: "/upload?token=abc", wantStatus: http.StatusAccepted},
        {name: "missing token", method: http.MethodPost, path: "/upload", wantStatus: http.StatusForbidden},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req := httptest.NewRequest(tt.method, tt.path, tt.body)
            rec := httptest.NewRecorder()
            handler(rec, req)
            if rec.Code != tt.wantStatus {
                t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
            }
        })
    }
}
```

**Prioritize:**
1. `desktop/app.go` — mock the server and audio, test URL building, event handling, share flow logic. This is the highest-risk file with zero coverage.
2. `beamsync/server.go` extracted handlers — test each handler independent of `StartServer`.
3. `beamsync/event.go` — test emit under load, buffer-full behavior.

**What NOT to use:**

| Library | Why Not |
|---------|---------|
| `testify/assert` | Adds dep, encourages assertion-chaining. Stdlib `t.Fatalf`/`t.Errorf` is already used and sufficient. |
| `ginkgo` / `gomega` | BDD style is counter-idiomatic in Go. Heavy, slow compilation. |
| `mock` / `gomock` | Hand-rolled test fakes are easier to read and have zero import cost. The codebase has 2-3 interfaces max. |

**Confidence:** High (10/10) — existing tests already follow this pattern.
**Effort:** Medium (4-8 hours for `desktop/` coverage, 2-4 hours for remaining `beamsync/` gaps).

---

## 3. Component Splitting for Svelte

### Current state
`App.svelte` (2160 lines) holds: transfer progress, settings, device panel, transfer history, QR display, drag-and-drop, event binding, sound toggles, session log, version display. Everything in one component.

### Recommendations

Split into these components:

```
desktop/frontend/src/
  App.svelte           # Shell: layout shell, imports child components, minimal glue state
  TransferProgress.svelte  # Progress bars, file status, transfer speed/ETA
  SettingsPanel.svelte     # Save path, port, TLS toggle, sound toggle, trusted devices
  DevicePanel.svelte       # Current device info, IP, QR code display
  TransferHistory.svelte   # List of completed transfers, per-session stats
  SessionLog.svelte        # Raw event log (dev/debug view)
  DragDropZone.svelte      # File drop target with drag-over state
```

**Svelte patterns to use:**
- **Props** for parent→child data. `export let` declarations.
- **Events** (`createEventDispatcher`) for child→parent communication. Keep the event dispatch in `App.svelte` since it bridges to Wails runtime.
- **Stores** (`writable`/`derived`) only for shared state that crosses unrelated components. Prefer props for parent-child.
- **`$:` reactive statements** for computed values (ETA, transfer speed formatting, file size formatting).
- **`{#if}` / `{#each}`** blocks instead of imperative DOM manipulation.

**File-per-component convention:**
- One `.svelte` file per component, PascalCase name.
- Co-locate component-specific CSS in `<style>` tag.
- No component CSS files — Svelte scopes styles by default.
- No TypeScript yet — the codebase uses plain JS. Adding TS is scope-creep for this pass.

**Extraction approach:**
1. Create child component with `export let` props for all data it needs.
2. Move the relevant template block from `App.svelte` to the child.
3. Import and use the child in `App.svelte`, passing props.
4. Repeat. No behavioral changes between steps.

**What NOT to use:**

| Library/Tool | Why Not |
|-------------|---------|
| Svelte 5 / runes | Would require rewriting all components. Keep Svelte 3 for this pass. |
| TypeScript | Works, but the whole frontend is plain JS. Mixing adds overhead without benefit for this quality pass. |
| Tailwind | Would replace all existing CSS. Not part of the scope. |
| svelte-routing | Not needed — single-page desktop app with no navigation. |
| Component libraries (Carbon, Skeleton) | Heavy, opinionated. The existing custom CSS works and is mobile-optimized. |

**Confidence:** High (9/10).
**Effort:** Medium (3-6 hours for extraction, 1 hour for cleanup).

---

## 4. CI/CD for Go Cross-Platform Desktop Apps

### Current state
No CI/CD. Build scripts exist (`desktop/build.sh`, `create_appimage.sh`). Wails cross-compilation requires specific system dependencies.

### Recommendations

**GitHub Actions** — the repo is on GitHub, `gh` CLI is available, convention in Go ecosystem.

**Two workflows:**

#### 4a. CI: `ci.yml` — runs on every PR/push to main

```yaml
jobs:
  go-lint-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.25.5" }
      - uses: actions/setup-node@v4
        with: { node-version: "24" }

      # Go: lint, vet, test
      - run: go vet ./...
        working-directory: beamsync
      - run: golangci-lint run
        working-directory: beamsync
      - run: go test -race -shuffle=on -count=1 ./...
        working-directory: beamsync
      - run: go vet ./...
        working-directory: desktop
      - run: golangci-lint run
        working-directory: desktop
      - run: go test -race -shuffle=on -count=1 ./...
        working-directory: desktop

      # Frontend: lint, test
      - run: npm ci
        working-directory: desktop/frontend
      - run: npm run lint
        working-directory: desktop/frontend
      - run: npm run test
        working-directory: desktop/frontend
```

**Key choices:**
- `-race` on all tests — data race detection is essential for a concurrent file transfer app.
- `-shuffle=on` — prevents order-dependent test bugs.
- `-count=1` — disables test caching for CI (fresh run each time).

#### 4b. CD: `release.yml` — on tag push (`v*`)

```yaml
jobs:
  build-matrix:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
        goarch: [amd64]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.25.5" }
      - uses: actions/setup-node@v4
        with: { node-version: "24" }

      # Install Wails system deps per platform
      - run: sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.0-dev libayatana-appindicator3-dev
        if: runner.os == 'Linux'

      - run: npm ci
        working-directory: desktop/frontend
      - run: wails build -clean -platform ${{ matrix.os }}/${{ matrix.goarch }}
        working-directory: desktop

      - uses: softprops/action-gh-release@v2
        with:
          files: desktop/build/bin/*
```

**Installation of Wails system dependencies on Linux:**
- `libgtk-3-dev`, `libwebkit2gtk-4.0-dev`, `libayatana-appindicator3-dev` — non-negotiable for Wails Linux builds.
- `libasound2-dev` — for audio engine.

**What NOT to use:**

| Tool | Why Not |
|------|---------|
| `goreleaser` | Heavy, configuration overhead, designed for Go-only projects. Wails needs the frontend build step first. A custom action is simpler and clearer for this stack. |
| Cross-compilation from single runner | Wails requires platform-specific native libs. Matrix build is the reliable path. |
| Docker-based builds | Adds complexity. Native GitHub runners + Wails system deps work directly. |
| CircleCI / Jenkins | Overkill for a single-developer OSS project. GitHub Actions is free and colocated with the code. |

**Confidence:** High (9/10) — standard pattern.
**Effort:** Medium (2-3 hours to set up workflows, test Wails build matrix).

---

## 5. Go Error Handling Conventions

### Current state
Already halfway there. The `auth_tokens.go` uses sentinel errors (`errInvalidToken`, `errExpiredToken`), `fmt.Errorf("...: %w", err)` wrapping, and callers use `errors.Is`. Good. But `server.go` uses inconsistent patterns — sometimes bare `fmt.Errorf`, sometimes `errors.New`, sometimes string-based checking.

### Recommendations

**Adopt these conventions project-wide:**

| Pattern | When | Example |
|---------|------|---------|
| Sentinel `var Err` | Domain errors callers need to distinguish | `var ErrTokenExpired = errors.New("token expired")` |
| `fmt.Errorf("%w")` | Wrap with context | `return fmt.Errorf("write manifest: %w", err)` |
| `fmt.Errorf("%v")` | User-facing messages (never wrap) | `return fmt.Errorf("%v: %s", ErrPortInUse, port)` |
| Typed struct error | Errors with structured data | Already use `clientRateLimitDecision` — keep this pattern |
| `errors.Is` | Check sentinel errors | `if errors.Is(err, ErrTokenExpired)` |
| `errors.As` | Check typed errors | Only if typed error structs are introduced |
| `_` named err vars | File-private sentinels | `errRateLimitExceeded` for package-internal checks |
| `panic` / `recover` | Never for control flow | Only in `eventDispatcher.run()` (legitimate) |

**Specific changes needed:**
1. `server.go` — replace ad-hoc string errors with exported sentinel vars where callers (e.g., `desktop/`) need to distinguish them.
2. `desktop/app.go` — wrap all errors from `beamsync` calls with `fmt.Errorf` before logging/display.
3. Add `// errs` file or per-file `var` blocks for sentinel errors.

**What NOT to use:**

| Library | Why Not |
|---------|---------|
| `github.com/pkg/errors` | Already a transitive dep. Do NOT use `errors.Wrap` / `errors.New` from it — Go 1.13+ stdlib `fmt.Errorf("%w")` and `errors.Is`/`As` are the modern approach. The `pkg/errors` import exists only as a transitive dep from `faiface/beep`. |
| `hashicorp/go-multierror` | Overkill. No operation aggregates multiple independent errors. |
| `uber-go/multierr` | Same — not needed for this codebase's error patterns. |
| `cockroachdb/errors` | Heavy, opinionated. Stdlib is sufficient. |

**Confidence:** High (10/10) — established Go idiom.
**Effort:** Small (1-2 hours to audit and normalize).

---

## 6. Linting and Static Analysis

### Current state
`gofmt` and `go vet` run in CI (per existing checks). No `golangci-lint`. No ESLint/Prettier for frontend.

### Recommendations

#### Go: `golangci-lint` v2.x

Run on both modules. Configuration in `.golangci.yml` at repo root.

**Enable:**

| Linter | Rationale |
|--------|-----------|
| `govet` | Already running. Catches shadowed vars, unreachable code, lock copying. |
| `errcheck` | Catches unchecked errors. The current code has many `fmt.Printf`-only error paths. |
| `staticcheck` | Broad static analysis. Catches dead code, redundant constructs, style issues. |
| `ineffassign` | Detects dead assignments. |
| `unused` | Dead code elimination — important after extraction. |
| `gosimple` | Simplify code — catches unnecessary `if` branches, redundant loops. |
| `prealloc` | Suggests slice preallocation where possible. |
| `gosec` | Security-oriented linting — relevant for token auth, file paths, random generation. |
| `errorlint` | Ensures `%w` wrapping is used correctly, finds `errors.Is` that should be used. |

**Disable:**

| Linter | Why Not |
|--------|---------|
| `funlen` | Will flag every function in the monolith. Defer until after extraction. |
| `gocyclo` | Same — structural debt, not style. |
| `lll` (line length) | Arbitrary, causes churn. |
| `wsl` (whitespace) | Opinionated, no safety value. |
| `nlreturn`, `goimports` | `gofmt` handles formatting. |

**Configuration:**

```yaml
# .golangci.yml
linters:
  enable:
    - govet
    - errcheck
    - staticcheck
    - ineffassign
    - unused
    - gosimple
    - gosec
    - errorlint
  disable:
    - funlen
    - gocyclo
    - lll

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - errcheck
        - gosec
```

#### Frontend: ESLint + Prettier

**ESLint** with:
- `eslint:recommended` — base JS rules.
- `eslint-plugin-svelte3` — Svelte component linting. Works with Svelte 3.
- `eslint-plugin-import` — import ordering and validation.

**Prettier** with:
- `bracketSameLine: true` — matches existing HTML style.
- `singleQuote: true` — Svelte convention.
- `svelteSortOrder: "options-scripts-markup-styles"` — Svelte Prettier plugin order.

**Add to `package.json`:**
```json
{
  "scripts": {
    "lint": "eslint 'src/**/*.{js,svelte}'",
    "format": "prettier --check 'src/**/*.{js,svelte,css}'",
    "format:fix": "prettier --write 'src/**/*.{js,svelte,css}'"
  },
  "devDependencies": {
    "eslint": "^8.x",
    "eslint-plugin-svelte3": "^4.x",
    "prettier": "^3.x",
    "prettier-plugin-svelte": "^2.x"
  }
}
```

**What NOT to use:**

| Tool | Why Not |
|------|---------|
| `biome` | Great tool, but the codebase uses Svelte 3 which Biome's Svelte support lags on. Stick with ESLint + Prettier for this pass. Revisit if Svelte 5 migration happens. |
| `eslint-plugin-svelte` (the newer one) | Requires Svelte 5 or specific parser config. `eslint-plugin-svelte3` is the stable choice for Svelte 3. |
| `jshint` / `jslint` | ESLint is the standard. No reason to use alternatives. |
| `stylelint` | The CSS is minimal and scoped per Svelte component. A CSS linter adds noise without catching real bugs in this codebase. |

#### Pre-commit hook (optional)

`husky` + `lint-staged` for automating formatting on commit. Low priority — can add post-CI.

**Confidence:** High (8/10 for Go linting, 9/10 for frontend).
**Effort:** Small (1 hour for `golangci-lint` setup, 1 hour for ESLint + Prettier).

---

## Summary — Integration Effort

| # | Area | Confidence | Effort | Dependencies |
|---|------|-----------|--------|-------------|
| 1 | HTTP server structure | 9/10 | 2-4h | None |
| 2 | Go HTTP testing | 10/10 | 6-12h | Needs #1 first for clean extraction |
| 3 | Svelte component split | 9/10 | 4-7h | None |
| 4 | CI/CD | 9/10 | 2-3h | Needs #6 (linting works in CI) |
| 5 | Error handling conventions | 10/10 | 1-2h | Can run in parallel with #1 |
| 6 | Linting / static analysis | 8/10 | 2-3h | None (can run first) |

**Recommended execution order:**
1. Linting setup (quick win, immediately improves dev experience)
2. Error handling conventions (light, document-driven)
3. HTTP server structure split (core architectural change, needs care)
4. Tests for `beamsync/` extracted modules (blocked on #3)
5. Tests for `desktop/` (can start partial before #3)
6. Component split (independent of Go work)
7. CI/CD (once tests and linting exist to run)
