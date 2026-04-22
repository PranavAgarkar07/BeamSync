<!--
  FileDropZone.svelte
  ─────────────────────────────────────────────────────────────────
  Drag-and-drop area with file list, sizes, and per-file progress.

  Props:
    files    {Array<{name, sizeBytes, progress}>}
              — Reactive list of queued/uploading files.
              progress is 0–100 (number) or null if not started.
    accept   {string}  — Optional native file input accept attr.
    multiple {boolean} — Allow multi-file selection.

  Events:
    on:filesSelected — { files: FileList } when user picks files
    on:dropped       — { files: FileList } on drag-drop

  Usage:
    <FileDropZone
      files={fileList}
      multiple
      on:filesSelected={handleFiles}
      on:dropped={handleDrop}
    />
-->
<script>
  import { createEventDispatcher } from "svelte";
  import { fly, fade } from "svelte/transition";
  import { cubicOut, backOut } from "svelte/easing";

  export let files = [];
  export let accept = "*/*";
  export let multiple = true;

  const dispatch = createEventDispatcher();

  let isDragging = false;
  let inputEl;

  // ── Stagger config ────────────────────────────────────────────
  const STAGGER_MS   = 55;   // delay per item (ms)
  const ENTER_MS     = 260;  // entry animation duration
  const ENTER_Y      = 18;   // upward travel (px)

  function formatBytes(bytes) {
    if (!bytes && bytes !== 0) return "—";
    if (bytes >= 1_073_741_824)
      return (bytes / 1_073_741_824).toFixed(2) + " GB";
    if (bytes >= 1_048_576) return (bytes / 1_048_576).toFixed(1) + " MB";
    if (bytes >= 1_024) return (bytes / 1_024).toFixed(0) + " KB";
    return bytes + " B";
  }

  // Icon map — SVG strings keyed by extension group
  function fileIcon(name = "") {
    const ext = name.split(".").pop().toLowerCase();
    const groups = {
      video:    new Set(["mp4","mov","mkv","avi","webm","hevc","m4v"]),
      audio:    new Set(["mp3","wav","flac","aac","ogg","m4a"]),
      image:    new Set(["jpg","jpeg","png","gif","webp","svg","bmp","tiff","ico"]),
      pdf:      new Set(["pdf"]),
      archive:  new Set(["zip","tar","gz","rar","7z","bz2","xz"]),
      doc:      new Set(["doc","docx","txt","md","rtf","odt","pages"]),
      sheet:    new Set(["xls","xlsx","csv","ods","numbers"]),
      code:     new Set(["js","ts","svelte","go","py","rs","java","kt","swift","cpp","c","css","html"]),
      apk:      new Set(["apk"]),
    };
    if (groups.video.has(ext))   return icons.video;
    if (groups.audio.has(ext))   return icons.audio;
    if (groups.image.has(ext))   return icons.image;
    if (groups.pdf.has(ext))     return icons.pdf;
    if (groups.archive.has(ext)) return icons.archive;
    if (groups.doc.has(ext))     return icons.doc;
    if (groups.sheet.has(ext))   return icons.sheet;
    if (groups.code.has(ext))    return icons.code;
    if (groups.apk.has(ext))     return icons.apk;
    return icons.file;
  }

  function iconColor(name = "") {
    const ext = name.split(".").pop().toLowerCase();
    const colors = {
      mp4:"#e85d75", mov:"#e85d75", mkv:"#e85d75", avi:"#e85d75", webm:"#e85d75",
      mp3:"#a78bfa", wav:"#a78bfa", flac:"#a78bfa", aac:"#a78bfa",
      jpg:"#38bdf8", jpeg:"#38bdf8", png:"#38bdf8", gif:"#38bdf8", webp:"#38bdf8",
      svg:"#38bdf8",
      pdf:"#f87171",
      zip:"#fb923c", tar:"#fb923c", gz:"#fb923c", rar:"#fb923c", "7z":"#fb923c",
      txt:"#94a3b8", md:"#94a3b8",
      xls:"#4ade80", xlsx:"#4ade80", csv:"#4ade80",
      js:"#fbbf24", ts:"#60a5fa", svelte:"#ff6e3c", go:"#00ADD8", py:"#4ade80",
      apk:"#34d399",
    };
    return colors[ext] || "var(--nb-text-muted)";
  }

  // ── SVG icons (inline, no external deps) ─────────────────────
  const _base = `width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="square"`;
  const icons = {
    file:    `<svg ${_base}><path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/><polyline points="13 2 13 9 20 9"/></svg>`,
    video:   `<svg ${_base}><polygon points="23 7 16 12 23 17 23 7"/><rect x="1" y="5" width="15" height="14" rx="1" ry="1"/></svg>`,
    audio:   `<svg ${_base}><path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/></svg>`,
    image:   `<svg ${_base}><rect x="3" y="3" width="18" height="18" rx="1"/><circle cx="8.5" cy="8.5" r="1.5"/><polyline points="21 15 16 10 5 21"/></svg>`,
    pdf:     `<svg ${_base}><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><polyline points="10 9 9 9 8 9"/></svg>`,
    archive: `<svg ${_base}><polyline points="21 8 21 21 3 21 3 8"/><rect x="1" y="3" width="22" height="5"/><line x1="10" y1="12" x2="14" y2="12"/></svg>`,
    doc:     `<svg ${_base}><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><line x1="10" y1="9" x2="8" y2="9"/></svg>`,
    sheet:   `<svg ${_base}><rect x="3" y="3" width="18" height="18" rx="1"/><path d="M3 9h18M3 15h18M9 3v18"/></svg>`,
    code:    `<svg ${_base}><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg>`,
    apk:     `<svg ${_base}><rect x="5" y="2" width="14" height="20" rx="2" ry="2"/><line x1="12" y1="18" x2="12.01" y2="18"/></svg>`,
    upload:  `<svg xmlns="http://www.w3.org/2000/svg" width="36" height="36" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="square" stroke-linejoin="miter" aria-hidden="true"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg>`,
  };

  function onDragEnter(e) { e.preventDefault(); isDragging = true; }
  function onDragOver(e)  { e.preventDefault(); isDragging = true; }
  function onDragLeave(e) { if (!e.currentTarget.contains(e.relatedTarget)) isDragging = false; }
  function onDrop(e) {
    e.preventDefault();
    isDragging = false;
    const dropped = e.dataTransfer?.files;
    if (dropped?.length) dispatch("dropped", { files: dropped });
  }
  function onInputChange(e) {
    const picked = e.target.files;
    if (picked?.length) dispatch("filesSelected", { files: picked });
  }
  function openPicker() { dispatch("requestPicker"); }

  // Total size summary
  $: totalBytes = files.reduce((s, f) => s + (f.sizeBytes || 0), 0);
