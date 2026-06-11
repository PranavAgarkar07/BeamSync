package main

import (
	"beamsync"
	"beamsync/audio"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	stdruntime "runtime"
	"sort"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed sounds/*.wav
var soundFS embed.FS

// currentVersion is the running build version — keep in sync with wails.json productVersion.
const currentVersion = "v2.4.0"

// App struct
type App struct {
	ctx          context.Context
	audio        *audio.AudioEngine
	serverApp    *beamsync.HTTPServer
	senderApp    *beamsync.HTTPServer
	eventChan    chan EventData
	lastSavePath string
	currentIP    string
	currentPort  string
}

// UpdateInfo is returned to the frontend.
type UpdateInfo struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
	ReleaseURL      string `json:"releaseUrl"`
	ReleaseNotes    string `json:"releaseNotes"`
}

// EventData holds event information
type EventData struct {
	Name string
	Data string
}

// ReceivedFile holds metadata about a file in the save directory.
type ReceivedFile struct {
	Name      string `json:"name"`
	SizeBytes int64  `json:"sizeBytes"`
	ModTime   string `json:"modTime"` // "HH:MM" local time
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		eventChan: make(chan EventData, 100),
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// CONFIG PERSISTENCE — ~/.config/beamsync/config.json
// ─────────────────────────────────────────────────────────────────────────────

type configData struct {
	SavePath         string                    `json:"savePath"`
	TransferSettings beamsync.TransferSettings `json:"transferSettings"`
}

func configPath() string {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		cfgDir = filepath.Join(os.TempDir(), ".config")
	}
	return filepath.Join(cfgDir, "beamsync", "config.json")
}

func loadConfig() configData {
	var cfg configData
	data, err := os.ReadFile(configPath())
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, &cfg)
	return cfg
}

func saveConfig(cfg configData) error {
	p := configPath()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

func defaultSavePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "received_files"
	}
	return filepath.Join(home, "Downloads", "BeamSync")
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	go a.processEvents()
	go a.startIPMonitor()
	go a.checkForUpdateAndNotify()

	a.audio = audio.NewAudioEngine()
	if err := a.audio.Init(); err != nil {
		fmt.Println("⚠️ Audio Init Failed:", err)
	} else {
		fmt.Println("🔊 Loading embedded sounds...")
		sounds := map[string]string{
			"hover":   "hover.wav",
			"click":   "click.wav",
			"blip":    "hover.wav",
			"connect": "connect.wav",
			"success": "transfer_complete.wav",
			"startup": "startup.wav",
		}
		for name, file := range sounds {
			f, err := soundFS.Open("sounds/" + file)
			if err != nil {
				fmt.Printf("⚠️ Failed to open embedded sound '%s': %v\n", file, err)
				continue
			}
			if err := a.audio.LoadSoundFromStream(name, f); err != nil {
				fmt.Printf("⚠️ Failed to load sound '%s': %v\n", name, err)
			} else {
				fmt.Printf("🔊 Loaded sound: %s\n", name)
			}
		}
	}
}

// processEvents handles backend events on a safe goroutine before relaying to Wails.
func (a *App) processEvents() {
	for event := range a.eventChan {
		if event.Name == "device_connected" {
			currentRealIP := getLocalIP()
			if a.currentIP != "" && a.currentIP != currentRealIP {
				fmt.Printf("🔄 IP Change Detected! Old: %s, New: %s\n", a.currentIP, currentRealIP)
				a.currentIP = currentRealIP
				newURL := fmt.Sprintf("%s://%s:%s", beamsync.ServerScheme(), a.currentIP, a.currentPort)
				a.safeEmit("url_changed", newURL)
			}
		}
		a.safeEmit(event.Name, event.Data)
	}
}

// safeEmit wraps Wails runtime.EventsEmit with panic recovery.
func (a *App) safeEmit(eventName string, data interface{}) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("⚠️ safeEmit panic for event '%s': %v\n", eventName, r)
		}
	}()
	if a.ctx == nil {
		fmt.Printf("⚠️ safeEmit: Context is nil, cannot emit event '%s'\n", eventName)
		return
	}
	runtime.EventsEmit(a.ctx, eventName, data)
	fmt.Printf("✅ Event emitted: %s\n", eventName)
}

// shutdown is called when the app is closing.
func (a *App) shutdown(ctx context.Context) {
	close(a.eventChan)
	if a.serverApp != nil {
		fmt.Println("🛑 Shutting down receiver server...")
		if err := a.serverApp.Shutdown(); err != nil {
			fmt.Println("⚠️ Server shutdown error:", err)
		}
	}
	if a.senderApp != nil {
		fmt.Println("🛑 Shutting down sender server...")
		if err := a.senderApp.Shutdown(); err != nil {
			fmt.Println("⚠️ Sender shutdown error:", err)
		}
	}
}

// PlaySound is exposed to the frontend.
func (a *App) PlaySound(name string) {
	if a.audio != nil {
		a.audio.Play(name)
	}
}

// ---------------------------------------------------------
// BRIDGE METHODS
// ---------------------------------------------------------

// makeCallback returns an EventCallback that queues events into the channel.
func (a *App) makeCallback() beamsync.EventCallback {
	return func(name string, data string) {
		a.eventChan <- EventData{Name: name, Data: data}
	}
}

// GetSavePath returns the current save path. Reads from config or falls back to default.
func (a *App) GetSavePath() string {
	if a.lastSavePath != "" {
		return a.lastSavePath
	}
	cfg := loadConfig()
	if cfg.SavePath != "" {
		a.lastSavePath = cfg.SavePath
		return cfg.SavePath
	}
	return defaultSavePath()
}

// SetSavePath opens a directory picker, persists the choice, restarts receiver, returns new URL.
func (a *App) SetSavePath() string {
	selection, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Choose Save Folder for Received Files",
		DefaultDirectory: a.GetSavePath(),
	})
	if err != nil || selection == "" {
		return "Cancelled"
	}

	// Persist
	cfg := loadConfig()
	cfg.SavePath = selection
	if err := saveConfig(cfg); err != nil {
		fmt.Println("⚠️ Failed to save config:", err)
	}
	a.lastSavePath = selection
	fmt.Printf("📁 Save path changed to: %s\n", selection)

	// Restart receiver on new path
	if a.serverApp != nil {
		a.serverApp.Shutdown()
		a.serverApp = nil
	}

	if err := os.MkdirAll(selection, 0755); err != nil {
		fmt.Println("⚠️ Failed to create save directory:", err)
		return "Error: Could not create save directory"
	}

	app, port, token := beamsync.StartServer(selection, 3000, a.getTransferSettings(), a.makeCallback())
	a.serverApp = app

	localIP := getLocalIP()
	a.currentIP = localIP
	a.currentPort = port

	url := fmt.Sprintf("%s://%s:%s/?token=%s", beamsync.ServerScheme(), localIP, port, token)
	fmt.Println("📡 Receiver restarted on new path:", url)
	return url
}

// StartReceiverDefault starts the receiver using the persisted save path (or default).
func (a *App) StartReceiverDefault() string {
	if a.serverApp != nil {
		fmt.Println("🔄 Stopping previous receiver server...")
		if err := a.serverApp.Shutdown(); err != nil {
			fmt.Println("⚠️ Failed to stop previous server:", err)
		}
		a.serverApp = nil
	}

	savePath := a.GetSavePath()
	a.lastSavePath = savePath

	if err := os.MkdirAll(savePath, 0755); err != nil {
		fmt.Println("⚠️ Failed to create save directory:", err)
		return "Error: Could not create save directory"
	}

	app, port, token := beamsync.StartServer(savePath, 3000, a.getTransferSettings(), a.makeCallback())
	a.serverApp = app

	localIP := getLocalIP()
	a.currentIP = localIP
	a.currentPort = port

	// Embed token in the URL so the mobile page's JS can attach it to requests.
	url := fmt.Sprintf("%s://%s:%s/?token=%s", beamsync.ServerScheme(), localIP, port, token)
	fmt.Println("📡 Receiver started:", url)
	return url
}

// StartReceiver lets the user pick a save folder.
func (a *App) StartReceiver() string {
	if a.serverApp != nil {
		fmt.Println("🔄 Stopping previous receiver server...")
		if err := a.serverApp.Shutdown(); err != nil {
			fmt.Println("⚠️ Failed to stop previous server:", err)
		}
		a.serverApp = nil
	}

	selection, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Folder to Save Received Files",
	})
	if err != nil || selection == "" {
		return "Cancelled"
	}
	a.lastSavePath = selection

	app, port, token := beamsync.StartServer(selection, 3000, a.getTransferSettings(), a.makeCallback())
	a.serverApp = app

	localIP := getLocalIP()
	a.currentIP = localIP
	a.currentPort = port

	url := fmt.Sprintf("%s://%s:%s/?token=%s", beamsync.ServerScheme(), localIP, port, token)
	fmt.Println("📡 Receiver started:", url)
	return url
}

// StartSender lets the user pick files and hosts them for download.
func (a *App) StartSender() string {
	if a.senderApp != nil {
		fmt.Println("🔄 Stopping previous sender server...")
		if err := a.senderApp.Shutdown(); err != nil {
			fmt.Println("⚠️ Failed to stop previous sender:", err)
		}
		a.senderApp = nil
	}

	selection, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select File(s) to Send",
	})
	if err != nil || len(selection) == 0 {
		return "Cancelled"
	}

	app, port, token := beamsync.StartSender(selection, a.makeCallback())
	a.senderApp = app

	localIP := getLocalIP()
	a.currentIP = localIP
	a.currentPort = port

	// Root page loads without token (acts as the landing), downloads require token.
	url := fmt.Sprintf("%s://%s:%s/", beamsync.ServerScheme(), localIP, port)

	fmt.Println("========================================")
	fmt.Println("📤 SENDER STARTED:", url)
	fmt.Printf("   token: %s\n", token)
	fmt.Println("========================================")

	go func() {
		time.Sleep(100 * time.Millisecond)
		a.safeEmit("sender_started", url)

		// Emit the selected file list so the desktop UI can show filenames.
		type fileEntry struct {
			Name      string `json:"name"`
			SizeBytes int64  `json:"sizeBytes"`
		}
		entries := make([]fileEntry, 0, len(selection))
		for _, p := range selection {
			entry := fileEntry{Name: filepath.Base(p)}
			if fi, err := os.Stat(p); err == nil {
				entry.SizeBytes = fi.Size()
			}
			entries = append(entries, entry)
		}
		if b, err := json.Marshal(entries); err == nil {
			a.safeEmit("sender_files", string(b))
		}
	}()

	return url
}

// StopReceiver stops the receiver server.
func (a *App) StopReceiver() string {
	if a.serverApp != nil {
		fmt.Println("🛑 Stopping receiver server...")
		if err := a.serverApp.Shutdown(); err != nil {
			return "Error stopping server"
		}
		a.serverApp = nil
		return "Receiver stopped"
	}
	return "No receiver running"
}

// StopSender stops the sender server.
func (a *App) StopSender() string {
	if a.senderApp != nil {
		fmt.Println("🛑 Stopping sender server...")
		if err := a.senderApp.Shutdown(); err != nil {
			return "Error stopping sender"
		}
		a.senderApp = nil
		return "Sender stopped"
	}
	return "No sender running"
}

// ResetApp stops all servers and resets state.
func (a *App) ResetApp() {
	fmt.Println("🔄 Resetting App State...")
	a.StopReceiver()
	a.StopSender()
	a.serverApp = nil
	a.senderApp = nil
	a.currentPort = ""
}

// OpenFile opens a received file with the system default application.
func (a *App) OpenFile(filename string) string {
	if a.lastSavePath == "" {
		return "Error: No active save directory"
	}

	fullPath := filepath.Join(a.lastSavePath, filepath.Base(filename))
	fmt.Println("📂 Opening file:", fullPath)

	var commandName string
	var args []string
	switch stdruntime.GOOS {
	case "windows":
		commandName = "cmd"
		args = []string{"/c", "start", "", fullPath}
	case "darwin":
		commandName = "open"
		args = []string{fullPath}
	default:
		commandName = "xdg-open"
		args = []string{fullPath}
	}

	if err := exec.Command(commandName, args...).Start(); err != nil {
		return fmt.Sprintf("Error opening file: %v", err)
	}
	return "File opened"
}

// GetReceivedFiles returns existing files in the save directory so the UI
// can restore the received-files log after a restart or reconnect.
func (a *App) GetReceivedFiles() []ReceivedFile {
	if a.lastSavePath == "" {
		return nil
	}
	entries, err := os.ReadDir(a.lastSavePath)
	if err != nil {
		fmt.Println("GetReceivedFiles: could not read dir:", err)
		return nil
	}

	type receivedFileWithModTime struct {
		file    ReceivedFile
		modTime time.Time
	}

	var files []receivedFileWithModTime
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, receivedFileWithModTime{
			file: ReceivedFile{
				Name:      e.Name(),
				SizeBytes: info.Size(),
				ModTime:   info.ModTime().Format("02 Jan - 15:04"),
			},
			modTime: info.ModTime(),
		})
	}
	// Sort newest-first by the mod time already loaded from the directory scan.
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.After(files[j].modTime)
	})

	result := make([]ReceivedFile, 0, len(files))
	for _, file := range files {
		result = append(result, file.file)
	}
	return result
}
// ---------------------------------------------------------
// TRANSFER PERMISSION METHODS
// ---------------------------------------------------------

// getTransferSettings loads settings from config, falling back to defaults.
func (a *App) getTransferSettings() beamsync.TransferSettings {
	cfg := loadConfig()
	if cfg.TransferSettings.Mode == "" {
		return beamsync.DefaultTransferSettings()
	}
	return cfg.TransferSettings
}

// GetTransferSettings returns the current transfer permission settings to the frontend.
func (a *App) GetTransferSettings() beamsync.TransferSettings {
	return a.getTransferSettings()
}

// SaveTransferSettings persists the given settings and live-updates the running server.
func (a *App) SaveTransferSettings(settings beamsync.TransferSettings) string {
	cfg := loadConfig()
	cfg.TransferSettings = settings
	if err := saveConfig(cfg); err != nil {
		return fmt.Sprintf("Error saving settings: %v", err)
	}
	// Update the running server's settings in-place (no restart needed)
	if a.serverApp != nil && a.serverApp.Settings() != nil {
		*a.serverApp.Settings() = settings
	}
	fmt.Println("✅ Transfer settings saved:", settings.Mode)
	return "ok"
}

// ApproveTransfer approves a pending transfer request by its ID.
func (a *App) ApproveTransfer(id string) {
	if a.serverApp != nil {
		a.serverApp.RespondToTransfer(id, true)
	}
}

// RejectTransfer rejects a pending transfer request by its ID.
func (a *App) RejectTransfer(id string) {
	if a.serverApp != nil {
		a.serverApp.RespondToTransfer(id, false)
	}
}

// ---------------------------------------------------------
// UPDATE CHECKER
// ---------------------------------------------------------

// CheckForUpdate calls the GitHub Releases API and returns update info.
// The User-Agent header encodes the current version + OS, giving GitHub's
// traffic analytics natural DAU and platform signals at zero privacy cost.
func (a *App) CheckForUpdate() UpdateInfo {
	info := UpdateInfo{CurrentVersion: currentVersion}

	userAgent := fmt.Sprintf(
		"BeamSync/%s (%s; %s)",
		currentVersion, stdruntime.GOOS, stdruntime.GOARCH,
	)

	req, err := http.NewRequest("GET",
		"https://api.github.com/repos/PranavAgarkar07/BeamSync/releases/latest", nil)
	if err != nil {
		fmt.Println("⚠️ UpdateCheck: could not build request:", err)
		return info
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("⚠️ UpdateCheck: request failed (offline?):", err)
		return info
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("⚠️ UpdateCheck: could not read response:", err)
		return info
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Body    string `json:"body"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		fmt.Println("⚠️ UpdateCheck: could not parse response:", err)
		return info
	}

	info.LatestVersion = release.TagName
	info.ReleaseURL = release.HTMLURL
	// Trim release notes to avoid overwhelming the UI
	notes := strings.TrimSpace(release.Body)
	if len(notes) > 280 {
		notes = notes[:280] + "…"
	}
	info.ReleaseNotes = notes
	info.UpdateAvailable = release.TagName != "" && release.TagName != currentVersion

	fmt.Printf("🔍 UpdateCheck: current=%s latest=%s available=%v\n",
		currentVersion, release.TagName, info.UpdateAvailable)
	return info
}

// checkForUpdateAndNotify runs in a goroutine on startup. Emits an event
// to the frontend only when a new release is available.
func (a *App) checkForUpdateAndNotify() {
	// Small delay so the UI finishes loading before we show the banner
	time.Sleep(3 * time.Second)
	info := a.CheckForUpdate()
	if info.UpdateAvailable {
		if data, err := json.Marshal(info); err == nil {
			a.safeEmit("update_available", string(data))
		}
	}
}

// ---------------------------------------------------------
// HELPERS
// ---------------------------------------------------------

func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		fmt.Println("⚠️ Failed to dial for local IP detection:", err)
		return "127.0.0.1"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

// startIPMonitor polls for IP changes every 3 seconds.
func (a *App) startIPMonitor() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			newIP := getLocalIP()
			if a.currentIP != "" && newIP != a.currentIP {
				fmt.Printf("🔄 Network Change! IP: %s → %s\n", a.currentIP, newIP)
				a.currentIP = newIP
				if a.currentPort != "" {
					newURL := fmt.Sprintf("%s://%s:%s", beamsync.ServerScheme(), a.currentIP, a.currentPort)
					fmt.Println("📡 Updating URL to:", newURL)
					a.safeEmit("url_changed", newURL)
				}
			}
		}
	}
}
