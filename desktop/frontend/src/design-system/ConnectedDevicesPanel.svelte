<!--
  ConnectedDevicesPanel.svelte
  ─────────────────────────────────────────────────────────────────
  Scrollable list of peers found on the local network.
  Shows device cards with a live ping badge and a scan button.

  Props:
    devices  {Array<{name, ip, os, status, speed, ping}>}
              — ping is a number in ms (or null if unknown)
    scanning {boolean} — Whether a network scan is in progress

  Events:
    on:select  — { device } when user clicks a device card
    on:scan    — when user clicks "Scan network" button

  Usage:
    <ConnectedDevicesPanel
      {devices}
      scanning={isScanning}
      on:select={handleSelect}
      on:scan={startScan}
    />
-->
<script>
  import { createEventDispatcher } from 'svelte';
  import DeviceCard from './DeviceCard.svelte';

  export let devices  = [];
  export let scanning = false;

  const dispatch = createEventDispatcher();

  // Lucide icons
  const iconWifi    = `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="square" aria-hidden="true"><path d="M1.42 9a16 16 0 0 1 21.16 0"/><path d="M5 12.55a11 11 0 0 1 14.08 0"/><path d="M8.53 16.11a6 6 0 0 1 6.95 0"/><line x1="12" y1="20" x2="12.01" y2="20"/></svg>`;
  const iconRefresh = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="square" aria-hidden="true"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>`;

  function pingClass(ping) {
    if (ping == null) return 'ping--unknown';
    if (ping < 10)   return 'ping--excellent';
    if (ping < 30)   return 'ping--good';
    if (ping < 100)  return 'ping--ok';
    return 'ping--poor';
  }

  function pingLabel(ping) {
    if (ping == null) return '—';
    return `${ping}ms`;
  }
</script>

<div class="cdp">
  <!-- Panel header -->
  <div class="cdp__header">
    <div class="cdp__title-group">
      <span class="cdp__icon" aria-hidden="true">{@html iconWifi}</span>
      <h2 class="cdp__title">Local Network</h2>
      <span class="cdp__count nb-mono" aria-label="{devices.length} devices">{devices.length}</span>
    </div>

    <button
      class="cdp__scan-btn"
      class:cdp__scan-btn--scanning={scanning}
      on:click={() => dispatch('scan')}
      disabled={scanning}
      aria-label={scanning ? 'Scanning…' : 'Scan network'}
    >
      <span class="cdp__scan-icon" class:spin={scanning} aria-hidden="true">
        {@html iconRefresh}
      </span>
      {scanning ? 'Scanning…' : 'Scan'}
    </button>
  </div>

  <!-- Device list -->
  {#if devices.length === 0}
    <div class="cdp__empty" role="status">
      {#if scanning}
        <span class="cdp__scanning-label">Discovering devices on your network…</span>
      {:else}
        <span>No devices found. Run a network scan to discover peers.</span>
      {/if}
    </div>
  {:else}
    <div class="cdp__list" role="list" aria-label="Network devices">
      {#each devices as device (device.ip)}
        <div class="cdp__item" role="listitem">
          <!-- Ping badge -->
          <div class="cdp__ping-row">
            <span class="cdp__ping {pingClass(device.ping)} nb-mono" aria-label="Ping {pingLabel(device.ping)}">
              <span class="cdp__ping-dot" aria-hidden="true"></span>
              {pingLabel(device.ping)}
            </span>
          </div>
          <DeviceCard
            name={device.name}
            ip={device.ip}
            os={device.os}
            status={device.status}
            speed={device.speed}
            on:select={(e) => dispatch('select', e.detail)}
          />
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .cdp {
    display: flex;
    flex-direction: column;
    gap: var(--nb-space-4);
    width: 100%;
  }

  /* Header */
  .cdp__header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--nb-space-4);
    padding-bottom: var(--nb-space-4);
    border-bottom: var(--nb-border-lg);
  }

  .cdp__title-group {
    display: flex;
    align-items: center;
    gap: var(--nb-space-3);
  }

  .cdp__icon {
    display: flex;
    align-items: center;
    color: var(--nb-text);
  }

  .cdp__title {
    margin: 0;
    font-family: var(--nb-font-display);
    font-size: var(--nb-text-xl);
    font-weight: var(--nb-fw-bold);
    color: var(--nb-text);
    letter-spacing: -0.02em;
  }

  .cdp__count {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 24px;
    height: 24px;
    padding: 0 var(--nb-space-2);
    background: var(--nb-secondary);
    border: 2px solid var(--nb-border-color);
    font-size: var(--nb-text-xs);
    font-weight: var(--nb-fw-bold);
    color: var(--nb-text);
    line-height: 1;
  }

  /* Scan button */
  .cdp__scan-btn {
    display: inline-flex;
    align-items: center;
    gap: var(--nb-space-2);
    padding: var(--nb-space-2) var(--nb-space-4);
    background: var(--nb-surface);
    border: var(--nb-border-lg);
    border-radius: var(--nb-radius);
    box-shadow: var(--nb-shadow-sm);
    font-family: var(--nb-font-mono);
    font-size: var(--nb-text-xs);
    font-weight: var(--nb-fw-bold);
    letter-spacing: 0.05em;
    text-transform: uppercase;
    color: var(--nb-text);
    cursor: pointer;
    transition: var(--nb-transition);
  }

  .cdp__scan-btn:hover:not(:disabled) {
    background: var(--nb-secondary);
    transform: translate(-2px, -2px);
    box-shadow: var(--nb-shadow-md);
  }

  .cdp__scan-btn:active:not(:disabled) {
    transform: translate(1px, 1px);
    box-shadow: none;
  }

  .cdp__scan-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .cdp__scan-btn--scanning {
    background: var(--nb-bg);
  }

  .cdp__scan-icon {
    display: flex;
    align-items: center;
  }

  .spin {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  /* Empty state */
  .cdp__empty {
    padding: var(--nb-space-8) var(--nb-space-6);
    background: var(--nb-bg);
    border: 2px dashed var(--nb-border-color);
    font-family: var(--nb-font-display);
    font-size: var(--nb-text-sm);
    color: var(--nb-text-muted);
    text-align: center;
  }

  .cdp__scanning-label {
    display: inline-flex;
    align-items: center;
    gap: var(--nb-space-2);
    font-weight: var(--nb-fw-medium);
  }

  /* Device list */
  .cdp__list {
    display: flex;
    flex-direction: column;
    gap: var(--nb-space-3);
    max-height: 480px;
    overflow-y: auto;
    scrollbar-width: thin;
    scrollbar-color: var(--nb-border-color) transparent;
    padding-right: var(--nb-space-1);
  }

  .cdp__item {
    display: flex;
    flex-direction: column;
    gap: var(--nb-space-1);
  }

  /* Ping badge */
  .cdp__ping-row {
    display: flex;
    justify-content: flex-end;
  }

  .cdp__ping {
    display: inline-flex;
    align-items: center;
    gap: var(--nb-space-1);
    font-size: 10px;
    font-weight: var(--nb-fw-bold);
    letter-spacing: 0.04em;
    padding: 1px var(--nb-space-2);
    border: 2px solid var(--nb-border-color);
  }

  .cdp__ping-dot {
    display: inline-block;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: currentColor;
    animation: ping-blink 1.8s ease-in-out infinite;
  }

  @keyframes ping-blink {
    0%, 100% { opacity: 1;   }
    50%       { opacity: 0.3; }
  }

  .ping--excellent { color: var(--nb-success); background: #f0fff8; }
  .ping--good      { color: #00a060;           background: #f0fff8; }
  .ping--ok        { color: var(--nb-warning);  background: #fffbf0; }
  .ping--poor      { color: var(--nb-danger);   background: #fff4f6; }
  .ping--unknown   { color: var(--nb-text-muted); background: var(--nb-bg); }

  .nb-mono { font-family: var(--nb-font-mono); }

  @media (prefers-reduced-motion: reduce) {
    .spin, .cdp__ping-dot { animation: none; }
    .cdp__scan-btn { transition: none; }
    .cdp__scan-btn:hover:not(:disabled) { transform: none; }
  }

  @media (max-width: 480px) {
    .cdp__list { max-height: 320px; }
  }
</style>
