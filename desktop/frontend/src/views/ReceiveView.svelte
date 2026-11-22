<script>
  /**
   * ReceiveView — RECEIVE mode of BeamSync.
   *
   * Extracted verbatim from App.svelte (Phase 5, Plan 05-04, SVE-02).
   * Presentation only: every Wails call stays in the App shell. User actions
   * are dispatched upward:
   *   - openFile        { name }  → shell calls the OpenFile binding
   *   - disconnectReset           → shell calls handleDisconnectReset
   *   - copyUrl                   → shell copies to clipboard and toasts
   *
   * No visual change: markup, class names, copy strings and tokens are
   * identical to the block this component replaced.
   */
  import { createEventDispatcher } from "svelte";
  import { fly } from "svelte/transition";
  import { TransferStatsDashboard, ActivityPanel } from "../design-system/index.js";

  export let connectionState = "IDLE";
  export let serverUrl = "";
  export let qrImage = "";
  export let displayUrl = "";
  export let receivedFiles = [];
  export let transferStats = {
    startedAt: "",
    filesReceived: 0,
    bytesReceived: 0,
    filesSent: 0,
    bytesSent: 0,
    activeUploads: 0,
    activeDownloads: 0,
    lastFilename: "",
    lastDirection: "",
  };
  export let transferStatsNow = Date.now();
  export let transferSpeeds = { receive: "Idle", send: "Idle" };
  export let activeSpeedDirection = "";
  export let transferHistory = [];
  export let sessionLog = [];
  export let formatSize;
  export let formatDuration;
  export let formatTransferTime;
  export let fileIcon;

  const dispatch = createEventDispatcher();

  // Same reactive derivation the shell performed before extraction.
  $: sortedFiles = [...receivedFiles];
</script>

