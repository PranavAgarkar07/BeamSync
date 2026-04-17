<!--
  DeviceCard.svelte
  ─────────────────────────────────────────────────────────────────
  Displays a peer device discovered on the local network.

  Props:
    name        {string}  — Device hostname, e.g. "Pranav's iPhone"
    ip          {string}  — IP address string, e.g. "192.168.1.42"
    os          {string}  — One of: "windows" | "macos" | "linux" | "android" | "ios" | "unknown"
    status      {string}  — One of: "connected" | "available" | "busy" | "offline"
    speed       {string}  — Current transfer speed string, e.g. "12.4 MB/s". Optional.
    on:select           — fires when card is clicked (for selecting as send target)

  Usage:
    <DeviceCard
      name="Pranav's iPhone"
      ip="192.168.1.42"
      os="ios"
      status="connected"
      speed="8.2 MB/s"
      on:select={handleSelect}
    />
-->
<script>
  import { createEventDispatcher } from 'svelte';

  export let name    = 'Unknown Device';
  export let ip      = '0.0.0.0';
  export let os      = 'unknown';
  export let status  = 'available';
  export let speed   = '';

  const dispatch = createEventDispatcher();

  // ── OS SVG icons (Lucide-style, 24x24 viewBox) ──────────────────────────
  const osIcons = {
    windows: `<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
      <path d="M3 5.557L10.643 4.5V11.5H3V5.557ZM11.357 4.393L21 3V11.5H11.357V4.393ZM3 12.5H10.643V19.5L3 18.443V12.5ZM11.357 12.5H21V21L11.357 19.607V12.5Z"/>
    </svg>`,
    macos: `<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
      <path d="M18.71 19.5c-.83 1.24-1.71 2.45-3.05 2.47-1.34.03-1.77-.79-3.29-.79-1.53 0-2 .77-3.27.82-1.31.05-2.3-1.32-3.14-2.53C4.25 17 2.94 12.45 4.7 9.39c.87-1.52 2.43-2.48 4.12-2.51 1.28-.02 2.5.87 3.29.87.78 0 2.26-1.07 3.8-.91.65.03 2.47.26 3.64 1.98l-.09.06c-.22.14-2.2 1.28-2.18 3.81.03 3.02 2.65 4.03 2.68 4.04-.03.07-.42 1.44-1.38 2.83M13 3.5c.73-.83 1.94-1.46 2.94-1.5.13 1.17-.34 2.35-1.04 3.19-.69.85-1.83 1.51-2.95 1.42-.15-1.15.41-2.35 1.05-3.11z"/>
    </svg>`,
    linux: `<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
      <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 15v-2h2v2h-2zm2-4h-2c0-3.25 3-3 3-5 0-1.1-.9-2-2-2s-2 .9-2 2H8c0-2.21 1.79-4 4-4s4 1.79 4 4c0 2.5-3 2.75-3 5z"/>
    </svg>`,
    android: `<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
      <path d="M17.523 15.341a1 1 0 1 0 0-2 1 1 0 0 0 0 2zm-11.046 0a1 1 0 1 0 0-2 1 1 0 0 0 0 2zM3.513 8.683A9.967 9.967 0 0 1 12 5c3.348 0 6.315 1.647 8.17 4.183l1.497-2.596A.5.5 0 0 0 21.5 6h-1.086a1 1 0 0 0-.5-.134h-.001l-2.121-3.673a.5.5 0 0 0-.863.5l1.413 2.446A9.95 9.95 0 0 0 12 3C8.697 3 5.73 4.482 3.73 6.834L5.14 4.403a.5.5 0 0 0-.863-.5L2.156 7.576A.5.5 0 0 0 2.5 8.25h1.013z"/>
      <path d="M12 6a8 8 0 0 0-8 8v2a1 1 0 0 0 1 1h14a1 1 0 0 0 1-1v-2a8 8 0 0 0-8-8z"/>
    </svg>`,
    ios: `<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
      <rect x="7" y="2" width="10" height="20" rx="2" ry="2"/>
      <line x1="12" y1="18" x2="12" y2="18" stroke="white" stroke-width="2" stroke-linecap="round"/>
    </svg>`,
    unknown: `<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="square" stroke-linejoin="miter" aria-hidden="true">
      <rect x="2" y="3" width="20" height="14" rx="0"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/>
    </svg>`,
  };

  const statusMap = {
    connected: { label: 'CONNECTED', cls: 'badge--success' },
    available: { label: 'AVAILABLE', cls: 'badge--info'    },
    busy:      { label: 'BUSY',      cls: 'badge--warning'  },
    offline:   { label: 'OFFLINE',   cls: 'badge--neutral'  },
  };

  $: icon   = osIcons[os]   ?? osIcons.unknown;
  $: badge  = statusMap[status] ?? statusMap.available;
  $: isClickable = status === 'available' || status === 'connected';
