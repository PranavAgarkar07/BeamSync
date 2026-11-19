# Phase 5: Svelte Component Split - Research

**Researched:** Thu Nov 19 2026
**Domain:** Svelte 3.49 frontend structural refactor (component extraction), Wails desktop bindings
**Confidence:** HIGH

## Summary

Phase 5 is a **structural refactor with NO visual change**: `desktop/frontend/src/App.svelte` (2232 lines) splits into focused components under `src/lib/`, `src/views/`, and `src/components/`. The app must render identically afterward — no new UI, no redesign. A central finding corrects the ROADMAP premise: the app makes **zero** `window.runtime.*` calls (grep count = 0); it imports generated Wails bindings (`../wailsjs/go/main/App.js` with 13 bound methods, `../wailsjs/runtime/runtime.js` with `EventsOn`/`EventsOffAll`/`BrowserOpenURL`/`OnFileDrop`/`OnFileDropOff`). There is one direct `window.go.main.App.GetVersion()` call at App.svelte:273 that the new store should centralize. The existing design system (`src/design-system/`, tokens.css + 8 barrel-exported components, 5 already consumed by App.svelte) remains the single source of truth for all tokens. Test tooling is already installed: vitest ^1.0.0, @testing-library/svelte ^4.0.0, jsdom ^24.0.0. No new npm packages are required.

**Primary recommendation:** Create `src/lib/wails.js` (store wrapping generated bindings) and `src/lib/events.js` (event name constants cross-referenced to `beamsync/events.go`) first, then extract the seven views/components as pure presentational Svelte 3 components (`export let` props + `createEventDispatcher`), passing all Wails access up through the App shell. Add Vitest + @testing-library/svelte component tests for each extracted component.

## User Constraints

### Locked Decisions (from phase framing / REQUIREMENTS.md SVE-01..SVE-09 / UI-SPEC.md)
- **No visual change:** The app must render identically after the split. No new UI, no redesign, no new copy. Every string measured verbatim from App.svelte must be preserved (UI-SPEC.md Copywriting Contract). [VERIFIED: .planning/phases/05-svelte-component-split/05-UI-SPEC.md lines 15, 124-141]
- **Svelte 3.49, no runes:** `$state`, `$derived`, `$props` do not exist. Use `export let` props, `createEventDispatcher`, `$:` reactive statements, slots. [VERIFIED: desktop/frontend/package.json line 17 `"svelte": "^3.49.0"`; UI-SPEC.md line 440]
- **No `window.runtime`:** Wrap generated Wails bindings (`wails.js` store), not `window.runtime`. The ROADMAP premise "wrapping window.runtime.*" is outdated — zero `window.runtime` calls exist in App.svelte (grep count = 0). [VERIFIED: desktop/frontend/src/App.svelte lines 17-24 imports block; grep `window.runtime` count = 0]
- **Use existing design system tokens:** Never hardcode colors/fonts/shadows/spacing. All values via `var(--nb-*)` from `tokens.css` (imported once via `app.css` line 9). Components must NOT re-import tokens.css. [VERIFIED: desktop/frontend/src/app.css line 9 `@import './design-system/tokens.css';`; UI-SPEC.md lines 381-389]
- **Preserve SVE-01..SVE-09 scope:** All 9 requirements from REQUIREMENTS.md Phase 5 must be addressed (wails.js store, ReceiveView, SendView, SettingsView, AboutView, ToastManager, ProgressOverlay, event constants, Vitest tests). UI-SPEC additionally references ROADMAP sub-plans 5.1–5.12. [VERIFIED: .planning/REQUIREMENTS.md lines 42-51; UI-SPEC.md lines 149-341]

**Note on CONTEXT.md:** No Phase 5 `CONTEXT.md` exists (`.planning/phases/05-svelte-component-split/` contains only `05-UI-SPEC.md`), so there are no user-locked decisions beyond the phase framing above; nothing is deferred out of scope for this research.

