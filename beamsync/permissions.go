package beamsync

import "strings"

// TransferMode controls how incoming file transfers are handled.
type TransferMode string

const (
	TransferModeAcceptAll   TransferMode = "accept_all"   // auto-accept everything
	TransferModeAskFirst    TransferMode = "ask_first"    // show prompt for every transfer
	TransferModeTrustedOnly TransferMode = "trusted_only" // auto-accept from approved IPs only
	TransferModeBlockAll    TransferMode = "block_all"    // reject all incoming transfers
)

// DeviceRule identifies a remote device by IP address with an optional friendly name.
type DeviceRule struct {
	IP           string `json:"ip"`
	FriendlyName string `json:"friendlyName"`
}

// TransferSettings holds the user-configured transfer permission preferences.
type TransferSettings struct {
	Mode              TransferMode `json:"mode"`
	MaxFileSizeMB     int64        `json:"maxFileSizeMB"`    // 0 = unlimited
	BlockedExtensions []string     `json:"blockedExtensions"` // e.g. [".exe", ".bat"]
	TrustedDevices    []DeviceRule `json:"trustedDevices"`
	BlockedDevices    []DeviceRule `json:"blockedDevices"`
}

// DefaultTransferSettings returns safe, user-friendly defaults.
func DefaultTransferSettings() TransferSettings {
	return TransferSettings{
		Mode:              TransferModeAskFirst,
		MaxFileSizeMB:     0,
		BlockedExtensions: []string{},
		TrustedDevices:    []DeviceRule{},
		BlockedDevices:    []DeviceRule{},
	}
}

// isDeviceBlocked returns true if the given IP is on the blocked list.
func (s *TransferSettings) isDeviceBlocked(ip string) bool {
	for _, d := range s.BlockedDevices {
		if d.IP == ip {
			return true
		}
	}
	return false
}

// isDeviceTrusted returns true if the given IP is on the trusted list.
func (s *TransferSettings) isDeviceTrusted(ip string) bool {
	for _, d := range s.TrustedDevices {
		if d.IP == ip {
			return true
		}
	}
	return false
}

// isExtensionBlocked returns true if the file extension is on the block list.
func (s *TransferSettings) isExtensionBlocked(filename string) bool {
	lower := strings.ToLower(filename)
	for _, ext := range s.BlockedExtensions {
		if strings.HasSuffix(lower, strings.ToLower(ext)) {
			return true
		}
	}
	return false
}

// friendlyNameForIP returns the friendly name for a device, or the IP itself.
func (s *TransferSettings) friendlyNameForIP(ip string) string {
	for _, d := range s.TrustedDevices {
		if d.IP == ip && d.FriendlyName != "" {
			return d.FriendlyName
		}
	}
	return ip
}
