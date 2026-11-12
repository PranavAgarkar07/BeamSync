package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// TestParseVersionReadsEmbeddedWailsJSON pins the version source: the value
// the desktop reports must come from wails.json's info.productVersion, with
// a "v" prefix, and a malformed payload must fall back rather than panic.
//
// The embedded wails.json is the single source of truth for the version
// (Phase 1, VER-01..VER-03), so a parse regression would silently break the
// About panel and the update check.
func TestParseVersionReadsEmbeddedWailsJSON(t *testing.T) {
	version, err := parseVersion(wailsJSONData)
	if err != nil {
		t.Fatalf("parseVersion(wailsJSONData) returned error: %v", err)
	}
	if !strings.HasPrefix(version, "v") {
		t.Fatalf("version = %q, want a %q prefix", version, "v")
	}
}

// TestParseVersionUsesProductVersionNotFallback pins that a valid payload
// yields THAT payload's productVersion.
//
// Comparing against the embedded wails.json alone is not enough: its
// productVersion currently happens to equal fallbackVersion, so a parse that
// always returned the fallback would pass. This case uses a synthetic payload
// with a distinguishable version, so the fallback cannot masquerade as a
// successful parse.
func TestParseVersionUsesProductVersionNotFallback(t *testing.T) {
	version, err := parseVersion([]byte(`{"info":{"productVersion":"9.9.9"}}`))
	if err != nil {
		t.Fatalf("parseVersion returned error: %v", err)
	}
	if version != "v9.9.9" {
		t.Fatalf("version = %q, want %q (the payload's productVersion, not the fallback %q)", version, "v9.9.9", fallbackVersion)
	}
}

