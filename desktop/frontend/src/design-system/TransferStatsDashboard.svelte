<script>
  export let stats = {
    startedAt: "",
    filesReceived: 0,
    bytesReceived: 0,
    activeUploads: 0,
    lastFilename: "",
  };
  export let now = Date.now();

  $: startedAtMs = stats.startedAt ? new Date(stats.startedAt).getTime() : now;
  $: elapsedMs = Number.isNaN(startedAtMs) ? 0 : Math.max(0, now - startedAtMs);
  $: sessionDuration = formatSessionDuration(elapsedMs);
  $: filesLabel = `${stats.filesReceived || 0} ${(stats.filesReceived || 0) === 1 ? "file" : "files"}`;
  $: activeLabel = stats.activeUploads > 0 ? `${stats.activeUploads} uploading` : "Idle";

  function formatSessionDuration(ms) {
    const seconds = Math.floor(ms / 1000);
    if (seconds < 60) return `${seconds}s`;
    const minutes = Math.floor(seconds / 60);
    const remainingSeconds = seconds % 60;
    if (minutes < 60) return `${minutes}m ${remainingSeconds}s`;
    const hours = Math.floor(minutes / 60);
    return `${hours}h ${minutes % 60}m`;
  }

  function formatBytes(bytes = 0) {
    if (bytes >= 1073741824) return `${(bytes / 1073741824).toFixed(2)} GB`;
    if (bytes >= 1048576) return `${(bytes / 1048576).toFixed(1)} MB`;
    if (bytes >= 1024) return `${(bytes / 1024).toFixed(0)} KB`;
    return `${bytes} B`;
  }
</script>

<section class="stats-dashboard" aria-label="Transfer statistics">
  <div class="stats-header">
    <h3>TRANSFER STATS</h3>
    <span class:active={stats.activeUploads > 0}>{activeLabel}</span>
  </div>

  <div class="stats-grid">
    <div class="stat-cell">
      <span class="stat-label">Files received</span>
      <strong>{filesLabel}</strong>
    </div>
    <div class="stat-cell">
      <span class="stat-label">Data transferred</span>
      <strong>{formatBytes(stats.bytesReceived || 0)}</strong>
    </div>
    <div class="stat-cell">
      <span class="stat-label">Session duration</span>
      <strong>{sessionDuration}</strong>
    </div>
    <div class="stat-cell stat-cell--wide">
      <span class="stat-label">Last file</span>
      <strong>{stats.lastFilename || "Waiting for files"}</strong>
    </div>
  </div>
</section>

<style>
  .stats-dashboard {
    width: 100%;
    background: var(--nb-surface);
    border: var(--nb-border-lg);
    box-shadow: var(--nb-shadow-md);
  }

  .stats-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--nb-space-3);
    background: var(--nb-bg);
    border-bottom: var(--nb-border-lg);
    padding: var(--nb-space-3) var(--nb-space-4);
  }

  .stats-header h3 {
    margin: 0;
    font-family: var(--nb-font-display);
    font-size: var(--nb-text-sm);
    font-weight: 800;
    letter-spacing: 0.05em;
  }

  .stats-header span {
    font-family: var(--nb-font-mono);
    font-size: var(--nb-text-xs);
    font-weight: 800;
    background: var(--nb-bg);
    border: var(--nb-border-md);
    padding: 2px 8px;
  }

  .stats-header span.active {
    background: var(--nb-secondary);
    color: var(--nb-secondary-text);
  }

  .stats-grid {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .stat-cell {
    min-width: 0;
    padding: var(--nb-space-4);
    border-right: 1px solid var(--nb-border-color);
  }

  .stat-cell:last-child {
    border-right: 0;
  }

  .stat-label {
    display: block;
    margin-bottom: var(--nb-space-2);
    color: var(--nb-text-muted);
    font-family: var(--nb-font-mono);
    font-size: var(--nb-text-xs);
    font-weight: 700;
    text-transform: uppercase;
  }

  .stat-cell strong {
    display: block;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--nb-font-display);
    font-size: var(--nb-text-lg);
    font-weight: 800;
  }

  @media (max-width: 760px) {
    .stats-grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .stat-cell:nth-child(2n) {
      border-right: 0;
    }

    .stat-cell:nth-child(-n + 2) {
      border-bottom: 1px solid var(--nb-border-color);
    }

    .stat-cell--wide {
      grid-column: 1 / -1;
      border-top: 1px solid var(--nb-border-color);
    }
  }
</style>