</script>

<!-- Hidden file input (Retained exclusively for web browser fallbacks) -->
<input
  bind:this={inputEl}
  type="file"
  {accept}
  {multiple}
  class="visually-hidden"
  aria-hidden="true"
  tabindex="-1"
  on:change={onInputChange}
/>

<div class="dropzone-wrapper">
  <!-- Drop area -->
  <button
    class="drop-zone"
    class:drop-zone--active={isDragging}
    class:drop-zone--compact={files.length > 0}
    on:click={openPicker}
    on:dragenter={onDragEnter}
    on:dragover={onDragOver}
    on:dragleave={onDragLeave}
    on:drop={onDrop}
    aria-label="Click to select files or drag and drop here"
  >
    <span class="drop-zone__icon" class:drop-zone__icon--lift={isDragging}>
      {@html icons.upload}
    </span>
    <div class="drop-zone__text">
      <span class="drop-zone__primary"
        >{isDragging ? "Drop files here" : files.length > 0 ? "Click to change selection" : "Click or drag files here"}</span
      >
      <span class="drop-zone__secondary"
        >{files.length > 0
          ? `${files.length} file${files.length > 1 ? "s" : ""} · ${formatBytes(totalBytes)} total`
          : "Any file type · No size limit"}</span
      >
    </div>
  </button>

  <!-- ── Staggered file list ──────────────────────────────────── -->
  {#if files.length > 0}
    <div class="file-list" role="list" aria-label="Selected files">

      <!-- Section header with count badge -->
      <div class="file-list__header" in:fly={{ y: -10, duration: 180, easing: cubicOut }}>
        <span class="file-list__label">SELECTED FILES</span>
        <span class="file-list__badge">{files.length}</span>
      </div>

      <!-- One card per file, staggered -->
      {#each files as file, i (file.name + i)}
        <div
          class="file-row"
          role="listitem"
          in:fly={{
            y:        ENTER_Y,
            x:        -8,
            duration: ENTER_MS,
            delay:    i * STAGGER_MS,
            easing:   backOut,
          }}
          out:fade={{ duration: 120 }}
        >
          <!-- Coloured icon -->
          <span class="file-row__icon" style="color: {iconColor(file.name)}" aria-hidden="true">
            {@html fileIcon(file.name)}
          </span>

          <div class="file-row__body">
            <div class="file-row__meta">
              <span class="file-row__name">{file.name}</span>
              <span class="file-row__size nb-mono">{formatBytes(file.sizeBytes)}</span>
            </div>

            <!-- Progress bar (only shown if progress is not null) -->
            {#if file.progress != null}
              <div
                class="progress-track"
                role="progressbar"
                aria-valuenow={file.progress}
                aria-valuemin="0"
                aria-valuemax="100"
              >
                <div
                  class="progress-fill"
                  class:progress-fill--complete={file.progress >= 100}
                  style="width: {Math.min(100, file.progress)}%"
                ></div>
              </div>
              <span class="progress-label nb-mono"
                >{file.progress >= 100 ? "DONE" : `${Math.round(file.progress)}%`}</span
              >
            {/if}
          </div>

          <!-- Status indicator dot -->
          <span
            class="file-row__dot"
            class:file-row__dot--active={file.progress != null && file.progress < 100}
            class:file-row__dot--done={file.progress != null && file.progress >= 100}
          ></span>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .visually-hidden {
    position: absolute;
    width: 1px; height: 1px;
    padding: 0; margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }

  /* ── Wrapper ──────────────────────────────────────────────── */
  .dropzone-wrapper {
    display: flex;
    flex-direction: column;
    gap: var(--nb-space-4);
    width: 100%;
  }

  /* ── Drop zone ────────────────────────────────────────────── */
  .drop-zone {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--nb-space-4);
    width: 100%;
    min-height: 200px;
    padding: var(--nb-space-8) var(--nb-space-6);
    background: var(--nb-bg);
    border: var(--nb-border-lg);
    box-shadow: 6px 6px 0px var(--nb-shadow-color);
    cursor: pointer;
    font-family: var(--nb-font-display);
    transition: transform 120ms cubic-bezier(0.2, 0, 0, 1),
                box-shadow 120ms cubic-bezier(0.2, 0, 0, 1),
                background 120ms;
  }

  /* Compact when files are loaded — less vertical space */
  .drop-zone--compact {
    min-height: 100px;
    padding: var(--nb-space-5) var(--nb-space-6);
  }

  .drop-zone:hover,
  .drop-zone--active {
    background: var(--nb-secondary);
    transform: translate(-3px, -3px);
    box-shadow: 10px 10px 0px var(--nb-shadow-color);
  }

  .drop-zone:hover .drop-zone__icon,
  .drop-zone--active .drop-zone__icon,
  .drop-zone:hover .drop-zone__primary,
  .drop-zone--active .drop-zone__primary,
  .drop-zone:hover .drop-zone__secondary,
  .drop-zone--active .drop-zone__secondary {
    color: var(--nb-secondary-text, #0a0a0a);
  }

  .drop-zone:active {
    transform: translate(4px, 4px);
    box-shadow: 2px 2px 0px var(--nb-shadow-color);
  }

  .drop-zone__icon {
    display: flex;
    color: var(--nb-text);
    transition: transform 200ms ease;
  }

  .drop-zone__icon--lift { transform: translateY(-6px); }

  .drop-zone__text {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--nb-space-1);
    text-align: center;
  }

  .drop-zone__primary {
    font-size: var(--nb-text-lg);
    font-weight: var(--nb-fw-bold);
    color: var(--nb-text);
    letter-spacing: -0.01em;
  }

  .drop-zone__secondary {
    font-size: var(--nb-text-sm);
    color: var(--nb-text-muted);
    font-weight: var(--nb-fw-medium);
  }

  /* ── File list container ──────────────────────────────────── */
  .file-list {
    display: flex;
    flex-direction: column;
    gap: 0;
    border: var(--nb-border-lg);
    box-shadow: 4px 4px 0px var(--nb-shadow-color);
    overflow: hidden;
  }

  /* Section header */
  .file-list__header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 7px 14px;
    background: var(--nb-primary);
    border-bottom: 2px solid var(--nb-border-color);
  }

  .file-list__label {
    font-family: var(--nb-font-display);
    font-size: 11px;
    font-weight: 800;
    letter-spacing: 0.07em;
    color: #ffffff;
    text-transform: uppercase;
  }

  .file-list__badge {
    font-family: var(--nb-font-display);
    font-size: 11px;
    font-weight: 800;
    color: #ffffff;
    background: rgba(255,255,255,0.25);
    padding: 0px 7px;
    letter-spacing: 0.04em;
  }

  /* ── File row ─────────────────────────────────────────────── */
  .file-row {
    display: flex;
    align-items: center;
    gap: var(--nb-space-3);
    padding: var(--nb-space-3) var(--nb-space-4);
    background: var(--nb-surface);
    border-bottom: 2px solid var(--nb-border-color);
    /* will-change reserved for animated elements */
    will-change: transform, opacity;
    transition: background 160ms ease;
  }

  .file-row:last-child { border-bottom: none; }

  .file-row:hover {
    background: color-mix(in srgb, var(--nb-primary) 6%, var(--nb-surface));
  }

  .file-row__icon {
    display: flex;
    align-items: center;
    flex-shrink: 0;
    margin-top: 0;
    /* Icon colour comes from the inline style property */
    filter: drop-shadow(0 0 6px currentColor);
    opacity: 0.85;
    transition: opacity 160ms, filter 160ms;
  }

  .file-row:hover .file-row__icon {
    opacity: 1;
    filter: drop-shadow(0 0 10px currentColor);
  }

  .file-row__body {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: var(--nb-space-2);
  }

  .file-row__meta {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: var(--nb-space-3);
    flex-wrap: wrap;
  }

  .file-row__name {
    font-family: var(--nb-font-display);
    font-weight: var(--nb-fw-semibold);
    font-size: var(--nb-text-sm);
    color: var(--nb-text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    flex: 1;
    min-width: 0;
  }

  .file-row__size {
    font-size: var(--nb-text-xs);
    color: var(--nb-text-muted);
    flex-shrink: 0;
  }

  /* Status dot */
  .file-row__dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
    background: var(--nb-border-color);
    transition: background 300ms;
  }

  .file-row__dot--active {
    background: var(--nb-primary);
    animation: dot-pulse 1s ease-in-out infinite;
  }

  .file-row__dot--done {
    background: var(--nb-success, #00B87C);
    animation: none;
  }

  @keyframes dot-pulse {
    0%, 100% { opacity: 1; }
    50%       { opacity: 0.3; }
  }

  /* ── Progress bar ─────────────────────────────────────────── */
  .progress-track {
    width: 100%;
    height: 6px;
    background: var(--nb-bg);
    border: 2px solid var(--nb-border-color);
    overflow: hidden;
  }

  .progress-fill {
    height: 100%;
    background: var(--nb-primary);
    transition: width 200ms cubic-bezier(0.4, 0, 0.2, 1);
  }

  .progress-fill--complete { background: var(--nb-success, #00B87C); }

  .progress-label {
    font-size: 10px;
    color: var(--nb-text-muted);
    font-weight: var(--nb-fw-bold);
  }

  .nb-mono { font-family: var(--nb-font-mono); }

  /* ── Accessibility: honour reduced-motion ─────────────────── */
  @media (prefers-reduced-motion: reduce) {
    .drop-zone,
    .progress-fill,
    .file-row__icon,
    .file-row {
      transition: none !important;
      animation: none !important;
    }
    .drop-zone:hover,
    .drop-zone--active {
      transform: none;
    }
    .drop-zone__icon--lift { transform: none; }
  }

  /* ── Responsive ───────────────────────────────────────────── */
  @media (max-width: 480px) {
    .drop-zone {
      min-height: 150px;
      padding: var(--nb-space-6) var(--nb-space-4);
    }
    .drop-zone--compact { min-height: 80px; }
  }
</style>
