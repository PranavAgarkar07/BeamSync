package main

import "testing"

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
