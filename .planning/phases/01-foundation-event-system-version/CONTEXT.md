# Phase 1: Foundation — Event System & Version

## Requirements

| ID | Description | Files |
|----|------------|-------|
| EVT-01 | Resize `eventDispatcher` buffer from 256 → 1024 | `beamsync/server.go:31` |
| EVT-02 | Resize `eventChan` buffer from 100 → 512 (named const) | `desktop/app.go:70` |
| EVT-03 | Add drop counters to event dispatcher | `beamsync/server.go:69-79` |
| VER-01 | Unify version source to `wails.json` | `desktop/app.go:29`, `desktop/wails.json` |
| VER-02 | Remove hardcoded version strings from `desktop/app.go` | `desktop/app.go:29,631,635,679,682` |
| VER-03 | Pass version to Svelte frontend via Wails bridge | `desktop/frontend/src/App.svelte:944,1260`, `TopNavBar.svelte`, `DesignSystemShowcase.svelte` |

## Decisions

1. **makeCallback drop counter:** Skip. The existing `fmt.Printf` log on `default:` branch is sufficient. Backend drops are covered by the dispatcher counter.
2. **Frontend exposure of drops:** Log only. `DroppedCount()` on the dispatcher for testing. No Wails-bound method.
3. **Version format:** `"v2.4.0"` — keep the `v` prefix to match current Go convention (`app.go:29`) and Git tags.
4. **DesignSystemShowcase version:** Update all instances (App.svelte x2, TopNavBar.svelte x2, Showcase.svelte).

## Approach

### EVT-01 — eventDispatcher buffer
- `server.go:31`: `eventDispatcherBufferSize = 256` → `= 1024`
- No other code depends on this constant — scoped to `newEventDispatcher` call on line 81.

### EVT-02 — eventChan buffer
- `app.go:70`: `make(chan EventData, 100)` → `make(chan EventData, eventChanCapacity)`
- Add `const eventChanCapacity = 512` before the `App` struct.

### EVT-03 — Drop counters on eventDispatcher
- Add `dropped atomic.Int64` field to `eventDispatcher` struct.
- Import `sync/atomic` in `server.go`.
- Increment in `emit()` default branch before `return false`.
- Add `DroppedCount() int64` method returning `d.dropped.Load()`.
- Log non-zero drops in `run()` periodically or at shutdown.

### VER-01/02 — Version from wails.json
- Embed `wails.json` (file is in `desktop/`, same as `app.go`):
  ```go
  //go:embed wails.json
  var wailsJSONData []byte
  ```
- Parse JSON in `startup()` to extract `info.productVersion`.
- Store version string as `App.version` field.
- Remove `const currentVersion = "v2.4.0"`.
- Add `GetVersion() string` Wails-bound method returning `"v" + a.version`.
- Update `CheckForUpdate()` to use `a.version` instead of `currentVersion`.

### VER-03 — Frontend version
- `App.svelte:944`: `appVersion="v2.2"` — replace with dynamic call:
  ```js
  let appVersion = 'v2.4.0';
  onMount(async () => {
    appVersion = await window.go.main.App.GetVersion();
  });
  ```
- `App.svelte:1260`: `v2.2` → `{appVersion}`
- `TopNavBar.svelte:36,23`: default `"v2.2"` → `"v2.4.0"`
- `DesignSystemShowcase.svelte:73`: `"v2.2"` → `"v2.4.0"`

## Files Changed

| File | Changes |
|------|---------|
| `beamsync/server.go` | Buffer constant, atomic field, import, DroppedCount method |
| `desktop/app.go` | Buffer const, embed wails.json, remove version const, add GetVersion |
| `desktop/frontend/src/App.svelte` | Replace hardcoded v2.2 with dynamic GetVersion call |
| `desktop/frontend/src/design-system/TopNavBar.svelte` | Update default "v2.2" → "v2.4.0" |
| `desktop/frontend/src/design-system/DesignSystemShowcase.svelte` | Update "v2.2" → "v2.4.0" |

## Risks

- **Silent double dispatcher** (PITFALLS.md 1a): When adding drop counters, ensure the same `eventDispatcher` instance is instrumented — the `defaultEventDispatcher` var on line 81 is the only instance.
- **Version embed path**: `wails.json` is in `desktop/`, not project root. The `//go:embed` directive in `desktop/app.go` resolves relative to that file's directory, so `wails.json` (not `../wails.json`) is correct.
- **Async GetVersion**: Frontend will briefly show the default `"v2.4.0"` until the async call resolves. This is acceptable — it matches the correct value.
