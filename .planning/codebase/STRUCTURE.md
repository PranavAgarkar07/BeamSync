# Codebase Structure

**Analysis Date:** 2026-07-20

## Directory Layout

```
BeamSync/
├── beamsync/                    # Core library (Go module)
│   ├── audio/                   # Audio playback engine
│   │   └── audio.go             # AudioEngine — WAV playback via faiface/beep
│   ├── ui/                      # Embedded mobile web UI
│   │   ├── upload.html          # Mobile upload page (served by Receiver)
│   │   ├── download.html        # Mobile download page (served by Sender)
│   │   └── logo.png             # BeamSync logo for mobile pages
│   ├── server.go                # ~1700 lines — HTTP servers, handlers, middleware, pipeline
│   ├── auth_tokens.go           # HMAC token issuance, validation, cleanup
│   ├── permissions.go           # TransferMode, TransferSettings, device rules
│   ├── history.go               # In-memory ring-buffer transfer history
│   ├── stats.go                 # Transfer statistics tracker
│   ├── tls.go                   # Optional TLS with self-signed ECDSA certs
│   ├── port_manager.go          # Dynamic port scanning
│   ├── firewall.go              # Linux firewall automation (ufw/firewalld/iptables)
│   ├── resumable_upload.go      # Chunked resumable upload support
│   ├── go.mod                   # Module: beamsync, Go 1.25.5
│   ├── go.sum
│   ├── *_test.go                # Tests for rate_limiter, server, auth_tokens, etc.
│   └── firewall_setup.sh        # Embedded firewall shell script
├── desktop/                     # Wails desktop application (Go + Svelte)
│   ├── main.go                  # Wails app entry — window, bind, options
│   ├── app.go                   # App struct — lifecycle, config, event bridge, bridges
│   ├── build.sh                 # Linux + Windows release build script
│   ├── create_appimage.sh       # AppImage packaging
│   ├── gen_sounds.py            # Sound generation utility script
│   ├── go.mod                   # Module: desktop, depends on beamsync + wails/v2
│   ├── go.sum
│   ├── wails.json               # Wails config — product version, build options
│   ├── sounds/                  # Embedded WAV sound files
│   │   ├── click.wav
│   │   ├── connect.wav
│   │   ├── hover.wav
│   │   ├── startup.wav
│   │   └── transfer_complete.wav
│   ├── frontend/                # Svelte 3 + Vite UI
│   │   ├── index.html           # Root HTML
│   │   ├── package.json         # Frontend deps (svelte, vite, qrcode)
│   │   ├── vite.config.js       # Vite config with Svelte plugin
│   │   ├── src/                 # Application source
│   │   │   ├── main.js          # Svelte app mount
│   │   │   ├── App.svelte       # Single-page application (~2160 lines)
│   │   │   ├── app.css          # Global resets, toast styles
│   │   │   ├── SplashScreen.svelte
│   │   │   ├── Typewriter.svelte
│   │   │   ├── vite-env.d.ts
│   │   │   ├── assets/          # Static assets (images, fonts)
│   │   │   │   ├── images/
│   │   │   │   │   ├── icon.png
│   │   │   │   │   ├── logo-universal.png
│   │   │   │   │   ├── appSS1.png / appSS2.png / appSS3.png (screenshots)
│   │   │   │   │   ├── V1.png / 1V1.png
│   │   │   │   │   └── ...
│   │   │   │   └── fonts/
│   │   │   │       ├── nunito-v16-latin-regular.woff2
│   │   │   │       └── OFL.txt
│   │   │   └── design-system/   # Neubrutalism design system
│   │   │       ├── tokens.css               # CSS custom properties (colors, spacing, shadows)
│   │   │       ├── index.js                 # Component barrel export
│   │   │       ├── TopNavBar.svelte
│   │   │       ├── DeviceCard.svelte
│   │   │       ├── FileDropZone.svelte
│   │   │       ├── TransferProgressBar.svelte
│   │   │       ├── TransferComplete.svelte
│   │   │       ├── TransferStatsDashboard.svelte
│   │   │       ├── ActivityPanel.svelte
│   │   │       ├── ConnectedDevicesPanel.svelte
│   │   │       └── DesignSystemShowcase.svelte
│   │   ├── dist/                # Built output
│   │   └── node_modules/
│   ├── build/                   # Build artifacts
│   │   ├── appicon.png
│   │   ├── bin/                 # Compiled binaries
│   │   ├── windows/             # Windows installer assets
│   │   └── README.md
│   └── packaging/               # Package definitions
│       └── arch/                # AUR PKGBUILD
├── build/                       # Linux packaging
│   └── linux/
│       ├── control              # Debian control file
│       ├── package_deb.sh       # DEB package builder
│       ├── beamsync.desktop     # Desktop entry
│       ├── firewall_setup.sh
│       └── build/               # DEB package output
├── community/                   # Community website (Astro)
│   ├── src/
│   │   ├── components/
│   │   ├── layouts/
│   │   ├── pages/
│   │   └── styles/
│   ├── package.json             # Dependencies (astro ^5.0.0)
│   ├── astro.config.mjs
│   └── tsconfig.json
├── design-system/               # Cross-project design reference
│   └── beamsync-community/
│       └── MASTER.md
├── docs/                        # Documentation
│   └── desktop-design-system.md
├── .github/                     # GitHub configuration
│   ├── workflows/
│   │   ├── ci.yml               # CI pipeline
│   │   ├── deploy-pages.yml     # Pages deployment
│   │   └── sync-pr-labels.yml   # Label sync
│   ├── ISSUE_TEMPLATE/
│   │   ├── bug_report.md
│   │   └── feature_request.md
│   └── PULL_REQUEST_TEMPLATE.md
├── .aur/                        # AUR package metadata
│   └── beamsync-bin/
├── .opencode/                   # OpenCode configuration + skills
│   ├── skills/
│   │   └── ui-ux-pro-max/       # UI/UX design intelligence skill
│   └── package.json
├── .agent/                      # Agent skills
│   └── skills/
│       └── ui-ux-pro-max/
├── .planning/                   # Planning state
│   └── codebase/                # This directory
├── graphify-out/                # Knowledge graph output
├── README.md                    # Project overview
├── API.md                       # HTTP API documentation
├── ROADMAP.md                   # Development roadmap
├── CONTRIBUTING.md              # Contribution guidelines
├── CODE_OF_CONDUCT.md
├── SECURITY.md
├── LICENSE                      # MIT License
├── GEMINI.md                    # Gemini AI config
├── release_notes.md
└── rewrite_app.py               # Python migration script
```

