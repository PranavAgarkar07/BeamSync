<script>
  import {
    StartReceiverDefault,
    StartSender,
    PlaySound,
    OpenFile,
  } from "../wailsjs/go/main/App.js";
  import { EventsOn, BrowserOpenURL } from "../wailsjs/runtime/runtime.js";
  import QRCode from "qrcode";
  import { onMount } from "svelte";
  import Typewriter from "./Typewriter.svelte";

  let appState = "HANDSHAKE"; // HANDSHAKE | COMMAND_DECK
  let status = ">> WAITING_FOR_UPLINK...";
  let transitionStage = 0; // 0: Idle, 1: Access Granted, 2: Collapse, 3: Expand/Dashboard
  let qrImage = "";
  let link = "";
  let receivedFiles = [];
  let progress = { filename: "", percent: 0, speed: "0 MB/s" };
  let lastProgressTime = 0;
  let lastLoaded = 0;

  // Sender Logic
  let senderUrl = "";
  let showUrlDialog = false;

  let isDragOver = false;

  let mouseX = 0;
  let mouseY = 0;
  let cursorEl;
  let rafId;

  onMount(async () => {
    await initHandshake();

    // Listen for sender_started event from backend
    EventsOn("sender_started", (url) => {
      senderUrl = url;
      showUrlDialog = true;
      // Also generate QR for mobile scanning convenience
      generateQR(url);
    });
  });

  async function initHandshake() {
    playSound("startup");
    try {
      link = await StartReceiverDefault();
    } catch (e) {
      console.error(e);
      link = "http://localhost:8080"; // Fallback
    }
    generateQR(link);
    status = ">> WAITING_FOR_UPLINK...";
  }

  function simulateConnection() {
    if (transitionStage > 0) return; // Prevent double trigger

    // Step 1: ACCESS GRANTED (Green QR)
    playSound("connect"); // Was success
    status = "[ ACCESS_GRANTED ]";
    transitionStage = 1;

    // Step 2: COLLAPSE (0.5s later)
    setTimeout(() => {
      transitionStage = 2;
      playSound("blip"); // Mechanical sound for collapse
    }, 800);

    // Step 3: EXPAND/DASHBOARD (0.5s later)
    setTimeout(() => {
      appState = "COMMAND_DECK";
      transitionStage = 3;
      status = ">> LINK_ESTABLISHED: EXTERNAL_UNIT_01";
      playSound("startup"); // Power up sound
    }, 1300);
  }

  async function startSend() {
    status = ">> INITIATING_UPLOAD_PROTOCOL...";
    let result = await StartSender();
    if (result === "Cancelled") {
      status = ">> UPLOAD_ABORTED";
      return;
    }
    // Update State
    link = result;
    senderUrl = result;
    showUrlDialog = true;
    generateQR(result);

    status = ">> PAYLOAD_READY_FOR_TRANSMISSION";
  }

  function generateQR(text) {
    console.log("📷 Generating QR for:", text);
    QRCode.toDataURL(
      text,
      {
        width: 256,
        margin: 2,
        color: {
          dark: "#00FF41",
          light: "#00000000",
        },
      },
      (err, url) => {
        if (err) {
          console.error("❌ QR Gen Error:", err);
          status = "QR GEN ERROR: " + err.message; // Show on UI
          return;
        }
        console.log("✅ QR Code generated successfully. Length:", url.length);
        qrImage = url;
      },
    );
  }

  // File Drag & Drop Handlers - GLOBAL
  function handleDragOver(e) {
    e.preventDefault();
    isDragOver = true;
  }

  function handleDragLeave(e) {
    e.preventDefault();
    // Only disable if we actually left the window (or dropped)
    if (e.clientX === 0 && e.clientY === 0) {
      isDragOver = false;
    }
  }

  function handleDrop(e) {
    e.preventDefault();
    isDragOver = false;
    playSound("success");

    // Simulate File Received
    status = ">> UPLINK_INITIATED: RECEIVING_PAYLOAD...";
    const files = e.dataTransfer?.files;
    if (files && files.length > 0) {
      // Mock progress
      progress = {
        filename: files[0].name,
        speed: "125 MB/S",
        percent: 0,
        received: "0.00 MB",
      };
      // If still in handshake, force transition
      if (appState === "HANDSHAKE") simulateConnection();
    } else {
      startSend(); // Fallback to dialog if no files dropped (or text dropped)
    }
  }

  function handleMouseMove(event) {
    if (rafId) return;
    rafId = requestAnimationFrame(() => {
      if (cursorEl) {
        cursorEl.style.transform = `translate3d(${event.clientX - 150}px, ${event.clientY - 150}px, 0)`;
      }
      rafId = null;
    });
  }

  // --- Sound Effects (Frontend Zero-Latency) ---
  // Replaced Wails backend call with native JS Audio for sub-10ms response
  function playSound(type) {
    // Call Backend
    PlaySound(type);
  }

  // Go Events
  EventsOn("device_connected", (data) => {
    // Only invoke if we are still in handshake mode
    if (appState === "HANDSHAKE") {
      simulateConnection();
    }
  });

  EventsOn("device_disconnected", (data) => {
    // Logic for disconnection
    status = ">> CONNECTION_LOST: SIGNAL_TERMINATED";
    playSound("click");
    // Optionally reset logic could go here:
    // appState = "HANDSHAKE";
    // transitionStage = 0;
    // generateQR(link);
  });

  EventsOn("file_received", (filename) => {
    receivedFiles = [...receivedFiles, filename];
    status = `>> DOWNLOAD_COMPLETE: ${filename}`;
    playSound("success");
    if (appState === "HANDSHAKE") simulateConnection();
  });

  EventsOn("url_changed", (newURL) => {
    console.log("🔄 URL Changed:", newURL);
    link = newURL;
    generateQR(newURL);
    // If we are in sender mode, update that too
    if (showUrlDialog) {
      senderUrl = newURL;
    }
  });

  EventsOn("upload_progress", (data) => {
    const parts = data.split("|");
    const filename = parts[0];
    const bytes = parseInt(parts[1]);
    const now = Date.now();
    const diffTime = (now - lastProgressTime) / 1000;
    if (diffTime >= 0.5) {
      const diffBytes = bytes - lastLoaded;
      const speedBytes = diffBytes / diffTime;
      const speedMB = (speedBytes / (1024 * 1024)).toFixed(2);
      progress = {
        filename: filename,
        percent: 0,
        speed: `${speedMB} MB/S`,
        received: (bytes / (1024 * 1024)).toFixed(2) + " MB",
      };
      lastLoaded = bytes;
      lastProgressTime = now;
    }
  });

  function openFile(filename) {
    // Call backend to open file (bypass sandbox restrictions)
    OpenFile(filename);
  }