### the agent's Discretion
- Whether `toast()` state moves to a shared writable store in `wails.js`/`utils.js` vs. stays in App shell and passes `toasts` as a prop to `ToastManager` (UI-SPEC Open Question #1).
- Whether `generateQR()` (qrcode) stays in App shell (passed as prop/function) vs. moved to a shared util (UI-SPEC Open Question #4).
- Whether `rememberDevice` on `TransferRequestModal` uses prop-in/event-out vs. `bind:checked` two-way (UI-SPEC Open Question #3).
- View import path: barrel `./design-system/index.js` (established pattern) vs. direct file paths (UI-SPEC Open Question #6).

### Deferred Ideas (OUT OF SCOPE)
- E2E testing (Playwright/Cypress) — too fragile for Wails desktop (REQUIREMENTS.md Out of Scope).
- Sharded rate limiter, sub-package splitting, DI containers, feature flags, coverage gates (REQUIREMENTS.md Out of Scope).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Window.runtime / Wails binding access | API / Backend (via generated bindings) | — | All Wails calls route through `wails.js` store → generated bindings → Go backend; no component touches bindings directly [VERIFIED: App.svelte lines 17-24, UI-SPEC lines 149-161] |
| Event name constants | API / Backend (shared) | Browser / Client | `events.js` constants must match `beamsync/events.go` (cross-reference gate) [VERIFIED: beamsync/events.go emit calls lines 117, 164-165, 190; UI-SPEC lines 163-188] |
| View rendering (Receive/Send/Settings/About) | Browser / Client | — | Presentational Svelte 3 components, `export let` props + `createEventDispatcher` [VERIFIED: App.svelte lines 1000-1368 markup] |
| Toast rendering | Browser / Client | — | `ToastManager.svelte` receives `toasts` prop, self-removes after 3200ms [VERIFIED: App.svelte lines 153-163, 961-965; UI-SPEC lines 298-308] |
| Progress overlay | Browser / Client | — | `ProgressOverlay.svelte` owns `.progress-float` markup + local `<style>`, 30s timeout [VERIFIED: App.svelte lines 1383-1406, 437-455; UI-SPEC lines 310-323] |
| Drag-and-drop shell | Browser / Client (App shell) | API / Backend (`OnFileDrop`) | `.app-dropzone`, `.drop-overlay` stay in App shell; native `OnFileDrop` handled in shell `onMount` [VERIFIED: App.svelte lines 896-902, 509-511; UI-SPEC line 375] |
| Theme / localStorage DOM writes | Browser / Client (App shell) | — | `applyTheme` toggles `document.documentElement`; SettingsView emits events, shell owns DOM [VERIFIED: App.svelte lines 126-134; UI-SPEC Open Question #5] |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| svelte | ^3.49.0 (installed) | UI framework, components, reactivity | Project's declared framework; no runes available [VERIFIED: desktop/frontend/package.json line 17] |
| vitest | ^1.0.0 (installed) | Test runner for Svelte components | Already scaffolded (TOL-05), configured in `vitest.config.ts` [VERIFIED: desktop/frontend/package.json line 23; vitest.config.ts lines 1-6] |
| @testing-library/svelte | ^4.0.0 (installed) | Svelte component rendering + assertions | Paired with vitest; `render`/`fireEvent` API [VERIFIED: desktop/frontend/package.json line 24] |
| jsdom | ^24.0.0 (installed) | DOM environment for vitest | `test.environment: 'jsdom'` in vitest.config.ts [VERIFIED: desktop/frontend/package.json line 25; vitest.config.ts line 5] |
| qrcode | ^1.5.4 (installed, dependency) | QR code data-URL generation | Already imported in App.svelte line 25; do not replace [VERIFIED: desktop/frontend/package.json line 29; App.svelte line 25] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| design-system (tokens.css + 8 components) | in-repo | Neubrutalism tokens + reusable UI primitives | All extracted components import from `./design-system/index.js` barrel [VERIFIED: App.svelte lines 33-39; design-system/index.js lines 13-20] |
| svelte-check | ^3.0.0 (installed) | Type checking for Svelte 3 | Run `npm run check` after split [VERIFIED: desktop/frontend/package.json line 26] |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `window.runtime.*` direct calls | `wails.js` store wrapping bindings | Chosen: centralizes access, matches ROADMAP SVE-01; direct calls are the anti-pattern being removed |
| New CSS framework (Tailwind/shadcn) | Existing `tokens.css` | Chosen: no new packages, render-identical, UI-SPEC Registry Safety PASS (no third-party registries) [VERIFIED: UI-SPEC lines 25-31, 414-421] |
| Inline event-string literals | `events.js` constants | Chosen: cross-reference gate with `beamsync/events.go` [VERIFIED: UI-SPEC lines 163-188] |

### Installation
No new packages. All dependencies already installed:
```bash
cd desktop/frontend
npm install   # qrcode, vitest, @testing-library/svelte, jsdom already in package.json
```

**Version verification:** Versions verified from `desktop/frontend/package.json` (read this session): `svelte ^3.49.0` (line 17), `vitest ^1.0.0` (line 23), `@testing-library/svelte ^4.0.0` (line 24), `jsdom ^24.0.0` (line 25), `qrcode ^1.5.4` (line 29), `svelte-check ^3.0.0` (line 26). `npm view` registry re-verification is unnecessary because these are already installed and locked in package.json; no new packages are introduced, so the Package Legitimacy Gate is not triggered.

## Package Legitimacy Audit

No new packages are recommended for this phase. All dependencies (`qrcode`, `vitest`, `@testing-library/svelte`, `jsdom`, `svelte`, `svelte-check`) are already declared in `desktop/frontend/package.json` and installed.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| — | — | — | — | — | — | No new packages — all dependencies already installed (qrcode, vitest, @testing-library/svelte, jsdom) |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram (text)

```
USER ACTION
    │
    ▼
┌─────────────────────────────────────┐
│  App.svelte shell (Browser/Client)  │  ← owns .app-dropzone, .drop-overlay,
│  - mode / connectionState state     │    theme/localStorage DOM, onMount events
│  - mounts/unmounts Wails listeners  │
└──────────────┬──────────────────────┘
               │ props + createEventDispatcher events
    ┌──────────┴──────────┬──────────┬──────────┐
    ▼                     ▼          ▼          ▼
ReceiveView           SendView   SettingsView AboutView
(Browser/Client)      (Browser)  (Browser)    (Browser)
│                     │          │            │
└──────────┬──────────┴──────────┴──────────┘
           │ props/events
           ▼
    ┌─────────────────────────────────────┐
    │  Shared components (Browser/Client) │
    │  ToastManager, ProgressOverlay,     │
    │  TransferRequestModal               │
    └──────────────┬──────────────────────┘
                   │ props
                   ▼
    ┌─────────────────────────────────────┐
    │  src/lib/wails.js  (JS module)      │  ← SVE-01: wraps generated bindings,
    │  - functions mirroring 13 App.js    │    NO window.runtime, NO window.go.main.App
    │    bound methods                    │    direct calls from components
    │  - EventsOn/OffAll, BrowserOpenURL, │
    │    OnFileDrop, OnFileDropOff        │
    └──────────────┬──────────────────────┘
                   │ imports
                   ▼
    ┌─────────────────────────────────────┐
    │  ../wailsjs/go/main/App.js          │  ← generated Wails bindings (13 methods)
    │  ../wailsjs/runtime/runtime.js      │  ← EventsOn, EventsOffAll, ... (5 helpers)
    └──────────────┬──────────────────────┘
                   │ Wails bridge
                   ▼
              Go backend (beamsync/)
                   │ emits events
                   ▼
    ┌─────────────────────────────────────┐
    │  src/lib/events.js  (JS module)     │  ← SVE-08: event name constants
    │  device_connected, transfer_request │    cross-referenced to beamsync/events.go
    │  file_received, transfer_stats, ... │
    └─────────────────────────────────────┘
```

Data flow: user gesture in a view → component dispatches event → App shell handler → `wails.js` function → generated Wails binding → Go backend → Go emits named event → `EventsOn` listener in App shell → updates shell state → props flow back down to views/components. Decision point: mode switch (`switchMode`) branches rendering between ReceiveView/SendView/SettingsView/AboutView.

### Recommended Project Structure
```
desktop/frontend/src/
├── App.svelte                  # shell: state, onMount listeners, mode switch, drop zone
├── app.css                     # global resets, tokens import, untouched
├── lib/
│   ├── wails.js                # SVE-01: store + fns wrapping generated bindings
│   ├── events.js               # SVE-08: event name constants
│   └── utils.js                # SVE-03/5.3: formatDuration, formatSize, fileIcon, isValidIPv4
├── views/
│   ├── ReceiveView.svelte      # SVE-02: QR, connection state, file list
│   ├── SendView.svelte         # SVE-03: FileDropZone, sender dialog, QR
│   ├── SettingsView.svelte     # SVE-04: appearance, save path, devices, extensions
│   └── AboutView.svelte        # SVE-05: version, credits, links
├── components/
│   ├── ToastManager.svelte     # SVE-06: toast rendering (toasts prop)
│   ├── ProgressOverlay.svelte  # SVE-07: floating progress bar (30s timeout)
│   └── TransferRequestModal.svelte # (from App.svelte lines 904-937)
├── design-system/              # existing: tokens.css + 8 components (untouched)
│   ├── index.js                # barrel export
│   ├── tokens.css              # single source of truth
│   └── DeviceCard.svelte       # NOT consumed in Phase 5 (leave as-is)
│   ├── FileDropZone.svelte     # consumed by SendView (bubbled to shell)
│   ├── TransferComplete.svelte # consumed by App shell
│   ├── ConnectedDevicesPanel.svelte # NOT consumed (leave as-is)
│   ├── TopNavBar.svelte        # consumed by App shell
│   ├── TransferStatsDashboard.svelte # consumed by Receive/Send views
│   └── ActivityPanel.svelte    # consumed by Receive/Send views
└── assets/images/icon.png
```

### Pattern 1: Wails store wrapping generated bindings (not window.runtime)
**What:** `src/lib/wails.js` centralizes all Wails access. Components import from `wails.js`, never `../wailsjs/...` or `window.go.main.App`.
**When to use:** Any extracted component that needs a Wails call.
**Example:**
```javascript
// Source: pattern derived from App.svelte lines 17-24 (imports block) and 263-512 (onMount bindings)
import {
  StartReceiverDefault, StartSender, SendFiles, PlaySound, OpenFile,
  ResetApp, GetReceivedFiles, GetSavePath, SetSavePath,
  GetTransferSettings, SaveTransferSettings, ApproveTransfer, RejectTransfer,
} from "../wailsjs/go/main/App.js";
import { EventsOn, EventsOffAll, BrowserOpenURL, OnFileDrop, OnFileDropOff }
  from "../wailsjs/runtime/runtime.js";

export { StartReceiverDefault, StartSender, SendFiles, PlaySound, OpenFile,
  ResetApp, GetReceivedFiles, GetSavePath, SetSavePath,
  GetTransferSettings, SaveTransferSettings, ApproveTransfer, RejectTransfer,
  EventsOn, EventsOffAll, BrowserOpenURL, OnFileDrop, OnFileDropOff };
```
**Contract rule:** Every extracted component that needs a Wails call imports `wails.js`, never `window.go.main.App` or `../wailsjs/...` directly. [VERIFIED: UI-SPEC lines 149-161; App.svelte line 273 direct `window.go.main.App.GetVersion()` call to centralize]

### Pattern 2: Event name constants (events.js)
**What:** `src/lib/events.js` exports string constants for all 14 event names measured from App.svelte `EventsOn` calls, cross-referenced to `beamsync/events.go`.
**When to use:** Any component/subscriber that registers or references event names.
**Example:**
```javascript
// Source: event names measured from App.svelte lines 281-500; Go counterparts from beamsync/events.go lines 117, 164-165, 190
export const EVT_DEVICE_CONNECTED = "device_connected";
export const EVT_DEVICE_DISCONNECTED = "device_disconnected";
export const EVT_TRANSFER_REQUEST = "transfer_request";
export const EVT_FILE_RECEIVED = "file_received";
export const EVT_TRANSFER_LOGGED = "transfer_logged";
export const EVT_TRANSFER_STATS = "transfer_stats";
export const EVT_UPLOAD_PROGRESS = "upload_progress";
export const EVT_DOWNLOAD_PROGRESS = "download_progress";
export const EVT_URL_CHANGED = "url_changed";
export const EVT_TOKEN_ROTATED = "token_rotated";
export const EVT_SENDER_STARTED = "sender_started";
export const EVT_SENDER_FILES = "sender_files";
export const EVT_UPDATE_AVAILABLE = "update_available";
export const EVT_SERVER_ERROR = "server_error";
```
**Contract rule:** Components import event names from `events.js`; no inline event-string literals in components. After Phase 4 `events.go` exists, run the cross-reference gate: every name must appear in both files. [VERIFIED: UI-SPEC lines 163-188]

### Pattern 3: Svelte 3 presentational component (export let + createEventDispatcher)
**What:** Views/components receive data via props and report intent via dispatched events; the App shell owns state and Wails calls.
**When to use:** Every extracted view and component.
**Example:**
```svelte
<!-- Source: Svelte 3 createEventDispatcher pattern [CITED: https://svelte.dev/docs/svelte/svelte; https://svelte.dev/tutorial/component-events] -->
<script>
  import { createEventDispatcher } from "svelte";
  const dispatch = createEventDispatcher();

  export let connectionState = "IDLE";
  export let qrImage = "";

  function handleSavePath() {
    dispatch("changeSavePath");
  }
</script>
```
**Note:** Svelte 3 component events do NOT bubble — the shell must listen with `on:eventName` on the component tag; child-to-child forwarding must be explicit. [CITED: https://dev.to/chillyhill/svelte-journey-part-2-4kb8]

### Anti-Patterns to Avoid
- **Don't re-import `tokens.css` inside a component `<style>`:** tokens are global via `app.css` `@import`; re-importing breaks the single-source-of-truth rule. [VERIFIED: UI-SPEC lines 365-366]
- **Don't put a component's classes in App.svelte's `<style>`:** Svelte scopes component `<style>` by class hashing; classes must live in the component that renders the markup. [VERIFIED: UI-SPEC lines 384-388]
- **Don't inline `window.go.main.App` or `window.runtime` in components:** route through `wails.js`. [VERIFIED: App.svelte line 273; UI-SPEC lines 149-161]
- **Don't add a second state store:** one `wails.js` store is the single access point; avoid inventing per-component stores. [ASSUMED — store count decision to be confirmed in planning]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| QR code rendering | Custom QR canvas/svg implementation | `qrcode` lib (`QRCode.toDataURL`) already imported in App.svelte line 25 | Edge cases (error correction, margin, color, data-URL output) already handled; no visual change allowed [VERIFIED: App.svelte lines 603-619; package.json line 29] |
| Centralized Wails access | Per-component `window.go.main.App` / `window.runtime` calls | `src/lib/wails.js` store wrapping generated bindings | ROADMAP SVE-01; avoids duplicated async error handling and binding imports [VERIFIED: UI-SPEC lines 149-161] |
| Event name strings | Inline `"device_connected"` literals in components | `src/lib/events.js` constants | Cross-reference gate with `beamsync/events.go` prevents drift [VERIFIED: UI-SPEC lines 163-188; beamsync/events.go lines 117, 164-165] |
| CSS framework / component registry | Tailwind, shadcn, radix, base-ui | Existing `tokens.css` + 8 design-system components | UI-SPEC Registry Safety: no shadcn, no third-party registries, no vetting required; render-identical [VERIFIED: UI-SPEC lines 25-31, 414-421] |
| Toast removal timer | `setInterval` polling or custom animation engine | Existing `setTimeout(..., 3200)` pattern + `aria-live` toast-rack in app.css | Measured 3200ms self-removal [VERIFIED: App.svelte lines 160-162; app.css lines 82-117] |

**Key insight:** Custom solutions in this domain reintroduce edge cases the existing libraries already solve, and any replacement risks breaking the render-identical requirement because visual output must not change.

## Common Pitfalls

### Pitfall 1: Svelte 3 scoped CSS when splitting the global `<style>`
**What goes wrong:** App.svelte's `<style>` (lines 1409-2232) defines classes like `.progress-float`, `.receive-layout`, `.sender-dialog` that target markup moving into child components. If those classes stay in App.svelte's scoped `<style>`, they won't apply to the child's markup (Svelte hashes class names per-component).
**Why it happens:** Svelte 3 scopes component `<style>` by adding hashed class names; cross-component class application silently fails.
**How to avoid:** Move each class into the `<style>` of the component that owns its markup per UI-SPEC style ownership (e.g., `.progress-float` → ProgressOverlay.svelte, `.receive-layout` → ReceiveView.svelte). Use global token-based classes (`.nb-btn`, `.nb-card`) from tokens.css without re-declaring. [VERIFIED: UI-SPEC lines 373-389]
**Warning signs:** Elements appear unstyled after split; classes present in markup but no computed styles.

### Pitfall 2: `createEventDispatcher` events don't bubble
**What goes wrong:** A child view dispatches `changeSavePath` or `consent`; a grandchild or nested listener never sees it because Svelte 3 component events do not bubble through the DOM.
**Why it happens:** Svelte 3 custom component events are `CustomEvent`-based but do not propagate up the component tree.
**How to avoid:** The App shell listens with `on:eventName={handler}` directly on the component tag; intermediate components must explicitly forward events if nested. [CITED: https://dev.to/chillyhill/svelte-journey-part-2-4kb8; https://svelte.dev/tutorial/component-events]

### Pitfall 3: Store reactivity timing
**What goes wrong:** Mutating a plain reactive variable (`mode = "SEND"`) inside an async Wails callback doesn't re-render dependent views if the mutation isn't wrapped in Svelte's reactivity tracking, or stores are read without `$` prefix / subscription.
**Why it happens:** Svelte 3 reactivity is assignment-triggered (`$:` statements re-run on assignment); async callbacks can lose the reactive context.
**How to avoid:** Keep state mutations in the App shell where `$:` reactive declarations live (`displayUrl`, `sortedFiles`); pass derived values as props; avoid storing mutable objects in writable stores without re-assignment. [ASSUMED — Svelte 3 reactivity semantics, to be confirmed in planning]

### Pitfall 4: Wails binding async error handling
**What goes wrong:** `await StartReceiverDefault()` rejects or returns `"Error:..."`; unhandled rejections crash `onMount` or leave `serverUrl` stale.
**Why it happens:** Generated bindings return promises or error-prefixed strings; existing code uses try/catch + `startsWith("Error:")` checks.
**How to avoid:** Preserve the exact try/catch + `"Error:"` prefix + `"Cancelled"` handling patterns from App.svelte lines 524-544, 570-601, 693-710 when moving calls into views via `wails.js`. [VERIFIED: App.svelte lines 524-544, 570-601, 693-710]

### Pitfall 5: vitest Svelte 3 component mounting (render vs fireEvent)
**What goes wrong:** Tests that `render()` a component then immediately assert DOM state miss `onMount`-driven async Wails calls (which jsdom won't fulfill); or use `fireEvent` on elements that need `render`'s async settling.
**Why it happens:** `onMount` in App.svelte calls async Wails bindings; in tests those are absent/undefined, so state initialization differs from production.
**How to avoid:** Mount extracted components in isolation with mocked props (no `onMount` Wails calls); use `render()` + `screen.getBy*`/`getByRole` assertions; use `fireEvent.click`/`fireEvent.input` for user interactions; wrap async assertions with `await waitFor()`. [CITED: https://vitest.dev/guide/browser/component-testing; https://testing-library.com/docs/svelte-testing-library/api/; VERIFIED: vitest.config.ts jsdom+globals, package.json @testing-library/svelte ^4.0.0]

## Code Examples

### Writable store pattern (Svelte 3)
**Source:** [CITED: https://svelte.dev/tutorial/svelte/stores — `export const count = writable(0);`]
```javascript
import { writable } from "svelte/store";
// A module-level store other modules import and subscribe to
export const mode = writable("RECEIVE");
// Components read via $ prefix in markup or .subscribe in JS
```

### createEventDispatcher component with props + events
**Source:** [CITED: https://svelte.dev/docs/svelte/svelte; https://svelte.dev/tutorial/component-events]
```svelte
<script>
  import { createEventDispatcher } from "svelte";
  const dispatch = createEventDispatcher();

  export let connectionState = "IDLE";
  export let qrImage = "";

  function onSavePath() {
    dispatch("changeSavePath");
  }
</script>
```
**Note:** `createEventDispatcher` must be called at component initialization (during `<script>` execution), not lazily. [CITED: https://dev.to/chillyhill/svelte-journey-part-2-4kb8]

### vitest render + assert pattern for Svelte 3
**Source:** [CITED: https://vitest.dev/guide/browser/component-testing; https://testing-library.com/docs/svelte-testing-library/api/]
```typescript
import { render, fireEvent } from "@testing-library/svelte";
import ReceiveView from "../views/ReceiveView.svelte";

test("ReceiveView renders connection status", () => {
  const { getByText } = render(ReceiveView, {
    props: { connectionState: "WAITING", qrImage: "", receivedFiles: [] },
  });
  expect(getByText(/Ready to Connect|Connect via/)).toBeInTheDocument();
});

test("ReceiveView dispatches openFile on file click", async () => {
  const { getByText, emitted } = render(ReceiveView, {
    props: { connectionState: "CONNECTED", receivedFiles: [{ name: "a.txt", sizeBytes: 100 }] },
  });
  await fireEvent.click(getByText("a.txt"));
  // assert emitted event payload
});
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Single 2232-line `App.svelte` with inline `<style>` | Split into `src/lib/`, `src/views/`, `src/components/` | Phase 5 (this refactor) | Render-identical, maintainable, testable components |
| ROADMAP premise: wrap `window.runtime.*` | Wrap generated Wails bindings via `wails.js` | Corrected this session (grep `window.runtime` = 0) | Components never touch `window.runtime` directly |
| Svelte 3 (`export let`, `createEventDispatcher`) | Svelte 5 runes (`$state`, `$derived`) — NOT adopted | Future milestone only | No runes available in 3.49; runes adoption is out of scope |

**Deprecated/outdated:**
- ROADMAP premise "wrapping window.runtime.*" — superseded by generated-bindings wrapping (zero `window.runtime` calls verified).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | One `wails.js` store is sufficient; no per-component stores needed. | Architecture Patterns > Anti-Patterns | If multiple stores needed, plan must add store wiring tasks |
| A2 | `toast()` state stays in App shell and `toasts` passes as a prop to `ToastManager` (UI-SPEC Open Question #1 unresolved). | Architecture Patterns > Pattern 1 / Don't Hand-Roll | If a shared toast store is required, `wails.js` and `ToastManager` need refactor |
| A3 | `generateQR()` stays in App shell and is passed as a prop/function to views (UI-SPEC Open Question #4 unresolved). | Architecture Patterns | If QR moves to util, view prop contracts change |
| A4 | `rememberDevice` uses prop-in/event-out (not `bind:checked`) on `TransferRequestModal`. | Architecture Patterns > Pattern 3 | If two-way bind needed, `bind:` syntax differs in Svelte 3 |
| A5 | Svelte 3 reactivity (assignment-triggered `$:`) applies when state moves out of App shell. | Common Pitfalls > Pitfall 3 | State mutations outside shell may not re-render views |
| A6 | `window.go.main.App.GetVersion()` (App.svelte:273) is the only direct `window.go.main.App` call to centralize. | Architecture Patterns > Pattern 1 | If more direct calls exist, `wails.js` must cover them |

## Open Questions (RESOLVED)

All six questions were resolved during planning (see `05-01-PLAN.md` through `05-12-PLAN.md`). No open questions remain.

1. **Toast state ownership — RESOLVED:** toast stays in the App shell and `toasts` is passed as a prop to `ToastManager` with a `dismiss` event. No shared store in `wails.js`/`utils.js`. [Resolved by Plans 03, 08; UI-SPEC Component Contract]
2. **Progress timeout value — RESOLVED:** measured `30000ms` (30s) at App.svelte line 437 stays as-is; no 120s change. ProgressOverlay keeps timeout + event subscription + UI as one cohesive unit. [Resolved by Plan 09; PITFALLS 3d]
3. **`rememberDevice` binding — RESOLVED:** prop-in/event-out (`rememberDevice` prop + `rememberDeviceChange` event), not `bind:checked`. [Resolved by Plan 10; UI-SPEC Component Contract]
4. **QR generation location — RESOLVED:** `generateQR()` (qrcode) stays in the App shell; `qrImage`/`qrGenerationTimer` passed as props/functions to ReceiveView and SendView. Not moved to a shared util. [Resolved by Plans 04, 05; UI-SPEC Component Contract]
5. **Theme access from SettingsView — RESOLVED:** `applyTheme`/`toggleTheme` touch `document.documentElement`; SettingsView receives `changeTheme`/`toggleTheme` events from the App shell which owns DOM access. SettingsView does not import theme logic directly. [Resolved by Plan 07; UI-SPEC Component Contract]
6. **View import path — RESOLVED:** extracted views import design-system components via the barrel `./design-system/index.js` (established pattern in App.svelte line 39), not direct file paths. [Resolved by all view plans 04-07; UI-SPEC Style Ownership Rules]

## Environment Availability

This phase is code/config-only within `desktop/frontend/`; no external services, databases, or CLIs beyond the Node toolchain are required.

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Node.js / npm | `npm install`, `npm test`, `npm run check` | ✓ (project toolchain) | package.json declared | — |
| vitest + @testing-library/svelte + jsdom | SVE-09 component tests | ✓ (installed, `vitest.config.ts`) | ^1.0.0 / ^4.0.0 / ^24.0.0 | — |
| qrcode | QR rendering in ReceiveView/SendView | ✓ (installed dependency) | ^1.5.4 | — |
| Go / Wails bindings | `wails.js` imports `../wailsjs/...` (generated, already present) | ✓ (`wailsjs/go/main/App.js`, `wailsjs/runtime/runtime.js` present) | generated | — |
| Wails dev server (`wails dev`) | Not required for this phase (no visual change, no new UI) | — | — | Tests run in jsdom, not a live Wails window |

**Missing dependencies with no fallback:** none
**Missing dependencies with fallback:** none

*Note: `wailsjs/` bindings were confirmed present on disk (App.js, runtime.js) this session; they are generated at build time but already checked in.*

## Validation Architecture

Skipped: `workflow.nyquist_validation` is explicitly `false` in `.planning/config.json`. [VERIFIED: .planning/config.json]

## Security Domain

Applicable ASVS categories for a pure Svelte UI structural split (no new auth, no new endpoints, no new data handling):

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — (unchanged, Wails-backed) |
| V3 Session Management | no | — (unchanged, localStorage theme/sound only) |
| V4 Access Control | no | — (unchanged) |
| V5 Input Validation | yes | Settings form inputs (IPv4 via `isValidIPv4`, device lists capped at 50, extension list) — existing validation moved to `utils.js` unchanged [VERIFIED: App.svelte lines 719-803] |
| V6 Cryptography | no | — (unchanged) |

Known threat patterns for this stack:

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Insecure direct `window.go.main.App` call bypassing store | Tampering | Centralize via `wails.js` (no component imports bindings directly) [VERIFIED: App.svelte line 273] |
| Hardcoded secrets/colors in component styles | Information Disclosure | All tokens via `var(--nb-*)` from `tokens.css`; no new hardcoded values [VERIFIED: UI-SPEC lines 381-389] |
| Unescaped dynamic inline styles (XSS via style attr) | Tampering | Preserve existing inline-style usage only where values are dynamic/computed; no new dynamic style sources introduced [VERIFIED: App.svelte lines 905-936, 1205-1212, 1390-1397] |

## Sources

### Primary (HIGH confidence — in-repo, read this session)
- `desktop/frontend/package.json` lines 17, 23-29 — svelte ^3.49.0, vitest ^1.0.0, @testing-library/svelte ^4.0.0, jsdom ^24.0.0, qrcode ^1.5.4, svelte-check ^3.0.0 [VERIFIED]
- `desktop/frontend/src/App.svelte` lines 17-24 (Wails imports block), 46-47 (mode state), 83-93 (progress object), 153-163 (toast), 273 (`window.go.main.App.GetVersion()`), 281-500 (EventsOn calls), 437-455 (30s progress timeout), 509-511 (OnFileDrop), 524-544, 570-601, 693-710 (async error handling), 719-803 (settings validation), 896-902 (svelte:window), 904-937 (transfer request modal), 961-965 (toast rack), 1000-1368 (view markup), 1383-1406 (progress overlay markup), 1409-2232 (local style) [VERIFIED]
- `desktop/frontend/src/app.css` line 9 (`@import './design-system/tokens.css';`), lines 82-117 (toasts), lines 16-30, 33-39 (resets/layout) [VERIFIED]
- `desktop/frontend/src/design-system/tokens.css` lines 43-82 (colors), 119-128 (spacing), 237-248 (`.nb-input` references undefined `--nb-font-sans`) [VERIFIED]
- `desktop/frontend/src/design-system/index.js` lines 13-20 (8-component barrel export) [VERIFIED]
- `desktop/frontend/vitest.config.ts` lines 1-6 (svelte plugin, jsdom env, globals) [VERIFIED]
- `beamsync/events.go` lines 117, 164-165, 190 (event emits: device_disconnected, url_changed, token_rotated, transfer_logged) [VERIFIED]
- `desktop/frontend/wailsjs/go/main/App.js` and `desktop/frontend/wailsjs/runtime/runtime.js` present on disk [VERIFIED via shell]
- `.planning/phases/05-svelte-component-split/05-UI-SPEC.md` (approved contract: component props/events, style ownership, copy, deviations, open questions) [VERIFIED]
- `.planning/REQUIREMENTS.md` lines 42-51 (SVE-01..SVE-09), Out of Scope lines 71-78 [VERIFIED]
- `.planning/config.json` (`nyquist_validation: false`) [VERIFIED]
- `.planning/STATE.md` (Phase 4 complete, Phase 5 not yet planned) [VERIFIED]
- No `CONTEXT.md` exists for Phase 5 (`has_context: false` from init.phase-op) [VERIFIED]

### Secondary (MEDIUM confidence — official documentation)
- Svelte 3 stores: https://svelte.dev/docs/svelte/stores and https://svelte.dev/tutorial/svelte/stores — `writable`/`readable` store pattern [CITED]
- Svelte 3 createEventDispatcher: https://svelte.dev/docs/svelte/svelte and https://svelte.dev/tutorial/component-events [CITED]
- Svelte 3 component events do not bubble: https://dev.to/chillyhill/svelte-journey-part-2-4kb8 [CITED]
- vitest component testing: https://vitest.dev/guide/browser/component-testing [CITED]
- @testing-library/svelte API: https://testing-library.com/docs/svelte-testing-library/api/ [CITED]

### Tertiary (LOW confidence — training knowledge, not verified this session)
- Svelte 3 reactivity is assignment-triggered; `$:` re-runs on assignment [ASSUMED]
- One `wails.js` store suffices; no per-component stores needed [ASSUMED]

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all versions read directly from package.json this session; no new packages.
- Architecture: HIGH — based on in-repo measurements (App.svelte, tokens.css, design-system, wailsjs bindings, events.go) and UI-SPEC approved contract; a few store/binding decisions flagged [ASSUMED].
- Pitfalls: MEDIUM — Svelte 3 scoping and createEventDispatcher bubbling confirmed via docs; reactivity timing and store-count decisions are [ASSUMED].

**Research date:** Thu Nov 19 2026
**Valid until:** 30 days (Svelte 3 / vitest stack is stable; re-verify if Wails bindings regenerate)
