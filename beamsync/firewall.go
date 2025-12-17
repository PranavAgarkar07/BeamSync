package beamsync

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// RunFirewallSetup attempts to run the firewall_setup.sh script using pkexec.
func RunFirewallSetup() error {
	fmt.Println("🛡️ Initiating Firewall Setup...")

	// 1. Locate the script
	// Try a few common locations:
	// - ./firewall_setup.sh (if copied next to binary)
	// - ./build/linux/firewall_setup.sh (dev environment)
	// - ../build/linux/firewall_setup.sh (dev environment alternate)

	potentialPaths := []string{
		"firewall_setup.sh",
		"build/linux/firewall_setup.sh",
		"../build/linux/firewall_setup.sh",
		// Add absolute path for safety in dev if needed found via inspection earlier
		"/home/pranav/Desktop/projects/golang/BeamSync/build/linux/firewall_setup.sh",
	}

	var scriptPath string
	for _, path := range potentialPaths {
		if _, err := os.Stat(path); err == nil {
			absPath, err := filepath.Abs(path)
			if err == nil {
				scriptPath = absPath
				break
			}
		}
	}

	if scriptPath == "" {
		return fmt.Errorf("firewall_setup.sh not found")
	}

	fmt.Printf("found script at: %s\n", scriptPath)

	// 2. Ensure it is executable
	if err := os.Chmod(scriptPath, 0755); err != nil {
		fmt.Printf("⚠️ Warning: Could not chmod script: %v\n", err)
	}

	// 3. Build the command
	// pkexec allows running commands as root with a GUI prompt
	cmd := exec.Command("pkexec", scriptPath)

	// Capture output
	// cmd.Stdout = os.Stdout // Let it write directly to terminal if possible?
	// Capturing is safer to avoid messing up the UI if we have one, but we are CLI based mostly now.
	// Let's capture comibined output
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("firewall setup failed: %v\nOutput: %s", err, string(output))
	}

	fmt.Println("✅ Firewall Setup Output:\n", string(output))
	return nil
}
