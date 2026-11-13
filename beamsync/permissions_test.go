package beamsync

import (
	"sync"
	"testing"
)

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

// TestUpdateSettingsIsRaceFreeWithConcurrentReaders pins that replacing the
// transfer settings while handlers read them is race-free.
//
// The settings carry slice fields (the device blocklist and trustlist), so a
// torn read could mis-apply the blocklist. UpdateSettings swaps the whole
// struct under a lock and the handlers read a snapshot.
func TestUpdateSettingsIsRaceFreeWithConcurrentReaders(t *testing.T) {
	server := &HTTPServer{settings: &TransferSettings{}}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			server.UpdateSettings(TransferSettings{
				Mode:              TransferModeAskFirst,
				BlockedDevices:    []DeviceRule{{IP: "192.168.1.99", FriendlyName: "blocked"}},
				TrustedDevices:    []DeviceRule{{IP: "192.168.1.50", FriendlyName: "trusted"}},
				BlockedExtensions: []string{".exe"},
				MaxFileSizeMB:     int64(100 + n),
			})
		}(i)
		go func() {
			defer wg.Done()
			current := server.currentSettings()
			if current != nil {
				_ = current.isDeviceBlocked("192.168.1.99")
				_ = current.isDeviceTrusted("192.168.1.50")
				_ = current.isExtensionBlocked("payload.exe")
			}
		}()
	}
	wg.Wait()
}

// TestSettingsReturnsSnapshotNotLivePointer pins that Settings() hands back a
// COPY, so a caller cannot mutate the server's live settings through it.
func TestSettingsReturnsSnapshotNotLivePointer(t *testing.T) {
	original := TransferSettings{Mode: TransferModeAskFirst}
	server := &HTTPServer{settings: &original}

	snapshot := server.Settings()
	if snapshot == nil {
		t.Fatal("Settings() returned nil")
	}
	snapshot.Mode = TransferModeBlockAll

	if original.Mode != TransferModeAskFirst {
		t.Fatalf("live settings were mutated through the snapshot: mode = %q", original.Mode)
	}
	if got := server.Settings().Mode; got != TransferModeAskFirst {
		t.Fatalf("server settings mode = %q, want %q", got, TransferModeAskFirst)
	}
}