</script>

<button
  class="device-card"
  class:device-card--clickable={isClickable}
  class:device-card--offline={status === 'offline'}
  disabled={!isClickable}
  on:click={() => isClickable && dispatch('select', { name, ip, os, status })}
  aria-label="Device {name} — {badge.label}"
>
  <!-- OS icon -->
  <span class="device-card__icon" aria-hidden="true">
    {@html icon}
  </span>

  <!-- Info block -->
  <div class="device-card__info">
    <div class="device-card__name">{name}</div>
    <div class="device-card__ip nb-mono">{ip}</div>
    {#if speed}
      <div class="device-card__speed nb-mono">
        <!-- Lucide Zap icon -->
        <svg xmlns="http://www.w3.org/2000/svg" width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="square" aria-hidden="true"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg>
        {speed}
      </div>
    {/if}
  </div>

  <!-- Status badge -->
  <span class="nb-badge {badge.cls}" aria-label={badge.label}>
    {badge.label}
  </span>
</button>

<style>
  .device-card {
    display: flex;
    align-items: center;
    gap: var(--nb-space-4);
    width: 100%;
    padding: var(--nb-space-4) var(--nb-space-5);
    background: var(--nb-surface);
    border: var(--nb-border-lg);
    border-radius: var(--nb-radius);
    box-shadow: var(--nb-shadow-md);
    text-align: left;
    font-family: var(--nb-font-display);
    cursor: default;
    transition: var(--nb-transition);
  }

  .device-card--clickable {
    cursor: pointer;
  }

  .device-card--clickable:hover {
    transform: translate(-2px, -2px);
    box-shadow: var(--nb-shadow-lg);
  }

  .device-card--clickable:active {
    transform: translate(2px, 2px);
    box-shadow: var(--nb-shadow-sm);
    border-color: var(--nb-primary);
  }

  .device-card--offline {
    opacity: 0.5;
  }

  .device-card__icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 44px;
    height: 44px;
    background: var(--nb-bg);
    border: var(--nb-border);
    flex-shrink: 0;
  }

  .device-card__info {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: var(--nb-space-1);
  }

  .device-card__name {
    font-size: var(--nb-text-base);
    font-weight: var(--nb-fw-bold);
    color: var(--nb-text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .device-card__ip {
    font-size: var(--nb-text-xs);
    color: var(--nb-text-muted);
  }

  .device-card__speed {
    display: inline-flex;
    align-items: center;
    gap: 3px;
    font-size: var(--nb-text-xs);
    font-weight: var(--nb-fw-bold);
    color: var(--nb-success);
    margin-top: 1px;
  }

  /* Badge overrides (inline, no import needed — tokens must be on the root) */
  .nb-badge {
    display: inline-flex;
    align-items: center;
    gap: var(--nb-space-1);
    padding: var(--nb-space-1) var(--nb-space-2);
    font-family: var(--nb-font-mono);
    font-size: 10px;
    font-weight: var(--nb-fw-bold);
    border: 2px solid var(--nb-border-color);
    border-radius: var(--nb-radius-chip);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    line-height: 1;
    flex-shrink: 0;
  }

  .badge--success { background: var(--nb-success);  color: #fff; }
  .badge--info    { background: var(--nb-primary);  color: #fff; }
  .badge--warning { background: var(--nb-warning);  color: #0A0A0A; }
  .badge--neutral { background: #E0DACE;             color: var(--nb-text); }

  @media (prefers-reduced-motion: reduce) {
    .device-card { transition: none; }
    .device-card--clickable:hover { transform: none; }
  }

  @media (max-width: 480px) {
    .device-card { padding: var(--nb-space-3) var(--nb-space-4); gap: var(--nb-space-3); }
    .device-card__icon { width: 36px; height: 36px; }
  }
</style>
