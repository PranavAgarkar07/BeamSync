# BeamSync Internal HTTP API Documentation

This document describes the internal peer-to-peer HTTP API for **BeamSync**. 

When you launch BeamSync in either **Receiver (default)** or **Sender** mode, it starts a local HTTP server on a dynamically assigned port. Devices connect to this server over the local area network (LAN) to transfer files.

---

## 🔒 Authentication & Token Scheme

To prevent unauthorized devices on the local network from sending or downloading files, BeamSync uses a **Session Token Authentication Scheme**:

1. **Token Generation:** When the server starts, it generates a cryptographically secure 16-byte (32-character hexadecimal) session token using Go's `crypto/rand` (`generateToken()`).
2. **Access Control:** All API endpoints (except the UI home page and logo asset) are protected by a token-validation middleware (`tokenMiddleware`).
3. **Usage:** Clients must supply the session token as a query parameter in every request:
   ```http
   ?token=your_32_character_hex_session_token
   ```
4. **Validation:** If the token is missing or incorrect, the server immediately rejects the request with a `403 Forbidden` response.

---

## 📡 HTTP Endpoints Reference

### 1. Serve Upload UI
*   **URL:** `/`
*   **HTTP Method:** `GET`
*   **Authentication:** None (Required to load the page that retrieves the token)
*   **Query Parameters:** None
*   **Request Body:** None
*   **Response Format:** `text/html`
*   **Status Codes:**
    *   `200 OK` — Success, UI page loaded.
    *   `500 Internal Server Error` — UI asset failed to load.
*   **Description:** Serves the responsive mobile upload page (`ui/upload.html`). The server dynamically injects the generated session token into the page template placeholder (`{{TOKEN}}`) so the mobile browser can authenticate subsequent API calls. Loading this page automatically registers a connection heartbeat.

#### Example Curl:
```bash
curl -X GET "http://192.168.1.50:3000/"
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
*   **Status Codes:**
    *   `200 OK` — Heartbeat successfully registered.
    *   `403 Forbidden` — Missing or invalid token.
    *   `405 Method Not Allowed` — HTTP method was not `POST`.
*   **Description:** Keeps the connection alive. The mobile browser pings this endpoint every 5 seconds. If the server's watchdog goroutine does not detect a heartbeat or active file transfer write within 15 seconds, it emits a `device_disconnected` event to the desktop app.

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
    *   `token` (string, required): Session authentication token.
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
*   **Authentication:** Token Query Parameter (`?token=...`)
*   **Query Parameters:**
    *   `token` (string, required): Session authentication token.
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
*   **Description:** Streams files from the desktop sender to a remote receiver. Emits progressive `download_progress` events to the desktop UI.

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
