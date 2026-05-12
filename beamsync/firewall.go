package beamsync

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

//go:embed firewall_setup.sh
var firewallScript []byte

// RunFirewallSetup attempts to run the embedded firewall_setup.sh script using pkexec.
func RunFirewallSetup() error {
	Info("Initiating Firewall Setup...")

	// Extract embedded script to a temp file
	tmpDir, err := os.MkdirTemp("", "beamsync-firewall-*")
	if err != nil {
		return fmt.Errorf("cannot create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	scriptPath := filepath.Join(tmpDir, "firewall_setup.sh")
	if err := os.WriteFile(scriptPath, firewallScript, 0755); err != nil {
		return fmt.Errorf("cannot write firewall script: %w", err)
	}

	cmd := exec.Command("pkexec", scriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("firewall setup failed: %v\nOutput: %s", err, string(output))
	}

	Info("Firewall Setup Output:\n%s", string(output))
	return nil
}
