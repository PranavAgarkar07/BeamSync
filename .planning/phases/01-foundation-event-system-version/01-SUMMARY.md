# Phase 1: Foundation — Event System & Version

**Status:** COMPLETE
**Date:** 2026-07-20

## Requirements Met

| ID | Description | Status |
|----|-------------|--------|
| EVT-01 | eventDispatcher buffer 256→1024 | ✓ Done |
| EVT-02 | eventChan buffer 100→512 (named const) | ✓ Done |
| EVT-03 | Drop counters on event dispatcher | ✓ Done |
| VER-01 | Version source unified to wails.json | ✓ Done |
| VER-02 | Hardcoded version removed from app.go | ✓ Done |
| VER-03 | Version passed to Svelte via Wails bridge | ✓ Done |

## Files Changed

| File | Changes |
|------|---------|
| `beamsync/server.go` | Buffer 256→1024, atomic.Int64 dropped field, import, DroppedCount method |
| `desktop/app.go` | eventChan capacity named const, wails.json embed, version field, GetVersion(), CheckForUpdate() refs, version parse in startup |
| `desktop/frontend/src/App.svelte` | Dynamic appVersion from GetVersion() on mount, template binding |
| `desktop/frontend/src/design-system/TopNavBar.svelte` | Default version updated to v2.4.0 |
| `desktop/frontend/src/design-system/DesignSystemShowcase.svelte` | Version updated to v2.4.0 |

## Decisions Applied

- makeCallback drop counter: Skipped (existing fmt.Printf sufficient)
- Drop exposure: Log only, DroppedCount() for testing only
- Version format: "v2.4.0" (with v prefix)
- All Svelte instances updated (App.svelte x2, TopNavBar.svelte x2, Showcase.svelte x1)
- fallbackVersion const for JSON parse failure
- Version parsed at top of startup(), before goroutines

## Pre-existing Issues (not introduced by Phase 1)

- `beamsync/auth_tokens.go`: errInvalidToken, errExpiredToken, errUsedToken undefined (Phase 6 scope)
- `beamsync/server.go`: unused imports (bytes, net/textproto, strconv), unused variables (Phase 4 scope)
