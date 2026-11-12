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
- **RESOLVED:** `df0e069` ("fix(lint): restore green CI") added the `wails "github.com/wailsapp/wails/v2"`
  alias to `desktop/main.go`, so `golangci-lint run .` is clean in `desktop/` again. Retained here as
  a record of the root cause.

## `beamsync/server.go` — `progressWriter` payload delimiter is ambiguous for filenames containing `|`

- **File:** `beamsync/server.go:381` (emit) / `desktop/frontend/src/App.svelte:383-390` (parser)
- **Symptom:** the progress payload is `eventName|filename|written|total`, '|'-delimited. A filename
  that itself contains '|' produces a payload with more than three fields, so the frontend's
  `const [filename, wStr, tStr] = parts` mis-assigns the fields (`written` becomes `NaN`).
- **Why not fixed here:** changing the wire format would break the Go->Svelte event contract, which
  is a cross-boundary change outside this plan's scope. Plan 03-02 records the limitation in
  `beamsync/progress_test.go` (`TestProgressWriterPayloadFormat/filename containing the delimiter is
  a known limitation`) instead of silently encoding it as correct behavior.
- **Suggested home:** a future payload-format change (escape the delimiter, or switch the payload to
  JSON) that touches the Go emit and the Svelte parser together.

## `beamsync/server.go` — receive path streams with `total = 0`

- **File:** `beamsync/server.go:969-975` (receive-path `progressWriter`)
- **Symptom:** the receive path does not set `total` on the writer (the multipart part size is not
  plumbed in), so every progress payload during the upload reports `|0` as the total. The frontend
  handles it (`percent = -1`, total shown as "Unknown") until the final emit at
  `beamsync/server.go:1007` reports the real size, so this is a cosmetic gap, not a defect.
- **Why not fixed here:** plumbing the part size into the writer is a production change outside the
  plan's `files_modified` scope, and it needs its own plan.
- **Suggested home:** a future upload-progress polish plan.

