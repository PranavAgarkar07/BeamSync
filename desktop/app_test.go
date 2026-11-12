package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestShareURLEmbedsStoredEndpoint is the first slice of the desktop app test
// suite (TST-04). It pins the shareURL happy path: the endpoint URL is built
// entirely from the stored IP, port, and token — no network lookup when the IP
// is already known.
func TestShareURLEmbedsStoredEndpoint(t *testing.T) {
	t.Setenv("BEAMSYNC_ENABLE_TLS", "")

	tests := []struct {
		name string
		port string
		want string
	}{
		{
			name: "stored ip port and token",
			port: "8080",
			want: "http://192.168.1.10:8080/?token=tok",
		},
		{
			name: "empty port",
			port: "",
			want: "http://192.168.1.10:/?token=tok",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := &App{currentIP: "192.168.1.10"}
			got := app.shareURL(tc.port, "tok")
			if got != tc.want {
				t.Fatalf("shareURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExtractTokenFromURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "plain query token", raw: "http://192.168.1.10:8080/?token=abc123", want: "abc123"},
		{name: "ampersand terminates token", raw: "http://192.168.1.10:8080/?token=abc&x=1", want: "abc"},
		{name: "hash terminates token", raw: "http://192.168.1.10:8080/?token=abc#frag", want: "abc"},
		{name: "whitespace is trimmed", raw: "http://192.168.1.10:8080/?token=  abc123  ", want: "abc123"},
		{name: "missing token", raw: "http://192.168.1.10:8080/", want: ""},
		{name: "empty token segment", raw: "http://192.168.1.10:8080/?token=", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractTokenFromURL(tc.raw); got != tc.want {
				t.Fatalf("extractTokenFromURL(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestExtractPortFromURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "strips scheme and path", raw: "http://192.168.1.10:8080/share?token=tok", want: "8080"},
		{name: "no port means empty", raw: "http://192.168.1.10/share?token=tok", want: ""},
		{name: "no colon means empty", raw: "192.168.1.10/share?token=tok", want: ""},
		{name: "ipv6 literal with port", raw: "http://[::1]:8080/share?token=tok", want: "8080"},
		{name: "ipv6 literal without port", raw: "http://[::1]/share?token=tok", want: ""},
		{name: "ipv6 literal without port or path", raw: "http://[fe80::1]", want: ""},
		{name: "ipv6 literal with port and no path", raw: "http://[fe80::1]:3000", want: "3000"},
		{name: "port only", raw: "http://192.168.1.10:9090", want: "9090"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractPortFromURL(tc.raw); got != tc.want {
				t.Fatalf("extractPortFromURL(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

// redirectUserConfigHome points every OS-specific user config/home directory
// the way Go resolves them at runtime (XDG_CONFIG_HOME, AppData, USERPROFILE
// fallbacks) at a single temp directory, so config round-trip tests never touch
// the developer's real config file.
//
// It asserts the redirection actually took effect. Without that check a silent
// miss (an env var Go does not consult on this platform) would let the test
// overwrite the developer's real config while still passing.
func redirectUserConfigHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("APPDATA", dir)
	t.Setenv("LOCALAPPDATA", dir)
	t.Setenv("USERPROFILE", dir)

	// configPath() must resolve inside the temp dir, not the real user dir.
	got := configPath()
	if !strings.HasPrefix(filepath.Clean(got), filepath.Clean(dir)) {
		t.Fatalf("configPath() = %q, want a path under the redirected temp dir %q", got, dir)
	}
	if _, err := os.Stat(filepath.Dir(got)); err != nil && !os.IsNotExist(err) {
		t.Fatalf("stat redirected config dir %q: %v", filepath.Dir(got), err)
	}
}

func TestLoadConfigRoundTripsSavePathInsideHome(t *testing.T) {
	home := t.TempDir()
	redirectUserConfigHome(t, home)

	cfg := configData{SavePath: filepath.Join(home, "Downloads", "BeamSync")}
	if err := saveConfig(cfg); err != nil {
		t.Fatalf("saveConfig returned error: %v", err)
	}

	got := loadConfig()
	if got.SavePath != cfg.SavePath {
		t.Fatalf("loadConfig().SavePath = %q, want %q", got.SavePath, cfg.SavePath)
	}
}

func TestLoadConfigClearsSavePathOutsideHome(t *testing.T) {
	home := t.TempDir()
	redirectUserConfigHome(t, home)

	if err := saveConfig(configData{SavePath: "/outside/BeamSync"}); err != nil {
		t.Fatalf("saveConfig returned error: %v", err)
	}

	got := loadConfig()
	if got.SavePath != "" {
		t.Fatalf("loadConfig().SavePath = %q, want empty for out-of-home save path", got.SavePath)
	}
}

// TestLoadConfigClearsSavePathInHomeSibling pins the path-boundary guard: a
// sibling of the home directory that merely shares its string prefix
// ("<home>-evil") is NOT inside the home and must be cleared.
//
// A raw strings.HasPrefix guard accepted "<home>-evil/..." as inside home,
// because the prefix matched. The guard now compares on a path separator
// boundary.
func TestLoadConfigClearsSavePathInHomeSibling(t *testing.T) {
	home := t.TempDir()
	redirectUserConfigHome(t, home)

	sibling := home + "-evil" + string(filepath.Separator) + "BeamSync"
	if err := saveConfig(configData{SavePath: sibling}); err != nil {
		t.Fatalf("saveConfig returned error: %v", err)
	}

	got := loadConfig()
	if got.SavePath != "" {
		t.Fatalf("loadConfig().SavePath = %q, want empty for home-sibling path %q", got.SavePath, sibling)
	}
}

func TestEnsureSaveDirRejectsEmptyPath(t *testing.T) {
	if err := ensureSaveDir(""); err == nil {
		t.Fatal("ensureSaveDir(\"\") = nil, want error")
	}
}

func TestEnsureSaveDirCreatesMissingDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "BeamSync")
	if err := ensureSaveDir(path); err != nil {
		t.Fatalf("ensureSaveDir(%q) returned error: %v", path, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat save dir: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("save dir = %v, want directory", info)
	}
}

func TestEnsureSaveDirReplacesFileWithDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "BeamSync")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir parent: %v", err)
	}
	if err := os.WriteFile(path, []byte("payload"), 0600); err != nil { //nolint:gosec
		t.Fatalf("write save-path file: %v", err)
	}

	if err := ensureSaveDir(path); err != nil {
		t.Fatalf("ensureSaveDir(%q) returned error: %v", path, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat save dir after replace: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("save path = %v, want directory after replacement", info)
	}

	backupInfo, err := os.Stat(path + ".bin.bak")
	if err != nil {
		t.Fatalf("backup file missing after rename: %v", err)
	}
	if !backupInfo.Mode().IsRegular() {
		t.Fatalf("backup = %v, want regular file", backupInfo)
	}
}