## Directory Purposes

**`beamsync/`:**
- Purpose: Core Go library — the file transfer engine. All HTTP server logic, auth, permissions, and utilities.
- Contains: Go source files, test files, embedded UI assets, embedded firewall script
- Key files: `server.go` (primary file, ~1700 lines), `auth_tokens.go`, `permissions.go`, `tls.go`

**`desktop/`:**
- Purpose: Wails v2 desktop application shell — bridges Go backend to Svelte frontend.
- Contains: Go entry point (`main.go`), `App` struct with all exposed methods, embedded sound files, Svelte frontend
- Key files: `main.go`, `app.go`, `frontend/src/App.svelte`, `frontend/src/design-system/tokens.css`

**`desktop/frontend/src/design-system/`:**
- Purpose: Neubrutalism design system — CSS tokens and reusable Svelte components.
- Contains: Central `tokens.css` (color palette, typography, spacing, shadows, dark mode), standalone Svelte components
- Key files: `tokens.css`, `TopNavBar.svelte`, `FileDropZone.svelte`, `TransferProgressBar.svelte`

**`beamsync/audio/`:**
- Purpose: WAV audio playback engine using `faiface/beep`.
- Contains: `audio.go` with `AudioEngine` struct and `Init`, `LoadSoundFromStream`, `Play` methods

**`build/linux/`:**
- Purpose: Linux DEB packaging — control files, build script, desktop entry.
- Contains: `package_deb.sh`, `control`, `beamsync.desktop`

**`community/`:**
- Purpose: Community website built with Astro 5.
- Contains: Astro project with pages, components, layouts
- Key files: `package.json`, `astro.config.mjs`

**`.github/`:**
- Purpose: CI/CD workflows and issue/PR templates.
- Contains: GitHub Actions workflows for CI, Pages deployment, label sync; bug report and feature request templates

## Key File Locations

**Entry Points:**
- `desktop/main.go`: Wails application entry — creates window, starts event loop
- `beamsync/server.go:770` (`StartServer`): Receiver HTTP server entry
- `beamsync/server.go:1364` (`StartSender`): Sender HTTP server entry
- `desktop/frontend/src/main.js`: Svelte app mount
- `desktop/frontend/src/App.svelte`: Main Svelte single-page application

**Configuration:**
- `desktop/wails.json`: Wails v2 project configuration
- `desktop/frontend/package.json`: Frontend Node.js dependencies
- `beamsync/go.mod`: Core library Go module definition
- `desktop/go.mod`: Desktop app Go module definition
- `community/astro.config.mjs`: Community site Astro config

**Core Logic:**
- `beamsync/server.go`: All HTTP server logic, middleware, handlers, write pipeline, rate limiter
- `beamsync/auth_tokens.go`: HMAC token issuance and validation
- `beamsync/permissions.go`: Transfer permission modes and device rules
- `beamsync/tls.go`: Optional TLS with self-signed certificates
- `beamsync/resumable_upload.go`: Chunked upload with integrity verification
- `desktop/app.go`: App lifecycle, config persistence, event bridge
- `desktop/frontend/src/App.svelte`: Full frontend logic (~2160 lines)

