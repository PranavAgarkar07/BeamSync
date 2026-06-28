# BeamSync Desktop Design System

**Style:** Neubrutalism (Neo-Brutalism). Zero border-radius, hard offset shadows (no blur), 3px black borders everywhere, bold colors, bold typography.

---

## 1. Color Palette

### Light Mode

| Token | Hex | Usage |
|---|---|---|
| `--nb-primary` | `#1A56FF` | Buttons, active tabs, links, badges |
| `--nb-primary-hover` | `#0039E0` | Primary hover |
| `--nb-secondary` | `#FF7A00` | Secondary buttons, badges |
| `--nb-secondary-dark` | `#E06B00` | Secondary pressed |
| `--nb-secondary-text` | `#0A0A0A` | Text on orange (AAA) |
| `--nb-accent` / `--nb-danger` | `#FF3C5F` | Danger badges, alerts |
| `--nb-accent-dark` | `#D4003D` | Danger darker |
| `--nb-bg` | `#F5F0E8` | Page background (warm off-white) |
| `--nb-surface` | `#FFFFFF` | Card/panel surface |
| `--nb-surface-raised` | `#FFF8ED` | Elevated/hover surface |
| `--nb-text` | `#0A0A0A` | Primary text |
| `--nb-text-muted` | `#4A4A4A` | Secondary text |
| `--nb-text-inverse` | `#FFFFFF` | Text on dark |
| `--nb-success` | `#00C875` | Success |
| `--nb-warning` | `#F5A623` | Warning (amber) |
| `--nb-info` | `#1A56FF` | Info (same as primary) |
| `--nb-border-color` | `#0A0A0A` | Borders |

### Dark Mode

| Token | Hex |
|---|---|
| `--nb-bg` | `#0C1120` |
| `--nb-surface` | `#141E30` |
| `--nb-surface-raised` | `#1C2840` |
| `--nb-text` | `#E2E8F0` |
| `--nb-text-muted` | `#64748B` |
| `--nb-text-inverse` | `#0C1120` |
| `--nb-border-color` | `#2D3F5E` |
| `--nb-primary` | `#4F8EFF` |
| `--nb-primary-hover` | `#3D7FFF` |
| `--nb-secondary` | `#FF8A00` |
| `--nb-success` | `#10D98A` |
| `--nb-danger` | `#FF5C7A` |
| `--nb-warning` | `#F5A623` (unchanged) |

### Semantic Colors

| Usage | Hex |
|---|---|
| Status ping dot | `#00e676` (neon green) |
| Speed >10 MB/s | `#00ff00` |
| Speed 5-10 MB/s | `#ffb000` |
| Speed <5 MB/s | `#ff6b6b` |

---

## 2. Typography

### Font Families

| Role | Font Stack |
|---|---|
| Display/Headings | `'Syne', system-ui, sans-serif` |
| Body text | `'Manrope', system-ui, sans-serif` |
| Mono (IPs, speeds, sizes, code) | `'Space Mono', 'Courier New', monospace` |

### Font Sizes (4px-grid rem scale)

| Token | rem | px | Usage |
|---|---|---|---|
| `--nb-text-xs` | 0.75rem | 12px | Meta, badges, stats |
| `--nb-text-sm` | 0.875rem | 14px | Buttons, descriptions |
| `--nb-text-base` | 1rem | 16px | Body |
| `--nb-text-lg` | 1.125rem | 18px | File names |
| `--nb-text-xl` | 1.25rem | 20px | Section titles |
| `--nb-text-2xl` | 1.5rem | 24px | Section headings |
| `--nb-text-3xl` | 1.875rem | 30px | Large headings |
| `--nb-text-4xl` | 2.25rem | 36px | Hero/display |

### Font Weights

| Token | Value | Usage |
|---|---|---|
| `--nb-fw-regular` | 400 | Body |
| `--nb-fw-medium` | 500 | Secondary text |
| `--nb-fw-semibold` | 600 | File names |
| `--nb-fw-bold` | 700 | Headings, buttons, nav |

Also uses 800 (extra-bold) on status labels and progress percentages.

### Letter Spacing

| Element | Value |
|---|---|
| Buttons | `0.04em` |
| Badges | `0.06em` |
| Nav tabs | `0.08em` |
| Mono labels | `0.05em` |

---

## 3. Spacing System (4px grid)

