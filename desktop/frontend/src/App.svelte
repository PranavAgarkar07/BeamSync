<script>
  import "./app.css";
  import {
    StartReceiverDefault,
    StartSender,
    PlaySound,
    OpenFile,
    ResetApp,
    GetReceivedFiles,
    GetSavePath,
    SetSavePath,
    GetTransferSettings,
    SaveTransferSettings,
    ApproveTransfer,
    RejectTransfer,
  } from "../wailsjs/go/main/App.js";
  import {
    EventsOn,
    EventsOffAll,
    BrowserOpenURL,
  } from "../wailsjs/runtime/runtime.js";
  import QRCode from "qrcode";
  import { onMount, onDestroy } from "svelte";
  import { fly } from "svelte/transition";
  import Typewriter from "./Typewriter.svelte";
  import SplashScreen from "./SplashScreen.svelte";

  // ── Splash screen ─────────────────────────────────────────────────────────
  let showSplash = true;

  import {
    TopNavBar,
    FileDropZone,
    TransferProgressBar,
    TransferComplete,
    ConnectedDevicesPanel,
  } from "./design-system/index.js";

  // Logo asset
  import logoImg from "./assets/images/icon.png";

  // ── App State ──────────────────────────────────────────────────────────────
  let mode = "RECEIVE"; // "RECEIVE" | "SEND" | "ABOUT" | "SETTINGS"
  let connectionState = "IDLE"; // "IDLE" | "WAITING" | "CONNECTED" | "DISCONNECTED"

  let qrImage = "";
  let serverUrl = "";
  let senderUrl = "";

  let receivedFiles = [];
  let progress = {
    active: false,
    filename: "",
    percent: 0,
    speed: "0 MB/s",
    received: "0.00 MB",
    total: "0.00 MB",
    timeRemaining: "—",
    totalTime: "0s",
    speedColor: "#ffb000",
  };
  let lastProgressTime = 0;
  let lastLoaded = 0;
  let progressStartTime = 0; // Track when transfer started
  let speedHistory = []; // Rolling average for smooth speed display

  let showSenderDialog = false;
  let isDragOver = false;
  let savePath = ""; // persisted save directory

  // ── Transfer Settings ──────────────────────────────────────────────────
  let settings = {
    mode: "ask_first",
    maxFileSizeMB: 0,
    blockedExtensions: [],
    trustedDevices: [],
    blockedDevices: [],
  };
  let settingsDirty = false;
  let transferRequest = null;
  let rememberDevice = false;
  let newBlockedExt = "";
  let newTrustedIP = "";
  let newTrustedName = "";
  let newBlockedIP = "";
  let newBlockedName = "";

  // ── Sound toggle ────────────────────────────────────────────────────────
  let soundEnabled = localStorage.getItem("beamsync_sound") !== "false";
  function toggleSound() {
    soundEnabled = !soundEnabled;
    localStorage.setItem("beamsync_sound", soundEnabled ? "true" : "false");
    if (soundEnabled) PlaySound("blip"); // confirm it's on
  }

  // ── Batch transfer tracking ──────────────────────────────────────────
  let batchCount = 0; // files received this session
  let batchTimer = null; // resets batchCount after idle
  let showTickAnim = false; // drives the "all done" tick overlay
  let lastBatchCount = 0;

  // ── Toast system ──────────────────────────────────────────────────────────
  let toasts = [];
  let _tid = 0;
  let _progressTimeout; // watchdog: clears stale progress if phone drops mid-upload

  function toast(msg, type = "info") {
    const id = ++_tid;
    toasts = [...toasts, { id, msg, type }];
    setTimeout(() => {
      toasts = toasts.filter((t) => t.id !== id);
    }, 3200);
  }

  // ── Cursor glow ─────────────────────────────────────────────────────────
  function handleMouseMove(e) {
    // legacy mouse glow removed
  }

  // ── Mount / Unmount ─────────────────────────────────────────────────────
  onMount(async () => {
    EventsOffAll();

    // Load settings
    try {
      settings = await GetTransferSettings();
    } catch {}

    EventsOn("device_connected", () => {
      connectionState = "CONNECTED";
      playSound("connect");
      toast("⚡ Device linked to network", "success");
    });
    EventsOn("device_disconnected", () => {
      connectionState = "DISCONNECTED";
      playSound("click");
      toast("💔 Signal lost — device disconnected", "warn");
    });
    EventsOn("transfer_request", (dataStr) => {
      playSound("connect");
      transferRequest = JSON.parse(dataStr);
    });
    EventsOn("file_received", (filename) => {
      refreshFileList();
      clearTimeout(_progressTimeout);
      progress = {
        active: false,
        filename: "",
        percent: 0,
        speed: "0 MB/s",
        received: "0.00 MB",
        total: "0.00 MB",
        timeRemaining: "—",
        totalTime: "0s",
        speedColor: "#ffb000",
      };
      lastLoaded = 0;
      lastProgressTime = 0;
      progressStartTime = 0;
      speedHistory = [];

      batchCount += 1;
      clearTimeout(batchTimer);
      batchTimer = setTimeout(() => {
        if (batchCount > 0) {
          playSound("success");
          lastBatchCount = batchCount;
          showTickAnim = true;
          batchCount = 0;
        }
      }, 2500);

      toast(`✅ Received: ${filename}`, "success");
    });

    const formatTime = (seconds) => {
      if (isNaN(seconds) || !isFinite(seconds)) return "—";
      if (seconds < 0) return "—";
      if (seconds < 60) return `${Math.round(seconds)}s`;
      const mins = Math.floor(seconds / 60);
      const secs = Math.round(seconds % 60);
      if (mins < 60) return `${mins}m ${secs}s`;
      const hours = Math.floor(mins / 60);
      const remainMins = mins % 60;
      return `${hours}h ${remainMins}m`;
    };

    const getSpeedColor = (speedMBps) => {
      if (speedMBps > 10) return "#00ff00";
      if (speedMBps > 5) return "#ffb000";
      return "#ff6b6b";
    };

    const calculateSmoothedSpeed = (currentSpeed) => {
      speedHistory.push(currentSpeed);
      if (speedHistory.length > 10) speedHistory.shift();
      const avg = speedHistory.reduce((a, b) => a + b, 0) / speedHistory.length;
      return avg;
    };

    const handleProgressUpdate = (data) => {
      const parts = data.split("|");
      if (parts.length < 3) return;
      const [filename, wStr, tStr] = parts;
      const written = parseInt(wStr);
      const total = parseInt(tStr);
      const now = Date.now();
      const dt = (now - lastProgressTime) / 1000;

      if (progressStartTime === 0) {
        progressStartTime = now;
      }

      let instantSpeed = 0;
      if (dt > 0 && lastProgressTime > 0) {
        instantSpeed = (written - lastLoaded) / dt / 1048576;
      }

      const smoothedSpeed = calculateSmoothedSpeed(Math.max(0, instantSpeed));
      const speedStr = `${Math.max(0, smoothedSpeed).toFixed(2)} MB/s`;
      const speedColor = getSpeedColor(smoothedSpeed);

      let timeRemaining = "—";
      if (smoothedSpeed > 0) {
        const remainingBytes = total - written;
        const secondsRemaining = remainingBytes / (smoothedSpeed * 1048576);
        timeRemaining = formatTime(secondsRemaining);
      }

      const elapsedSeconds = (now - progressStartTime) / 1000;
      const totalTimeStr = formatTime(elapsedSeconds);

      lastLoaded = written;
      lastProgressTime = now;
      const pct =
        total > 0 ? Math.min(100, Math.round((written / total) * 100)) : -1;

      progress = {
        active: true,
        filename,
        percent: pct,
        speed: speedStr,
        received: `${(written / 1048576).toFixed(2)} MB`,
        total: total > 0 ? `${(total / 1048576).toFixed(2)} MB` : 'Unknown',
        timeRemaining,
        totalTime: totalTimeStr,
        speedColor,
      };

      if (connectionState !== "CONNECTED") connectionState = "CONNECTED";
      clearTimeout(batchTimer);
      clearTimeout(_progressTimeout);
      _progressTimeout = setTimeout(() => {
        progress = {
          active: false,
          filename: "",
          percent: 0,
          speed: "0 MB/s",
          received: "0.00 MB",
          total: "0.00 MB",
          timeRemaining: "—",
          totalTime: "0s",
          speedColor: "#ffb000",
        };
        lastLoaded = 0;
        lastProgressTime = 0;
        progressStartTime = 0;
        speedHistory = [];
      }, 30000);
    };

    EventsOn("upload_progress", handleProgressUpdate);
    EventsOn("download_progress", handleProgressUpdate);
    EventsOn("url_changed", (newURL) => {
      serverUrl = newURL;
      generateQR(newURL);
      if (showSenderDialog) senderUrl = newURL;
      toast("🔄 Network changed — QR refreshed", "info");
    });
    EventsOn("sender_started", (url) => {
      senderUrl = url;
      showSenderDialog = true;
      generateQR(url);
    });

    await initReceiver();
    try {
      savePath = await GetSavePath();
    } catch {
      savePath = "";
    }
  });

  onDestroy(() => {
    EventsOffAll();
    clearTimeout(batchTimer);
    clearTimeout(_progressTimeout);
  });

  async function initReceiver() {
    connectionState = "WAITING";
    playSound("startup");
    try {
      serverUrl = await StartReceiverDefault();
    } catch {
      serverUrl = "";
      toast("❌ Failed to start receiver", "error");
      connectionState = "IDLE";
      return;
    }
    generateQR(serverUrl);
    await refreshFileList();
  }

  async function refreshFileList() {
    try {
      const files = await GetReceivedFiles();
      if (files) receivedFiles = files;
    } catch {
      /* non-blocking */
    }
  }

  async function switchMode(newMode) {
    const alreadySameMode = mode === newMode;
    if (alreadySameMode && connectionState === "CONNECTED") return;
    playSound("blip");
    mode = newMode;
    if (newMode === "RECEIVE" && connectionState !== "CONNECTED") {
      await resetAll();
      await initReceiver();
    }
  }

  function openLink(url) {
    BrowserOpenURL(url);
  }

  async function startSend() {
    playSound("click");
    const result = await StartSender();
    if (result === "Cancelled") {
      toast("Sender cancelled", "info");
      return;
    }
    senderUrl = result;
    showSenderDialog = true;
    generateQR(result);
  }

  function generateQR(text) {
    if (!text) return;
    QRCode.toDataURL(
      text,
      {
        width: 220,
        margin: 2,
        color: { dark: "#0A0A0A", light: "#00000000" },
      },
      (err, url) => {
        if (!err) qrImage = url;
      },
    );
  }

  function playSound(type) {
    if (soundEnabled) PlaySound(type);
  }
  function openFile(name) {
    OpenFile(name);
  }

  function formatSize(bytes) {
    if (!bytes) return "—";
    if (bytes >= 1048576) return (bytes / 1048576).toFixed(1) + " MB";
    if (bytes >= 1024) return (bytes / 1024).toFixed(0) + " KB";
    return bytes + " B";
  }

  function fileIcon(name = "") {
    const ext = name.split(".").pop().toLowerCase();
    const m = {
      pdf: "📄",
      jpg: "🖼️",
      jpeg: "🖼️",
      png: "🖼️",
      gif: "🖼️",
      webp: "🖼️",
      svg: "🖼️",
      mp4: "🎬",
      mov: "🎬",
      mkv: "🎬",
      avi: "🎬",
      mp3: "🎵",
      wav: "🎵",
      flac: "🎵",
      zip: "📦",
      tar: "📦",
      gz: "📦",
      rar: "📦",
      txt: "📝",
      md: "📝",
      doc: "📝",
      docx: "📝",
      apk: "📱",
      exe: "⚙️",
    };
    return m[ext] || "📁";
  }

  async function resetAll() {
    playSound("click");
    await ResetApp();
    connectionState = "IDLE";
    qrImage = "";
    serverUrl = "";
    senderUrl = "";
    showSenderDialog = false;
    progress = {
      active: false,
      filename: "",
      percent: 0,
      speed: "0 MB/s",
      received: "0.00 MB",
      total: "0.00 MB",
      timeRemaining: "—",
      totalTime: "0s",
      speedColor: "#ffb000",
    };
    lastLoaded = 0;
    lastProgressTime = 0;
  }

  async function changeSavePath() {
    playSound("click");
    const result = await SetSavePath();
    if (result === "Cancelled") {
      toast("Folder selection cancelled", "info");
      return;
    }
    if (result.startsWith("Error:")) {
      toast("❌ " + result, "error");
      return;
    }
    serverUrl = result;
    generateQR(result);
    savePath = await GetSavePath();
    connectionState = "WAITING";
    toast(`📁 Save path updated`, "success");
  }

  async function handleDisconnectReset() {
    await resetAll();
    mode = "RECEIVE";
    await initReceiver();
  }

  // ── Settings Logic ──────────────────────────────────────────────────
  async function saveSettings() {
    playSound("click");
    settings.maxFileSizeMB = Number(settings.maxFileSizeMB) || 0;
    await SaveTransferSettings(settings);
    settingsDirty = false;
    toast("Settings saved", "success");
  }

  function addBlockedExt() {
    let ext = newBlockedExt.trim().toLowerCase();
    if (!ext) return;
    if (!ext.startsWith(".")) ext = "." + ext;
    if (!settings.blockedExtensions.includes(ext)) {
      settings.blockedExtensions = [...settings.blockedExtensions, ext];
      settingsDirty = true;
    }
    newBlockedExt = "";
  }

  function removeBlockedExt(ext) {
    settings.blockedExtensions = settings.blockedExtensions.filter(e => e !== ext);
    settingsDirty = true;
  }

  function addTrustedDevice() {
    const ip = newTrustedIP.trim();
    if (!ip) return;
    if (!settings.trustedDevices.find(d => d.ip === ip)) {
      settings.trustedDevices = [...settings.trustedDevices, { ip, friendlyName: newTrustedName.trim() || ip }];
      settingsDirty = true;
    }
    newTrustedIP = "";
    newTrustedName = "";
  }

  function removeTrustedDevice(ip) {
    settings.trustedDevices = settings.trustedDevices.filter(d => d.ip !== ip);
    settingsDirty = true;
  }

  function addBlockedDevice() {
    const ip = newBlockedIP.trim();
    if (!ip) return;
    if (!settings.blockedDevices.find(d => d.ip === ip)) {
      settings.blockedDevices = [...settings.blockedDevices, { ip, friendlyName: newBlockedName.trim() || ip }];
      settingsDirty = true;
    }
    newBlockedIP = "";
    newBlockedName = "";
  }

  function removeBlockedDevice(ip) {
    settings.blockedDevices = settings.blockedDevices.filter(d => d.ip !== ip);
    settingsDirty = true;
  }

  function handleConsent(approve) {
    if (!transferRequest) return;
    playSound("click");
    if (approve) {
      ApproveTransfer(transferRequest.id);
      if (rememberDevice) {
        settings.trustedDevices = [...settings.trustedDevices, { ip: transferRequest.senderIP, friendlyName: transferRequest.senderName }];
        SaveTransferSettings(settings);
      }
    } else {
      RejectTransfer(transferRequest.id);
      if (rememberDevice) {
        settings.blockedDevices = [...settings.blockedDevices, { ip: transferRequest.senderIP, friendlyName: transferRequest.senderName }];
        SaveTransferSettings(settings);
      }
    }
    transferRequest = null;
    rememberDevice = false;
  }

  // Drag & drop
  function handleDragOver(e) {
    e.preventDefault();
    isDragOver = true;
  }
  function handleDragLeave(e) {
    e.preventDefault();
    if (e.clientX === 0 && e.clientY === 0) isDragOver = false;
  }
  function handleDrop(e) {
    e.preventDefault();
    isDragOver = false;
    mode = "SEND";
    startSend();
  }

  $: displayUrl = serverUrl.replace(/\/\?token=.*$/, "");
  $: sortedFiles = [...receivedFiles];
