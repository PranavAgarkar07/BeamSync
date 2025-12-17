package main

import (
	"beamsync"
	"beamsync/audio"
	"context"
	"embed"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	stdruntime "runtime"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed sounds/*.wav
var soundFS embed.FS

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

// EventData holds event information
type EventData struct {
	Name string
	Data string
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		eventChan: make(chan EventData, 100), // Buffered channel
	}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Start event processor on main thread
	go a.processEvents()

	// Start IP Monitor
	go a.startIPMonitor()

	// Initialize Audio Engine
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
			// Access embedded file
			f, err := soundFS.Open("sounds/" + file)
			if err != nil {
				fmt.Printf("⚠️ Failed to open embedded sound '%s': %v\n", file, err)
				continue
			}

			// LoadSoundFromStream (closes f automatically)
			if err := a.audio.LoadSoundFromStream(name, f); err != nil {
				fmt.Printf("⚠️ Failed to load sound '%s': %v\n", name, err)
			} else {
				fmt.Printf("🔊 Loaded sound: %s\n", name)
			}
		}
	}
}

// processEvents handles events on a safe goroutine
func (a *App) processEvents() {
	for event := range a.eventChan {
		// Intercept device_connected to re-verify IP
		if event.Name == "device_connected" {
			currentRealIP := getLocalIP()
			if a.currentIP != "" && a.currentIP != currentRealIP {
				fmt.Printf("🔄 IP Change Detected! Old: %s, New: %s\n", a.currentIP, currentRealIP)
				a.currentIP = currentRealIP
				newURL := fmt.Sprintf("http://%s:%s", a.currentIP, a.currentPort)
				a.safeEmit("url_changed", newURL)
			}
		}
		a.safeEmit(event.Name, event.Data)
	}
}

// safeEmit safely emits an event to the frontend, handling panics and nil context
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

// shutdown is called when the app is closing
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

// PlaySound exposed to Frontend
func (a *App) PlaySound(name string) {
	if a.audio != nil {
		a.audio.Play(name)
	}
}

// ---------------------------------------------------------
// BRIDGE METHODS
// ---------------------------------------------------------

// StartReceiverDefault: silent startup using Downloads folder
func (a *App) StartReceiverDefault() string {
	if a.serverApp != nil {
		fmt.Println("🔄 Stopping previous receiver server...")
		if err := a.serverApp.Shutdown(); err != nil {
			fmt.Println("⚠️ Failed to stop previous server:", err)
		}
		a.serverApp = nil
	}

	home, err := os.UserHomeDir()
	savePath := "received_files"
	if err == nil {
		savePath = filepath.Join(home, "Downloads", "BeamSync")
	}

	a.lastSavePath = savePath // Store for OpenFile

	if err := os.MkdirAll(savePath, 0755); err != nil {
		fmt.Println("⚠️ Failed to create save directory:", err)
		return "Error: Could not create save directory"
	}

	// Setup callback - Thread-safe via Channel
	beamsync.SetEventCallback(func(name string, data string) {
		a.eventChan <- EventData{Name: name, Data: data}
	})

	app, port := beamsync.StartServer(savePath, 3000)
	a.serverApp = app

	localIP := getLocalIP()
	url := "http://" + localIP + ":" + port

	a.currentIP = localIP
	a.currentPort = port

	fmt.Println("📡 Receiver started:", url)
	return url
}

// StartReceiver: Tells the Brain to listen for files
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

	a.lastSavePath = selection // Store for OpenFile

	// Setup callback - Thread-safe via Channel
	beamsync.SetEventCallback(func(name string, data string) {
		a.eventChan <- EventData{Name: name, Data: data}
	})

	app, port := beamsync.StartServer(selection, 3000)
	a.serverApp = app

	localIP := getLocalIP()
	url := "http://" + localIP + ":" + port

	a.currentIP = localIP
	a.currentPort = port

	fmt.Println("📡 Receiver started:", url)
	return url
}

// StartSender: Asks user for a file, then tells Brain to host it
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

	app, port := beamsync.StartSender(selection)
	a.senderApp = app

	localIP := getLocalIP()
	url := "http://" + localIP + ":" + port

	a.currentIP = localIP
	a.currentPort = port

	// Display the URL prominently
	fmt.Println("========================================")
	fmt.Println("📤 SENDER STARTED")
	fmt.Println("========================================")
	fmt.Printf("📱 Open this URL on your mobile device:\n")
	fmt.Printf("   %s\n", url)
	fmt.Println("========================================")

	// Emit event to frontend with the URL
	go func() {
		time.Sleep(100 * time.Millisecond)
		a.safeEmit("sender_started", url)
	}()

	return url
}

// StopReceiver: Stop the receiver server
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

// StopSender: Stop the sender server
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

// ResetApp: Stops all servers and resets state
func (a *App) ResetApp() {
	fmt.Println("🔄 Resetting App State...")
	a.StopReceiver()
	a.StopSender()
	a.serverApp = nil
	a.senderApp = nil
	// We don't reset IP/Port here because we might want to restart immediately
	// But we should probably clear the currentPort so IP monitor doesn't emit url_changed
	a.currentPort = ""
}

// OpenFile opens a file using the default system application.
func (a *App) OpenFile(filename string) string {
	if a.lastSavePath == "" {
		return "Error: No active save directory"
	}

	fullPath := filepath.Join(a.lastSavePath, filepath.Base(filename))
	fmt.Println("📂 Opening file:", fullPath)

	var cmd *exec.Cmd
	var commandName string
	var args []string

	switch stdruntime.GOOS {
	case "windows":
		commandName = "cmd"
		args = []string{"/c", "start", "", fullPath}
	case "darwin":
		commandName = "open"
		args = []string{fullPath}
	default: // linux, freebsd, openbsd, netbsd
		commandName = "xdg-open"
		args = []string{fullPath}
	}

	cmd = exec.Command(commandName, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Sprintf("Error opening file: %v", err)
	}
	return "File opened"
}

// ---------------------------------------------------------
// HELPER
// ---------------------------------------------------------
func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		fmt.Println("⚠️ Failed to dial for local IP detection:", err)
		return "127.0.0.1"
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// startIPMonitor checks for IP changes periodically
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
				fmt.Printf("🔄 Network Change Detected! IP changed from %s to %s\n", a.currentIP, newIP)
				a.currentIP = newIP

				// Only emit update if we have an active port (server running)
				if a.currentPort != "" {
					newURL := fmt.Sprintf("http://%s:%s", a.currentIP, a.currentPort)
					fmt.Println("📡 Updating URL to:", newURL)
					a.safeEmit("url_changed", newURL)
				}
			}
		}
	}
}