**Testing:**
- `beamsync/server_test.go`: Integration tests for HTTP handlers (~902 lines)
- `beamsync/auth_tokens_test.go`: Token validation tests
- `beamsync/rate_limiter_test.go`: Rate limiter unit tests (~258 lines)
- `beamsync/history_test.go`: Transfer history tests
- `beamsync/stats_test.go`: Transfer stats tests
- `beamsync/permissions_test.go`: Permission logic tests
- `beamsync/port_manager_test.go`: Port scanning tests
- `beamsync/resumable_upload_test.go`: Resumable upload tests
- `beamsync/tls_test.go`: TLS certificate tests

## Naming Conventions

**Files:**
- Go source: `snake_case.go` (e.g., `auth_tokens.go`, `port_manager.go`, `resumable_upload.go`)
- Go test: `*_test.go` (e.g., `rate_limiter_test.go`)
- Svelte components: `PascalCase.svelte` (e.g., `TopNavBar.svelte`, `FileDropZone.svelte`)
- Svelte utilities: `camelCase.svelte` (e.g., `SplashScreen.svelte`, `Typewriter.svelte`)
- CSS: `kebab-case.css` (e.g., `app.css`, `tokens.css`)
- HTML: `lowercase.html` (e.g., `upload.html`, `download.html`)
- Scripts: `snake_case.sh` / `snake_case.py` (e.g., `build.sh`, `create_appimage.sh`, `gen_sounds.py`)
- Config: `camelCase.*` (e.g., `wails.json`, `vite.config.js`, `astro.config.mjs`)

**Directories:**
- `kebab-case` for most directories (e.g., `design-system`, `port-manager` not used, but `frontend`, `audio`)
- Package names in Go follow Go convention: single word lowercase (e.g., `beamsync`, `audio`)

**Functions (Go):**
- Go convention: `PascalCase` for exported, `camelCase` for unexported (e.g., `StartServer`, `FindAvailablePort`, `serverTLSFingerprint`, `processEvents`)

**Functions (Svelte):**
- JavaScript/TypeScript: `camelCase` for variables and functions (e.g., `startReceiver`, `generateQRCode`, `handleFileDrop`)

## Where to Add New Code

**New Feature (e.g., new HTTP endpoint):**
- Primary code: Add handler in `beamsync/server.go` and wire into `mux.HandleFunc` in `StartServer()` or `StartSender()`
- Tests: `beamsync/server_test.go`
- If new middleware: add as separate file in `beamsync/`

**New Desktop UI Component:**
- Implementation: `desktop/frontend/src/design-system/<ComponentName>.svelte`
- Export from `desktop/frontend/src/design-system/index.js`
- CSS tokens: `desktop/frontend/src/design-system/tokens.css`
- Wire into main app: `desktop/frontend/src/App.svelte`

**New Backend Service/Module:**
- Core logic: `beamsync/<name>.go` (package beamsync)
- Tests: `beamsync/<name>_test.go`
- Wire into server lifecycle in `beamsync/server.go`

**New Configuration:**
- User-facing config: extend `configData` struct in `desktop/app.go:79`
- Persistence: `loadConfig()` / `saveConfig()` in `desktop/app.go`
- Expose to frontend via new Wails-bound method

**Utilities:**
- Shared Go helpers: `beamsync/` package (e.g., `sha256Hex`, `generateID`, `clientIP` in `beamsync/server.go`)
- Shared frontend utilities: inline in `desktop/frontend/src/App.svelte`

**Mobile Web UI:**
- Upload page: `beamsync/ui/upload.html`
- Download page: `beamsync/ui/download.html`
- Images: `beamsync/ui/logo.png` (embedded via `//go:embed`)

**Audio Assets:**
- WAV files: `desktop/sounds/`
- Registration: `desktop/app.go:135` (map of sound name → filename)

## Special Directories

**`beamsync/ui/`:**
- Purpose: HTML/CSS/JS pages embedded into the Go binary via `//go:embed`
- Generated: No
- Committed: Yes
- These are served directly by the Go HTTP server to mobile browsers (no separate web server needed)

**`desktop/build/`:**
- Purpose: Build artifacts — compiled binaries, platform-specific installers
- Generated: Yes
- Committed: Yes (appicon.png, windows installer assets)

**`graphify-out/`:**
- Purpose: Knowledge graph analysis output from the graphify skill
- Generated: Yes
- Committed: Yes

**`.opencode/` and `.agent/`:**
- Purpose: OpenCode configuration and agent skill definitions
- Generated: No
- Committed: Yes

**`desktop/frontend/dist/`:**
- Purpose: Vite production build output
- Generated: Yes (by `npm run build`)
- Committed: Yes (used by Wails to embed in Go binary)

---

*Structure analysis: 2026-07-20*
