package beamsync

import (
	"context"
	"embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

type EventCallback func(eventName string, data string)

var sendEvent EventCallback

var (
	lastHeartbeat time.Time
	isConnected   bool = false
	stateMutex    sync.Mutex
)

//go:embed ui/*.html
var uiFS embed.FS

func SetEventCallback(callback EventCallback) {
	sendEvent = callback
	fmt.Println("🔧 Event callback registered")
}

// safeEmit is now package-level to be shared between Receiver and Sender
func safeEmit(event, data string) {
	go func(evt, dt string) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("⚠️ Event callback panic: %v\n", r)
			}
		}()

		fmt.Printf("📡 Emitting event: %s | data: %s\n", evt, dt)

		if sendEvent != nil {
			sendEvent(evt, dt)
			fmt.Printf("✅ Event emitted successfully: %s\n", evt)
		}
	}(event, data)
}

// HTTPServer wraps http.Server so we can shut it down
type HTTPServer struct {
	server *http.Server
	cancel context.CancelFunc
}

func (s *HTTPServer) Shutdown() error {
	if s.cancel != nil {
		s.cancel()
	}
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

// StartServer using standard net/http (GTK-compatible)
func StartServer(uploadDir string) (*HTTPServer, string) {
	fmt.Println("🚀 StartServer() called")

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("🚨 PANIC IN StartServer: %v\n", r)
			fmt.Printf("Stack trace:\n%s\n", debug.Stack())
		}
	}()

	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			fmt.Println("❌ Failed to create upload directory:", err)
			return nil, ""
		}
	}
	fmt.Printf("📁 Upload directory: %s\n", uploadDir)

	fmt.Printf("📁 Upload directory: %s\n", uploadDir)

	ctx, cancel := context.WithCancel(context.Background())

	// Watchdog
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("⚠️ Watchdog panic: %v\n", r)
			}
		}()

		fmt.Println("👁️ Watchdog started")

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				fmt.Println("🛑 Watchdog stopped")
				return
			case <-ticker.C:
				stateMutex.Lock()
				connected := isConnected
				last := lastHeartbeat
				stateMutex.Unlock()

				if connected && time.Since(last) > 15*time.Second {
					stateMutex.Lock()
					if isConnected {
						isConnected = false
						stateMutex.Unlock()
						safeEmit("device_disconnected", "")
						fmt.Println("💔 Device Disconnected (Timeout)")
					} else {
						stateMutex.Unlock()
					}
				}
			}
		}
	}()

	mux := http.NewServeMux()

	// Heartbeat endpoint
	mux.HandleFunc("/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		fmt.Println("💓 Heartbeat received")
		stateMutex.Lock()
		lastHeartbeat = time.Now()
		wasConnected := isConnected
		if !isConnected {
			isConnected = true
		}
		stateMutex.Unlock()

		if !wasConnected {
			safeEmit("device_connected", "Android Device")
			fmt.Println("💚 Device Connected!")
		}
		w.WriteHeader(http.StatusOK)
	})

	// Serve UI
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		fmt.Println("🌐 GET / - Serving UI")
		w.Header().Set("Content-Type", "text/html")

		content, err := uiFS.ReadFile("ui/upload.html")
		if err != nil {
			http.Error(w, "UI Load Error", http.StatusInternalServerError)
			return
		}
		w.Write(content)
	})

	// Upload handler
	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("📤 POST /upload - Upload started")

		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("❌ PANIC in upload handler: %v\n", r)
				fmt.Printf("Stack trace:\n%s\n", debug.Stack())
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Update heartbeat
		stateMutex.Lock()
		lastHeartbeat = time.Now()
		if !isConnected {
			isConnected = true
		}
		stateMutex.Unlock()

		// Parse multipart form with 20GB limit
		err := r.ParseMultipartForm(20 * 1024 * 1024 * 1024)
		if err != nil {
			fmt.Println("❌ Failed to parse multipart form:", err)
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}

		fmt.Println("✅ Multipart form parsed successfully")

		files := r.MultipartForm.File["documents"]
		if len(files) == 0 {
			http.Error(w, "No files uploaded", http.StatusBadRequest)
			return
		}

		fmt.Printf("📦 Processing %d file(s)\n", len(files))

		for i, fileHeader := range files {
			fmt.Printf("📄 Processing file #%d: %s\n", i+1, fileHeader.Filename)

			file, err := fileHeader.Open()
			if err != nil {
				fmt.Println("❌ Failed to open file:", err)
				continue
			}

			filename := filepath.Base(fileHeader.Filename)
			if filename == "" || filename == "." {
				filename = fmt.Sprintf("upload_%d.bin", time.Now().Unix())
			}

			dstPath := filepath.Join(uploadDir, filename)
			fmt.Printf("💾 Saving to: %s\n", dstPath)

			dst, err := os.Create(dstPath)
			if err != nil {
				fmt.Println("❌ File creation error:", err)
				file.Close()
				continue
			}

			written, err := io.Copy(dst, file)
			dst.Close()
			file.Close()

			if err != nil {
				fmt.Println("❌ Copy error:", err)
				continue
			}

			fmt.Printf("✅ File saved: %s (%d bytes)\n", filename, written)

			// Emit event asynchronously
			go func(fname string) {
				time.Sleep(100 * time.Millisecond)
				safeEmit("file_received", fname)
			}(filename)
		}

		fmt.Println("✅ Upload handler completed successfully")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("✅ Upload Complete"))
		fmt.Println("🔄 Server still running, waiting for more requests...")
	})

	// Find an available EVEN port for Receiver (3000, 3002, ...)
	portInt, listener, err := FindAvailablePort(3000, 2, 50)
	if err != nil {
		fmt.Println("❌ Failed to find available port for Receiver:", err)

		// Attempt auto-fix for permissions
		if strings.Contains(err.Error(), "permission") || strings.Contains(err.Error(), "access") {
			fmt.Println("🔒 Permission error detected. Attempting to run firewall setup...")
			if fwErr := RunFirewallSetup(); fwErr != nil {
				fmt.Printf("❌ Firewall setup failed: %v\n", fwErr)
			} else {
				fmt.Println("✅ Firewall setup completed. Retrying port binding...")
				portInt, listener, err = FindAvailablePort(3000, 2, 50)
				if err != nil {
					fmt.Println("❌ Still failed to find port after firewall setup:", err)
					cancel()
					return nil, ""
				}
			}
		} else {
			cancel() // Fix context leak
			return nil, ""
		}
	}
	portStr := fmt.Sprintf("%d", portInt)

	server := &http.Server{
		Handler: mux,
	}

	httpServer := &HTTPServer{server: server, cancel: cancel}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("❌ Server panic: %v\n", r)
			}
		}()

		fmt.Printf("🚀 Starting HTTP server on :%s...\n", portStr)
		// Use Serve instead of ListenAndServe since we already have a listener
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Printf("❌ Server error: %v\n", err)
		}
	}()

	fmt.Println("✅ StartServer() completed")
	return httpServer, portStr
}

