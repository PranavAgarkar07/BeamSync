import sys

with open("desktop/frontend/src/App.svelte", "r") as f:
    content = f.read()

parts = content.split("</script>")
if len(parts) >= 2:
    header = parts[0] + "</script>\n\n"
else:
    print("Error splitting")
    sys.exit(1)

new_template = """<svelte:window on:mousemove={handleMouseMove} />

<!-- Drop zone layer (always active behind nav for drag-drop SEND initiation) -->
<div class="app-dropzone" on:dragover={handleDragOver} on:drop={handleDrop} on:dragleave={handleDragLeave}>
  {#if isDragOver}
    <div class="drop-overlay">
      <div class="drop-message">[ DROP → INITIATE_SEND ]</div>
    </div>
  {/if}

  <!-- Toast rack -->
  <div class="toast-rack" aria-live="polite">
    {#each toasts as t (t.id)}
      <div class="toast toast--{t.type}">{t.msg}</div>
    {/each}
  </div>

  <div id="app" class="nb-theme">
    <TopNavBar
      activeTab={mode.toLowerCase()}
      networkStatus={connectionState.toLowerCase()}
      serverUrl={displayUrl}
      appVersion="v2.2"
      on:tabChange={({ detail }) => switchMode(detail.tab.toUpperCase())}
      on:settings={toggleSound}
      on:reset={handleDisconnectReset}
    />

    <main class="main-content">
      {#if mode === "RECEIVE"}
        {#if connectionState !== "CONNECTED"}
          <div class="receive-standby">
            <h1 class="standby-title">
              {#if connectionState === "WAITING"}
                // WAITING_FOR_UPLINK...
              {:else if connectionState === "DISCONNECTED"}
                // SIGNAL_LOST
              {:else}
                // STANDBY
              {/if}
            </h1>
            
            {#if qrImage}
              <div class="qr-container">
                <img src={qrImage} alt="QR Code" class="qr-code" draggable="false" />
              </div>
            {:else}
              <div class="qr-loading">GENERATING_LINK...</div>
            {/if}
            
            <p class="instr">1. Connect to same Wi-Fi &nbsp;2. Scan QR code &nbsp;3. Select files</p>
            
            {#if displayUrl}
              <div class="url-box">
                <span class="url-text">{displayUrl}</span>
                <button class="nb-btn nb-btn--ghost" on:click={() => { navigator.clipboard.writeText(displayUrl); toast("Copied!", "success"); }}>COPY</button>
              </div>
            {/if}
            
            <div class="save-path-row">
              <span class="save-path-lbl">Save to:</span>
              <span class="save-path-val">{savePath || 'Default'}</span>
              <button class="nb-btn nb-btn--ghost" on:click={changeSavePath}>CHANGE</button>
            </div>
            
            {#if connectionState === "DISCONNECTED"}
              <button class="nb-btn nb-btn--danger" on:click={handleDisconnectReset}>RECONNECT</button>
            {/if}
          </div>
        {:else}
          <!-- Connected Receive Mode -->
          <div class="receive-active">
            <h2 class="active-title">// LINK_ESTABLISHED</h2>
            
            {#if progress.active}
              <TransferProgressBar
                filename={progress.filename}
                percent={progress.percent}
                speed={progress.speed}
                received={progress.received}
                total={progress.total}
                eta={progress.timeRemaining}
                elapsed={progress.totalTime}
                role="receiver"
                active={true}
                on:cancel={() => resetAll()}
              />
            {:else}
              <div class="ready-banner">
                ▶ READY — Waiting for files…
              </div>
            {/if}
            
            <div class="files-panel">
              <div class="files-header">
                <h3>RECEIVED FILES ({receivedFiles.length})</h3>
              </div>
              <div class="files-list">
                {#each sortedFiles as file}
                  <button class="file-item" on:click={() => openFile(file.name)}>
                    <span class="file-icon">{fileIcon(file.name)}</span>
                    <span class="file-name">{file.name}</span>
                    <span class="file-size">{formatSize(file.sizeBytes)}</span>
                    <span class="file-time">{file.modTime}</span>
                  </button>
                {/each}
              </div>
            </div>
          </div>
        {/if}

      {:else if mode === "SEND"}
        <div class="send-layout">
          <FileDropZone on:selectFiles={startSend} />
          
          {#if showSenderDialog}
            <div class="sender-dialog">
              <h3>// PAYLOAD_READY</h3>
              <p>Scan on receiving device to download</p>
              {#if qrImage}
                <img src={qrImage} alt="Sender QR" class="sender-qr" />
              {/if}
              <div class="url-box">
                <input class="url-input nb-mono" readonly value={senderUrl} />
                <button class="nb-btn nb-btn--ghost" on:click={() => { navigator.clipboard.writeText(senderUrl); toast("Copied!", "success"); }}>COPY</button>
              </div>
              <button class="nb-btn nb-btn--secondary" on:click={() => (showSenderDialog = false)}>CLOSE</button>
            </div>
          {/if}
        </div>

      {:else if mode === "ABOUT"}
        <div class="about-layout">
          <img src={logoImg} class="about-logo" alt="BeamSync Logo" />
          <h1>BEAMSYNC v2.2</h1>
          <p>Fast, token-secured file transfers over your local network. No cloud. No accounts.</p>
          <div class="about-tags">
            <span class="nb-badge nb-badge--info">LAN ONLY</span>
            <span class="nb-badge nb-badge--success">ZERO CLOUD</span>
          </div>
          
          <hr />
          <h3>DEVELOPER</h3>
          <p>Pranav Agarkar</p>
          <div class="about-links">
            <button class="nb-btn nb-btn--ghost" on:click={() => openLink("https://github.com/PranavAgarkar07")}>GitHub</button>
            <button class="nb-btn nb-btn--ghost" on:click={() => openLink("https://pranavagarkar07.github.io/portfolio-svelte/")}>Portfolio</button>
          </div>
        </div>
      {/if}
    </main>
  </div>

  {#if showTickAnim}
    <TransferComplete />
  {/if}
</div>

<style>
  /* 
   * Local App.svelte styles for new layout composition 
   * All design systems tokens are global and handled by app.css/tokens.css
   */
  .app-dropzone {
    width: 100vw;
    height: 100vh;
    position: relative;
  }
  
  .drop-overlay {
    position: fixed;
    inset: 0;
    z-index: 1000;
    background: rgba(0, 0, 0, 0.85);
    display: flex;
    align-items: center;
    justify-content: center;
    border: 4px dashed var(--nb-primary);
  }

  .drop-message {
    background: var(--nb-surface);
    border: var(--nb-border-lg);
    font-size: var(--nb-text-2xl);
    font-weight: var(--nb-fw-bold);
    padding: var(--nb-space-4) var(--nb-space-6);
    box-shadow: var(--nb-shadow-lg);
  }

  /* Main Nav/Content Setup */
  #app {
    display: flex;
    flex-direction: column;
    height: 100%;
  }

  .main-content {
    flex: 1;
    overflow-y: auto;
    padding: var(--nb-space-6) var(--nb-space-8);
    display: flex;
    flex-direction: column;
    align-items: center;
  }

  /* Receive Standby */
  .receive-standby, .receive-active, .about-layout, .send-layout {
    width: 100%;
    max-width: 800px;
    display: flex;
    flex-direction: column;
    gap: var(--nb-space-6);
  }

  .receive-standby {
    align-items: center;
    text-align: center;
    margin-top: var(--nb-space-6);
  }

  .standby-title {
    font-size: var(--nb-text-2xl);
  }

  .qr-container {
    padding: var(--nb-space-3);
    background: #fff;
    border: var(--nb-border-lg);
    box-shadow: var(--nb-shadow-md);
  }

  .qr-code {
    width: 200px;
    height: 200px;
    display: block;
  }

  .url-box {
    display: inline-flex;
    align-items: center;
    gap: var(--nb-space-3);
    background: var(--nb-surface);
    border: var(--nb-border-lg);
    padding: var(--nb-space-2) var(--nb-space-3);
    box-shadow: var(--nb-shadow-sm);
  }
  
  .url-text, .url-input {
    font-family: var(--nb-font-mono);
    font-size: var(--nb-text-base);
    min-width: 250px;
  }

  .url-input {
    background: transparent;
    border: none;
    outline: none;
    color: var(--nb-text);
  }

  .save-path-row {
    font-size: var(--nb-text-sm);
    display: flex;
    align-items: center;
    gap: var(--nb-space-3);
  }

  /* Receive Active Components */
  .active-title {
    font-size: var(--nb-text-xl);
    border-bottom: var(--nb-border-lg);
    padding-bottom: var(--nb-space-2);
  }

  .ready-banner {
    padding: var(--nb-space-4);
    background: var(--nb-surface);
    border: var(--nb-border-lg);
    box-shadow: var(--nb-shadow-md);
    font-family: var(--nb-font-mono);
    font-weight: var(--nb-fw-bold);
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

  .file-item {
    display: flex;
    align-items: center;
    gap: var(--nb-space-4);
    width: 100%;
    padding: var(--nb-space-3) var(--nb-space-4);
    border-bottom: 1px solid var(--nb-border-color);
    text-align: left;
  }
  
  .file-item:hover {
    background: var(--nb-bg);
  }

  .file-name { flex: 1; font-weight: var(--nb-fw-bold); }
  .file-size, .file-time { font-family: var(--nb-font-mono); font-size: var(--nb-text-xs); color: var(--nb-text-muted); }

  /* Sender Dialog */
  .sender-dialog {
    padding: var(--nb-space-5);
    background: var(--nb-surface);
    border: var(--nb-border-lg);
    box-shadow: var(--nb-shadow-lg);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--nb-space-4);
    margin-top: var(--nb-space-6);
  }

  .sender-qr {
    width: 180px;
    height: 180px;
    background: #fff;
    padding: var(--nb-space-2);
    border: var(--nb-border-lg);
  }

  /* About Layout */
  .about-layout {
    align-items: center;
    text-align: center;
    margin-top: var(--nb-space-8);
  }

  .about-logo {
    width: 80px;
    height: 80px;
    margin-bottom: var(--nb-space-4);
  }

  .about-tags {
    display: flex;
    gap: var(--nb-space-2);
  }

  .about-links {
    display: flex;
    gap: var(--nb-space-3);
  }

  hr {
    width: 100%;
    border: none;
    border-top: var(--nb-border-lg);
    margin: var(--nb-space-4) 0;
  }
</style>
"""

with open("desktop/frontend/src/App.svelte", "w") as f:
    f.write(header + new_template)

