# Nav Highlight Bleed Fix — Blog/Contact stays lit on other sections

## Symptom
Clicking **Blog** lights up **Contact**. More generally, **Contact** (last nav item / footer-adjacent section) stays highlighted while any other section is in view.

## Root cause
`community/src/components/ScrollReveal.astro:26-36` (old):

- One `IntersectionObserver` with `threshold: 0.4`, no `rootMargin` for the 56px fixed navbar.
- Per-entry loop set inline `a.style.color` immediately. When 2 sections intersect at once (short sections, tall viewport, footer at page bottom), the **last entry wins non-deterministically**.
- Observed set (`section[id]`) ≠ linked set (`.nav-links a[href^="#"]`). Sections with no link (e.g. `hero`) cleared everything; missing `#blog`/`#contact` IDs left a stale highlight.
- No `.active` class, no `aria-current`. Inline color fought `:hover`, `.nav-new !important`, and relied on color alone (WCAG fail).
- `Navbar.astro` logo `href="#"` also matched `a[href^="#"]`.

Desktop `TopNavBar.svelte` could not double-highlight (mutually exclusive `===`), but `settings` had no active visual, no normalization, and no `:focus-visible`.

## Changes

### 1. `community/src/components/ScrollReveal.astro`
- Center-band scrollspy: `rootMargin: '-45% 0px -50% 0px', threshold: 0` — only one section crosses the middle line at a time (MDN + Bootstrap Scrollspy pattern).
- Winner election: pick intersecting entry with max `intersectionRatio`.
- Single-active helper `setActiveNav(id)`: toggles `.active` + `aria-current="true"`, removes from all others.
- Click sets active immediately (covers Blog/Contact + future `#id` links before observer fires).
- Only observes sections that actually have a nav link (`linkById` filter); excludes `href="#"` logo and external `http` links.
- Clears legacy inline `style.color`.
- Deep-link `/#contact` highlights on load.

### 2. `community/src/components/Navbar.astro`
- Added `.nav-links a.active` + `[aria-current="true"]` style: background + border + underline bar + semibold — not color-alone.

### 3. `desktop/frontend/src/design-system/TopNavBar.svelte`
- `$: normActive = (activeTab ?? '').toLowerCase().trim()` single source of truth.
- `receive`/`send`/`about` compare against `normActive` (case-safe).
- Settings button gets `--active` visual + `aria-pressed`.
- Added `:focus-visible` outline for tab + icon button.

No routing, no section IDs, no content changed. Highlight-only.

## Specialist swarm (parallel, internet-informed)
Ran 4 parallel agents before editing:

1. **Scroll-interaction specialist** — IntersectionObserver / Bootstrap Scrollspy v5.3 / MDN. Diagnosed last-write-wins race, `threshold`/`rootMargin` offset, footer trap, inline-style flaw. Recommended center-band + single-active class.
2. **WCAG 2.2 AA auditor** — Found missing `aria-current` on Astro nav, ghost-tabbable closed mobile menu, missing `:focus-visible` on Svelte, color-alone active. Prescribed class + `aria-current` + focus ring.
3. **Svelte state specialist** — Proved `TopNavBar` bleed impossible (`===` exclusivity), found `settings` blank-state. Prescribed normalization + settings active.
4. **QA strategist** — Gave build/lint/test matrix + 5 manual checks below.

## Verification
```bash
cd community && npm run build          # PASS
cd desktop/frontend && npm run check   # 0 errors
cd desktop/frontend && npm run lint    # 0 errors
cd desktop/frontend && npm run test    # pass (no tests)
cd desktop/frontend && npm run build   # PASS
```

Reviewer follow-ups applied:
- `prefers-reduced-motion` no longer early-returns — reveal is skipped but scrollspy still runs.
- All `IntersectionObserver` uses guarded (`hasIO`), no throw on old browsers.

Manual:
1. Click each nav link → exactly one `.active` at a time.
2. Slow scroll top→bottom → active follows viewport, no dual-active, no flicker.
3. Click Blog → Blog active; scroll to Contact → Contact takes over, Blog clears. Direct `/#blog`, `/#contact` loads highlight correctly.
4. Mobile ≤1100px: hamburger opens, tap link closes menu + sets active.
5. Keyboard Tab/Enter + `prefers-reduced-motion`: focus visible, instant jump, active still updates, no console errors.
