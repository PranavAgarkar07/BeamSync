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

  export let files = [];
  export let accept = "*/*";
  export let multiple = true;

  const dispatch = createEventDispatcher();

  let isDragging = false;
  let inputEl;

  function formatBytes(bytes) {
    if (!bytes && bytes !== 0) return "—";
    if (bytes >= 1_073_741_824)
      return (bytes / 1_073_741_824).toFixed(2) + " GB";
    if (bytes >= 1_048_576) return (bytes / 1_048_576).toFixed(1) + " MB";
    if (bytes >= 1_024) return (bytes / 1_024).toFixed(0) + " KB";
    return bytes + " B";
  }

  function onDragEnter(e) {
    e.preventDefault();
    isDragging = true;
  }
  function onDragOver(e) {
    e.preventDefault();
    isDragging = true;
  }
  function onDragLeave(e) {
    if (!e.currentTarget.contains(e.relatedTarget)) isDragging = false;
  }
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
  function openPicker() {
    dispatch("requestPicker");
  }

  // Lucide icons (inline SVG)
  const iconUpload = `<svg xmlns="http://www.w3.org/2000/svg" width="36" height="36" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="square" stroke-linejoin="miter" aria-hidden="true"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg>`;
  const iconFile = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="square" aria-hidden="true"><path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/><polyline points="13 2 13 9 20 9"/></svg>`;
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
    on:click={openPicker}
    on:dragenter={onDragEnter}
    on:dragover={onDragOver}
    on:dragleave={onDragLeave}
    on:drop={onDrop}
    aria-label="Click to select files or drag and drop here"
  >
    <span class="drop-zone__icon" class:drop-zone__icon--lift={isDragging}>
      {@html iconUpload}
    </span>
    <div class="drop-zone__text">
      <span class="drop-zone__primary"
        >{isDragging ? "Drop files here" : "Click or drag files here"}</span
      >
      <span class="drop-zone__secondary">Any file type · No size limit</span>
    </div>
  </button>

  <!-- File list -->
  {#if files.length > 0}
    <div class="file-list" role="list" aria-label="Selected files">
      {#each files as file, i (file.name + i)}
        <div class="file-row" role="listitem">
          <!-- Icon + name -->
          <span class="file-row__icon" aria-hidden="true">{@html iconFile}</span
          >
          <div class="file-row__body">
            <div class="file-row__meta">
              <span class="file-row__name">{file.name}</span>
              <span class="file-row__size nb-mono"
                >{formatBytes(file.sizeBytes)}</span
              >
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
                >{file.progress >= 100
                  ? "DONE"
                  : `${Math.round(file.progress)}%`}</span
              >
            {/if}
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .visually-hidden {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }

  .dropzone-wrapper {
    display: flex;
    flex-direction: column;
    gap: var(--nb-space-4);
    width: 100%;
  }

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
    transition:
      transform 120ms,
      box-shadow 120ms;
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

  .drop-zone__icon--lift {
    transform: translateY(-6px);
  }

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

  /* File list */
  .file-list {
    display: flex;
    flex-direction: column;
    gap: 0;
    border: var(--nb-border-lg);
    box-shadow: var(--nb-shadow-sm);
  }

  .file-row {
    display: flex;
    align-items: flex-start;
    gap: var(--nb-space-3);
    padding: var(--nb-space-3) var(--nb-space-4);
    background: var(--nb-surface);
    border-bottom: 2px solid var(--nb-border-color);
  }

  .file-row:last-child {
    border-bottom: none;
  }

  .file-row__icon {
    display: flex;
    align-items: center;
    color: var(--nb-text-muted);
    margin-top: 2px;
    flex-shrink: 0;
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

  /* Progress bar */
  .progress-track {
    width: 100%;
    height: 8px;
    background: var(--nb-bg);
    border: 2px solid var(--nb-border-color);
    overflow: hidden;
  }

  .progress-fill {
    height: 100%;
    background: var(--nb-primary);
    transition: width 200ms ease-out;
  }

  .progress-fill--complete {
    background: var(--nb-success);
  }

  .progress-label {
    font-size: 10px;
    color: var(--nb-text-muted);
    font-weight: var(--nb-fw-bold);
  }

  .nb-mono {
    font-family: var(--nb-font-mono);
  }

  @media (prefers-reduced-motion: reduce) {
    .drop-zone,
    .progress-fill {
      transition: none;
    }
    .drop-zone:hover,
    .drop-zone--active {
      transform: none;
    }
    .drop-zone__icon--lift {
      transform: none;
    }
  }

  @media (max-width: 480px) {
    .drop-zone {
      min-height: 150px;
      padding: var(--nb-space-6) var(--nb-space-4);
    }
  }
</style>
