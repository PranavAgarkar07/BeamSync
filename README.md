<div align="center">
  <img src="desktop/frontend/src/assets/images/icon.png" alt="BeamSync Logo" width="128">
  <h1>// BEAMSYNC_SYS</h1>
  <p>
    <b>Secure. Fast. Local.</b><br>
    The ultimate offline file transfer protocol for the modern web.
  </p>
  <p>
    <a href="#system-capabilities">System Capabilities</a> •
    <a href="#visual-intel">Visual Intel</a> •
    <a href="#deployment">Deployment</a> •
    <a href="#operational-manual">Operational Manual</a>
  </p>
</div>

---

## // MISSION_BRIEFING

**BeamSync** is a high-performance, offline-first peer-to-peer file transfer system built with **Go (Wails)** and **Svelte**. Engineered for speed and reliability, it bypasses the need for internet access by creating a direct data link over your local network.

Wrapped in a **Cyberpunk-inspired terminal interface**, BeamSync turns mundane file transfers into an immersive experience complete with mechanical sound effects, visual feedback, and zero-latency performance.

## // VISUAL_INTEL

<div align="center">
  <img src="desktop/frontend/src/assets/images/appSS1.png" alt="Main Terminal Interface" width="45%">
  <img src="desktop/frontend/src/assets/images/appSS2.png" alt="Data Uplink in Progress" width="45%">
</div>

## // SYSTEM_CAPABILITIES

- **⚡ Hyper-Fast Local Transfer**: Leverages direct HTTP/UDP protocols for maximum bandwidth utilization on your LAN.
- **🔒 Offline Protocol**: Completely functional without an internet connection. Your data never leaves your local network.
- **🖥️ Cyberpunk Interface**: A fully reactive, immersive UI with scanlines, typewriter effects, and terminal aesthetics.
- **🔊 Auditory Feedback**: Integrated audio engine provides satisfying mechanical clicks, blips, and connection sounds.
- **📂 Drag & Drop Uplink**: Simply drag files onto the terminal to initiate immediate transmission.
- **📱 Cross-Device Sync**: Instantly connect mobile devices via dynamic QR code generation—no app install required on the phone.
- **🧠 Smart Intelligence**:
  - **Auto-IP Detection**: Automatically resolves the optimal network interface.
  - **Dynamic Port Scouting**: Avoids conflicts by finding open ports automatically.
  - **Resilient Backend**: "Zombie" process handling keeps the system stable.

## // TECH_STACK

The core architecture is built upon a robust foundation:

- **Frontend Core**: [Svelte](https://svelte.dev/) (Vite)
- **Backend Engine**: [Go](https://go.dev/) (Wails v2)
- **Protocol Layer**: HTTP/UDP (Discovery & Transport)
- **Styling**: Custom CSS variables & Animations

## // DEPLOYMENT

### Prerequisites

Initialize your environment with the following dependencies:
- [Go](https://go.dev/) (1.18+)
- [Node.js](https://nodejs.org/) (npm)
- [Wails CLI](https://wails.io/)

### Build Sequence

1. **Clone the repository**:
   ```bash
   git clone https://github.com/PranavAgarkar07/BeamSync.git
   ```

2. **Navigate to the core module**:
   ```bash
   cd desktop
   ```

3. **Install frontend dependencies**:
   ```bash
   cd frontend && npm install && cd ..
   ```

4. **Initiate Development Mode** (Hot Reload):
   ```bash
   wails dev
   ```

5. **Compile for Production**:
   ```bash
   wails build
   ```
   > *Artifacts will be generated in `desktop/build/bin`*

## // OPERATIONAL_MANUAL

### Receiver Mode (Default)
1. Launch BeamSync.
2. The system enters **HANDSHAKE** mode and generates a unique QR code.
3. **Scan the QR code** with a mobile device or sender unit.
4. Connection is established automatically.
5. Incoming files are saved to your `Downloads/BeamSync` directory.

### Sender Mode
1. Click **[ SEND ]** on the Command Deck or drag files onto the window.
2. Select the files you wish to transmit.
3. A unique Uplink URL/QR code is generated.
4. The receiver device opens this URL to begin the download.

---

<div align="center">
  <sub>// END_OF_FILE</sub>
</div>
