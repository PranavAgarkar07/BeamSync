// desktop/frontend/src/lib/utils.js
// Pure presentation helpers extracted verbatim from App.svelte (Phase 5, Plan 05-03).
//
// Rules for this module:
//   - No Svelte imports, no Wails imports, no DOM access — pure functions only.
//   - No behavior change: every function body below is byte-identical to the
//     App.svelte version it replaced, except normalizeTransferStats which takes
//     the component's previous `startedAt` as an explicit argument instead of
//     closing over App state (same output, now pure).
//   - toast() intentionally stays in App.svelte — its state ownership is
//     UI-SPEC Open Question #1 and is out of scope for this plan.

/**
 * Format a millisecond duration as a short human string.
 * App.svelte original lines 242-247.
 * @param {number} [ms]
 * @returns {string} "<1s" | "${s}s" | "${m}m ${s}s"
 */
export function formatDuration(ms = 0) {
  if (!ms || ms < 1000) return "<1s";
  const seconds = Math.round(ms / 1000);
  if (seconds < 60) return `${seconds}s`;
  return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
}

/**
 * Format a byte count as B / KB / MB.
 * App.svelte original lines 643-648.
 * @param {number} bytes
 * @returns {string} "—" | "${B} B" | "${KB} KB" | "${MB} MB"
 */
export function formatSize(bytes) {
  if (!bytes) return "—";
  if (bytes >= 1048576) return (bytes / 1048576).toFixed(1) + " MB";
  if (bytes >= 1024) return (bytes / 1024).toFixed(0) + " KB";
  return bytes + " B";
}

/**
 * Pick an emoji for a filename based on its extension.
 * App.svelte original lines 650-682.
 * @param {string} [name]
 * @returns {string} emoji, default "📁"
 */
export function fileIcon(name = "") {
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

/**
 * Validate a dotted-quad IPv4 address string.
 * App.svelte original lines 759-766 — logic unchanged.
 * @param {string} ip
 * @returns {boolean}
 */
export function isValidIPv4(ip) {
  const parts = ip.split(".");
  return parts.length === 4 && parts.every((part) => {
    if (!/^\d+$/.test(part)) return false;
    const value = Number(part);
    return value >= 0 && value <= 255 && String(value) === String(Number(part));
  });
}

/**
 * Normalize an incoming transfer-stats payload into the full TransferStats shape.
 * App.svelte original lines 194-206. The original closed over the component's
 * `transferStats.startedAt` as the fallback for a missing payload `startedAt`;
 * that fallback is now an explicit argument so this function stays pure.
 * @param {object} nextStats
 * @param {string} [fallbackStartedAt] previous transferStats.startedAt
 * @returns {object} normalized TransferStats
 */
export function normalizeTransferStats(nextStats, fallbackStartedAt = "") {
  return {
    startedAt: nextStats.startedAt || fallbackStartedAt,
    filesReceived: Number(nextStats.filesReceived) || 0,
    bytesReceived: Number(nextStats.bytesReceived) || 0,
    filesSent: Number(nextStats.filesSent) || 0,
    bytesSent: Number(nextStats.bytesSent) || 0,
    activeUploads: Number(nextStats.activeUploads) || 0,
    activeDownloads: Number(nextStats.activeDownloads) || 0,
    lastFilename: nextStats.lastFilename || "",
    lastDirection: nextStats.lastDirection || "",
  };
}
