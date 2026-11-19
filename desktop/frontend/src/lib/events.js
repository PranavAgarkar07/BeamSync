// desktop/frontend/src/lib/events.js
// Event name constants shared between the Svelte frontend and the Go backend.
//
// Every value must match an event emitted somewhere in beamsync/ or
// desktop/app.go. Cross-reference gate (Plan 05-02 Task 2.2): no orphaned
// names — each constant below must appear in the Go backend, otherwise the
// frontend would listen for events that never fire (silent failures).
//
// Go homes (verified against beamsync/ + desktop/app.go):
//   beamsync/events.go:        device_disconnected, url_changed, token_rotated,
//                             transfer_logged
//   beamsync/server.go:        device_connected, transfer_request
//   beamsync/upload_handler.go: file_received, transfer_stats, upload_progress
//   beamsync/download_handler.go: transfer_stats, download_progress
//   desktop/app.go:            sender_started, sender_files, update_available,
//                             server_error
//
// Components import event names from here; no inline event-string literals.

export const EVT_DEVICE_CONNECTED = "device_connected";
export const EVT_DEVICE_DISCONNECTED = "device_disconnected";
export const EVT_TRANSFER_REQUEST = "transfer_request";
export const EVT_FILE_RECEIVED = "file_received";
export const EVT_TRANSFER_LOGGED = "transfer_logged";
export const EVT_TRANSFER_STATS = "transfer_stats";
export const EVT_UPLOAD_PROGRESS = "upload_progress";
export const EVT_DOWNLOAD_PROGRESS = "download_progress";
export const EVT_URL_CHANGED = "url_changed";
export const EVT_TOKEN_ROTATED = "token_rotated";
export const EVT_SENDER_STARTED = "sender_started";
export const EVT_SENDER_FILES = "sender_files";
export const EVT_UPDATE_AVAILABLE = "update_available";
export const EVT_SERVER_ERROR = "server_error";