<div class="mode-wrapper" in:fly={{ y: 15, duration: 250 }}>
  <div
    class="receive-layout"
    class:receive-layout--split={connectionState === "CONNECTED"}
  >
    <div
      class="receive-standby"
      class:receive-standby--compact={connectionState === "CONNECTED"}
    >
      <div class="nb-card home-card">
        <div class="home-card__header">
          <div
            class="status-indicator"
            class:pulse={connectionState === "WAITING" || connectionState === "CONNECTED"}
          ></div>
          <h1 class="standby-title">
            {#if connectionState === "WAITING"}
              Connect via {serverUrl
                .replace(/^https?:\/\//, "")
                .split(":")[0] || "Wi-Fi"}
            {:else if connectionState === "CONNECTED"}
              Device Connected
            {:else if connectionState === "DISCONNECTED"}
              Connection Lost
            {:else}
              Ready to Connect
            {/if}
          </h1>
        </div>

        <div class="home-card__body">
          {#if qrImage}
            <div class="qr-wrapper" class:qr-wrapper--compact={connectionState === "CONNECTED"}>
              <img
                src={qrImage}
                alt="QR Code"
                class="qr-code"
                class:qr-code--compact={connectionState === "CONNECTED"}
                draggable="false"
              />
            </div>
          {:else}
            <div class="qr-wrapper qr-loading">GENERATING_LINK...</div>
          {/if}

          <div class="instructions-list">
            <div class="instr-step">
              <span class="step-num">1</span> Connect to same Wi-Fi
            </div>
            <div class="instr-step">
              <span class="step-num">2</span> Scan QR code
            </div>
            <div class="instr-step">
              <span class="step-num">3</span> Select files
            </div>
          </div>
        </div>

        <div class="home-card__footer">
          {#if displayUrl}
            <div class="url-group">
              <span class="url-text">{displayUrl}</span>
              <button
                class="nb-btn nb-btn--primary"
                on:click={() => dispatch("copyUrl")}>COPY</button
              >
            </div>
          {/if}
        </div>
      </div>

      {#if connectionState === "DISCONNECTED"}
        <button
          class="nb-btn nb-btn--danger reconnect-btn"
          on:click={() => dispatch("disconnectReset")}>RECONNECT</button
        >
      {/if}
    </div>

    {#if connectionState === "CONNECTED"}
      <div class="receive-active" in:fly={{ x: 20, duration: 300, delay: 100 }}>
        <div class="ready-banner pulse-bg">
          <div class="radar-ping"></div>
          <div class="ready-content">
            <span class="status-badge">READY</span>
            <span class="status-text">WAITING FOR FILES...</span>
          </div>
        </div>

        <TransferStatsDashboard
          stats={transferStats}
          now={transferStatsNow}
          direction="receive"
          currentSpeed={transferSpeeds.receive}
          speedActive={activeSpeedDirection === "receive"}
        />

        <div class="files-panel">
          <div class="files-header">
            <h3>RECEIVED FILES ({receivedFiles.length})</h3>
          </div>
          <div class="files-list" class:empty={receivedFiles.length === 0}>
            {#if receivedFiles.length === 0}
              <div class="empty-state">
                <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="square"><rect x="3" y="3" width="18" height="18" rx="0" ry="0"/><line x1="9" y1="3" x2="9" y2="21"/><path d="M13 8h4"/><path d="M13 12h4"/></svg>
                <p>INBOX EMPTY<br><small>Incoming data will appear here</small></p>
              </div>
            {/if}
            {#each sortedFiles as file}
              <button
                class="file-item"
                on:click={() => dispatch("openFile", { name: file.name })}
              >
                <span class="file-icon">{fileIcon(file.name)}</span>
                <span class="file-name">{file.name}</span>
                <span class="file-size">{formatSize(file.sizeBytes)}</span>
                <span class="file-time">{file.modTime}</span>
              </button>
            {/each}
          </div>
        </div>

        <ActivityPanel {transferHistory} {sessionLog} {formatSize} {formatDuration} {formatTransferTime} />
      </div>
    {/if}
  </div>
</div>

<style>
  .receive-layout {
    display: flex;
    flex-direction: row;
    align-items: flex-start;
    gap: var(--nb-space-6);
    width: 100%;
    max-width: 500px;
    transition: max-width 400ms cubic-bezier(.2, 0, 0, 1);
    will-change: max-width;
  }
  .receive-layout--split {
    max-width: 1000px;
  }

  .receive-standby {
    display: flex;
    flex-direction: column;
    gap: var(--nb-space-6);
    align-items: center;
    width: 100%;
    max-width: 500px;
    flex-shrink: 0;
    transition: max-width 400ms cubic-bezier(.2, 0, 0, 1);
    will-change: max-width;
    margin-top: var(--nb-space-4);
  }
  .receive-standby--compact {
    max-width: 340px;
  }
  .receive-standby--compact .home-card__body {
    flex-direction: row;
    flex-wrap: wrap;
    justify-content: center;
    padding: var(--nb-space-4);
    gap: var(--nb-space-4);
  }
  .receive-standby--compact .instructions-list {
    flex: 1;
    min-width: 200px;
    max-width: 320px;
  }

  .receive-active {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: var(--nb-space-6);
  }

  .home-card {
    width: 100%;
    padding: 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }

  .home-card__header {
    background: var(--nb-primary);
    color: var(--nb-primary-text, #ffffff);
    padding: var(--nb-space-4) var(--nb-space-5);
    border-bottom: var(--nb-border-lg);
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--nb-space-4);
  }

  .status-indicator {
    width: 16px;
    height: 16px;
    background: #00e676; /* Neon green ping */
    border: 2px solid var(--nb-border-color);
    border-radius: 50%;
  }

  .pulse {
    animation: pulse 1.5s infinite alternate;
  }

  @keyframes pulse {
    0% {
      transform: scale(0.85);
      box-shadow: 0 0 0 0 rgba(0, 230, 118, 0.4);
    }
    100% {
      transform: scale(1.15);
      box-shadow: 0 0 0 6px rgba(0, 230, 118, 0);
    }
  }

  .standby-title {
    font-size: var(--nb-text-xl);
    font-family: var(--nb-font-mono);
    font-weight: 800;
    color: var(--nb-primary-text, #ffffff);
    margin: 0;
    line-height: 1.1;
    letter-spacing: -0.04em;
    text-transform: uppercase;
  }

  .home-card__body {
    padding: var(--nb-space-6) var(--nb-space-4);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--nb-space-6);
    background: var(--nb-surface);
  }

  .qr-wrapper {
    background: #ffffff;
    padding: var(--nb-space-4);
    border: var(--nb-border-lg);
    box-shadow: 8px 8px 0px var(--nb-border-color);
    transition: transform 0.2s, box-shadow 0.2s;
  }
  .qr-wrapper:hover {
    transform: translate(-4px, -4px);
    box-shadow: 12px 12px 0px var(--nb-border-color);
  }

  .qr-code {
    width: 220px;
    height: 220px;
    display: block;
  }
  .qr-code--compact {
    width: 140px;
    height: 140px;
  }
  .qr-wrapper--compact {
    padding: var(--nb-space-3);
    box-shadow: 4px 4px 0px var(--nb-border-color);
  }
  .qr-wrapper--compact:hover {
    transform: translate(-2px,-2px);
    box-shadow: 6px 6px 0px var(--nb-border-color);
  }

  .qr-loading {
    width: 220px;
    height: 220px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-family: var(--nb-font-mono);
    font-weight: 800;
    color: #0a0a0a;
  }

  .instructions-list {
    display: flex;
    flex-direction: column;
    gap: var(--nb-space-3);
    width: 100%;
    max-width: 380px;
  }

  .instr-step {
    display: flex;
    align-items: center;
    gap: var(--nb-space-4);
    font-family: var(--nb-font-body);
    font-size: var(--nb-text-base);
    font-weight: var(--nb-fw-bold);
    letter-spacing: -0.01em;
    padding: var(--nb-space-3) var(--nb-space-4);
    background: var(--nb-bg);
    border: var(--nb-border-lg);
    box-shadow: 4px 4px 0px var(--nb-border-color);
    color: var(--nb-text);
  }

  .step-num {
    background: var(--nb-secondary);
    color: #0a0a0a;
    font-family: var(--nb-font-mono);
    font-weight: 800;
    font-size: var(--nb-text-lg);
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    border: var(--nb-border-lg);
    flex-shrink: 0;
  }

  .home-card__footer {
    padding: var(--nb-space-4);
    background: var(--nb-bg);
    border-top: var(--nb-border-lg);
    display: flex;
    flex-direction: column;
    gap: var(--nb-space-4);
  }

  .url-group {
    display: flex;
    align-items: stretch;
    border: var(--nb-border-lg);
    background: var(--nb-surface);
    overflow: hidden;
    box-shadow: 4px 4px 0px var(--nb-border-color);
  }

  .url-text {
    flex: 1;
    font-family: var(--nb-font-mono);
    font-size: var(--nb-text-base);
    font-weight: 800;
    color: var(--nb-text);
    padding: 0 var(--nb-space-4);
    display: flex;
    align-items: center;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .url-group .nb-btn {
    border: none;
    border-left: var(--nb-border-lg);
    border-radius: 0;
    margin: 0;
    box-shadow: none;
    font-size: var(--nb-text-sm);
    padding: 0 var(--nb-space-6);
  }
  .url-group .nb-btn:hover {
    transform: none;
    background: var(--nb-primary);
    color: var(--nb-primary-text, #ffffff);
  }

  .reconnect-btn {
    margin-top: var(--nb-space-4);
  }

  .ready-banner {
    padding: var(--nb-space-4);
    background: var(--nb-surface);
    border: var(--nb-border-lg);
    box-shadow: var(--nb-shadow-md);
    margin-bottom: var(--nb-space-5);
    display: flex;
    align-items: center;
    gap: var(--nb-space-4);
    position: relative;
    overflow: hidden;
  }

  .pulse-bg {
    background: repeating-linear-gradient(45deg, var(--nb-primary) 0, var(--nb-primary) 2px, transparent 2px, transparent 10px);
    background-color: var(--nb-bg);
  }

  .ready-content {
    display: flex;
    align-items: center;
    gap: var(--nb-space-3);
    background: var(--nb-surface);
    padding: var(--nb-space-2) var(--nb-space-4);
    border: var(--nb-border-lg);
    box-shadow: var(--nb-shadow-sm);
    z-index: 1;
  }

  .status-badge {
    background: var(--nb-secondary);
    color: var(--nb-secondary-text);
    padding: 4px 8px;
    font-family: var(--nb-font-display);
    font-weight: 800;
    font-size: var(--nb-text-sm);
    border: var(--nb-border-md);
  }

  .status-text {
    font-family: var(--nb-font-mono);
    font-weight: var(--nb-fw-bold);
    color: var(--nb-text);
  }

  .radar-ping {
    position: absolute;
    right: 30px;
    width: 20px;
    height: 20px;
    background: var(--nb-secondary);
    border-radius: 50%;
    animation: ping 2s cubic-bezier(0, 0, 0.2, 1) infinite;
  }

  /* Local copy of the shell's `ping` keyframes: Svelte scopes @keyframes per
     component, so the shell's copy (still used by .radar-ping-small in the
     SEND view) cannot be referenced from here. Identical definition. */
  @keyframes ping {
    75%, 100% {
      transform: scale(3);
      opacity: 0;
    }
  }

  .files-panel {
    background: var(--nb-surface);
    border: var(--nb-border-lg);
    box-shadow: var(--nb-shadow-md);
    display: flex;
    flex-direction: column;
  }

  .files-header {
    background: var(--nb-bg);
    padding: var(--nb-space-3) var(--nb-space-4);
    border-bottom: var(--nb-border-lg);
  }

  .files-header h3 {
    font-size: var(--nb-text-sm);
    letter-spacing: 0.05em;
  }

  .files-list {
    max-height: 300px;
    overflow-y: auto;
  }

  .files-list.empty {
    padding: var(--nb-space-6);
  }

  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--nb-space-3);
    color: var(--nb-text-muted);
    text-align: center;
    padding: var(--nb-space-4);
  }

  .empty-state svg {
    color: var(--nb-primary);
    margin-bottom: var(--nb-space-2);
  }

  .empty-state p {
    font-family: var(--nb-font-display);
    font-weight: 800;
    font-size: var(--nb-text-lg);
    line-height: 1.2;
    margin: 0;
    color: var(--nb-text);
  }

  .empty-state small {
    font-family: var(--nb-font-mono);
    font-weight: 400;
    font-size: var(--nb-text-sm);
    color: var(--nb-text-muted);
  }

  .file-item {
    display: flex;
    align-items: center;
    gap: var(--nb-space-4);
    width: 100%;
    padding: var(--nb-space-3) var(--nb-space-4);
    border-bottom: 1px solid var(--nb-border-color);
    text-align: left;
    color: var(--nb-text);
  }

  .file-item:hover {
    background: var(--nb-bg);
  }

  .file-name {
    flex: 1;
    font-weight: var(--nb-fw-bold);
  }
  .file-size,
  .file-time {
    font-family: var(--nb-font-mono);
    font-size: var(--nb-text-xs);
    color: var(--nb-text-muted);
  }
</style>