</script>

<svelte:window on:mousemove={handleMouseMove} />

<!-- Global Drop Zone Overlay -->
<div
  class="drop-overlay"
  class:visible={isDragOver}
  on:dragover={handleDragOver}
  on:dragleave={handleDragLeave}
  on:drop={handleDrop}
>
  <div class="drop-message blink">[ DROP_TO_INITIATE_UPLINK ]</div>
  <div class="drop-border"></div>
</div>

<main on:dragover={handleDragOver} on:drop={handleDrop}>
  <div class="cursor-glow" bind:this={cursorEl}></div>
  <div class="scanlines"></div>

  <div
    class="container"
    class:collapsed={transitionStage === 2}
    class:expanded={transitionStage === 3}
  >
    <div class="header">
      <h1 class="title">// BEAMSYNC_SYS</h1>
      <div class="status-line">
        {#if transitionStage === 1}
          <span style="color: #00FF41; font-weight: bold;">{status}</span>
        {:else}
          <Typewriter text={status} speed={30} />
        {/if}
      </div>
    </div>

    <!-- MAIN TERMINAL CARD -->
    <div class="terminal-card">
      <div class="corner-bracket top-left"></div>
      <div class="corner-bracket top-right"></div>
      <div class="corner-bracket bottom-right"></div>
      <div class="corner-bracket bottom-left"></div>

      <!-- STATE 1: HANDSHAKE -->
      {#if appState === "HANDSHAKE"}
        <div class="handshake-view" class:access-granted={transitionStage >= 1}>
          {#if qrImage}
            <div class="qr-frame">
              <div class="label">DATA_LINK</div>
              <img src={qrImage} alt="QR CODE" class="qr-code" />
              <div class="qr-scanline"></div>
            </div>
          {/if}
          <div class="instruction-text blink">
            // WAITING_FOR_DEVICE_HANDSHAKE
          </div>
          <!-- DEBUG: Show link -->
          <div
            style="color: red; font-size: 1rem; margin-top: 10px; border: 1px solid red; padding: 5px;"
          >
            DEBUG INFO:<br />
            LINK: {link || "Wait..."}<br />
            QR LEN: {qrImage ? qrImage.length : "0"}
          </div>
        </div>
      {/if}

      <!-- STATE 2: COMMAND DECK -->
      {#if appState === "COMMAND_DECK"}
        <div class="command-deck-view">
          <div class="action-grid">
            <button
              class="cyber-btn receive-btn"
              on:click={() => {
                playSound("click");
                status = ">> LISTENING_FOR_PACKETS...";
              }}
              on:mouseenter={() => playSound("blip")}
            >
              <div class="btn-content">
                <div class="icon-art">
                  <span class="ascii-icon">
                    &#9604;<br />
                    &#9565;&#9552;&#9562;
                  </span>
                </div>
                <span>RECEIVE</span>
              </div>
            </button>

            <button
              class="cyber-btn send-btn"
              on:click={() => {
                playSound("click");
                startSend();
              }}
              on:mouseenter={() => playSound("blip")}
            >
              <div class="btn-content">
                <div class="icon-art">
                  <span class="ascii-icon">
                    &#8593;<br />
                    &#9629;&#9472;&#9624;
                  </span>
                </div>
                <span>SEND</span>
              </div>
            </button>
          </div>

          <!-- Progress / File Info -->
          {#if progress.filename}
            <div class="data-block">
              <div class="data-row">
                <span>FILE: {progress.filename}</span>
                <span class="accent">{progress.speed}</span>
              </div>
              <div class="progress-bar">
                <div class="progress-fill" style="width: 100%"></div>
              </div>
              <div class="data-row">
                <span>TRANSFERRED: {progress.received}</span>
              </div>
            </div>
          {/if}

          {#if receivedFiles.length > 0}
            <div class="log-block">
              <div class="log-header">>> RECEIVED_DATA_LOG</div>
              <ul>
                {#each receivedFiles as file}
                  <li>
                    <button class="link-btn" on:click={() => openFile(file)}>
                      > {file}
                    </button>
                  </li>
                {/each}
              </ul>
            </div>
          {/if}
        </div>
      {/if}
    </div>
  </div>
</main>

<!-- URL DISPLAY DIALOG (Cyberpunk Style) -->
{#if showUrlDialog}
  <div class="url-dialog-overlay">
    <div class="url-card">
      <div class="corner-bracket top-left"></div>
      <div class="corner-bracket top-right"></div>
      <div class="corner-bracket bottom-right"></div>
      <div class="corner-bracket bottom-left"></div>

      <h2 class="dialog-title">// UPLINK_COORDINATES</h2>
      <p class="dialog-msg">ACCESS_REQUIRED_ON_MOBILE_UNIT</p>

      <div class="qr-container">
        {#if qrImage}
          <img src={qrImage} alt="QR CODE" class="dialog-qr" />
        {/if}
      </div>

      <div class="url-box">
        <input type="text" value={senderUrl} readonly />
        <button
          class="copy-btn"
          on:click={() => navigator.clipboard.writeText(senderUrl)}
        >
          COPY
        </button>
      </div>

      <button class="close-btn" on:click={() => (showUrlDialog = false)}>
        [ ABORT_VIEW ]
      </button>
    </div>
  </div>
{/if}

<style>
  /* --- GLOBAL VARIABLES --- */
  :global(:root) {
    --bg-color: #000000;
    --primary: #00ff41;
    --accent: #ffb000;
    --grid-line: rgba(0, 255, 65, 0.15);
  }

  /* ... (Existing global styles) ... */

  /* --- DIALOG STYLES --- */
  .url-dialog-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.9);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 2000;
    backdrop-filter: blur(5px);
  }

  .url-card {
    background: #000;
    border: 2px solid var(--accent); /* Amber for Sender mode */
    padding: 30px;
    max-width: 500px;
    width: 90%;
    text-align: center;
    position: relative;
    box-shadow: 0 0 30px rgba(255, 176, 0, 0.2);
  }

  .url-card .corner-bracket {
    border-color: var(--accent);
  }

  .dialog-title {
    color: var(--accent);
    font-size: 2rem;
    margin: 0 0 10px 0;
    text-transform: uppercase;
    text-shadow: 0 0 5px var(--accent);
  }

  .dialog-msg {
    color: var(--primary);
    margin-bottom: 20px;
    font-size: 1.2rem;
  }

  .qr-container {
    margin: 20px auto;
    border: 1px dashed var(--accent);
    padding: 10px;
    display: inline-block;
  }
  .dialog-qr {
    width: 150px;
    height: 150px;
    image-rendering: pixelated;
  }

  .url-box {
    display: flex;
    gap: 10px;
    margin-bottom: 20px;
  }

  .url-box input {
    flex: 1;
    background: #111;
    border: 1px solid var(--accent);
    color: var(--accent);
    font-family: "VT323", monospace;
    font-size: 1.5rem;
    padding: 5px 10px;
    outline: none;
  }

  .copy-btn {
    background: var(--accent);
    color: #000;
    border: none;
    font-family: "VT323", monospace;
    font-size: 1.2rem;
    padding: 0 20px;
    cursor: pointer;
    font-weight: bold;
  }
  .copy-btn:hover {
    background: #fff;
  }

  .close-btn {
    background: transparent;
    border: 1px solid var(--primary);
    color: var(--primary);
    font-family: "VT323", monospace;
    font-size: 1.2rem;
    padding: 10px 30px;
    cursor: pointer;
    transition: all 0.2s;
  }
  .close-btn:hover {
    background: var(--primary);
    color: #000;
    box-shadow: 0 0 10px var(--primary);
  }

  .link-btn {
    background: none;
    border: none;
    color: var(--primary);
    font-family: inherit;
    font-size: inherit;
    cursor: pointer;
    text-align: left;
    padding: 0;
    text-decoration: underline;
  }
  .link-btn:hover {
    color: #fff;
    text-shadow: 0 0 5px #fff;
  }

  :global(body) {
    margin: 0;
    background: var(--bg-color);
    font-family: "VT323", monospace;
    color: var(--primary);
    overflow: hidden;
    cursor: crosshair;
  }

  /* --- DROP OVERLAY --- */
  .drop-overlay {
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    z-index: 999;
    background: rgba(0, 0, 0, 0.8);
    display: flex;
    align-items: center;
    justify-content: center;
    pointer-events: none;
    opacity: 0;
    transition: opacity 0.2s ease;
  }
  .drop-overlay.visible {
    pointer-events: all;
    opacity: 1;
  }
  .drop-border {
    position: absolute;
    top: 20px;
    left: 20px;
    right: 20px;
    bottom: 20px;
    border: 4px dashed var(--primary);
    box-shadow: 0 0 20px var(--primary);
  }
  .drop-message {
    font-size: 2rem;
    background: #000;
    padding: 10px 20px;
    border: 1px solid var(--primary);
    z-index: 1000;
  }

  /* --- STRUCTURE --- */
  main {
    position: relative;
    width: 100vw;
    height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  /* Background Grid */
  main::before {
    content: "";
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-image: linear-gradient(var(--grid-line) 1px, transparent 1px),
      linear-gradient(90deg, var(--grid-line) 1px, transparent 1px);
    background-size: 40px 40px;
    z-index: 0;
    pointer-events: none;
    backface-visibility: hidden;
  }

  .scanlines {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background: repeating-linear-gradient(
      0deg,
      rgba(0, 0, 0, 0.2),
      rgba(0, 0, 0, 0.2) 1px,
      transparent 1px,
      transparent 2px
    );
    z-index: 2;
    pointer-events: none;
    opacity: 0.3;
    will-change: opacity;
  }

  .container {
    position: relative;
    z-index: 10;
    width: 100%;
    max-width: 500px;
    padding: 20px;
    display: flex;
    flex-direction: column;
    gap: 30px;
    transition:
      transform 0.5s cubic-bezier(0.16, 1, 0.3, 1),
      opacity 0.5s ease-out;
  }

  /* Transition Logic for Container */
  .container.collapsed {
    /* Collapse to a thin line */
    transform: scaleY(0.01) scaleX(0.5);
    opacity: 0; /* Fade out during collapse */
  }
  .container.expanded {
    /* Expand back to full */
    transform: scale(1);
    opacity: 1;
  }

  .header {
    text-align: center;
    text-shadow: 0 0 5px var(--primary);
  }
  .title {
    font-size: 3.5rem;
    margin: 0;
    font-weight: 400;
    letter-spacing: 0.1em;
  }
  .status-line {
    font-size: 1.2rem;
    margin-top: 10px;
    color: var(--primary);
    background: #000;
    display: inline-block;
    padding: 2px 10px;
    border: 1px solid var(--primary);
    min-width: 300px;
    text-align: left;
  }

  .terminal-card {
    position: relative;
    padding: 40px;
    background: #000;
    box-shadow: 0 0 20px rgba(0, 255, 65, 0.1);
    border: 1px solid var(--primary);
    transition: all 0.5s;
    min-height: 300px; /* Keep height stable during transitions */
    overflow: hidden; /* Hide content during collapse */
  }

  .corner-bracket {
    position: absolute;
    width: 15px;
    height: 15px;
    border: 3px solid var(--primary);
    transition: all 0.3s;
  }
  .top-left {
    top: -4px;
    left: -4px;
    border-right: none;
    border-bottom: none;
  }
  .top-right {
    top: -4px;
    right: -4px;
    border-left: none;
    border-bottom: none;
  }
  .bottom-right {
    bottom: -4px;
    right: -4px;
    border-left: none;
    border-top: none;
  }
  .bottom-left {
    bottom: -4px;
    left: -4px;
    border-right: none;
    border-top: none;
  }

  .handshake-view {
    display: flex;
    flex-direction: column;
    align-items: center;
    cursor: default; /* Changed from pointer as it's auto-triggered */
    transition: transform 0.3s;
  }

  .handshake-view.access-granted .qr-frame {
    border-color: #fff;
    box-shadow: 0 0 30px #00ff41;
    transform: scale(1.1);
  }

  .qr-frame {
    position: relative;
    border: 2px solid var(--primary);
    padding: 10px;
    margin-bottom: 20px;
    box-shadow: 0 0 10px var(--grid-line);
    transition: all 0.3s;
  }
  .label {
    position: absolute;
    top: -12px;
    left: 10px;
    background: #000;
    color: var(--primary);
    padding: 0 5px;
    font-size: 1rem;
  }
  .qr-code {
    display: block;
    width: 200px;
    height: 200px;
    image-rendering: pixelated;
  }
  .qr-scanline {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 5px;
    background: var(--primary);
    opacity: 0.5;
    animation: scan 2s infinite linear;
    pointer-events: none;
    will-change: top;
  }

  .blink {
    animation: blink 1.5s infinite;
  }

  .command-deck-view {
    animation: fade-in 0.8s ease-in;
  }

  .action-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 20px;
    margin-bottom: 30px;
  }

  .cyber-btn {
    background: transparent;
    border: 2px solid var(--primary);
    color: var(--primary);
    height: 120px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-family: "VT323", monospace;
    font-size: 1.5rem;
    cursor: pointer;
    transition: all 0.1s;
    position: relative;
    overflow: hidden;
  }

  .send-btn {
    border-color: var(--accent);
    color: var(--accent);
  }

  .btn-content {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 10px;
    z-index: 2;
  }

  .ascii-icon {
    font-size: 2rem;
    line-height: 1rem;
    display: block;
    text-align: center;
  }

  .cyber-btn:hover {
    color: #fff;
    box-shadow: 0 0 5px var(--primary);
    text-shadow: 0 0 2px var(--primary);
    background: repeating-linear-gradient(
      0deg,
      rgba(0, 255, 65, 0.4),
      rgba(0, 255, 65, 0.4) 2px,
      transparent 2px,
      transparent 4px
    );
  }
  .send-btn:hover {
    color: #fff;
    box-shadow: 0 0 5px var(--accent);
    text-shadow: 0 0 2px var(--accent);
    background: repeating-linear-gradient(
      0deg,
      rgba(255, 176, 0, 0.4),
      rgba(255, 176, 0, 0.4) 2px,
      transparent 2px,
      transparent 4px
    );
  }

  .data-block {
    border: 1px dashed var(--primary);
    padding: 15px;
    margin-bottom: 20px;
  }
  .data-row {
    display: flex;
    justify-content: space-between;
    font-size: 1.1rem;
    margin-bottom: 5px;
  }
  .accent {
    color: var(--accent);
  }

  .progress-bar {
    width: 100%;
    height: 10px;
    background: #111;
    border: 1px solid var(--primary);
    margin: 5px 0;
  }
  .progress-fill {
    height: 100%;
    background: repeating-linear-gradient(
      45deg,
      var(--primary),
      var(--primary) 10px,
      #000 10px,
      #000 20px
    );
    animation: stripe-anim 1s linear infinite;
    will-change: background-position;
  }

  .log-block {
    font-size: 1rem;
    color: var(--primary);
    border-top: 2px solid var(--primary);
    padding-top: 10px;
  }
  .log-header {
    margin-bottom: 5px;
    opacity: 0.7;
  }
  .log-block ul {
    list-style: none;
    padding: 0;
    margin: 0;
  }
  .log-block li {
    margin-bottom: 2px;
  }

  @keyframes scan {
    0% {
      top: 0%;
      opacity: 0;
    }
    50% {
      opacity: 1;
    }
    100% {
      top: 100%;
      opacity: 0;
    }
  }
  @keyframes blink {
    0%,
    100% {
      opacity: 1;
    }
    50% {
      opacity: 0;
    }
  }
  @keyframes stripe-anim {
    from {
      background-position: 0 0;
    }
    to {
      background-position: 50px 0;
    }
  }
  @keyframes fade-in {
    from {
      opacity: 0;
    }
    to {
      opacity: 1;
    }
  }

  .cursor-glow {
    position: fixed;
    top: 0;
    left: 0;
    width: 300px;
    height: 300px;
    opacity: 0.4;
    pointer-events: none;
    background: radial-gradient(
      circle,
      rgba(0, 255, 65, 0.4) 0%,
      transparent 70%
    );
    z-index: 5;
    will-change: transform;
    transform: translate3d(-1000px, -1000px, 0);
  }
</style>
