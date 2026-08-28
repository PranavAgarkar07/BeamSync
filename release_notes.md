## What's New in v2.5.0 🚀

* **Complete Mobile UI Overhaul**: Radically redesigned the mobile web UI with a more structured layout, making it much easier to track which files are queued, uploading, and completed.
* **Intelligent File Lists**: Selected files now automatically cascade down in a staggered entrance animation on the desktop. The mobile download view now features a dedicated "DOWNLOADED FILES" section that dynamically populates as downloads complete!
* **Rich iconography**: Added dynamic, color-coded SVG icons tailored to dozens of different file types including code, video, images, scripts, text, and zip formats.
* **Enhanced File Metadata Handling**: Crucial fixes to the backend header mechanisms! The correct, original filename is now properly extracted using a newly passed `X-Filename` HTTP header. 
* **Accessibility**: Fully incorporated `prefers-reduced-motion` compliance ensuring sleek usability across heavily animated elements.
* **Secure Token Rotation & Big-File Stability (v2.5.0)**: QR bootstrap credentials now auto-rotate every ~4.5 minutes and the desktop QR/URL updates instantly via `url_changed`/`token_rotated` events — no manual refresh needed. Previous QRs are revoked immediately. Download (transfer) tokens now live 30 minutes (vs 5) so large files >5 min no longer fail while queued; active transfers defer rotation and keep the stream alive via `activeUploads` watchdog guard (`beamsync/server.go:600`). Mobile download heartbeat no longer treats a mid-download 401/403 as fatal.

These UX and architecture refinements bring BeamSync significantly closer to its vision of frictionless and beautiful local file transfers.