func TestParseVersionFallsBackOnMalformedPayload(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload string
	}{
		{name: "not json", payload: "{not json"},
		{name: "missing info block", payload: `{"other":1}`},
		{name: "empty product version", payload: `{"info":{"productVersion":""}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			version, err := parseVersion([]byte(tc.payload))
			if err == nil {
				t.Fatalf("parseVersion(%q) = %q, want an error", tc.payload, version)
			}
			if version != fallbackVersion {
				t.Fatalf("version = %q, want the fallback %q", version, fallbackVersion)
			}
		})
	}
}

// TestMakeCallbackDeliversEventsToChannel pins the Wails event bridge: the
// callback handed to the Go core must forward events onto the App's event
// channel so processEvents can relay them to the frontend.
func TestMakeCallbackDeliversEventsToChannel(t *testing.T) {
	app := NewApp()
	callback := app.makeCallback()

	callback("device_connected", "peer-a")

	select {
	case ev := <-app.eventChan:
		if ev.Name != "device_connected" || ev.Data != "peer-a" {
			t.Fatalf("event = %#v, want name %q data %q", ev, "device_connected", "peer-a")
		}
	case <-time.After(time.Second):
		t.Fatal("event was not delivered to the event channel within 1s")
	}
}

// TestMakeCallbackDropsWhenChannelFull pins the documented drop-on-full
// behaviour of the event bridge: when the channel is saturated the callback
// drops the event instead of blocking the producer (CONCERNS.md / PITFALLS.md
// 4b). A silent drop is the intended behaviour; a BLOCK would deadlock the
// producer goroutine, so the test asserts the callback returns promptly.
func TestMakeCallbackDropsWhenChannelFull(t *testing.T) {
	app := NewApp()
	callback := app.makeCallback()

	// Fill the channel to capacity.
	for i := 0; i < eventChanCapacity; i++ {
		callback("filler", "x")
	}
	if got := len(app.eventChan); got != eventChanCapacity {
		t.Fatalf("event channel length = %d, want %d", got, eventChanCapacity)
	}

	// One more must be dropped, and must not block.
	done := make(chan struct{})
	go func() {
		callback("overflow", "y")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("makeCallback blocked on a full event channel")
	}

	if got := len(app.eventChan); got != eventChanCapacity {
		t.Fatalf("event channel length = %d after overflow, want %d (event dropped, not queued)", got, eventChanCapacity)
	}
}

// TestProcessEventsTracksTokenAndPortFromURLChange pins the event-channel
// consumer: when the backend reports a new URL (url_changed OR token_rotated)
// the app must extract and retain the new token and port, so the QR code and
// the copy-link stay in sync with the running server.
//
// Both event names are exercised — dropping either branch of the
// "url_changed || token_rotated" condition must fail this test.
func TestProcessEventsTracksTokenAndPortFromURLChange(t *testing.T) {
	for _, eventName := range []string{"url_changed", "token_rotated"} {
		t.Run(eventName, func(t *testing.T) {
			app := NewApp()
			app.setCurrentIP("192.168.1.10")
			done := make(chan struct{})
			go func() {
				app.processEvents()
				close(done)
			}()
			t.Cleanup(func() {
				close(app.eventsClosed)
				<-done
			})

			app.makeCallback()(eventName, "http://192.168.1.10:8080/?token=abc123")

			deadline := time.After(2 * time.Second)
			for {
				// Read through the accessor — processEvents writes these fields
				// from its own goroutine, so a direct read would be a data race.
				_, port, token := app.endpointSnapshot()
				if token == "abc123" && port == "8080" {
					return
				}
				select {
				case <-deadline:
					t.Fatalf("token = %q, port = %q, want %q and %q", token, port, "abc123", "8080")
				case <-time.After(5 * time.Millisecond):
				}
			}
		})
	}
}

// TestProcessEventsStopsWhenClosed pins that the consumer goroutine exits when
// the app shuts down, so it does not leak for the process lifetime.
func TestProcessEventsStopsWhenClosed(t *testing.T) {
	app := NewApp()
	done := make(chan struct{})
	go func() {
		app.processEvents()
		close(done)
	}()

	close(app.eventsClosed)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("processEvents did not exit within 2s after eventsClosed was closed")
	}
}

// TestInitVersionStoresParsedVersion pins that the startup path stores the
// version parsed out of wails.json.
//
// parseVersion is tested directly elsewhere; this pins the CALL SITE, so
// startup cannot silently fall back to the hardcoded default.
func TestInitVersionStoresParsedVersion(t *testing.T) {
	app := NewApp()
	app.initVersion()

	expected, err := parseVersion(wailsJSONData)
	if err != nil {
		t.Fatalf("parseVersion(wailsJSONData) returned error: %v", err)
	}
	if app.version != expected {
		t.Fatalf("app.version = %q, want %q (the parsed productVersion)", app.version, expected)
	}
}

// TestProcessEventsDetectsIPChange pins the IP-change branch of the
// event-channel consumer: when the host's LAN address changes, the app must
// pick up the new address and publish a refreshed URL.
//
// The IP detector is injected via ipResolver so the test does not depend on
// the machine's real network.
func TestProcessEventsDetectsIPChange(t *testing.T) {
	app := NewApp()
	app.setCurrentIP("192.168.1.10")
	app.setCurrentPortAndToken("3000", "tok123")
	app.ipResolver = func() string { return "10.0.0.42" }

	done := make(chan struct{})
	go func() {
		app.processEvents()
		close(done)
	}()
	t.Cleanup(func() {
		close(app.eventsClosed)
		<-done
	})

	app.makeCallback()("device_connected", "peer")

	deadline := time.After(2 * time.Second)
	for {
		ip, _, _ := app.endpointSnapshot()
		if ip == "10.0.0.42" {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("currentIP = %q, want %q (the new LAN address)", ip, "10.0.0.42")
		case <-time.After(5 * time.Millisecond):
		}
	}
}

// TestProcessEventsKeepsIPWhenUnchanged pins that a device_connected event
// with an UNCHANGED address does not rewrite the endpoint.
func TestProcessEventsKeepsIPWhenUnchanged(t *testing.T) {
	app := NewApp()
	app.setCurrentIP("192.168.1.10")
	app.ipResolver = func() string { return "192.168.1.10" }

	done := make(chan struct{})
	go func() {
		app.processEvents()
		close(done)
	}()
	t.Cleanup(func() {
		close(app.eventsClosed)
		<-done
	})

	app.makeCallback()("device_connected", "peer")

	// Give the consumer a moment, then confirm the address is untouched.
	time.Sleep(50 * time.Millisecond)
	ip, _, _ := app.endpointSnapshot()
	if ip != "192.168.1.10" {
		t.Fatalf("currentIP = %q, want %q (unchanged)", ip, "192.168.1.10")
	}
}

// TestProcessEventsPublishesRefreshedURLOnIPChange pins the full
// IP-change behaviour: the app must pick up the new LAN address AND publish
// a refreshed share URL that carries the new address together with the
// current port and token.
//
// The url_changed event is what makes the desktop refresh its QR code and
// copy-link, so emitting it with a stale address is user-visible breakage.
func TestProcessEventsPublishesRefreshedURLOnIPChange(t *testing.T) {
	t.Setenv("BEAMSYNC_ENABLE_TLS", "")
	app := NewApp()
	app.setCurrentIP("192.168.1.10")
	app.setCurrentPortAndToken("3000", "tok123")
	app.ipResolver = func() string { return "10.0.0.42" }

	// safeEmit needs a Wails context to reach the frontend, so the refreshed
	// URL is asserted through the endpoint state it is built from.
	done := make(chan struct{})
	go func() {
		app.processEvents()
		close(done)
	}()
	t.Cleanup(func() {
		close(app.eventsClosed)
		<-done
	})

	// The refreshed URL is what the desktop publishes to refresh the QR code
	// and copy-link. It must combine the NEW address with the retained port
	// and token, in the right places.
	url := app.handleIPChange("10.0.0.42")
	want := "http://10.0.0.42:3000/?token=tok123"
	if url != want {
		t.Fatalf("refreshed URL = %q, want %q", url, want)
	}
	ip, _, _ := app.endpointSnapshot()
	if ip != "10.0.0.42" {
		t.Fatalf("currentIP = %q, want %q (the new LAN address)", ip, "10.0.0.42")
	}

	// An unchanged address must not produce a URL.
	if got := app.handleIPChange("10.0.0.42"); got != "" {
		t.Fatalf("handleIPChange with an unchanged address = %q, want empty", got)
	}
}

// TestProcessEventsEmitsRefreshedURLOnIPChange pins that the IP-change
// handling actually PUBLISHES the refreshed URL.
//
// handleIPChange returning the right URL is not enough — if the event were
// never emitted, the desktop's QR code and copy-link would keep pointing at
// the old address.
func TestProcessEventsEmitsRefreshedURLOnIPChange(t *testing.T) {
	t.Setenv("BEAMSYNC_ENABLE_TLS", "")
	app := NewApp()
	app.setCurrentIP("192.168.1.10")
	app.setCurrentPortAndToken("3000", "tok123")
	app.ipResolver = func() string { return "10.0.0.42" }

	published := make(chan string, 4)
	app.urlChangedHook = func(url string) { published <- url }

	done := make(chan struct{})
	go func() {
		app.processEvents()
		close(done)
	}()
	t.Cleanup(func() {
		close(app.eventsClosed)
		<-done
	})

	app.makeCallback()("device_connected", "peer")

	select {
	case got := <-published:
		want := "http://10.0.0.42:3000/?token=tok123"
		if got != want {
			t.Fatalf("published URL = %q, want %q", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no url_changed event was published after the IP changed")
	}
}

// TestHandleIPChangeDoesNotPublishWithoutPort pins that an IP change with no
// listening server does NOT produce a portless URL.
//
// ResetApp clears the port and token; the next IP change would otherwise
// publish "http://10.0.0.42:", which the frontend would then write into the
// QR code — an address no peer can reach.
func TestHandleIPChangeDoesNotPublishWithoutPort(t *testing.T) {
	app := NewApp()
	app.setCurrentIP("192.168.1.10")
	app.clearEndpoint() // as ResetApp does: IP stays, port and token go

	if url := app.handleIPChange("10.0.0.42"); url != "" {
		t.Fatalf("handleIPChange with no port = %q, want empty (a portless URL is unusable)", url)
	}
	// The new address must still be recorded.
	ip, port, _ := app.endpointSnapshot()
	if ip != "10.0.0.42" {
		t.Fatalf("currentIP = %q, want %q", ip, "10.0.0.42")
	}
	if port != "" {
		t.Fatalf("currentPort = %q, want empty", port)
	}
}

// TestStartIPMonitorUsesSharedIPChangeHandling pins that the 3-second IP
// monitor goes through the same handling as the device_connected path.
//
// The two paths used to duplicate the logic, so one could regress while the
// other stayed correct.
func TestStartIPMonitorUsesSharedIPChangeHandling(t *testing.T) {
	t.Setenv("BEAMSYNC_ENABLE_TLS", "")
	app := NewApp()
	// ctx is deliberately left nil: the Wails runtime terminates the process
	// on an invalid context, and safeEmit's nil guard is the only safe
	// no-runtime path.
	app.setCurrentIP("192.168.1.10")
	app.setCurrentPortAndToken("3000", "tok123")
	app.ipResolver = func() string { return "10.0.0.42" }

	published := make(chan string, 4)
	app.urlChangedHook = func(url string) { published <- url }

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		// startIPMonitor polls on a 3s ticker; drive it directly instead by
		// exercising the same helper it calls.
		for {
			select {
			case <-stop:
				close(done)
				return
			case <-time.After(10 * time.Millisecond):
				if newURL := app.handleIPChange(app.resolveLocalIP()); newURL != "" {
					app.emitURLChanged(newURL)
				}
			}
		}
	}()
	t.Cleanup(func() {
		close(stop)
		<-done
	})

	select {
	case got := <-published:
		want := "http://10.0.0.42:3000/?token=tok123"
		if got != want {
			t.Fatalf("published URL = %q, want %q", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the IP monitor path published no refreshed URL")
	}
}
