<!--
  TopNavBar.svelte
  ─────────────────────────────────────────────────────────────────
  Application top navigation bar — BeamSync logo, tab navigation,
  network status indicator, and settings button.

  Props:
    activeTab      {string}  — "receive" | "send" | "about"
    networkStatus  {string}  — "idle" | "waiting" | "connected" | "disconnected"
    serverUrl      {string}  — Current server URL (displayed in status)
    appVersion     {string}  — App version string e.g. "v2.2"

  Events:
    on:tabChange   — { tab } when a nav tab is clicked
    on:settings    — when settings icon is clicked
    on:reset       — when the disconnect/reset button is clicked

  Usage:
    <TopNavBar
      activeTab="receive"
      networkStatus="connected"
      serverUrl="http://192.168.1.10:8080"
      appVersion="v2.2"
      on:tabChange={({ detail }) => switchMode(detail.tab)}
      on:settings={openSettings}
      on:reset={resetAll}
    />
-->
<script>
  import { createEventDispatcher } from 'svelte';
  import logoImg from '../assets/images/icon.png';

  export let activeTab     = 'receive';
  export let networkStatus = 'idle';
  export let serverUrl     = '';
  export let appVersion    = '';

  const dispatch = createEventDispatcher();

  const tabs = [
    { id: 'receive', label: 'Receive' },
    { id: 'send',    label: 'Send'    },
  ];

  const statusConfig = {
    idle:         { label: 'OFFLINE',    cls: 'status--idle'  },
    waiting:      { label: 'LISTENING',  cls: 'status--wait'  },
    connected:    { label: 'LINKED',     cls: 'status--ok'    },
    disconnected: { label: 'LOST',       cls: 'status--err'   },
  };

  $: status = statusConfig[networkStatus] ?? statusConfig.idle;

  // Lucide icons
  const iconSettings = `<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="square" aria-hidden="true"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>`;

</script>

