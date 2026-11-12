# Deferred Items — Phase 3

Out-of-scope discoveries logged during execution. Not fixed here (see the SCOPE BOUNDARY rule
in the GSD executor: pre-existing issues in files this phase does not own are logged, not fixed).

## `desktop/main.go` — golangci-lint typecheck error (pre-existing)

- **File:** `desktop/main.go:19`
- **Symptom:** `main.go:19:9: undefined: wails (typecheck)` when running `golangci-lint run .`
  from `desktop/`.
- **Root cause:** `main.go:6` imports `github.com/wailsapp/wails/v2` — the module path's last
  element (`v2`) differs from the package name (`wails`). golangci-lint v1.64's `typecheck`
  analyzer does not resolve the package name for `/v2`-suffixed import paths, so it reports the
  symbol as undefined. `go build` / `go test` / `go vet` are unaffected — the Go toolchain
  resolves the package name from the package clause.
- **Why not fixed here:** plan 03-03 owns `desktop/app_test.go` only; `desktop/main.go` is not in
  the plan's `files_modified` list. Fixing it would mean changing the import to an explicit
  alias (`wails "github.com/wailsapp/wails/v2"`) — a production-file change outside this plan's
  scope, and it would need its own plan / review.
- **Verified pre-existing:** reproduced in a clean copy of the `desktop/` module with
  `app_test.go` removed — the same error appears. Not introduced by plan 03-03.
- **Suggested home:** a future DevEx / lint-config plan, or an explicit alias in `desktop/main.go`.