</script>

<svelte:window on:mousemove={handleMouseMove} />

{#if transferRequest}
  <div class="nb-card" style="position: fixed; top: 50%; left: 50%; transform: translate(-50%, -50%); z-index: 10000; width: 440px; padding: 2.5rem; border: 4px solid var(--nb-border-color); background: var(--nb-surface);">
    <div style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 1.5rem;">
      <h2 style="margin: 0; font-size: 1.5rem; font-weight: bold; letter-spacing: -0.01em;">Incoming Transfer</h2>
      <span class="nb-badge nb-badge--primary" style="font-size: 0.75rem; padding: 0.25rem 0.6rem; animation: nb-pulse 2s infinite;">PENDING</span>
    </div>
    
    <div style="background: var(--nb-bg); border: 2px solid var(--nb-border-color); padding: 1.25rem; margin-bottom: 1.5rem;">
      <div style="display: flex; justify-content: space-between; margin-bottom: 0.75rem; border-bottom: 2px dashed var(--nb-border-color); padding-bottom: 0.75rem;">
        <span style="color: var(--nb-text-muted); font-weight: 600;">From Device</span>
        <strong style="font-size: 1rem;">{transferRequest.senderName || transferRequest.senderIP}</strong>
      </div>
      <div style="display: flex; justify-content: space-between; margin-bottom: 0.75rem; border-bottom: 2px dashed var(--nb-border-color); padding-bottom: 0.75rem;">
        <span style="color: var(--nb-text-muted); font-weight: 600;">File Name</span>
        <strong style="font-size: 1rem; word-break: break-all; max-width: 65%; text-align: right;">{transferRequest.filename}</strong>
      </div>
      <div style="display: flex; justify-content: space-between;">
        <span style="color: var(--nb-text-muted); font-weight: 600;">File Size</span>
        <strong style="font-size: 1rem;">{transferRequest.sizeMB}</strong>
      </div>
    </div>
    
    <label style="display: flex; gap: 0.75rem; align-items: center; margin-bottom: 1.75rem; cursor: pointer; font-size: 1rem; font-weight: 600;">
      <input type="checkbox" bind:checked={rememberDevice} style="width: 20px; height: 20px; accent-color: var(--nb-primary); border: 2px solid var(--nb-border-color);"> 
      Always accept transfers from this device
    </label>
    
    <div style="display: flex; gap: 1.5rem;">
      <button class="nb-btn nb-btn--danger" style="flex: 1; padding: 0.75rem; font-size: 1rem;" on:click={() => handleConsent(false)}>Decline</button>
      <button class="nb-btn nb-btn--primary" style="flex: 1; padding: 0.75rem; font-size: 1rem;" on:click={() => handleConsent(true)}>Accept Transfer</button>
    </div>
  </div>
  <div style="position: fixed; inset: 0; background: rgba(0, 0, 0, 0.7); z-index: 9999;"></div>
{/if}

{#if showSplash}
  <SplashScreen on:done={() => (showSplash = false)} />
{/if}

<div
  class="app-dropzone"
  on:dragover={handleDragOver}
  on:drop={handleDrop}
  on:dragleave={handleDragLeave}
>
  {#if isDragOver}
    <div class="drop-overlay">
      <div class="drop-message">[ DROP → INITIATE_SEND ]</div>
    </div>
  {/if}

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
      on:settings={() => switchMode('SETTINGS')}
      on:reset={handleDisconnectReset}
    />

    <main class="main-content">
      {#if mode === "RECEIVE"}
        <div class="mode-wrapper" in:fly={{ y: 15, duration: 250 }}>
        {#if connectionState !== "CONNECTED"}
          <div class="receive-standby">
            <div class="nb-card home-card">
              <div class="home-card__header">
                <div
                  class="status-indicator"
                  class:pulse={connectionState === "WAITING"}
                ></div>
                <h1 class="standby-title">
                  {#if connectionState === "WAITING"}
                    Connect via {serverUrl
                      .replace(/^https?:\/\//, "")
                      .split(":")[0] || "Wi-Fi"}
                  {:else if connectionState === "DISCONNECTED"}
                    Connection Lost
                  {:else}
                    Ready to Connect
                  {/if}
                </h1>
              </div>

              <div class="home-card__body">
                {#if qrImage}
                  <div class="qr-wrapper">
                    <img
                      src={qrImage}
                      alt="QR Code"
                      class="qr-code"
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
                      on:click={() => {
                        navigator.clipboard.writeText(displayUrl);
                        toast("Copied!", "success");
                      }}>COPY</button
                    >
                  </div>
                {/if}
              </div>
            </div>

            {#if connectionState === "DISCONNECTED"}
              <button
                class="nb-btn nb-btn--danger reconnect-btn"
                on:click={handleDisconnectReset}>RECONNECT</button
              >
            {/if}
          </div>
        {:else}
          <div class="receive-active">
            <h2 class="active-title">Device Connected</h2>

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
              <div class="ready-banner pulse-bg">
                <div class="radar-ping"></div>
                <div class="ready-content">
                  <span class="status-badge">READY</span>
                  <span class="status-text">WAITING FOR FILES...</span>
                </div>
              </div>
            {/if}

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
                    on:click={() => openFile(file.name)}
                  >
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
        </div>
      {:else if mode === "SEND"}
        <div class="mode-wrapper send-layout" in:fly={{ y: 15, duration: 250 }}>
          <FileDropZone on:selectFiles={startSend} on:requestPicker={startSend} />

          {#if showSenderDialog}
            <div class="sender-dialog">
              <div class="sender-header">
                <span class="radar-ping-small"></span>
                <h3>READY TO SEND</h3>
              </div>
              <p class="sender-desc">Scan the QR code on the receiving device to download</p>
              
              {#if qrImage}
                <div class="qr-frame">
                  <img src={qrImage} alt="Sender QR" class="sender-qr" />
                </div>
              {/if}
              
              <div class="url-action-bar">
                <span class="url-label">Or share this link:</span>
                <div class="url-box">
                  <input class="url-input nb-mono" readonly value={senderUrl} />
                  <button
                    class="nb-btn nb-btn--primary"
                    on:click={() => {
                      navigator.clipboard.writeText(senderUrl);
                      toast("Link copied!", "success");
                    }}>COPY</button
                  >
                </div>
              </div>
              
              <button
                class="nb-btn nb-btn--danger close-btn"
                on:click={() => (showSenderDialog = false)}>CLOSE SESSION</button
              >
            </div>
          {/if}
        </div>
      {:else if mode === "SETTINGS"}
        <div class="mode-wrapper" in:fly={{ y: 15, duration: 250 }}>
          <div class="nb-card" style="padding: 2rem;">
            <h2 style="margin-top: 0;">Transfer Settings</h2>
            
            <div style="margin-bottom: 2rem;">
              <h3>Save Location</h3>
              <div style="display: flex; gap: 0.5rem; align-items: center; border: 2px solid var(--nb-border-color); padding: 0.5rem; background: var(--nb-bg);">
                <span class="nb-badge" style="background: var(--nb-border-color); color: var(--nb-surface);">Path</span>
                <span style="flex: 1; word-break: break-all; font-family: monospace;">{savePath || "Default"}</span>
                <button class="nb-btn nb-btn--ghost nb-btn--sm" on:click={changeSavePath} style="padding: 0.25rem 0.5rem;">CHANGE</button>
              </div>
              <p style="color: var(--nb-text-muted); font-size: 0.8rem; margin-top: 0.5rem;">Changing the save location will temporarily restart the receiver.</p>
            </div>

            <div style="margin-bottom: 2rem;">
              <h3>Transfer Mode</h3>
              {#each [{v: "ask_first", l: "Ask First"}, {v: "accept_all", l: "Accept All"}, {v: "block_all", l: "Block All"}] as opt}
                <label style="display: block; margin: 0.5rem 0;">
                  <input type="radio" bind:group={settings.mode} value={opt.v} on:change={() => settingsDirty = true}> {opt.l}
                </label>
              {/each}
            </div>

            <div style="margin-bottom: 2rem;">
              <h3>Blocked Extensions</h3>
              <div style="display: flex; gap: 0.75rem;">
                <input class="nb-input" bind:value={newBlockedExt} placeholder=".exe" style="flex: 1;">
                <button class="nb-btn nb-btn--secondary" on:click={addBlockedExt}>ADD</button>
              </div>
              <div style="margin-top: 0.5rem;">
                {#each settings.blockedExtensions as ext}
                  <span class="nb-badge" on:click={() => removeBlockedExt(ext)} style="cursor: pointer;">{ext} ✕</span>
                {/each}
              </div>
            </div>

            <div style="margin-bottom: 2rem;">
              <h3>Application Sounds</h3>
              <label style="display: flex; gap: 0.75rem; align-items: center; margin-bottom: 1.75rem; cursor: pointer; font-size: 1rem; font-weight: 600;">
                <input type="checkbox" bind:checked={soundEnabled} on:change={() => { localStorage.setItem("beamsync_sound", soundEnabled.toString()); playSound("click"); }} style="width: 20px; height: 20px; accent-color: var(--nb-primary); border: 2px solid var(--nb-border-color);"> 
                Enable application sound effects
              </label>
            </div>

            {#if settingsDirty}
              <button class="nb-btn nb-btn--primary" on:click={saveSettings}>SAVE SETTINGS</button>
            {/if}
          </div>
        </div>
      {:else if mode === "ABOUT"}
        <div class="mode-wrapper about-layout" in:fly={{ y: 15, duration: 250 }}>
          <div class="about-card">
            <div class="about-header">
              <div class="logo-box">
                <img src={logoImg} class="about-logo" alt="BeamSync Logo" />
              </div>
              <div class="about-title">
                <h1>BEAMSYNC</h1>
                <span class="version-badge">v2.2</span>
              </div>
            </div>
            
            <p class="about-desc">
              Fast, token-secured file transfers over your local network. No
              cloud. No accounts.
            </p>
            
            <div class="about-tags">
              <span class="nb-badge nb-badge--info">LAN ONLY</span>
              <span class="nb-badge nb-badge--success">ZERO CLOUD</span>
            </div>
          </div>

          <div class="developer-card">
            <div class="dev-header">
              <h3>SYSTEM ARCHITECT</h3>
            </div>
            <div class="dev-body">
              <span class="dev-name">Pranav Agarkar</span>
              <div class="about-links">
                <button
                  class="nb-btn nb-btn--primary"
                  on:click={() => openLink("https://github.com/PranavAgarkar07")}
                  >GITHUB</button
                >
                <button
                  class="nb-btn nb-btn--secondary"
                  on:click={() =>
                    openLink("https://pranavagarkar07.github.io/portfolio-svelte/")}
                  >PORTFOLIO</button
                >
              </div>
            </div>
          </div>
        </div>
      {/if}
    </main>
  </div>

  {#if showTickAnim}
    <TransferComplete
      show={true}
      fileCount={lastBatchCount}
      on:dismiss={() => (showTickAnim = false)}
    />
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
  .receive-standby,
  .receive-active,
  .about-layout,
  .send-layout {
    width: 100%;
    max-width: 800px;
    display: flex;
    flex-direction: column;
    gap: var(--nb-space-6);
  }

  .receive-standby {
    align-items: center;
    width: 100%;
    max-width: 500px;
    margin: 0 auto;
    margin-top: var(--nb-space-4);
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

  .save-path-row {
    font-size: var(--nb-text-sm);
    font-family: var(--nb-font-mono);
    font-weight: 700;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--nb-space-3);
    background: var(--nb-surface);
    border: var(--nb-border-lg);
    border-style: dashed;
    padding: var(--nb-space-3) var(--nb-space-4);
  }

  .save-path-val {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--nb-text-muted);
  }

  .save-path-val {
    flex: 1;
    min-width: 0;
    font-family: var(--nb-font-mono);
    font-size: var(--nb-text-xs);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .reconnect-btn {
    margin-top: var(--nb-space-4);
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

  /* Sender Dialog */
  .sender-dialog {
    padding: var(--nb-space-6);
    background: var(--nb-surface);
    border: var(--nb-border-lg);
    box-shadow: 6px 6px 0px var(--nb-shadow-color);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--nb-space-4);
    margin-top: var(--nb-space-6);
  }

  .sender-header {
    display: flex;
    align-items: center;
    gap: var(--nb-space-3);
    background: var(--nb-bg);
    border: var(--nb-border-md);
    padding: var(--nb-space-2) var(--nb-space-4);
  }

  .sender-header h3 {
    margin: 0;
    font-family: var(--nb-font-display);
    font-size: var(--nb-text-lg);
    font-weight: 800;
  }

  .sender-desc {
    color: var(--nb-text);
    font-weight: var(--nb-fw-bold);
    margin-bottom: var(--nb-space-2);
    text-align: center;
  }

  .radar-ping-small {
    width: 12px;
    height: 12px;
    background: var(--nb-primary);
    border-radius: 50%;
    animation: ping 1.5s cubic-bezier(0, 0, 0.2, 1) infinite;
  }

  .qr-frame {
    background: var(--nb-bg);
    padding: var(--nb-space-4);
    border: var(--nb-border-lg);
    box-shadow: var(--nb-shadow-md);
    margin: var(--nb-space-2) 0;
  }

  .sender-qr {
    width: 200px;
    height: 200px;
    display: block;
    background: #ffffff;
  }

  .url-action-bar {
    width: 100%;
    max-width: 400px;
    display: flex;
    flex-direction: column;
    gap: var(--nb-space-2);
    margin-bottom: var(--nb-space-3);
  }

  .url-label {
    font-family: var(--nb-font-mono);
    font-size: var(--nb-text-sm);
    font-weight: var(--nb-fw-bold);
  }

  .url-box {
    display: flex;
    width: 100%;
    border: var(--nb-border-md);
    background: var(--nb-bg);
  }

  .url-input {
    flex: 1;
    background: transparent;
    border: none;
    padding: var(--nb-space-3);
    outline: none;
    min-width: 0;
    color: var(--nb-text);
  }

  .close-btn {
    width: 100%;
    max-width: 400px;
    margin-top: var(--nb-space-2);
  }

  /* About Layout */
  .about-layout {
    display: flex;
    flex-direction: column;
    gap: var(--nb-space-6);
    margin-top: var(--nb-space-5);
    max-width: 600px;
    width: 100%;
    margin-left: auto;
    margin-right: auto;
  }

  .about-card {
    background: var(--nb-surface);
    border: var(--nb-border-lg);
    box-shadow: 6px 6px 0px var(--nb-shadow-color);
    padding: var(--nb-space-6);
    display: flex;
    flex-direction: column;
    gap: var(--nb-space-4);
  }

  .about-header {
    display: flex;
    align-items: center;
    gap: var(--nb-space-4);
    border-bottom: var(--nb-border-md);
    padding-bottom: var(--nb-space-4);
  }

  .logo-box {
    background: #000000;
    border: var(--nb-border-lg);
    box-shadow: 4px 4px 0px var(--nb-shadow-color);
    padding: var(--nb-space-1);
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .about-logo {
    width: 64px;
    height: 64px;
    display: block;
  }

  .about-title {
    display: flex;
    flex-direction: row;
    gap: var(--nb-space-3);
    align-items: center;
  }

  .about-title h1 {
    margin: 0;
    font-family: var(--nb-font-display);
    font-weight: 800;
    font-size: 2rem;
    line-height: 1;
  }

  .version-badge {
    background: var(--nb-primary);
    color: var(--nb-primary-text);
    padding: 4px 8px;
    font-family: var(--nb-font-mono);
    font-size: var(--nb-text-sm);
    font-weight: 800;
    border: var(--nb-border-md);
  }

  .about-desc {
    font-family: var(--nb-font-mono);
    font-size: var(--nb-text-sm);
    line-height: 1.6;
    margin: 0;
    color: var(--nb-text-muted);
  }

  .about-tags {
    display: flex;
    gap: var(--nb-space-2);
    margin-top: var(--nb-space-3);
  }

  .developer-card {
    background: var(--nb-bg);
    border: var(--nb-border-lg);
    box-shadow: 4px 4px 0px var(--nb-shadow-color);
    display: flex;
    flex-direction: column;
  }

  .dev-header {
    background: var(--nb-primary);
    border-bottom: var(--nb-border-lg);
    padding: var(--nb-space-2) var(--nb-space-4);
  }

  .dev-header h3 {
    margin: 0;
    font-family: var(--nb-font-display);
    font-weight: 800;
    font-size: var(--nb-text-sm);
    letter-spacing: 0.05em;
    color: var(--nb-primary-text);
  }

  .dev-body {
    padding: var(--nb-space-4);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--nb-space-4);
  }

  .dev-name {
    font-weight: var(--nb-fw-bold);
    font-size: var(--nb-text-lg);
    font-family: var(--nb-font-display);
  }

  .about-links {
    display: flex;
    gap: var(--nb-space-3);
  }
</style>
