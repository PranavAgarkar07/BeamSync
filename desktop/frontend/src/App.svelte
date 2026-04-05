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
  } from "../wailsjs/go/main/App.js";
  import { EventsOn, BrowserOpenURL } from "../wailsjs/runtime/runtime.js";
  import QRCode from "qrcode";
  import { onMount } from "svelte";
  import Typewriter from "./Typewriter.svelte";

  // Logo asset
  import logoImg from "./assets/images/icon.png";

  // ── App State ──────────────────────────────────────────────────────────────
  let mode = "RECEIVE"; // "RECEIVE" | "SEND" | "ABOUT"
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
    total: 0,
    remaining: "—",
  };
  let lastProgressTime = 0;
  let lastLoaded = 0;

  let showSenderDialog = false;
  let isDragOver = false;
  let savePath = ""; // persisted save directory

  // ── Toast system ──────────────────────────────────────────────────────────
  // Each toast: { id, msg, type }
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
  let cursorEl;
  let _raf;
  function handleMouseMove(e) {
    if (_raf) return;
    _raf = requestAnimationFrame(() => {
      if (cursorEl)
        cursorEl.style.transform = `translate3d(${e.clientX - 150}px,${e.clientY - 150}px,0)`;
      _raf = null;
    });
  }

  // ── Mount ─────────────────────────────────────────────────────────────────
  onMount(async () => {
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
    EventsOn("file_received", (filename) => {
      refreshFileList();
      // Fully reset progress — don't spread stale filename/numbers
      clearTimeout(_progressTimeout);
      progress = {
        active: false,
        filename: "",
        percent: 0,
        speed: "0 MB/s",
        received: "0.00 MB",
        total: 0,
        remaining: "—",
      };
      lastLoaded = 0;
      lastProgressTime = 0;
      playSound("success");
      toast(`✅ Received: ${filename}`, "success");
    });
    EventsOn("upload_progress", (data) => {
      const parts = data.split("|");
      if (parts.length < 4) return;
      const [filename, wStr, tStr, totalStr] = parts;
      const written = parseInt(wStr);
      const total = parseInt(tStr);
      const totalBytes = parseInt(totalStr);
      const now = Date.now();
      const dt = (now - lastProgressTime) / 1000;
      let speed = progress.speed;
      if (dt > 0 && lastProgressTime > 0)
        speed = `${((written - lastLoaded) / dt / 1048576).toFixed(2)} MB/s`;
      lastLoaded = written;
      lastProgressTime = now;
      const pct =
        totalBytes > 0 ? Math.min(100, Math.round((written / totalBytes) * 100)) : 0;
      const remainingBytes = totalBytes - written;
      const remainingSeconds = remainingBytes > 0 ? remainingBytes / (speed * 1048576) : 0;
      const remainingTime = remainingSeconds > 60 ? `${Math.floor(remainingSeconds / 60)}m ${Math.floor(remainingSeconds % 60)}s` : `${Math.floor(remainingSeconds)}s`;

      progress = {
        active: true,
        filename,
        percent: pct,
        speed,
        received: `${(written / 1048576).toFixed(2)} MB`,
        total: `${(totalBytes / 1048576).toFixed(2)} MB`,
        remaining: remainingTime,
      };
      if (connectionState !== "CONNECTED") connectionState = "CONNECTED";
      // Reset stale-progress watchdog: clears if no progress event for 30s
      clearTimeout(_progressTimeout);
      _progressTimeout = setTimeout(() => {
        progress = {
          active: false,
          filename: "",
          percent: 0,
          speed: "0 MB/s",
          received: "0.00 MB",
          total: 0,
          remaining: "—",
        };
        lastLoaded = 0;
        lastProgressTime = 0;
      }, 30000);
    });
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
    // Load persisted save path for sidebar display
    try {
      savePath = await GetSavePath();
    } catch {
      savePath = "";
    }
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
    // Re-allow switching to RECEIVE even if already there but connection is lost
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
        color: { dark: "#00FF41", light: "#00000000" },
      },
      (err, url) => {
        if (!err) qrImage = url;
      },
    );
  }

  function playSound(type) {
    PlaySound(type);
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
      total: 0,
      remaining: "—",
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
    // SetSavePath restarts receiver and returns new URL
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

  // ── Derived (computed once, not on every render tick) ─────────────────────
  $: connLabel = {
    IDLE: "OFFLINE",
    WAITING: "LISTENING",
    CONNECTED: "LINKED",
    DISCONNECTED: "LOST",
  }[connectionState];
  $: connClass = {
    IDLE: "st--idle",
    WAITING: "st--wait",
    CONNECTED: "st--ok",
    DISCONNECTED: "st--err",
  }[connectionState];
  $: displayUrl = serverUrl.replace(/\/\?token=.*$/, "");
  $: sortedFiles = [...receivedFiles]; // backend returns newest-first
</script>

<svelte:window on:mousemove={handleMouseMove} />

<!-- Cursor glow — isolated fixed layer, GPU-promoted via will-change -->
<div class="cursor-glow" bind:this={cursorEl} aria-hidden="true"></div>

<!-- Drop overlay -->
<div
  class="drop-overlay"
  class:visible={isDragOver}
  on:dragover={handleDragOver}
  on:dragleave={handleDragLeave}
  on:drop={handleDrop}
>
  <div class="drop-message blink">[ DROP → INITIATE_SEND ]</div>
  <div class="drop-border"></div>
</div>

<!-- Toast rack -->
<div class="toast-rack" aria-live="polite">
  {#each toasts as t (t.id)}
    <div class="toast toast--{t.type}">{t.msg}</div>
  {/each}
</div>

<!-- App Shell -->
<div class="shell" on:dragover={handleDragOver} on:drop={handleDrop}>
  <!-- ── SIDEBAR ─────────────────────────────────────────────────────────── -->
  <aside class="sidebar">
    <!-- Logo + name -->
    <div class="sidebar__logo">
      <img src={logoImg} alt="BeamSync" class="logo-img" />
      <div>
        <div class="logo-title">BEAMSYNC</div>
        <div class="logo-ver">v2.0</div>
      </div>
    </div>

    <!-- Connection status pill -->
    <div class="conn-pill {connClass}">
      <span class="conn-dot"></span>
      <span class="conn-label">{connLabel}</span>
    </div>

    <!-- Mode tabs -->
    <nav class="sidebar__nav">
      <button
        class="nav-btn"
        class:active={mode === "RECEIVE"}
        on:click={() => switchMode("RECEIVE")}
        on:mouseenter={() => playSound("blip")}
      >
        <span class="nav-icon">⬇</span><span class="nav-label">RECEIVE</span>
      </button>
      <button
        class="nav-btn"
        class:active={mode === "SEND"}
        on:click={() => {
          switchMode("SEND");
          if (mode === "SEND") startSend();
        }}
        on:mouseenter={() => playSound("blip")}
      >
        <span class="nav-icon">⬆</span><span class="nav-label">SEND</span>
      </button>
    </nav>

    <!-- Network info -->
    {#if displayUrl}
      <div class="net-info">
        <div class="net-label">SERVER</div>
        <div class="net-url">{displayUrl}</div>
      </div>
    {/if}

    {#if savePath}
      <div class="save-path-box">
        <div class="save-path-header">
          <span class="save-path-icon">📂</span>
          <span class="save-path-label">SAVE TO</span>
        </div>
        <div class="save-path-value" title={savePath}>
          {savePath.split('/').slice(-2).join('/')}
        </div>
        <button
          class="save-path-btn"
          on:click={changeSavePath}
          on:mouseenter={() => playSound("blip")}
        >
          ✎ CHANGE
        </button>
      </div>
    {/if}

    <div class="sidebar__spacer"></div>

    <!-- About nav at the bottom -->
    <button
      class="nav-btn nav-btn--about"
      class:active={mode === "ABOUT"}
      on:click={() => {
        playSound("blip");
        mode = "ABOUT";
      }}
      on:mouseenter={() => playSound("blip")}
    >
      <span class="nav-icon">◈</span><span class="nav-label">ABOUT</span>
    </button>

    <button
      class="terminate-btn"
      on:click={resetAll}
      on:mouseenter={() => playSound("blip")}
      title="Terminate all connections"
    >
      ☠ TERMINATE
    </button>
  </aside>

  <!-- ── MAIN ───────────────────────────────────────────────────────────── -->
  <main class="main">
    <!-- RECEIVE mode -->
    {#if mode === "RECEIVE"}
      {#if connectionState !== "CONNECTED"}
        <section class="panel qr-panel">
          <div class="panel__header">
            <h2 class="panel__title">
              {#if connectionState === "WAITING"}
                <Typewriter text="// WAITING_FOR_UPLINK..." speed={30} />
              {:else if connectionState === "DISCONNECTED"}
                // SIGNAL_LOST
              {:else}
                // STANDBY
              {/if}
            </h2>
          </div>

          <div class="qr-stage">
            {#if qrImage}
              <div
                class="qr-wrap"
                class:qr-pulse={connectionState === "WAITING"}
              >
                <div class="qr-label-top">DATA_LINK</div>
                <img
                  src={qrImage}
                  alt="Connection QR Code"
                  class="qr-img"
                  draggable="false"
                />
                {#if connectionState === "WAITING"}
                  <div class="qr-scan-line" aria-hidden="true"></div>
                {/if}
              </div>
            {:else}
              <div class="qr-loading blink">GENERATING_LINK...</div>
            {/if}
          </div>

          <div class="qr-instructions">
            <div class="instr-row">
              <span class="instr-num">01</span><span
                >Connect both devices to the same Wi-Fi network</span
              >
            </div>
            <div class="instr-row">
              <span class="instr-num">02</span><span
                >Scan the QR code or open the URL on your phone</span
              >
            </div>
            <div class="instr-row">
              <span class="instr-num">03</span><span
                >Select files and tap Upload</span
              >
            </div>
          </div>

          {#if connectionState === "DISCONNECTED"}
            <button
              class="action-btn action-btn--cyan"
              on:click={handleDisconnectReset}
              on:mouseenter={() => playSound("blip")}
            >
              ↺ RECONNECT
            </button>
          {/if}

          {#if displayUrl}
            <div class="url-strip">
              <span class="url-strip__label">URL</span>
              <span class="url-strip__val">{displayUrl}</span>
              <button
                class="url-strip__copy"
                on:click={() => {
                  navigator.clipboard.writeText(displayUrl);
                  toast("Copied!", "info");
                }}>COPY</button
              >
            </div>
          {/if}
        </section>
      {:else}
        <!-- Connected dashboard -->
        <section class="panel connected-panel">
          <div class="panel__header">
            <h2 class="panel__title connected-title">// LINK_ESTABLISHED</h2>
            <div class="conn-badge">DEVICE ONLINE</div>
          </div>

          {#if progress.active}
            <div class="transfer-block">
              <div class="transfer-header">
                <span class="transfer-icon">📡</span>
                <div class="transfer-info">
                  <div class="transfer-name">{progress.filename}</div>
                  <div class="transfer-meta">
                    {progress.received} / {progress.total} · {progress.speed}
                  </div>
                </div>
                <div class="transfer-pct">{progress.percent}%</div>
              </div>
              <div class="transfer-bar-track">
                <div
                  class="transfer-bar-fill"
                  style="width:{progress.percent}%"
                ></div>
              </div>
              <div class="transfer-footer">
                <span>{progress.remaining}</span>
              </div>
            </div>
          {:else}
            <div class="ready-msg">
              <span class="ready-icon blink">▶</span>
              <span>READY — Waiting for incoming files…</span>
            </div>
          {/if}

          <div class="file-log">
            <div class="file-log__header">
              <span>📥 RECEIVED FILES</span>
              <span class="file-log__count"
                >{receivedFiles.length} file{receivedFiles.length !== 1
                  ? "s"
                  : ""}</span
              >
            </div>
            {#if sortedFiles.length > 0}
              <div class="file-list">
                {#each sortedFiles as file (file.name + file.modTime)}
                  <button class="file-row" on:click={() => openFile(file.name)}>
                    <span class="file-row__icon">{fileIcon(file.name)}</span>
                    <span class="file-row__name">{file.name}</span>
                    <span class="file-row__size"
                      >{formatSize(file.sizeBytes)}</span
                    >
                    <span class="file-row__time">{file.modTime}</span>
                    <span class="file-row__open">↗</span>
                  </button>
                {/each}
              </div>
            {:else}
              <div class="file-empty">
                No files yet — waiting for transmission…
              </div>
            {/if}
          </div>
        </section>
      {/if}

      <!-- SEND mode -->
    {:else if mode === "SEND"}
      <section class="panel send-panel">
        <div class="panel__header">
          <h2 class="panel__title send-title">// UPLINK_READY</h2>
        </div>
        <div
          class="send-drop-zone"
          on:click={startSend}
          role="button"
          tabindex="0"
          on:keydown={(e) => e.key === "Enter" && startSend()}
        >
          <div class="send-drop__icon">⬆</div>
          <div class="send-drop__primary">Click to select files</div>
          <div class="send-drop__secondary">or drag &amp; drop anywhere</div>
        </div>
        <div class="send-info-cards">
          <div class="info-card">
            <div class="info-card__icon">🔒</div>
            <div>
              <div class="info-card__title">Secure</div>
              <div class="info-card__desc">
                Token-authenticated local transfer
              </div>
            </div>
          </div>
          <div class="info-card">
            <div class="info-card__icon">⚡</div>
            <div>
              <div class="info-card__title">Fast</div>
              <div class="info-card__desc">Direct LAN — no cloud required</div>
            </div>
          </div>
          <div class="info-card">
            <div class="info-card__icon">📡</div>
            <div>
              <div class="info-card__title">Multi-file</div>
              <div class="info-card__desc">Send multiple files at once</div>
            </div>
          </div>
        </div>
      </section>

      <!-- ABOUT mode -->
    {:else if mode === "ABOUT"}
      <section class="panel about-panel">
        <!-- App block -->
        <div class="about-app">
          <img
            src={logoImg}
            alt="BeamSync"
            class="about-app__logo"
            draggable="false"
          />
          <div class="about-app__info">
            <div class="about-app__name">BEAMSYNC</div>
            <div class="about-app__version">v2.0.0</div>
            <p class="about-app__desc">
              Fast, token-secured file transfers over your local network. No
              cloud. No accounts. Just beam it.
            </p>
            <div class="about-app__chips">
              <span class="chip">🔒 LAN Only</span>
              <span class="chip">⚡ Zero Cloud</span>
              <span class="chip">📡 Real-time Progress</span>
              <span class="chip">🖥 Desktop + Mobile</span>
            </div>
          </div>
        </div>

        <div class="about-divider">
          <span class="about-divider__line"></span>
          <span class="about-divider__label">// DEVELOPER</span>
          <span class="about-divider__line"></span>
        </div>

        <!-- Developer block -->
        <div class="about-dev">
          <div class="about-dev__avatar-wrap">
            <img
              src={logoImg}
              alt="Pranav Agarkar"
              class="about-dev__avatar"
              draggable="false"
            />
            <div class="about-dev__avatar-ring"></div>
          </div>
          <div class="about-dev__info">
            <div class="about-dev__name">Pranav Agarkar</div>
            <div class="about-dev__tagline">
              // Building tools that make life simpler — one commit at a time
            </div>
            <div class="about-dev__links">
              <button
                class="link-btn link-btn--gh"
                on:click={() => openLink("https://github.com/PranavAgarkar07")}
                on:mouseenter={() => playSound("blip")}
              >
                <span>⌥</span> GitHub
              </button>
              <button
                class="link-btn link-btn--web"
                on:click={() =>
                  openLink(
                    "https://pranavagarkar07.github.io/portfolio-svelte/",
                  )}
                on:mouseenter={() => playSound("blip")}
              >
                <span>⬡</span> Portfolio
              </button>
            </div>
          </div>
        </div>

        <div class="about-footer">
          Built with Wails · Go · Svelte &nbsp;·&nbsp; © 2025 Pranav Agarkar
        </div>
      </section>
    {/if}
  </main>
</div>

<!-- Sender URL dialog -->
{#if showSenderDialog}
  <div class="dialog-backdrop" on:click|self={() => (showSenderDialog = false)}>
    <div class="dialog">
      <div class="dialog__corner tl"></div>
      <div class="dialog__corner tr"></div>
      <div class="dialog__corner bl"></div>
      <div class="dialog__corner br"></div>
      <button class="dialog__close" on:click={() => (showSenderDialog = false)}
        >✕</button
      >
      <h2 class="dialog__title">// PAYLOAD_READY</h2>
      <p class="dialog__sub">Scan on receiving device to download</p>
      {#if qrImage}
        <div class="dialog__qr-wrap">
          <img
            src={qrImage}
            alt="Sender QR Code"
            class="dialog__qr"
            draggable="false"
          />
        </div>
      {/if}
      <div class="dialog__url-row">
        <input
          class="dialog__url-input"
          type="text"
          value={senderUrl}
          readonly
        />
        <button
          class="dialog__copy-btn"
          on:click={() => {
            navigator.clipboard.writeText(senderUrl);
            toast("URL copied!", "success");
          }}>COPY</button
        >
      </div>
      <button
        class="action-btn action-btn--amber"
        on:click={() => (showSenderDialog = false)}>CLOSE</button
      >
    </div>
  </div>
{/if}