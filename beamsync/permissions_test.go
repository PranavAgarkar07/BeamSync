package beamsync

import "testing"

func TestIsDeviceBlocked(t *testing.T) {
	s := TransferSettings{
		BlockedDevices: []DeviceRule{
			{IP: "192.168.1.100", FriendlyName: "BlockedPhone"},
			{IP: "10.0.0.5", FriendlyName: "BlockedLaptop"},
		},
	}
	tests := []struct {
		ip    string
		want  bool
	}{
		{"192.168.1.100", true},
		{"10.0.0.5", true},
		{"192.168.1.101", false},
		{"10.0.0.1", false},
		{"", false},
	}
	for _, tc := range tests {
		got := s.isDeviceBlocked(tc.ip)
		if got != tc.want {
			t.Errorf("isDeviceBlocked(%q) = %v, want %v", tc.ip, got, tc.want)
		}
	}
}

func TestIsDeviceTrusted(t *testing.T) {
	s := TransferSettings{
		TrustedDevices: []DeviceRule{
			{IP: "192.168.1.50", FriendlyName: "TrustedPhone"},
			{IP: "10.0.0.10", FriendlyName: "TrustedDesktop"},
		},
	}
	tests := []struct {
		ip    string
		want  bool
	}{
		{"192.168.1.50", true},
		{"10.0.0.10", true},
		{"192.168.1.51", false},
		{"10.0.0.1", false},
		{"", false},
	}
	for _, tc := range tests {
		got := s.isDeviceTrusted(tc.ip)
		if got != tc.want {
			t.Errorf("isDeviceTrusted(%q) = %v, want %v", tc.ip, got, tc.want)
		}
	}
}

func TestIsExtensionBlocked(t *testing.T) {
	s := TransferSettings{
		BlockedExtensions: []string{".exe", ".bat", ".cmd"},
	}
	tests := []struct {
		filename string
		want     bool
	}{
		{"virus.exe", true},
		{"script.bat", true},
		{"command.cmd", true},
		{"document.pdf", false},
		{"image.png", false},
		{"file.EXE", true},
		{"EXE", false},
		{"", false},
	}
	for _, tc := range tests {
		got := s.isExtensionBlocked(tc.filename)
		if got != tc.want {
			t.Errorf("isExtensionBlocked(%q) = %v, want %v", tc.filename, got, tc.want)
		}
	}
}

func TestFriendlyNameForIP(t *testing.T) {
	s := TransferSettings{
		TrustedDevices: []DeviceRule{
			{IP: "192.168.1.50", FriendlyName: "MyPhone"},
			{IP: "10.0.0.10"},
		},
	}
	tests := []struct {
		ip   string
		want string
	}{
		{"192.168.1.50", "MyPhone"},
		{"192.168.1.51", "192.168.1.51"},
		{"10.0.0.10", "10.0.0.10"},
		{"", ""},
	}
	for _, tc := range tests {
		got := s.friendlyNameForIP(tc.ip)
		if got != tc.want {
			t.Errorf("friendlyNameForIP(%q) = %q, want %q", tc.ip, got, tc.want)
		}
	}
}

func TestDefaultTransferSettings(t *testing.T) {
	ds := DefaultTransferSettings()
	if ds.Mode != TransferModeAskFirst {
		t.Errorf("DefaultTransferSettings().Mode = %q, want %q", ds.Mode, TransferModeAskFirst)
	}
	if ds.MaxFileSizeMB != 0 {
		t.Errorf("DefaultTransferSettings().MaxFileSizeMB = %d, want 0", ds.MaxFileSizeMB)
	}
	if ds.MinFreeSpaceMB != 100 {
		t.Errorf("DefaultTransferSettings().MinFreeSpaceMB = %d, want 100", ds.MinFreeSpaceMB)
	}
	if ds.BlockedExtensions == nil {
		t.Error("DefaultTransferSettings().BlockedExtensions = nil, want empty slice")
	}
	if ds.TrustedDevices == nil {
		t.Error("DefaultTransferSettings().TrustedDevices = nil, want empty slice")
	}
	if ds.BlockedDevices == nil {
		t.Error("DefaultTransferSettings().BlockedDevices = nil, want empty slice")
	}
}
