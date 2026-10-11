package beamsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Failover contract: startup failures must come back as user-facing errors
// (shown in-app with manual fix steps), never as silent nil servers.

func TestFirewallManualStepsMentionsPortRange(t *testing.T) {
	steps := FirewallManualSteps()
	if !strings.Contains(steps, "3000") {
		t.Fatalf("FirewallManualSteps() = %q, want port range mentioned", steps)
	}
}

func TestStartServerESurfacesSaveDirError(t *testing.T) {
	// A regular file where the upload dir should be: MkdirAll must fail.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatalf("write blocker fixture: %v", err)
	}

	server, port, token, err := StartServerE(blocker, 3000, DefaultTransferSettings(), nil)
	if err == nil {
		if server != nil {
			_ = server.Shutdown()
		}
		t.Fatal("StartServerE with file-as-dir: want error, got nil")
	}
	if server != nil || port != "" || token != "" {
		if server != nil {
			_ = server.Shutdown()
		}
		t.Fatal("StartServerE on error must return nil server with empty port/token")
	}
}

func TestStartServerESurfacesExhaustedPortRange(t *testing.T) {
	// Invalid start port: every bind attempt fails fast, exercising the
	// busy-range failover message (no firewall prompt involved).
	server, _, _, err := StartServerE(t.TempDir(), 99999, DefaultTransferSettings(), nil)
	if err == nil {
		if server != nil {
			_ = server.Shutdown()
		}
		t.Fatal("StartServerE with unusable port range: want error, got nil")
	}
	if !strings.Contains(err.Error(), "no free local port") {
		t.Fatalf("error = %q, want busy-range failover message", err.Error())
	}
	if server != nil {
		_ = server.Shutdown()
		t.Fatal("StartServerE on error must return nil server")
	}
}
