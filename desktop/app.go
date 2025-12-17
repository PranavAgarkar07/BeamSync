package main

import (
	"beamsync"
	"beamsync/audio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	stdruntime "runtime"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx          context.Context
	audio        *audio.AudioEngine
	serverApp    *beamsync.HTTPServer
	senderApp    *beamsync.HTTPServer
	eventChan    chan EventData
	lastSavePath string
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

	// Initialize Audio Engine
	a.audio = audio.NewAudioEngine()
	if err := a.audio.Init(); err != nil {
		fmt.Println("⚠️ Audio Init Failed:", err)
	} else {
		soundDir, err := findSoundDir()
		if err != nil {
			fmt.Println("⚠️ Could not locate sound directory:", err)
		} else {
			fmt.Println("🔊 Found sound directory:", soundDir)
			sounds := map[string]string{
				"hover":   "hover.wav",
				"click":   "click.wav",
				"blip":    "hover.wav",
				"connect": "connect.wav",
				"success": "transfer_complete.wav",
				"startup": "startup.wav",
			}

			for name, file := range sounds {
				path := filepath.Join(soundDir, file)
				if err := a.audio.LoadSound(name, path); err != nil {
					fmt.Printf("⚠️ Failed to load sound '%s' (%s): %v\n", name, path, err)
				} else {
					fmt.Printf("🔊 Loaded sound: %s\n", name)
				}
			}
		}
	}
}

func findSoundDir() (string, error) {
	// Priority list of paths to check
	possiblePaths := []string{
		"build/bin/sounds",                 // Dev: standard relative path
		"../build/bin/sounds",              // Dev: alternative relative
		"sounds",                           // Binary: adjacent folder
		"/usr/share/beamsync/sounds",       // Linux: system install
		"/usr/local/share/beamsync/sounds", // Linux: local install
	}

	// Also check executable path
	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		possiblePaths = append([]string{filepath.Join(exeDir, "sounds")}, possiblePaths...)
		// Check "resources" folder for mac/windows style if needed later
		possiblePaths = append([]string{filepath.Join(exeDir, "../Resources/sounds")}, possiblePaths...)
	}

	for _, path := range possiblePaths {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			absPath, _ := filepath.Abs(path)
			return absPath, nil
		}
	}
	return "", fmt.Errorf("no valid sound directory found")
}

// processEvents handles events on a safe goroutine
func (a *App) processEvents() {
	for event := range a.eventChan {
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
