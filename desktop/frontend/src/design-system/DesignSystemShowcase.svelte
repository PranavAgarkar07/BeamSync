<!--
  DesignSystemShowcase.svelte
  ────────────────────────────────────────────────────────────────
  Live visual preview of the entire BeamSync Neubrutalism design system.
  Use this during development to QA all components at once.

  Mount in main.js temporarily:
    import DesignSystemShowcase from './design-system/DesignSystemShowcase.svelte';
    const app = new DesignSystemShowcase({ target: document.body });
-->
<script>
  import DeviceCard            from './DeviceCard.svelte';
  import FileDropZone          from './FileDropZone.svelte';
  import TransferProgressBar   from './TransferProgressBar.svelte';
  import ConnectedDevicesPanel from './ConnectedDevicesPanel.svelte';
  import TopNavBar             from './TopNavBar.svelte';
  import TransferComplete      from './TransferComplete.svelte';

  // ── Demo state ──────────────────────────────────────────────
  let activeTab     = 'receive';
  let networkStatus = 'connected';
  let scanning      = false;
  let showComplete  = false;
  let demoFileCount = 12;

  let progress = 72;
  let transferActive = true;

  // Simulate progress ticking
  import { onMount, onDestroy } from 'svelte';
  let interval;
  onMount(() => {
    interval = setInterval(() => {
      if (transferActive) {
        progress = (progress + 0.5) % 101;
      }
    }, 120);
  });
  onDestroy(() => clearInterval(interval));

  const mockDevices = [
    { name: "Pranav's iPhone 15 Pro", ip: '192.168.1.42', os: 'ios',     status: 'connected', speed: '11.2 MB/s', ping: 4  },
    { name: 'MacBook Pro M3',         ip: '192.168.1.10', os: 'macos',   status: 'available', speed: '',          ping: 8  },
    { name: 'Windows Gaming Rig',     ip: '192.168.1.88', os: 'windows', status: 'busy',      speed: '2.1 MB/s',  ping: 22 },
    { name: 'Ubuntu Server',           ip: '192.168.1.5',  os: 'linux',   status: 'offline',   speed: '',          ping: null},
    { name: 'Pixel 8 Pro',            ip: '192.168.1.77', os: 'android', status: 'available', speed: '',          ping: 15 },
  ];

  const mockFiles = [
    { name: 'IMG_vacation_2026.jpg',  sizeBytes: 4_250_000,  progress: 100 },
    { name: 'project_archive.zip',    sizeBytes: 185_000_000, progress: 72  },
    { name: 'video_highlights.mp4',   sizeBytes: 512_000_000, progress: 18  },
    { name: 'design_tokens_v3.pdf',   sizeBytes: 820_000,    progress: null },
  ];

  function handleScan() {
    scanning = true;
    setTimeout(() => scanning = false, 2500);
  }
</script>

<svelte:head>
  <title>BeamSync Design System — Neubrutalism</title>
  <meta name="description" content="Visual showcase of the BeamSync Neubrutalism design system components." />
</svelte:head>

