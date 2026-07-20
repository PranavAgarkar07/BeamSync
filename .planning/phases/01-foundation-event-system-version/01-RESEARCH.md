# Phase 1: Foundation — Event System & Version — Research

**Researched:** 2026-07-20
**Domain:** Go event system hardening, version source-of-truth unification, Wails/Svelte bridge
**Confidence:** HIGH — all changes use stdlib, no new dependencies

## Summary

Phase 1 is a brownfield quality pass on two independent concerns: (1) hardening the event dispatch pipeline (buffer resizing + drop instrumentation) and (2) unifying the app version string to `wails.json` as single source of truth. Both are low-complexity, low-risk, mechanically sympathetic changes — every requirement has a single clear file to edit.

**Key structural finding:** Two Go modules — `beamsync/` (library) and `desktop/` (Wails app, depends on beamsync via `replace`). The `//go:embed wails.json` directive in `desktop/app.go` resolves relative to `desktop/`, so plain `wails.json` is correct.

## Requirement Coverage

| ID | Approach | File |
|----|----------|------|
| EVT-01 | `eventDispatcherBufferSize = 256` → `1024` | `beamsync/server.go:31` |
| EVT-02 | `const eventChanCapacity = 512`, replace magic number 100 | `desktop/app.go:70` |
| EVT-03 | `atomic.Int64 dropped` field on `eventDispatcher`, increment in `emit()` default branch | `beamsync/server.go:39-79` |
| VER-01 | `//go:embed wails.json`, parse `info.productVersion` at startup | `desktop/app.go` |
| VER-02 | Remove `const currentVersion`, replace all refs with `a.version` | `desktop/app.go:29,631,635,679,682` |
| VER-03 | `GetVersion()` Wails-bound method; `onMount` async call in `App.svelte` | Svelte files |

## Key Patterns

### Embed + Parse wails.json
```go
//go:embed wails.json
var wailsJSONData []byte

// In startup(), before goroutines:
type wailsInfo struct {
    Info struct {
        ProductVersion string `json:"productVersion"`
    } `json:"info"`
}
var info wailsInfo
if err := json.Unmarshal(wailsJSONData, &info); err != nil {
    a.version = fallbackVersion
} else {
    a.version = "v" + info.Info.ProductVersion
}
```

### Atomic Drop Counter
```go
type eventDispatcher struct {
    queue   chan eventDispatchJob
    dropped atomic.Int64
}

func (d *eventDispatcher) emit(job eventDispatchJob) bool {
    if job.emit == nil { return true }
    select {
    case d.queue <- job: return true
    default:
        d.dropped.Add(1)
        return false
    }
}

func (d *eventDispatcher) DroppedCount() int64 {
    return d.dropped.Load()
}
```

### Frontend Version Bridge
```svelte
let appVersion = 'v2.4.0';
onMount(async () => {
    try {
        appVersion = await window.go.main.App.GetVersion();
    } catch { /* fallback already set */ }
});
```

## Risks & Pitfalls

- **Pitfall: embed path resolution.** `//go:embed` resolves relative to source file's directory. `app.go` is in `desktop/`, `wails.json` is in `desktop/` — use `wails.json`, NOT `../wails.json`.
- **Pitfall: atomic increment after return.** Default branch must increment BEFORE `return false`.
- **Pitfall: wails.json JSON parse failure.** Embed always succeeds (compile error if missing), but JSON parse can fail if Wails changes schema. Use a `const fallbackVersion = "v2.4.0"` as fallback.
- **Self-verifying:** `const currentVersion` deletion will cause compile errors if any reference remains — all four uses (field assign, User-Agent, comparison, log) must be replaced.
