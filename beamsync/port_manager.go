package beamsync

import (
	"fmt"
	"net"
	"os"
	"strings"
)

// FindAvailablePort tries to find a free port starting from startPort.
// It iterates by 'step' (e.g. 2 for even/odd only) up to maxAttempts.
// It returns the allocated port, the active listener, and any error.
// By default, binds to 127.0.0.1 (loopback) only. Set BEAMSYNC_LAN_MODE=true to bind to all interfaces.
func FindAvailablePort(startPort int, step int, maxAttempts int) (int, net.Listener, error) {
	bindHost := "127.0.0.1"
	if os.Getenv("BEAMSYNC_LAN_MODE") == "true" {
		bindHost = "0.0.0.0"
	}

	for i := 0; i < maxAttempts; i++ {
		port := startPort + (i * step)
		addr := fmt.Sprintf("%s:%d", bindHost, port)
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			fmt.Printf("🎯 Found available port: %d (bind: %s)\n", port, bindHost)
			return port, listener, nil
		}

		// If it's a permission error, don't keep trying, it's likely a system restriction
		// typically "bind: permission denied" or "listen tcp :3000: bind: permission denied"
		msg := err.Error()
		if len(msg) > 0 && (strings.Contains(msg, "permission denied") || strings.Contains(msg, "access denied")) {
			return 0, nil, err
		}

		fmt.Printf("⚠️ Port %d is busy/unavailable (%v), trying next...\n", port, err)
	}
	return 0, nil, fmt.Errorf("no available ports found after %d attempts", maxAttempts)
}