<div class="showcase">
  <!-- ── Top Nav ─────────────────────────────────────────── -->
  <TopNavBar
    {activeTab}
    {networkStatus}
    serverUrl="http://192.168.1.10:8096"
    appVersion="v2.4.0"
    on:tabChange={({ detail }) => activeTab = detail.tab}
    on:settings={() => alert('Settings!')}
    on:reset={() => networkStatus = 'idle'}
  />

  <main class="showcase__content">
    <div class="showcase__intro">
      <h1 class="showcase__heading">BeamSync <span>Design System</span></h1>
      <p class="showcase__sub">Neubrutalism — Professional, Bold, Zero-gradient, Hard-shadows</p>
    </div>

    <!-- ── Token swatches ──────────────────────────────────── -->
    <section class="section" aria-labelledby="section-tokens">
      <h2 id="section-tokens" class="section__title">01 — Color Tokens</h2>
      <div class="swatches">
        <div class="swatch" style="--sw: var(--nb-primary)">
          <div class="swatch__block"></div>
          <div class="swatch__label">Primary<br><code>#1A56FF</code></div>
        </div>
        <div class="swatch" style="--sw: var(--nb-secondary)">
          <div class="swatch__block"></div>
          <div class="swatch__label">Secondary<br><code>#FFD000</code></div>
        </div>
        <div class="swatch" style="--sw: var(--nb-accent)">
          <div class="swatch__block"></div>
          <div class="swatch__label">Accent<br><code>#FF3C5F</code></div>
        </div>
        <div class="swatch" style="--sw: var(--nb-bg)">
          <div class="swatch__block" style="border:2px solid #ccc"></div>
          <div class="swatch__label">Background<br><code>#F5F0E8</code></div>
        </div>
        <div class="swatch" style="--sw: var(--nb-surface)">
          <div class="swatch__block" style="border:2px solid #ccc"></div>
          <div class="swatch__label">Surface<br><code>#FFFFFF</code></div>
        </div>
        <div class="swatch" style="--sw: var(--nb-text)">
          <div class="swatch__block"></div>
          <div class="swatch__label">Text<br><code>#0A0A0A</code></div>
        </div>
        <div class="swatch" style="--sw: var(--nb-danger)">
          <div class="swatch__block"></div>
          <div class="swatch__label">Danger<br><code>#FF3C5F</code></div>
        </div>
        <div class="swatch" style="--sw: var(--nb-success)">
          <div class="swatch__block"></div>
          <div class="swatch__label">Success<br><code>#00C875</code></div>
        </div>
      </div>
    </section>

    <!-- ── Typography ─────────────────────────────────────── -->
    <section class="section" aria-labelledby="section-type">
      <h2 id="section-type" class="section__title">02 — Typography</h2>
      <div class="type-samples">
        <div class="type-sample">
          <p class="type-display">BeamSync — File Transfer</p>
          <code class="type-meta">Syne 700/800 — Display / Nav / Headings (CRED-tier geometric)</code>
        </div>
        <div class="type-sample">
          <p class="type-body">Transfer speed: 11.2 MB/s · ETA: 2m 30s · 12 files from Pranav’s iPhone</p>
          <code class="type-meta">Manrope 400 — Body / Device names / Descriptions (#5 Mockuuups 2026)</code>
        </div>
        <div class="type-sample">
          <p class="type-mono">192.168.001.042:8096</p>
          <code class="type-meta">Space Mono 400 — Data / IPs / Speeds / Code</code>
        </div>
      </div>
    </section>

    <!-- ── Device Card ─────────────────────────────────────── -->
    <section class="section" aria-labelledby="section-device">
      <h2 id="section-device" class="section__title">03 — Device Card</h2>
      <p class="section__desc">Click an available or connected card to select a send target.</p>
      <div class="card-grid">
        {#each mockDevices as d}
          <DeviceCard
            name={d.name} ip={d.ip} os={d.os}
            status={d.status} speed={d.speed}
            on:select={({ detail }) => alert(`Selected: ${detail.name}`)}
          />
        {/each}
      </div>
    </section>

    <!-- ── File Drop Zone ─────────────────────────────────── -->
    <section class="section" aria-labelledby="section-drop">
      <h2 id="section-drop" class="section__title">04 — File Drop Zone</h2>
      <p class="section__desc">Drag files onto the zone or click to open a picker.</p>
      <div class="half">
        <FileDropZone files={mockFiles} multiple on:filesSelected={() => {}} on:dropped={() => {}} />
      </div>
    </section>

    <!-- ── Transfer Progress Bar ──────────────────────────── -->
    <section class="section" aria-labelledby="section-xfer">
      <h2 id="section-xfer" class="section__title">05 — Transfer Progress Bar</h2>
      <p class="section__desc">Live demo — progress ticks automatically.</p>
      <div class="half">
        <TransferProgressBar
          filename="IMG_vacation_2026.jpg"
          percent={Math.round(progress)}
          speed="11.4 MB/s"
          received="{(progress * 1.8).toFixed(1)} MB"
          total="180 MB"
          eta="1m 20s"
          elapsed="12s"
          role="receiver"
          active={transferActive}
          on:cancel={() => { transferActive = false; progress = 0; }}
        />
        <div class="demo-toggle">
          <button class="nb-btn nb-btn--secondary" on:click={() => { transferActive = !transferActive; if(transferActive) progress = 0; }}>
            {transferActive ? 'Pause Demo' : 'Resume Demo'}
          </button>
        </div>
      </div>
    </section>

    <!-- ── Connected Devices Panel ────────────────────────── -->
    <section class="section" aria-labelledby="section-panel">
      <h2 id="section-panel" class="section__title">06 — Connected Devices Panel</h2>
      <div class="half">
        <ConnectedDevicesPanel
          devices={mockDevices}
          {scanning}
          on:select={({ detail }) => alert(`Sending to: ${detail.name}`)}
          on:scan={handleScan}
        />
      </div>
    </section>

    <!-- ── Buttons ────────────────────────────────────────── -->
    <section class="section" aria-labelledby="section-btns">
      <h2 id="section-btns" class="section__title">07 — Buttons</h2>
      <div class="btn-row">
        <button class="nb-btn nb-btn--primary">Primary Action</button>
        <button class="nb-btn nb-btn--secondary">Secondary</button>
        <button class="nb-btn nb-btn--ghost">Ghost / Outline</button>
        <button class="nb-btn nb-btn--danger">Danger</button>
      </div>
    </section>

    <!-- ── Transfer Complete Animation ───────────────────── -->
    <section class="section" aria-labelledby="section-anim">
      <h2 id="section-anim" class="section__title">08 — Transfer Complete Animation</h2>
      <p class="section__desc">Fires when all files are received. Neubrutalism confetti + checkmark draw + DONE. stamp.</p>
      <div class="btn-row">
        <button
          class="nb-btn nb-btn--primary"
          on:click={() => { showComplete = true; }}
        >
          ▶ Play Animation (12 files)
        </button>
        <button class="nb-btn nb-btn--ghost" on:click={() => { demoFileCount = Math.floor(Math.random() * 50) + 1; showComplete = true; }}>
          Random file count
        </button>
      </div>
    </section>

    <!-- ── Shadow scale ───────────────────────────────────── -->
    <section class="section" aria-labelledby="section-shadows">
      <h2 id="section-shadows" class="section__title">08 — Shadow Scale</h2>
      <div class="shadow-row">
        <div class="shadow-box" style="box-shadow: var(--nb-shadow-sm)">sm<br><code>2px 2px</code></div>
        <div class="shadow-box" style="box-shadow: var(--nb-shadow-md)">md<br><code>4px 4px</code></div>
        <div class="shadow-box" style="box-shadow: var(--nb-shadow-lg)">lg<br><code>6px 6px</code></div>
        <div class="shadow-box" style="box-shadow: var(--nb-shadow-xl)">xl<br><code>8px 8px</code></div>
        <div class="shadow-box" style="box-shadow: var(--nb-shadow-primary)">primary</div>
        <div class="shadow-box" style="box-shadow: var(--nb-shadow-secondary)">secondary</div>
      </div>
    </section>
  </main>
</div>

<!-- Transfer complete animation overlay -->
<TransferComplete
  fileCount={demoFileCount}
  show={showComplete}
  on:dismiss={() => showComplete = false}
/>

<style>
  /* ── Showcase wrapper ───────────────────────────────────────── */
  :global(*, *::before, *::after) { box-sizing: border-box; }
  :global(body) {
    margin: 0;
    background: var(--nb-bg);
    font-family: var(--nb-font-body, var(--nb-font-display));
    color: var(--nb-text);
  }

  .showcase {
    display: flex;
    flex-direction: column;
    min-height: 100vh;
  }

  .showcase__content {
    flex: 1;
    max-width: 1080px;
    width: 100%;
    margin: 0 auto;
    padding: var(--nb-space-8) var(--nb-space-6) var(--nb-space-16);
    display: flex;
    flex-direction: column;
    gap: var(--nb-space-12);
  }

  /* Intro */
  .showcase__intro {
    padding: var(--nb-space-8) var(--nb-space-8);
    background: var(--nb-secondary);
    border: var(--nb-border-lg);
    box-shadow: var(--nb-shadow-xl);
  }

  .showcase__heading {
    margin: 0 0 var(--nb-space-2);
    font-size: var(--nb-text-4xl);
    font-weight: var(--nb-fw-bold);
    color: var(--nb-text);
    letter-spacing: -0.03em;
    line-height: 1;
  }

  .showcase__heading span {
    color: var(--nb-primary);
  }

  .showcase__sub {
    margin: 0;
    font-size: var(--nb-text-sm);
    font-family: var(--nb-font-mono);
    color: var(--nb-text-muted);
    font-weight: var(--nb-fw-regular);
  }

  /* Section */
  .section {
    display: flex;
    flex-direction: column;
    gap: var(--nb-space-5);
  }

  .section__title {
    margin: 0;
    font-size: var(--nb-text-2xl);
    font-weight: var(--nb-fw-bold);
    color: var(--nb-text);
    letter-spacing: -0.02em;
    padding-bottom: var(--nb-space-4);
    border-bottom: var(--nb-border-lg);
  }

  .section__desc {
    margin: 0;
    font-size: var(--nb-text-sm);
    color: var(--nb-text-muted);
  }

  /* Color swatches */
  .swatches {
    display: flex;
    flex-wrap: wrap;
    gap: var(--nb-space-4);
  }

  .swatch { display: flex; flex-direction: column; gap: var(--nb-space-2); }

  .swatch__block {
    width: 80px;
    height: 64px;
    background: var(--sw);
    border: var(--nb-border);
    box-shadow: var(--nb-shadow-sm);
  }

  .swatch__label {
    font-size: var(--nb-text-xs);
    font-family: var(--nb-font-mono);
    color: var(--nb-text-muted);
    line-height: 1.5;
  }

  .swatch__label code { font-size: 10px; }

  /* Typography samples */
  .type-samples { display: flex; flex-direction: column; gap: var(--nb-space-5); }

  .type-sample {
    padding: var(--nb-space-5);
    background: var(--nb-surface);
    border: var(--nb-border);
    box-shadow: var(--nb-shadow-sm);
    display: flex;
    flex-direction: column;
    gap: var(--nb-space-2);
  }

  .type-display {
    margin: 0;
    font-family: var(--nb-font-display);
    font-weight: 700;
    font-size: var(--nb-text-3xl);
    color: var(--nb-text);
    letter-spacing: -0.03em;
    line-height: 1.1;
  }

  .type-body {
    margin: 0;
    font-family: var(--nb-font-body, var(--nb-font-display));
    font-weight: var(--nb-fw-regular);
    font-size: var(--nb-text-base);
    color: var(--nb-text);
  }

  .type-mono {
    margin: 0;
    font-family: var(--nb-font-mono);
    font-weight: var(--nb-fw-regular);
    font-size: var(--nb-text-lg);
    color: var(--nb-primary);
    letter-spacing: 0.04em;
  }

  .type-meta {
    font-family: var(--nb-font-mono);
    font-size: var(--nb-text-xs);
    color: var(--nb-text-muted);
  }

  /* Card grid */
  .card-grid {
    display: flex;
    flex-direction: column;
    gap: var(--nb-space-3);
  }

  /* Half-width sections */
  .half {
    max-width: 600px;
    display: flex;
    flex-direction: column;
    gap: var(--nb-space-4);
  }

  /* Button row */
  .btn-row {
    display: flex;
    flex-wrap: wrap;
    gap: var(--nb-space-4);
    align-items: center;
  }

  /* nb-btn (must repeat here since no global import of tokens.css classes) */
  :global(.nb-btn) {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: var(--nb-space-2);
    padding: var(--nb-space-3) var(--nb-space-5);
    font-family: var(--nb-font-display);
    font-weight: var(--nb-fw-bold);
    font-size: var(--nb-text-sm);
    letter-spacing: 0.04em;
    text-transform: uppercase;
    border: 3px solid var(--nb-border-color);
    border-radius: 0px;
    box-shadow: var(--nb-shadow-md);
    cursor: pointer;
    transition: transform 120ms ease, box-shadow 120ms ease;
    user-select: none;
    text-decoration: none;
    line-height: 1;
    white-space: nowrap;
  }
  :global(.nb-btn:hover) { transform: translate(-2px, -2px); box-shadow: var(--nb-shadow-lg); }
  :global(.nb-btn:active) { transform: translate(2px, 2px); box-shadow: var(--nb-shadow-sm); }
  :global(.nb-btn--primary)   { background: var(--nb-primary); color: #fff; }
  :global(.nb-btn--secondary) { background: var(--nb-secondary); color: var(--nb-secondary-text, #0A0A0A); }
  :global(.nb-btn--ghost)     { background: transparent; color: var(--nb-text); }
  :global(.nb-btn--ghost:hover) { background: var(--nb-secondary); color: var(--nb-secondary-text, #0A0A0A); }
  :global(.nb-btn--danger)    { background: var(--nb-danger); color: #fff; }

  .demo-toggle { margin-top: var(--nb-space-2); }

  /* Shadow scale */
  .shadow-row {
    display: flex;
    flex-wrap: wrap;
    gap: var(--nb-space-8);
    align-items: flex-end;
  }

  .shadow-box {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    width: 100px;
    height: 80px;
    background: var(--nb-surface);
    border: var(--nb-border);
    font-family: var(--nb-font-mono);
    font-size: var(--nb-text-xs);
    text-align: center;
    gap: var(--nb-space-1);
  }

  .shadow-box code { font-size: 9px; color: var(--nb-text-muted); }

  @media (max-width: 640px) {
    .showcase__content { padding: var(--nb-space-6) var(--nb-space-4); }
    .showcase__heading { font-size: var(--nb-text-3xl); }
    .half { max-width: 100%; }
  }
</style>