| Token | Value | Typical Usage |
|---|---|---|
| `--nb-space-1` | 4px | Badge padding, tiny gaps |
| `--nb-space-2` | 8px | Button icon gaps, toast padding |
| `--nb-space-3` | 12px | Button padding, card internal gaps |
| `--nb-space-4` | 16px | Page/card padding, list items |
| `--nb-space-5` | 20px | Card padding |
| `--nb-space-6` | 24px | Main content padding |
| `--nb-space-8` | 32px | Content horizontal padding |
| `--nb-space-12` | 48px | Large section gaps |
| `--nb-space-16` | 64px | Footer spacing |

---

## 4. Border Radius

| Token | Value | Usage |
|---|---|---|
| `--nb-radius` | **0px** | Everything: buttons, cards, inputs, modals |
| `--nb-radius-chip` | **2px** | Badges only (only rounded thing in system) |

Circular (`50%`): status dots, ping dots, radar ping, progress dots.

---

## 5. Shadows (zero blur, hard offset)

### Standard Shadows

| Token | Value |
|---|---|
| `--nb-shadow-sm` | `2px 2px 0px var(--nb-border-color)` |
| `--nb-shadow-md` | `4px 4px 0px var(--nb-border-color)` |
| `--nb-shadow-lg` | `6px 6px 0px var(--nb-border-color)` |
| `--nb-shadow-xl` | `8px 8px 0px var(--nb-border-color)` |

### Colored Shadows

| Token | Value |
|---|---|
| `--nb-shadow-primary` | `4px 4px 0px var(--nb-primary)` |
| `--nb-shadow-secondary` | `4px 4px 0px var(--nb-secondary-dark)` |
| `--nb-shadow-accent` | `4px 4px 0px var(--nb-accent-dark)` |
| `--nb-shadow-success` | `4px 4px 0px #009060` |

### Interaction Rule

| State | Transform | Shadow |
|---|---|---|
| Default | none | md (4px) |
| Hover | `translate(-2px, -2px)` | lg (6px) |
| Active/Pressed | `translate(2px, 2px)` | sm (2px) |

---

## 6. Component Specifications

### Buttons

```
font: Syne bold 14px, uppercase, letter-spacing 0.04em
border: 3px solid #0A0A0A
border-radius: 0px
padding: 12px 20px
gap: 8px
box-shadow: 4px 4px 0px #0A0A0A
transition: transform 120ms ease, box-shadow 120ms ease
Hover: translate(-2px, -2px), shadow: 6px 6px
Active: translate(2px, 2px), shadow: 2px 2px
```

**Variants:**

| Variant | Background | Text Color |
|---|---|---|
| Primary | `#1A56FF` | `#FFFFFF` |
| Secondary | `#FF7A00` | `#0A0A0A` |
| Ghost | Transparent (hover: `#FF7A00`) | `#0A0A0A` |
| Danger | `#FF3C5F` | `#FFFFFF` |
| Small | 4px 8px padding (device lists) | — |

### Cards

```
background: #FFFFFF (--nb-surface)
border: 3px solid #0A0A0A
border-radius: 0px
box-shadow: 4px 4px 0px #0A0A0A
padding: 20px
```

Interactive variant adds: hover `translate(-2px,-2px)` + shadow `6px 6px`, active `translate(2px,2px)` + shadow `2px 2px`.

### Badges

```
font: Space Mono bold 12px, uppercase, letter-spacing 0.06em
padding: 4px 8px
border: 2px solid #0A0A0A
border-radius: 2px (only rounded element in the system)
```

**Variants:**

| Variant | Background | Text Color |
|---|---|---|
| Success | `#00C875` | `#0A0A0A` |
| Danger | `#FF3C5F` | `#FFFFFF` |
| Warning | `#F5A623` | `#0A0A0A` |
| Info | `#1A56FF` | `#FFFFFF` |
| Neutral | `#E0DACE` | `#0A0A0A` |

### Inputs / Form Fields

```
font: Manrope 16px, weight 500
border: 3px solid #0A0A0A
border-radius: 0px
padding: 12px 16px
box-shadow: inset 2px 2px 0px rgba(0,0,0,0.05)
Focus: border-color primary, box-shadow: 2px 2px 0px, translate(-1px,-1px)
placeholder: --nb-text-muted, opacity 0.8
```

### Progress Bars

| Property | Value |
|---|---|
| Track height | 6-12px |
| Track background | `--nb-bg` |
| Fill color | `--nb-primary` or speed-color |
| Fill transition | `width 0.2s linear` |

### Floating Progress Container