<header class="navbar">
  <!-- ── Logo ───────────────────────────────────────────────── -->
  <div class="navbar__logo" aria-label="BeamSync {appVersion}">
    <img src={logoImg} alt="BeamSync Icon" class="navbar__logo-img" aria-hidden="true" />
    <span class="navbar__logo-text">BeamSync</span>
    <span class="navbar__logo-ver nb-mono">{appVersion}</span>
  </div>

  <!-- ── Tab Navigation ────────────────────────────────────── -->
  <nav class="navbar__nav" aria-label="Primary navigation">
    {#each tabs as tab}
      <button
        id="nav-tab-{tab.id}"
        class="navbar__tab"
        class:navbar__tab--active={activeTab === tab.id}
        on:click={() => dispatch('tabChange', { tab: tab.id })}
        aria-current={activeTab === tab.id ? 'page' : undefined}
      >
        {tab.label}
      </button>
    {/each}

    <div class="nav-spacer" style="flex: 1;"></div>
    
    <button
      id="nav-tab-about"
      class="navbar__tab"
      style="border-left: var(--nb-border-lg); border-right: none;"
      class:navbar__tab--active={activeTab === 'about'}
      on:click={() => dispatch('tabChange', { tab: 'about' })}
      aria-current={activeTab === 'about' ? 'page' : undefined}
    >
      About
    </button>
  </nav>

  <!-- ── Right side: status + settings ─────────────────────── -->
  <div class="navbar__right">
    <!-- Network status pill -->
    <div class="navbar__status {status.cls}" role="status" aria-live="polite">
      <span class="status__dot" aria-hidden="true"></span>
      <span class="status__label nb-mono" aria-label="Network status: {status.label}">
        {status.label}
      </span>
      {#if serverUrl && networkStatus !== 'idle'}
        <span class="status__url nb-mono" title={serverUrl}>
          {serverUrl.replace(/^https?:\/\//, '').split('/')[0]}
        </span>
      {/if}
    </div>

    <!-- Settings button -->
    <button
      id="settings-btn"
      class="navbar__icon-btn"
      on:click={() => dispatch('settings')}
      aria-label="Open settings"
      title="Settings"
    >
      {@html iconSettings}
    </button>
  </div>
</header>

<style>
  .navbar {
    display: flex;
    align-items: stretch;
    width: 100%;
    height: 56px;
    background: var(--nb-surface);
    border-bottom: var(--nb-border-lg);
    box-shadow: 0 3px 0 var(--nb-border-color);
    position: relative;
    z-index: var(--nb-z-raised);
    user-select: none;
    -webkit-user-select: none;
    flex-shrink: 0;
  }

  /* Logo */
  .navbar__logo {
    display: flex;
    align-items: center;
    gap: var(--nb-space-2);
    padding: 0 var(--nb-space-5);
    border-right: var(--nb-border-lg);
    font-family: var(--nb-font-display);
    font-weight: var(--nb-fw-bold);
    flex-shrink: 0;
  }

  .navbar__logo-img {
    width: 32px;
    height: 32px;
    object-fit: contain;
  }

  .navbar__logo-text {
    font-size: var(--nb-text-lg);
    font-weight: var(--nb-fw-bold);
    color: var(--nb-text);
    letter-spacing: -0.03em;
  }

  .navbar__logo-ver {
    font-size: var(--nb-text-xs);
    color: var(--nb-text-muted);
    padding: 1px var(--nb-space-1);
    border: 1px solid currentColor;
    line-height: 1.4;
  }

  /* Tab navigation */
  .navbar__nav {
    display: flex;
    align-items: stretch;
    gap: 0;
    flex: 1;
    border-left: var(--nb-border-lg);
  }

  .navbar__tab {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: 0 var(--nb-space-6);
    background: transparent;
    border: none;
    border-right: var(--nb-border-lg);
    font-family: var(--nb-font-display);
    font-size: var(--nb-text-base);
    font-weight: 700;
    color: var(--nb-text-muted);
    letter-spacing: 0.08em;
    text-transform: uppercase;
    cursor: pointer;
    transition: background 150ms, color 150ms;
    position: relative;
  }

  .navbar__tab:hover {
    color: var(--nb-text);
    background: var(--nb-bg);
  }

  .navbar__tab:active {
    box-shadow: inset 4px 4px 0px rgba(0, 0, 0, 0.4);
  }

  .navbar__tab--active {
    color: var(--nb-primary-text);
    background: var(--nb-primary);
  }

  .navbar__tab--active:hover {
    background: var(--nb-primary);
  }

  .navbar__tab--active::after {
    display: none;
  }

  /* Right side */
  .navbar__right {
    display: flex;
    align-items: center;
    gap: var(--nb-space-3);
    padding: 0 var(--nb-space-4);
    border-left: var(--nb-border-lg);
    flex-shrink: 0;
  }

  /* Status pill */
  .navbar__status {
    display: flex;
    align-items: center;
    gap: var(--nb-space-2);
    padding: var(--nb-space-1) var(--nb-space-3);
    border: 2px solid var(--nb-border-color);
    background: var(--nb-surface);
  }

  .status__dot {
    display: inline-block;
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: currentColor;
    animation: dot-blink 2s ease-in-out infinite;
    flex-shrink: 0;
  }

  .status__label {
    font-size: 10px;
    font-weight: var(--nb-fw-bold);
    letter-spacing: 0.08em;
  }

  .status__url {
    font-size: 10px;
    color: var(--nb-text-muted);
    max-width: 140px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    padding-left: var(--nb-space-2);
    border-left: 1px solid var(--nb-border-color);
  }

  /* Status color variants */
  .status--idle  { color: var(--nb-text-muted); background: var(--nb-surface); border-color: var(--nb-border-color); }
  .status--wait  { color: var(--nb-secondary-dark); background: var(--nb-surface); border-color: var(--nb-secondary); }
  .status--ok    { color: var(--nb-success); background: var(--nb-surface); border-color: var(--nb-success); }
  .status--err   { color: var(--nb-danger); background: var(--nb-surface); border-color: var(--nb-danger); }

  /* Status dot animation */
  @keyframes dot-blink {
    0%, 100% { opacity: 1; }
    50%       { opacity: 0.25; }
  }

  /* Icon button (settings) */
  .navbar__icon-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 36px;
    height: 36px;
    background: transparent;
    border: var(--nb-border);
    border-radius: var(--nb-radius);
    box-shadow: var(--nb-shadow-sm);
    color: var(--nb-text);
    cursor: pointer;
    transition: var(--nb-transition);
  }

  .navbar__icon-btn:hover {
    background: var(--nb-secondary);
    color: var(--nb-secondary-text, #0A0A0A);
    transform: translate(-1px, -1px);
    box-shadow: var(--nb-shadow-md);
  }

  .navbar__icon-btn:active {
    transform: translate(1px, 1px);
    box-shadow: none;
    border-color: var(--nb-primary);
  }

  .nb-mono { font-family: var(--nb-font-mono); }

  @media (prefers-reduced-motion: reduce) {
    .navbar__tab, .navbar__icon-btn { transition: none; }
    .status__dot { animation: none; }
    .navbar__icon-btn:hover { transform: none; }
  }

  /* Responsive: collapse URL on small screens */
  @media (max-width: 640px) {
    .status__url    { display: none; }
    .navbar__tab    { padding: 0 var(--nb-space-4); }
    .navbar__logo-ver { display: none; }
  }

  @media (max-width: 375px) {
    .navbar__logo-text { display: none; }
    .navbar__tab { padding: 0 var(--nb-space-3); font-size: var(--nb-text-xs); }
  }
</style>
