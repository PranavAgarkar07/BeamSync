<!--
  TransferProgressBar.svelte
  ─────────────────────────────────────────────────────────────────
  Full-width transfer status block — shows sender/receiver labels,
  filename, speed, ETA, and a cancel button.

  Props:
    filename    {string}  — Active filename being transferred
    percent     {number}  — 0–100 progress, or -1 for indeterminate
    speed       {string}  — Formatted speed string, e.g. "8.4 MB/s"
    received    {string}  — Formatted bytes done, e.g. "42.3 MB"
    total       {string}  — Formatted total, e.g. "180 MB"
    eta         {string}  — Time remaining string, e.g. "1m 20s"
    elapsed     {string}  — Elapsed time string, e.g. "0m 30s"
    role        {string}  — "sender" | "receiver"
    active      {boolean} — Whether a transfer is in progress

  Events:
    on:cancel — fired when Cancel button is clicked

  Usage:
    <TransferProgressBar
      filename="IMG_vacation.jpg"
      percent={72}
      speed="11.2 MB/s"
      received="82.4 MB"
      total="114.0 MB"
      eta="3s"
      elapsed="12s"
      role="receiver"
      active
      on:cancel={handleCancel}
    />
-->
<script>
  import { createEventDispatcher } from 'svelte';

  export let filename = '';
  export let percent  = 0;
  export let speed    = '0 MB/s';
  export let received = '0 MB';
  export let total    = '0 MB';
  export let eta      = '—';
  export let elapsed  = '0s';
  export let role     = 'receiver';   // 'sender' | 'receiver'
  export let active   = false;

  const dispatch = createEventDispatcher();

  // Lucide icons
  const iconArrowDown = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="square" aria-hidden="true"><line x1="12" y1="5" x2="12" y2="19"/><polyline points="19 12 12 19 5 12"/></svg>`;
  const iconArrowUp   = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="square" aria-hidden="true"><line x1="12" y1="19" x2="12" y2="5"/><polyline points="5 12 12 5 19 12"/></svg>`;
  const iconZap       = `<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="square" aria-hidden="true"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg>`;
  const iconClock     = `<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="square" aria-hidden="true"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>`;
  const iconX         = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="square" aria-hidden="true"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>`;

  $: isIndeterminate = percent === -1;
  $: displayPct = isIndeterminate ? '—' : `${percent}%`;
  $: roleLabel = role === 'sender' ? 'SENDING' : 'RECEIVING';
  $: roleIcon  = role === 'sender' ? iconArrowUp : iconArrowDown;
  $: roleCls   = role === 'sender' ? 'role--sender' : 'role--receiver';
</script>

<div class="xfer" class:xfer--active={active} role="region" aria-label="Transfer status">
  <!-- Header row -->
  <div class="xfer__header">
    <span class="xfer__role {roleCls}">
      <span class="xfer__role-icon" aria-hidden="true">{@html roleIcon}</span>
      {roleLabel}
    </span>

    <span class="xfer__filename" title={filename}>{filename || 'Waiting…'}</span>

    <span class="xfer__pct nb-mono" aria-label="Progress {displayPct}">
      {displayPct}
    </span>
  </div>

  <!-- Progress bar -->
  <div
    class="xfer__track"
    role="progressbar"
    aria-valuenow={isIndeterminate ? undefined : percent}
    aria-valuemin="0"
    aria-valuemax="100"
    aria-valuetext={displayPct}
  >
    <div
      class="xfer__fill"
      class:xfer__fill--indeterminate={isIndeterminate}
      style={isIndeterminate ? '' : `width: ${Math.min(100, percent)}%`}
    ></div>
  </div>

  <!-- Stats row -->
  <div class="xfer__stats">
    <span class="xfer__stat nb-mono">
      {@html iconZap}
      {speed}
    </span>
    <span class="xfer__stat nb-mono">
      {received} / {total}
    </span>
    <span class="xfer__stat nb-mono">
      {@html iconClock}
      ETA {eta}
    </span>
    <span class="xfer__stat nb-mono">
      ◷ {elapsed}
    </span>

    <!-- Spacer -->
    <span style="flex:1"></span>

    {#if active}
      <button
        class="xfer__cancel"
        on:click={() => dispatch('cancel')}
        aria-label="Cancel transfer"
      >
        {@html iconX}
        CANCEL
      </button>
    {/if}
  </div>
</div>

<style>
  .xfer {
    display: flex;
    flex-direction: column;
    gap: var(--nb-space-3);
    padding: var(--nb-space-5);
    background: var(--nb-surface);
    border: var(--nb-border-lg);
    border-radius: var(--nb-radius);
    box-shadow: var(--nb-shadow-md);
  }

  .xfer--active {
    border-color: var(--nb-primary);
    box-shadow: var(--nb-shadow-primary);
  }

  /* Header */
  .xfer__header {
    display: flex;
    align-items: center;
    gap: var(--nb-space-3);
    flex-wrap: wrap;
  }

  .xfer__role {
    display: inline-flex;
    align-items: center;
    gap: var(--nb-space-1);
    font-family: var(--nb-font-mono);
    font-size: 10px;
    font-weight: var(--nb-fw-bold);
    letter-spacing: 0.08em;
    padding: 3px var(--nb-space-2);
    border: 2px solid var(--nb-border-color);
    flex-shrink: 0;
  }

  .role--receiver {
    background: var(--nb-primary);
    color: #fff;
  }

  .role--sender {
    background: var(--nb-secondary);
    color: var(--nb-secondary-text, #0A0A0A);
  }

  .xfer__role-icon {
    display: flex;
    align-items: center;
  }

  .xfer__filename {
    flex: 1;
    font-family: var(--nb-font-display);
    font-weight: var(--nb-fw-bold);
    font-size: var(--nb-text-base);
    color: var(--nb-text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }

  .xfer__pct {
    font-size: var(--nb-text-xl);
    font-weight: var(--nb-fw-bold);
    color: var(--nb-text);
    min-width: 56px;
    text-align: right;
    flex-shrink: 0;
  }

  /* Track */
  .xfer__track {
    width: 100%;
    height: 12px;
    background: var(--nb-bg);
    border: 2px solid var(--nb-border-color);
    overflow: hidden;
  }

  .xfer__fill {
    height: 100%;
    background: var(--nb-primary);
    transition: width 250ms ease-out;
  }

  .xfer__fill--indeterminate {
    width: 40% !important;
    animation: indeterminate-slide 1.4s ease-in-out infinite alternate;
  }

  @keyframes indeterminate-slide {
    from { transform: translateX(-100%); }
    to   { transform: translateX(300%); }
  }

  /* Stats */
  .xfer__stats {
    display: flex;
    align-items: center;
    gap: var(--nb-space-4);
    flex-wrap: wrap;
  }

  .xfer__stat {
    display: inline-flex;
    align-items: center;
    gap: 3px;
    font-size: var(--nb-text-xs);
    color: var(--nb-text-muted);
    font-weight: var(--nb-fw-regular);
  }

  /* Cancel button */
  .xfer__cancel {
    display: inline-flex;
    align-items: center;
    gap: var(--nb-space-1);
    padding: var(--nb-space-1) var(--nb-space-3);
    background: transparent;
    border: 2px solid var(--nb-danger);
    border-radius: var(--nb-radius);
    color: var(--nb-danger);
    font-family: var(--nb-font-mono);
    font-size: 10px;
    font-weight: var(--nb-fw-bold);
    letter-spacing: 0.06em;
    cursor: pointer;
    transition: var(--nb-transition);
    flex-shrink: 0;
  }

  .xfer__cancel:hover {
    background: var(--nb-danger);
    color: #fff;
    transform: translate(-1px, -1px);
    box-shadow: 2px 2px 0 #6b0018;
  }

  .xfer__cancel:active {
    transform: translate(1px, 1px);
    box-shadow: none;
  }

  .nb-mono {
    font-family: var(--nb-font-mono);
  }

  @media (prefers-reduced-motion: reduce) {
    .xfer__fill, .xfer__cancel { transition: none; }
    .xfer__fill--indeterminate { animation: none; width: 60% !important; }
    .xfer__cancel:hover { transform: none; }
  }

  @media (max-width: 480px) {
    .xfer__stats { gap: var(--nb-space-3); }
    .xfer__pct   { font-size: var(--nb-text-lg); }
  }
</style>
