# BeamSync

## What This Is

BeamSync is a cross-platform desktop application for offline-first peer-to-peer file transfer over LAN. It runs a local HTTP server that serves a mobile-optimized web UI — the sender shares a QR code, the receiver scans it with their phone, and files transfer directly over the local network. No internet, no cloud, no accounts.

## Core Value

A sender can share files from their desktop to any mobile device on the same LAN in under 5 seconds — scan a QR code, download. That's it.

## Context

**2026-07-20 — Brownfield quality overhaul.** The codebase was built fast ("vibe coded") and has accumulated significant structural debt. The goal of this milestone is to transform it into a production-quality, maintainable project without changing user-facing behavior.

## Requirements

### Validated (already work)

- ✓ **Go workspace** with `beamsync/` (core library) and `desktop/` (Wails app) modules
- ✓ **Two-server architecture** — receiver accepts uploads, sender serves downloads, both via HTTP
- ✓ **HMAC-SHA256 token auth** — session-scoped tokens with IP binding and expiry
- ✓ **Wails v2 desktop shell** — cross-platform native window (Windows, macOS, Linux)
- ✓ **Svelte 3 frontend** — desktop UI with QR code, drag-and-drop, settings, transfer history
- ✓ **Embedded mobile web UI** — upload/download HTML pages served by the Go HTTP server
- ✓ **QR code sharing** — visual token exchange between desktop and mobile
- ✓ **Rate limiting** — per-client token-bucket on all endpoints
- ✓ **Optional TLS** — self-signed cert for HTTPS mode
- ✓ **Resumable uploads** — chunked uploads with progress tracking
- ✓ **Transfer history** — in-memory log of completed transfers
- ✓ **Transfer stats** — per-session bandwidth and file counts
- ✓ **Audio feedback** — sound effects for connect/transfer/click events
- ✓ **OS firewall setup** — automatic port allowance on macOS/Linux
- ✓ **Cross-platform builds** — build scripts for all three platforms

### Active (to address in this milestone)

- [ ] **TECH-01** — Split `beamsync/server.go` (1726 lines) into focused modules: upload handler, download handler, middleware, rate limiter, progress tracking
- [ ] **TECH-02** — Split `desktop/frontend/src/App.svelte` (2160 lines) into focused components
- [ ] **TECH-03** — Add tests to `desktop/` (Go module) — currently zero coverage
- [ ] **TECH-04** — Add tests to `desktop/frontend/` (Svelte) — currently zero coverage
- [ ] **TECH-05** — Merge duplicate `progressWriter` / `downloadProgressWriter` types
- [ ] **TECH-06** — Deduplicate URL construction pattern (6 copies in `desktop/app.go`)
- [ ] **TECH-07** — Unify version string from a single source of truth
- [ ] **TECH-08** — Establish Go error handling conventions (typed errors, consistent wrapping)
- [ ] **TECH-09** — Add linting/formatting pipeline for frontend (ESLint, Prettier)
- [ ] **TECH-10** — Set CI/CD with automated tests, lint, build for all platforms
- [ ] **TECH-11** — Document architecture, setup, and contribution guide
- [ ] **TECH-12** — Address all issues from CONCERNS.md analysis

### Out of Scope

- New features or UI changes — this is a quality-only pass
- Replacing Wails with another framework
- Cloud or internet-based transfer modes
- End-to-end encryption beyond TLS
- Mobile-native apps (iOS/Android)
- Multi-user or server-mode operation

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Quality pass only | User explicitly wants cleanup, not features | — Pending |
| Prefer Go stdlib patterns | Idiomatic Go keeps dependencies minimal | — Pending |
| Testability drives refactoring | Can't refactor without tests — add tests first, then refactor | — Pending |
| Desktop Wails cross-platform | Keep existing stack, don't churn framework | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition:**
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After milestone completion:**
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-07-20 after initialization*
