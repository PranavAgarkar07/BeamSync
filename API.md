# BeamSync Internal HTTP API Documentation

This document describes the internal peer-to-peer HTTP API for **BeamSync**. 

When you launch BeamSync in either **Receiver (default)** or **Sender** mode, it starts a local HTTP server on a dynamically assigned port. Devices connect to this server over the local area network (LAN) to transfer files.

Set `BEAMSYNC_ENABLE_TLS=true` before launching BeamSync to serve the same endpoints over HTTPS. BeamSync stores a self-signed ECDSA certificate and private key at `~/.config/beamsync/cert.pem` and `~/.config/beamsync/key.pem`, generating them only when they do not already exist; clients may need to accept the browser warning for that local certificate.

---

## 🔒 Authentication & Token Scheme

To prevent copied URLs from granting another device access, BeamSync uses short-lived HMAC credentials:

1. **QR bootstrap:** Receiver and sender QR URLs contain a five-minute, single-use bootstrap credential. Loading the URL exchanges it for a session token bound to the connecting client IP. The bootstrap credential **auto-rotates every ~4.5 minutes** (or when the network changes); the desktop QR and URL update instantly via `url_changed` / `token_rotated` events. The previous bootstrap is revoked immediately — stale QRs return `403`.
2. **HMAC binding:** Tokens are signed with a per-server secret over the server session, TLS certificate fingerprint (or the explicit plaintext-HTTP context), client IP, scope, expiration, and a random nonce.
3. **Expiry and renewal:** Bootstrap and session credentials expire after **five minutes**; **transfer credentials expire after 30 minutes** to allow large-file downloads without re-scanning. Successful heartbeats return a replacement session token in the `X-BeamSync-Token` response header, which the bundled web clients adopt automatically. While a file transfer is active (`activeUploads`/`activeDownloads` > 0) the watchdog does not disconnect and the active stream is allowed to complete even if the pre-transfer token would otherwise expire; bootstrap rotation is deferred until the transfer finishes.
4. **Transfer scope:** Download URLs use client-bound, single-use transfer tokens (30-minute lifetime for big files). Replaying a completed download URL returns `403 Forbidden`.
5. **Usage:** Clients supply their current credential as a query parameter:
   ```http
   ?token=your_hmac_credential
   ```
6. **Validation:** Missing, expired, replayed, wrong-scope, wrong-client, or tampered credentials are rejected with `403 Forbidden`. Token responses use `Cache-Control: no-store`.

---

## 📡 HTTP Endpoints Reference

### 1. Serve Upload UI
*   **URL:** `/`
*   **HTTP Method:** `GET`
*   **Authentication:** Receiver and sender modes require the single-use QR bootstrap token.
*   **Query Parameters:**
    *   `token`: Five-minute QR bootstrap credential.
*   **Request Body:** None
*   **Response Format:** `text/html`
*   **Status Codes:**
    *   `200 OK` — Success, UI page loaded.
    *   `500 Internal Server Error` — UI asset failed to load.
*   **Description:** Serves the responsive mobile page and injects an IP-bound five-minute session token into `{{TOKEN}}`. The bootstrap credential is consumed during this exchange. Loading the page automatically registers a connection heartbeat.

#### Example Curl:
```bash
curl -X GET "http://192.168.1.50:3000/?token=<qr-bootstrap-token>"
# With BEAMSYNC_ENABLE_TLS=true:
curl -k -X GET "https://192.168.1.50:3000/?token=<qr-bootstrap-token>"
```

---

### 2. Keep-Alive Ping (Heartbeat)
*   **URL:** `/heartbeat`
*   **HTTP Method:** `POST`
*   **Authentication:** Token Query Parameter (`?token=...`)
*   **Query Parameters:**
    *   `token` (string, required): Session authentication token.
*   **Request Body:** None
*   **Response Format:** Empty response body
*   **Response Headers:**
    *   `X-BeamSync-Token`: Replacement IP-bound session token with a fresh five-minute lifetime.
    *   `Cache-Control: no-store`