// StartSender remains with Fiber (sender doesn't have the same issue)
// StartSender with Heartbeat support
func StartSender(filePaths []string) (*HTTPServer, string) {
	mux := http.NewServeMux()

	// 1. Heartbeat Handler (same as Receiver)
	mux.HandleFunc("/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		fmt.Println("💓 Sender Heartbeat received")
		stateMutex.Lock()
		lastHeartbeat = time.Now()
		wasConnected := isConnected
		if !isConnected {
			isConnected = true
		}
		stateMutex.Unlock()

		if !wasConnected {
			// Reuse the same event for simplicity, or add a specific one
			safeEmit("device_connected", "Mobile (Downloader)")
			fmt.Println("💚 Device Connected to Sender!")
		}
		w.WriteHeader(http.StatusOK)
	})

	// 2. Serve Files
	if len(filePaths) == 1 {
		filePath := filePaths[0]
		filename := filepath.Base(filePath)

		// Serve the actual file at a specific path
		mux.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
			http.ServeFile(w, r, filePath)
		})

		// Serve HTML page with Heartbeat script
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
			w.Header().Set("Content-Type", "text/html")

			content, err := uiFS.ReadFile("ui/download.html")
			if err != nil {
				http.Error(w, "UI Load Error", http.StatusInternalServerError)
				return
			}

			// Simple Single File Template
			html := string(content)
			fileBlock := fmt.Sprintf(`<div class="file-card">
				<div class="file-info">%s</div>
				<a href="/download" class="download-btn" onclick="startDownload()">⬇️ SAVE</a>
			</div>
			<script>function startDownload() { setTimeout(() => alert("Download Started"), 500); }</script>`, filename)

			html = strings.Replace(html, "{{FILES}}", fileBlock, 1)
			w.Write([]byte(html))
		})
	} else {
		// Multi-file mode
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
			w.Header().Set("Content-Type", "text/html")

			content, err := uiFS.ReadFile("ui/download.html")
			if err != nil {
				http.Error(w, "UI Load Error", http.StatusInternalServerError)
				return
			}

			// Generate File List
			var builder strings.Builder
			for i, path := range filePaths {
				fname := filepath.Base(path)
				builder.WriteString(fmt.Sprintf(`<div class="file-card">
					<div class="file-info">%s</div>
					<a href="/download/%d" class="download-btn">⬇️ SAVE</a>
				</div>`, fname, i))
			}

			html := string(content)
			html = strings.Replace(html, "{{FILES}}", builder.String(), 1)
			w.Write([]byte(html))
		})

		for i, path := range filePaths {
			idx := i
			filePath := path
			mux.HandleFunc(fmt.Sprintf("/download/%d", idx), func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
				w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(filePath)))
				http.ServeFile(w, r, filePath)
			})
		}
	}

	// Find an available ODD port for Sender (3005, 3007, ...)
	portInt, listener, err := FindAvailablePort(3005, 2, 50)
	if err != nil {
		fmt.Println("❌ Failed to find available port for Sender:", err)

		// Attempt auto-fix for permissions
		if strings.Contains(err.Error(), "permission") || strings.Contains(err.Error(), "access") {
			fmt.Println("🔒 Permission error detected. Attempting to run firewall setup...")
			if fwErr := RunFirewallSetup(); fwErr != nil {
				fmt.Printf("❌ Firewall setup failed: %v\n", fwErr)
			} else {
				fmt.Println("✅ Firewall setup completed. Retrying port binding...")
				portInt, listener, err = FindAvailablePort(3005, 2, 50)
				if err != nil {
					fmt.Println("❌ Still failed to find port after firewall setup:", err)
					return nil, ""
				}
			}
		} else {
			return nil, ""
		}
	}
	portStr := fmt.Sprintf("%d", portInt)

	server := &http.Server{
		Handler: mux,
	}

	httpServer := &HTTPServer{server: server}

	go func() {
		fmt.Printf("🚀 Starting sender on :%s...\n", portStr)
		// Use Serve instead of ListenAndServe since we already have a listener
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Println("❌ Sender error:", err)
		}
	}()

	return httpServer, portStr
}