- Fixed position bottom-right (24px from edges)
- 320px wide, z-index 5000
- Contains: label badge, filename, percentage, bar track, stats row

---

## 7. Layout

### App Shell

```
┌──────────────────────────────────────┐
│ TopNavBar (56px, fixed)              │
│ Logo | [Receive] [Send] [About] | Status │
├──────────────────────────────────────┤
│ Main Content (flex:1, overflow:auto) │
│ max-width: 800px                     │
│ padding: 24px 32px                   │
└──────────────────────────────────────┘
```

### Page Layouts

| Page | Layout |
|---|---|
| Receive / Standby | Centered card, max-width 500px |
| Receive / Active | Title + StatsDashboard + Files list + ActivityPanel |
| Send | Dropzone + optional sender dialog + ActivityPanel |
| Settings | Single card with sections |
| About | Centered, max-width 600px |

### Responsive Breakpoints

| Breakpoint | Changes |
|---|---|
| ≤760px | ActivityPanel → single column, StatsDashboard → 2 columns |
| ≤640px | Navbar URL hidden, tab padding reduced |
| ≤480px | DeviceCard padding reduced, dropzone shrinks |

---

## 8. Icons

**Approach:** No external icon library — all inline SVGs, Lucide-style (24x24 viewBox, stroke-width 2.5, `stroke-linecap="square"`).

### Icon Inventory

| Icon | Used In |
|---|---|
| Settings gear | TopNavBar |
| OS icons (Windows, Apple, Tux, Android, iOS, Monitor) | DeviceCard |
| Lightning bolt | DeviceCard (speed), TransferProgressBar |
| Upload arrow | FileDropZone |
| File type icons (file, video, audio, image, PDF, archive, doc, sheet, code, APK) | FileDropZone |
| Arrow down/up | TransferProgressBar |
| Clock | TransferProgressBar |
| Close (X) | TransferProgressBar |
| Wi-Fi signal | ConnectedDevicesPanel |
| Refresh | ConnectedDevicesPanel |
| Empty state (box + folder + document) | App.svelte |
| Checkmark circle | TransferComplete |

**Emoji fallback:** 📄 🖼️ 🎬 🎵 📦 📝 📱 ⚙️ 📁

---

## 9. Animations / Transitions

| Animation | Timing | Notes |
|---|---|---|
| Button hover | 120ms ease | translate + shadow |
| Card hover | 120ms ease | translate + shadow |
| Page transitions | 250ms | Fly (y=15px) |
| Progress bar fill | 200ms linear | width transition |
| TransferComplete entry | 420-560ms | Overshoot spring |
| Splash doors | 700ms | Slide apart |
| Status dot pulse | 1.5s infinite | scale 0.85 ↔ 1.15 |
| Indeterminate bar | 1.4s infinite | translateX slide |
| Toast in | 200ms ease | translateY 100% → 0 |
| Toast out | 200ms ease | translateY 20px + fade |
| Update banner | 300ms ease | slideDown |

### Disable Animations

All animations disabled when `prefers-reduced-motion: reduce` is active.

---

## 10. Source Files

| File | Path |
|---|---|
| Token definitions | `desktop/frontend/src/design-system/tokens.css` |
| Global app CSS | `desktop/frontend/src/app.css` |
| App shell | `desktop/frontend/src/App.svelte` |
| Top nav bar | `desktop/frontend/src/design-system/TopNavBar.svelte` |
| Device card | `desktop/frontend/src/design-system/DeviceCard.svelte` |
| File drop zone | `desktop/frontend/src/design-system/FileDropZone.svelte` |
| Transfer progress bar | `desktop/frontend/src/design-system/TransferProgressBar.svelte` |
| Transfer complete | `desktop/frontend/src/design-system/TransferComplete.svelte` |
| Connected devices panel | `desktop/frontend/src/design-system/ConnectedDevicesPanel.svelte` |
| Activity panel | `desktop/frontend/src/design-system/ActivityPanel.svelte` |
| Transfer stats dashboard | `desktop/frontend/src/design-system/TransferStatsDashboard.svelte` |
| Splash screen | `desktop/frontend/src/SplashScreen.svelte` |
| Design system showcase | `desktop/frontend/src/design-system/DesignSystemShowcase.svelte` |
| Community site CSS | `community/style.css` |

---

> Use this as the source of truth when building the KMP mobile app. Colors, fonts, spacing, shadows, and interaction patterns should match exactly to maintain brand consistency across desktop and mobile.
