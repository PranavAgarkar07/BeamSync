<div align="center">
  <img src="desktop/frontend/src/assets/images/icon.png" alt="BeamSync Logo" width="128">
  <h1>BeamSync</h1>
  <p>
    <b>Secure. Fast. Local.</b><br>
    A high-performance, offline-first peer-to-peer file transfer system.
  </p>

  <p>
    <img src="https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go">
    <img src="https://img.shields.io/badge/Svelte-4A4A55?style=for-the-badge&logo=svelte&logoColor=FF3E00" alt="Svelte">
    <img src="https://img.shields.io/badge/Wails-CC0000?style=for-the-badge&logo=wails&logoColor=white" alt="Wails">
    <img src="https://img.shields.io/badge/Vite-646CFF?style=for-the-badge&logo=vite&logoColor=white" alt="Vite">
    <br>
    <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow?style=for-the-badge" alt="License: MIT"></a>
    <a href="https://aur.archlinux.org/packages/beamsync-bin"><img src="https://img.shields.io/badge/AUR-beamsync--bin-1793D1?style=for-the-badge&logo=archlinux&logoColor=white" alt="AUR"></a>
  </p>
</div>

---

<details>
<summary>Table of Contents</summary>

- [Overview](#overview)
- [Features](#features)
- [Screenshots](#screenshots)
- [Architecture](#architecture)
- [Installation](#installation)
- [Building from Source](#building-from-source)
- [Usage Guide](#usage-guide)
- [Network Requirements & Connectivity Notes](#network-requirements--connectivity-notes)
- [Developer API](#developer-api)
- [API Documentation](API.md)
- [Security & Privacy](#security--privacy)
- [License](#license)
- [Contributing](#contributing)

</details>

## Overview

**BeamSync** is a cross-platform, desktop file transfer application engineered for speed and reliability. Built with **Go** ([Wails v2](https://wails.io/)) and **Svelte**, it establishes direct, high-bandwidth connections over your local area network — no internet required, no cloud involved, no accounts needed. Files go straight from one device to another.

## Features

- ⚡ **High-Speed Local Transfer** — Streams files directly over HTTP on your LAN for maximum throughput.
- 🔒 **Fully Offline** — Works without internet access. Data never leaves your local network.
- 📱 **Cross-Device via QR Code** — Connect any mobile device instantly by scanning a QR code — no app install required on the phone.
- 📂 **Drag & Drop** — Drop files onto the window to begin transfer immediately.
- 🧠 **Smart Networking** —
  - Auto-detects the optimal local IP address.
  - Finds open ports automatically to avoid conflicts.
  - Configures firewall rules on Linux when needed.
- 🔊 **Auditory Feedback** — Integrated audio engine provides real-time interaction sounds.
- ♿ **Accessible** — Respects `prefers-reduced-motion` for all animations.

## Screenshots

<div align="center">
  <img src="desktop/frontend/src/assets/images/appSS1.png" alt="BeamSync — Main Interface" width="80%">
</div>
<br>
<div align="center">
  <img src="desktop/frontend/src/assets/images/appSS2.png" alt="BeamSync — File Transfer" width="80%">
</div>
<br>
<div align="center">
  <img src="desktop/frontend/src/assets/images/appSS3.png" alt="BeamSync — Mobile Interface" width="80%">
</div>

## Architecture

The project is structured as a **Go workspace** with two modules:

```
BeamSync/
├── beamsync/          # Core library (Go module)
│   ├── server.go      # StartServer(), StartSender() — HTTP file streams
│   ├── port_manager.go# FindAvailablePort() — dynamic port binding
│   ├── firewall.go    # RunFirewallSetup() — Linux firewall automation
│   └── ...
├── desktop/           # Wails desktop application (Go + Svelte)
│   ├── app.go         # App lifecycle, config, event bridge
│   ├── frontend/      # Svelte + Vite UI
│   └── ...
└── go.work            # Go workspace root
```

| Layer | Technology | Key Components |
|-------|-----------|----------------|
| **Frontend** | Svelte + Vite | Reactive UI, drag-and-drop, QR display |
| **Application Shell** | Go + Wails v2 | `App` — event loop, config persistence (`loadConfig`, `saveConfig`) |
| **Network Engine** | Go (net/http) | `StartServer`, `StartSender`, `writeFileToDisk` — concurrent file I/O |
| **Utilities** | Go | `getLocalIP`, `FindAvailablePort`, `RunFirewallSetup` |
| **Media** | Go | `AudioEngine` — native audio playback |

> Architecture details derived from [graphify](graphify-out/GRAPH_REPORT.md) analysis: 172 nodes, 223 edges, 20 communities.

## Installation

### Arch Linux (AUR)

```bash
yay -S beamsync-bin
```

### Pre-built Binaries

Download the latest release from the [Releases](https://github.com/PranavAgarkar07/BeamSync/releases) page.

## Building from Source

### Prerequisites

- [Go](https://go.dev/) 1.25+
- [Node.js](https://nodejs.org/) (with npm)
- [Wails CLI](https://wails.io/)

### Steps

1. **Clone the repository:**
   ```bash
   git clone https://github.com/PranavAgarkar07/BeamSync.git
   cd BeamSync
   ```

2. **Install frontend dependencies:**
   ```bash
   cd desktop/frontend && npm install && cd ../..
   ```

3. **Run in development mode** (hot reload):
   ```bash
   cd desktop && wails dev
   ```

4. **Build for production:**
   ```bash
   cd desktop && wails build
   ```
   Compiled binaries are output to `desktop/build/bin/`.

## Usage Guide

### Receiving Files (Default Mode)

1. Launch BeamSync.
2. A connection QR code and URL are displayed automatically.
3. Scan the QR code from a mobile device, or open the URL from another machine on the network.
4. Incoming files are saved to `Downloads/BeamSync/`.

### Sending Files

1. Click **Send** or drag files onto the window.
2. Select the files to transfer.
3. A unique URL and QR code are generated.
4. Open the URL or scan the QR on the receiving device to start the download.

## Network Requirements & Connectivity Notes

BeamSync works by starting a local HTTP server that other devices on your LAN connect to (via browser or curl). In some network environments, the connection may not work as expected.

### Important Requirements

- All devices must be connected to the **same local network / subnet**. Devices on different VLANs or isolated network segments may not be able to reach each other.
- Both devices must be able to communicate over LAN (HTTP reachability, not automatic discovery)
- **macOS (14+ Sonoma/Sequoia):** Grant Local Network permission — go to **System Settings → Privacy & Security → Local Network → enable BeamSync**. The app will prompt on first launch.
- **Mobile hotspots:** Some enforce client isolation; tethered devices may not reach the host

### Common Network Restrictions

The following network types may block or limit BeamSync functionality:

- **Guest WiFi networks** often isolate devices from each other
- **Public WiFi networks (cafes, airports, hotels)** often restrict device-to-device traffic
- **Enterprise, office, or school networks** may use client isolation or VLAN segmentation that prevents devices from reaching each other
- Routers with **AP isolation / client isolation enabled** block all LAN device communication
- **NAT hairpinning:** On single-router home networks, some routers cannot route traffic back to a device on the same LAN when using the external IP or hostname. Use the local IP (e.g. `192.168.x.x`) directly.
- **Third-party antivirus software** (Norton, McAfee, Kaspersky, etc.) may silently block LAN HTTP servers. Temporarily disable or add an exclusion for BeamSync.

### Firewall & Router Considerations

- Firewalls on Linux, Windows, or macOS may block incoming connections
- BeamSync uses TCP ports **3000–3098** for local transfers. Ensure these ports are allowed through your firewall when necessary.
- If a transfer fails because the selected port is unavailable or blocked, check the firewall rules for the BeamSync port range before troubleshooting the network connection further.
- On Linux, BeamSync can automatically configure firewall rules when required through its built-in firewall setup mechanism (`ufw`, `firewalld`, or `iptables`).
- Ensure BeamSync is allowed through your system firewall if prompted

### Troubleshooting

If devices cannot connect, try these diagnostics:

```
# Check basic network reachability
ping <receiver-ip>

# Verify BeamSync's HTTP server is responding
curl -v http://<receiver-ip>:<port>/

# Check if the port is listening (from the receiver)
netstat -an | grep <port>

# Confirm both devices are on the same subnet
ip addr show | grep "inet "
```

Quick steps:
- Confirm both devices are on the same WiFi network
- Disable VPNs or proxies temporarily
- Try connecting via the device's local IP address instead of hostname
- Test with a mobile hotspot or a different home network
- Temporarily disable third-party antivirus to isolate the cause

## Developer API

For developers, contributors, and security auditors who want to understand the underlying peer-to-peer file transfer protocol or build custom scripts/clients, the complete HTTP API and authentication specifications are available in the **[API Documentation (API.md)](API.md)**.

## Security & Privacy

- **Local-only** — All transfers happen over your LAN. No external servers are contacted.
- **Optional HTTPS** — Set `BEAMSYNC_ENABLE_TLS=true` before launching BeamSync to serve receiver and sender pages over HTTPS with a persisted ECDSA local certificate in `~/.config/beamsync/`.
- **Ephemeral credentials** — HMAC-signed sessions expire after five minutes, bind to the connecting client and TLS context, renew through heartbeats, and use single-use links for downloads.
- **Zero data collection** — BeamSync does not store, transmit, or analyze your files beyond the direct LAN transfer.
- **Open source** — The entire codebase is available for audit.

## License

This project is licensed under the **MIT License**. See the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome. Please open an issue or submit a pull request.
