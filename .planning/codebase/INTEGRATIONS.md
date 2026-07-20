# External Integrations

**Analysis Date:** 2026-07-20

## APIs & External Services

**GitHub API (Releases):**
- Used for: Automatic update checking on desktop app startup
  - Endpoint: `GET https://api.github.com/repos/PranavAgarkar07/BeamSync/releases/latest`
  - Implementation: `desktop/app.go` — `CheckForUpdate()` (line 630)
  - Auth: None (unauthenticated, rate-limited)
  - Headers: `User-Agent: BeamSync/{version} ({os}; {arch})`, `Accept: application/vnd.github+json`
  - Timeout: 8 seconds
  - Frequency: Once per app startup (with 3s initial delay, `checkForUpdateAndNotify()` line 688)

## Data Storage

**Databases:**
- None. No SQL or NoSQL databases used.

**File Storage:**
- Local filesystem only
  - Uploads saved to user-configured directory (default: `~/Downloads/BeamSync/`)
  - Resumable uploads stored in `.beamsync-resume/` subdirectory within the upload path
  - Config persisted to `~/.config/beamsync/config.json`
  - TLS certs persisted to `~/.config/beamsync/cert.pem` + `key.pem`
  - All persistence handled by Go standard library (`os.WriteFile`, `json.MarshalIndent`)

**Caching:**
- None. No Redis, in-memory cache, or any caching system used beyond Go's `sync.Pool` for 8 MB chunk buffers (`beamsync/server.go` line 88).

## Authentication & Identity

**Auth Provider:**
- Custom (no external provider)
  - Implementation: `beamsync/auth_tokens.go`
  - Pattern: HMAC-SHA256 signed token with per-session secret
  - Token scopes: `bootstrap` (single-use, via QR), `session` (renewable), `transfer` (single-use download)
  - Tokens are client-IP-bound to prevent replay from different hosts
  - TLS fingerprint bound when `BEAMSYNC_ENABLE_TLS=1`
  - All tokens TTL: 5 minutes (`defaultTokenTTL`)
  - No OAuth, no SSO, no password auth

## Monitoring & Observability

**Error Tracking:**
- None. No Sentry, Datadog, or similar error-monitoring service.

**Logs:**
- stdout only via `fmt.Printf` — no structured logging library
  - Prefix emoji convention: `✅` success, `❌` error, `⚠️` warning, `💚` connection, etc.
  - No log files, no log levels, no rotation

## CI/CD & Deployment

**Hosting:**
- Community site hosted on GitHub Pages at `https://pranavagarkar07.github.io/BeamSync/`
  - Deploy configuration: `community/astro.config.mjs` (site, base, outDir)
  - Source: `community/` directory

**CI Pipeline:**
- GitHub Actions (`ci.yml`)
  - Triggers: PRs, pushes to `main`, manual dispatch
  - Checks: gofmt, `go test`, `go vet`, `go build` on both `beamsync/` and `desktop/`
  - Also builds frontend assets (Svelte/Vite) and runs `npm run lint --if-present`
  - OS: ubuntu-latest
  - Go version: 1.25.5
  - Node version: 24

**Pages Deploy:**
- GitHub Actions (`deploy-pages.yml`)
  - Triggers: pushes to `main` touching `community/**`, manual dispatch
  - Builds Astro site with Node 20
  - Deploys to `gh-pages` branch via `peaceiris/actions-gh-pages@v4`
  - Force-orphan deploy (rewrites entire pages branch each time)

**Release Artifacts:**
- Linux amd64 binary (`wails build -platform linux/amd64`)
- Windows amd64 EXE + NSIS installer (`wails build -platform windows/amd64 -nsis`)
- Build script: `desktop/build.sh`
- AUR package: `.aur/beamsync-bin/PKGBUILD`

## Environment Configuration

**Required env vars:**
- None. The app works with zero configuration.

**Optional env vars:**
- `BEAMSYNC_ENABLE_TLS` — Set to `"1"` to enable HTTPS with a self-signed cert (read in `beamsync/tls.go`).

**Secrets location:**
- No secrets management. TLS private keys stored at `~/.config/beamsync/key.pem` (file mode 0o600).

## Webhooks & Callbacks

**Incoming:**
- None. No incoming webhook endpoints.

**Outgoing:**
- None. No outgoing webhook calls.

## Social / Distribution

**GitHub:**
- Repository: `github.com/PranavAgarkar07/BeamSync`
- PR template: `.github/PULL_REQUEST_TEMPLATE.md`
- Issue templates: `.github/ISSUE_TEMPLATE/`
- Label sync workflow: `.github/workflows/sync-pr-labels.yml`

**AUR (Arch User Repository):**
- Package: `beamsync-bin`
- Location: `.aur/beamsync-bin/`
- Purpose: Distribution of Linux binaries on Arch Linux

---

*Integration audit: 2026-07-20*