*   **Status Codes:**
    *   `200 OK` — Heartbeat successfully registered.
    *   `403 Forbidden` — Missing or invalid token.
    *   `405 Method Not Allowed` — HTTP method was not `POST`.
*   **Description:** Keeps the connection alive and renews the short-lived session. The mobile browser pings every 5 seconds and adopts `X-BeamSync-Token`. If the watchdog does not detect a heartbeat or active file transfer within 15 seconds, it emits `device_disconnected`.

#### Example Curl:
```bash
curl -X POST "http://192.168.1.50:3000/heartbeat?token=8a3ef2e987c654ab23f0de1a87e5b602"
```

---

### 3. Request File Transfer
*   **URL:** `/request-transfer`
*   **HTTP Method:** `POST`
*   **Authentication:** Token Query Parameter (`?token=...`)
*   **Query Parameters:**
    *   `token` (string, required): Current client-bound session token.
*   **Request Body:** `application/json`
    ```json
    {
      "filename": "vacation_video.mp4",
      "sizeBytes": 104857600,
      "mimeType": "video/mp4"
    }
    ```
*   **Response Format:** `text/plain`
*   **Status Codes:**
    *   `200 OK` — Approved (Response body: `"approved"`).
    *   `400 Bad Request` — Invalid or corrupt JSON request body.
    *   `403 Forbidden` — Transfer rejected (invalid token, device blocked, file type blocked, size limit exceeded, or manually rejected by the desktop user).
    *   `405 Method Not Allowed` — HTTP method was not `POST`.
    *   `408 Request Timeout` — The desktop user did not respond to the transfer prompt within 60 seconds.
*   **Description:** Requests permission to transfer files.
    *   **Accept All Mode:** Automatically responds with `200 OK (approved)`.
    *   **Blocked Devices:** Rejects immediately if the sender IP is blocked.
    *   **Block All Mode:** Rejects immediately with a 403.
    *   **Blocked Extensions / Max Size Limits:** Rejects if extension is blacklisted (e.g. `.exe`) or size exceeds limit.
    *   **Ask First Mode (Default):** Generates a pending transfer request, triggers a desktop notification, and blocks the HTTP request for up to 60 seconds awaiting the user's manual "Accept" or "Reject" response.

#### Example Curl:
```bash
curl -X POST "http://192.168.1.50:3000/request-transfer?token=8a3ef2e987c654ab23f0de1a87e5b602" \
     -H "Content-Type: application/json" \
     -d '{"filename":"document.pdf", "sizeBytes":1542000, "mimeType":"application/pdf"}'
```

---

### 4. Upload Files
*   **URL:** `/upload`
*   **HTTP Method:** `POST`
*   **Authentication:** Token Query Parameter (`?token=...`)
*   **Headers:**
    *   `Content-Type: multipart/form-data; boundary=...`
*   **Query Parameters:**
    *   `token` (string, required): Session authentication token.
*   **Request Body:** `multipart/form-data` payload containing:
    1.  `beam_manifest` (Form field, JSON manifest metadata of all files in this batch):
        ```json
        [
          {"name": "document.pdf", "size": 1542000}
        ]
        ```
    2.  `documents` (Form field, containing the binary streams of the files)
*   **Response Format:** `text/plain`
*   **Status Codes:**
    *   `200 OK` — Upload completed and files saved (Response body: `"✅ Upload Complete"`).
    *   `400 Bad Request` — Missing multipart boundary, invalid manifest, or corrupt data.
    *   `403 Forbidden` — Missing or invalid token.
    *   `405 Method Not Allowed` — HTTP method was not `POST`.
    *   `500 Internal Server Error` — Server ran out of disk space, write permission failed, or panicked.
*   **Description:** Uploads one or more files to the desktop.
    *   Guarded against runaway transfers by a maximum payload size limit of 100 GB.
    *   Uses an adaptive concurrent write pipeline: small files (≤64 MB) are fully buffered in memory and processed by background worker goroutines to prevent sequential disk bottlenecks, while large files (>64 MB) are streamed sequentially to avoid memory exhaustion.
    *   Emits real-time progress events to the desktop app UI (`upload_progress` and `file_received`).

