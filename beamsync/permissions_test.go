package beamsync

import "testing"

func TestDefaultTransferSettings(t *testing.T) {
	settings := DefaultTransferSettings()

	if settings.Mode != TransferModeAskFirst {
		t.Fatalf("default mode = %q, want %q", settings.Mode, TransferModeAskFirst)
	}
	if settings.MaxFileSizeMB != 0 {
		t.Fatalf("default max file size = %d, want unlimited", settings.MaxFileSizeMB)
	}
	if len(settings.BlockedExtensions) != 0 {
		t.Fatalf("default blocked extensions = %v, want empty", settings.BlockedExtensions)
	}
	if len(settings.TrustedDevices) != 0 {
		t.Fatalf("default trusted devices = %v, want empty", settings.TrustedDevices)
	}
	if len(settings.BlockedDevices) != 0 {
		t.Fatalf("default blocked devices = %v, want empty", settings.BlockedDevices)
	}
}

func TestTransferSettingsDeviceRules(t *testing.T) {
	settings := TransferSettings{
		TrustedDevices: []DeviceRule{{IP: "192.168.1.10", FriendlyName: "Phone"}},
		BlockedDevices: []DeviceRule{{IP: "192.168.1.11", FriendlyName: "Old laptop"}},
	}

	if !settings.isDeviceTrusted("192.168.1.10") {
		t.Fatal("expected trusted device to match")
	}
	if settings.isDeviceTrusted("192.168.1.99") {
		t.Fatal("unexpected trusted match for unknown device")
	}
	if !settings.isDeviceBlocked("192.168.1.11") {
		t.Fatal("expected blocked device to match")
	}
	if settings.isDeviceBlocked("192.168.1.99") {
		t.Fatal("unexpected blocked match for unknown device")
	}
	if got := settings.friendlyNameForIP("192.168.1.10"); got != "Phone" {
		t.Fatalf("friendlyNameForIP = %q, want Phone", got)
	}
	if got := settings.friendlyNameForIP("192.168.1.99"); got != "192.168.1.99" {
		t.Fatalf("friendlyNameForIP fallback = %q", got)
	}
}

func TestTransferSettingsBlockedExtensionsAreCaseInsensitive(t *testing.T) {
	settings := TransferSettings{
		BlockedExtensions: []string{".EXE", ".bat"},
	}

	for _, filename := range []string{"setup.exe", "SETUP.EXE", "script.BAT"} {
		if !settings.isExtensionBlocked(filename) {
			t.Fatalf("expected %q to be blocked", filename)
		}
	}
	if settings.isExtensionBlocked("notes.txt") {
		t.Fatal("did not expect .txt file to be blocked")
	}
}
