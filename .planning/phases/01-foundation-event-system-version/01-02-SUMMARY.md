# Plan 1.2: Desktop App Event Channel & Version Unification

**Status:** COMPLETE
**Date:** 2026-07-20

## Requirements Met

| ID | Description | Status |
|----|-------------|--------|
| EVT-02 | `eventChan` buffer 100 → 512 (named constant) | ✓ Done |
| VER-01 | Version source unified to `wails.json` | ✓ Done |
| VER-02 | Hardcoded version strings removed from `desktop/app.go` | ✓ Done |

## Files Changed

| File | Changes |
|------|---------|
| `desktop/app.go` | `//go:embed wails.json`; `eventChanCapacity = 512`; `fallbackVersion`; `version` field; `GetVersion()`; `CheckForUpdate()` refs; version parse in `startup()` |

## Verification

- `//go:embed wails.json` — `desktop/app.go:27`
- `const eventChanCapacity = 512` — `desktop/app.go:30`
- `const fallbackVersion = "v2.5.0"` — `desktop/app.go:31` (fallback only, not a source of truth)
- Version parsed at the top of `startup()`, before goroutines start

## Decisions Applied

- Version format: `"v" + productVersion` (with `v` prefix).
- `fallbackVersion` retained purely for the JSON-parse-failure path.

## Note

`desktop/frontend/src/design-system/TopNavBar.svelte:36` still carries a default
`export let appVersion = 'v2.4.0'`. That is a prop default that is always overwritten by
the parent at runtime, not a second source of truth — `App.svelte` passes `appVersion` down.
Left as-is; flagged here so it is not mistaken for drift.

---
*Phase-level rollup lives in `01-SUMMARY.md`.*