#### Example Curl:
```bash
# Create dummy files
echo "Sample PDF Content" > document.pdf
echo '[{"name": "document.pdf", "size": 19}]' > manifest.json

# Send upload request
curl -X POST "http://192.168.1.50:3000/upload?token=8a3ef2e987c654ab23f0de1a87e5b602" \
     -F "beam_manifest=<manifest.json;type=application/json" \
     -F "documents=@document.pdf"
```

---

### 5. Download File Stream
*   **URL:** `/download` (Single-file transfer) or `/download/:index` (Multi-file transfer, e.g. `/download/0`, `/download/1`)
*   **HTTP Method:** `GET`
*   **Authentication:** Single-use, client-bound transfer token generated into the sender page.
*   **Query Parameters:**
    *   `token` (string, required): Single-use transfer token.
*   **Request Body:** None
*   **Response Headers:**
    *   `Content-Disposition: attachment; filename="filename.ext"`
    *   `X-Filename: filename.ext` (Exposed to allow frontends to extract the real file name during stream downloads)
    *   `Content-Length: <file_size>`
    *   `Content-Type: <detected_mime_type>` (Defaults to `application/octet-stream`)
*   **Response Format:** Binary stream of the file content
*   **Status Codes:**
    *   `200 OK` — Success, streaming binary data.
    *   `403 Forbidden` — Missing or invalid token.
    *   `404 Not Found` — File not found or index out of range.
    *   `500 Internal Server Error` — Failed to open or read file on disk.
*   **Description:** Streams one file from the desktop sender and consumes the URL token before the handler runs. Replaying or copying the URL to another client is rejected. Emits progressive `download_progress` events to the desktop UI.

#### Example Curl:
```bash
curl -L -o downloaded_file.pdf "http://192.168.1.50:3005/download?token=8a3ef2e987c654ab23f0de1a87e5b602"
```

---

### 6. App Logo Asset
*   **URL:** `/logo.png`
*   **HTTP Method:** `GET`
*   **Authentication:** None
*   **Query Parameters:** None
*   **Request Body:** None
*   **Response Headers:**
    *   `Content-Type: image/png`
    *   `Cache-Control: public, max-age=31536000`
*   **Response Format:** PNG image
*   **Status Codes:**
    *   `200 OK` — Logo successfully served.
    *   `404 Not Found` — Asset not found.
*   **Description:** Serves the brand logo image to decorate the mobile upload/download pages.

---

## 🧠 Internal Transfer Control Lifecycle (Programmatic APIs)

In BeamSync, actions such as **approving/rejecting** or **cancelling** transfers are not exposed as public HTTP routes to prevent network intrusion or cross-site requests. Instead, the desktop application runs as the server and controls these flows **programmatically** from the backend:

### 1. Desktop Manual Approval (`RespondToTransfer`)
*   **Go Signature:** `func (s *HTTPServer) RespondToTransfer(id string, approved bool)`
*   **How it Works:** When `/request-transfer` triggers, the server blocks on a channel (`pt.approved`). When the desktop user clicks "Accept" or "Reject" in the Wails GUI, the Wails bridge invokes the backend's `RespondToTransfer` function:
    *   `approved = true`: Passes `true` to the channel, causing `/request-transfer` to return `200 OK (approved)`.
    *   `approved = false`: Passes `false` to the channel, causing `/request-transfer` to return `403 Forbidden`.

### 2. Cancel Active Transfer / Shutdown
*   **Go Signature:** `func (s *HTTPServer) Shutdown() error`
*   **How it Works:** When the desktop user clicks the **CANCEL** button in the transfer progress bar, the Wails frontend invokes the application's context cancellation. This triggers `Shutdown()`, closing the server's HTTP listener and calling `s.cancel()`. This halts the network stream, stops the watchdog ping monitor, and securely resets the connection state.
