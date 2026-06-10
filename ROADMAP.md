# BeamSync Release Roadmap

This document outlines the architectural vision and implementation timeline for future versions of BeamSync. Tasks are sequenced to prioritize foundational networking stability before ecosystem expansion.

---

## 🚀 Milestone 3.0: Core Infrastructure & Automated Discovery

The focus of this release is separating the transport engine from the desktop presentation layer and eliminating connection friction over local area networks.

### 1. Headless CLI Mode for Servers (`v3.0.0`)
* **Description:** Completely extract the core network daemon (`beamsync/`) away from the Wails runtime environment. Introduce a native command-line interface (`beamsync-cli`) using the Cobra/Viper ecosystem in Go.
* **Key Capabilities:** Allows remote Linux servers, headless NAS boxes, and terminal environments to act as permanent seeds or receivers. Features flag-based inputs for path, port overrides, and authentication tokens.
* **Target Scope:** Backend (Go)

### 2. "Radar" mDNS Device Discovery (`v3.0.0`)
* **Description:** Implement Multicast DNS (ZeroConf) protocol layers to continuously broadcast and discover BeamSync nodes on the local subnet without external directory servers.
* **Key Capabilities:** Eliminates manual IP typing and QR code dependencies for desktop-to-desktop connections. The Svelte interface will render a reactive, aerospace-inspired tracking grid that visually "pings" active nodes as targets.
* **Target Scope:** Backend (Go) & Frontend (Svelte)

---

## 📊 Milestone 3.5: Diagnostics & Utility Layer

This phase enriches the local file-transfer experience with deep protocol analytics and low-latency productivity features.

### 3. Live Telemetry & Network Diagnostics (`v3.5.0`)
* **Description:** Expose the underlying TCP/HTTP connection mechanics to the user interface via real-time data streaming over the Wails event bridge.
* **Key Capabilities:** Highly detailed dashboard capturing instantaneous throughput graphs, packet drop rates, current TCP window sizing, and estimated completion matrixes styled in an Industrial Brutalist theme.
* **Target Scope:** Frontend (Svelte) & Core Network Layer

### 4. Universal P2P Clipboard (`v3.5.0`)
* **Description:** Create a low-overhead text/data broadcasting channel alongside the core file stream to facilitate real-time system clipboard synchronization.
* **Key Capabilities:** Securely intercepts textual clips, authorization strings, or markdown blobs on one node and mirrors them to the system clipboard of a specified peer with cryptographic verification.
* **Target Scope:** Core Network Layer & OS Clipboard Bindings

---

## 📱 Milestone 4.0: Mobile Native Ecosystem

Transitioning BeamSync from a desktop-centric utility into a cross-device application suite.

### 5. Native Android & iOS Applications (`v4.0.0`)
* **Description:** Compile the decoupled Go core engine into mobile library frameworks via `gomobile`. Build thin, ultra-performant user interfaces using Kotlin (Android) and Swift (iOS).
* **Key Capabilities:** Enables mobile devices to act as full peer-to-peer hosting nodes on the LAN instead of passive web-browser download clients. Fully integrates with native share sheets and file managers.
* **Target Scope:** Mobile Systems & Go Compilation

---

## ⚡ Milestone 5.0: Scale & Specialized Media

The final tier upgrades the transport architecture to handle concurrent distribution patterns and big-screen hardware environments.

### 6. "Swarm" Broadcasting (1-to-Many) (`v5.0.0`)
* **Description:** Refactor the HTTP download distribution architecture to support simultaneous multi-peer data streaming.
* **Key Capabilities:** Utilizes custom chunk-casting algorithms and concurrent `io.Reader` multiplexing in Go to stream a single large file to multiple local targets simultaneously without causing local disk I/O thrashing.
* **Target Scope:** Protocol & Concurrency Engineering

### 7. Android TV Application (`v5.0.0`)
* **Description:** Build a dedicated Leanback UI target utilizing the Android application framework, specialized for television displays and directional remote controls.
* **Key Capabilities:** Allows rapid, painless streaming of ultra-high-definition 4K video files from local development rigs or personal laptops directly to home entertainment hardware over the local network fabric.
* **Target Scope:** Specialized Frontend & Android Ecosystem
