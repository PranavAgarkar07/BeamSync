<script>
  export let transferHistory = [];
  export let sessionLog = [];
  export let formatSize;
  export let formatDuration;
  export let formatTransferTime;
</script>

<div class="activity-panel">
  <div class="activity-column">
    <div class="activity-header">
      <h3>TRANSFER HISTORY</h3>
      <span>{transferHistory.length}</span>
    </div>
    <div class="activity-list" class:empty={transferHistory.length === 0}>
      {#if transferHistory.length === 0}
        <p>No transfers recorded in this session</p>
      {/if}
      {#each transferHistory as item (item.id)}
        <div class="activity-item activity-item--{item.status}">
          <span class="activity-item__name">{item.filename}</span>
          <span class="activity-item__meta">
            {item.direction === "send" ? "SENT" : "RECEIVED"} · {formatSize(item.sizeBytes)} · {formatDuration(item.durationMillis)}
          </span>
          <span class="activity-item__time">{formatTransferTime(item.completedAt)}</span>
        </div>
      {/each}
    </div>
  </div>

  <div class="activity-column">
    <div class="activity-header">
      <h3>SESSION LOG</h3>
      <span>{sessionLog.length}</span>
    </div>
    <div class="activity-list" class:empty={sessionLog.length === 0}>
      {#if sessionLog.length === 0}
        <p>Session events will appear here</p>
      {/if}
      {#each sessionLog as item (item.id)}
        <div class="session-item session-item--{item.type}">
          <span class="session-item__title">{item.title}</span>
          <span class="session-item__detail">{item.detail}</span>
          <span class="session-item__time">{item.time}</span>
        </div>
      {/each}
    </div>
  </div>
</div>

<style>
  .activity-panel {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: var(--nb-space-4);
    width: 100%;
  }

  .activity-column {
    background: var(--nb-surface);
    border: var(--nb-border-lg);
    box-shadow: var(--nb-shadow-md);
    min-width: 0;
  }

  .activity-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--nb-space-3);
    background: var(--nb-bg);
    border-bottom: var(--nb-border-lg);
    padding: var(--nb-space-3) var(--nb-space-4);
  }

  .activity-header h3 {
    margin: 0;
    font-family: var(--nb-font-display);
    font-size: var(--nb-text-sm);
    font-weight: 800;
    letter-spacing: 0.05em;
  }

  .activity-header span {
    font-family: var(--nb-font-mono);
    font-size: var(--nb-text-xs);
    font-weight: 800;
    background: var(--nb-secondary);
    color: var(--nb-secondary-text);
    border: var(--nb-border-md);
    padding: 2px 8px;
  }

  .activity-list {
    max-height: 220px;
    overflow-y: auto;
  }

  .activity-list.empty {
    padding: var(--nb-space-4);
  }

  .activity-list.empty p {
    margin: 0;
    color: var(--nb-text-muted);
    font-family: var(--nb-font-mono);
    font-size: var(--nb-text-xs);
    font-weight: 700;
    text-align: center;
  }

  .activity-item,
  .session-item {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 4px var(--nb-space-3);
    padding: var(--nb-space-3) var(--nb-space-4);
    border-bottom: 1px solid var(--nb-border-color);
    min-width: 0;
  }

  .activity-item--failed,
  .session-item--error {
    background: rgba(255, 107, 107, 0.12);
  }

  .session-item--warn {
    background: rgba(255, 176, 0, 0.12);
  }

  .activity-item__name,
  .session-item__title {
    font-family: var(--nb-font-plex);
    font-weight: 700;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .activity-item__meta,
  .session-item__detail {
    grid-column: 1 / -1;
    font-family: var(--nb-font-mono);
    font-size: var(--nb-text-xs);
    color: var(--nb-text-muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .activity-item__time,
  .session-item__time {
    font-family: var(--nb-font-mono);
    font-size: var(--nb-text-xs);
    font-weight: 800;
    color: var(--nb-text-muted);
  }

  @media (max-width: 760px) {
    .activity-panel {
      grid-template-columns: 1fr;
    }
  }
</style>
