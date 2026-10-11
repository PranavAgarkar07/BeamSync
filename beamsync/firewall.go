package beamsync

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

//go:embed firewall_setup.sh
var firewallScript []byte

// windowsFirewallPorts mirrors the range opened by firewall_setup.sh on Linux.
const windowsFirewallPorts = "3000-3100"

// FirewallManualSteps returns OS-specific manual instructions that the app
// shows the user when automatic firewall setup fails, is cancelled, or is
// unavailable. This is the user-facing failover: never fail silently.
func FirewallManualSteps() string {
	switch runtime.GOOS {
	case "windows":
		return "Allow TCP ports 3000-3100: Windows Security > Firewall & network " +
			"protection > Allow an app through firewall (enable BeamSync on Private " +
			"networks), or admin PowerShell: New-NetFirewallRule -DisplayName " +
			"BeamSync -Direction Inbound -Protocol TCP -LocalPort 3000-3100 -Action Allow"
	case "darwin":
		return "Allow BeamSync in System Settings > Network > Firewall, and grant " +
			"Local Network access in System Settings > Privacy & Security > Local Network."
	default:
		return "Allow TCP ports 3000-3100: sudo ufw allow 3000:3100/tcp " +
			"(or firewalld: firewall-cmd --permanent --add-port=3000-3100/tcp && " +
			"firewall-cmd --reload)"
	}
}

// RunFirewallSetup opens the BeamSync TCP port range in the host firewall.
// On Linux it runs the embedded firewall_setup.sh via pkexec (ufw/firewalld/iptables).
// On Windows it adds a Windows Defender Firewall inbound rule via netsh
// (requires an elevated process; otherwise returns an error telling the user
// what to allow, since the first-run Windows Firewall prompt is the normal path).
func RunFirewallSetup() error {
	if runtime.GOOS == "windows" {
		return runWindowsFirewallSetup()
	}
	fmt.Println("🛡️ Initiating Firewall Setup...")

	tmpDir, err := os.MkdirTemp("", "beamsync-firewall-*")
	if err != nil {
		return fmt.Errorf("cannot create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	scriptPath := filepath.Join(tmpDir, "firewall_setup.sh")
	if err := os.WriteFile(scriptPath, firewallScript, 0755); err != nil { //nolint:gosec // script needs exec
		return fmt.Errorf("cannot write firewall script: %w", err)
	}

	cmd := exec.Command("pkexec", scriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("firewall setup failed: %w\nOutput: %s", err, string(output)) //nolint:errorlint // output is additional context
	}

	fmt.Println("✅ Firewall Setup Output:\n", string(output))
	return nil
}

// runWindowsFirewallSetup adds an inbound Windows Defender Firewall rule for the
// BeamSync TCP port range. Must run elevated; on failure it returns an error
// with the manual netsh command so the user can allow it themselves.
func runWindowsFirewallSetup() error {
	fmt.Println("🛡️ Initiating Firewall Setup (Windows)...")
	cmd := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name=BeamSync", "dir=in", "action=allow", "protocol=TCP",
		"localport="+windowsFirewallPorts)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("firewall setup failed (run as administrator or allow BeamSync in Windows Defender Firewall > Allow an app, TCP ports %s): %w\nOutput: %s", windowsFirewallPorts, err, string(output)) //nolint:errorlint // output is additional context
	}

	fmt.Println("✅ Firewall Setup Output:\n", string(output))
	return nil
}
