package beamsync

import "testing"

func TestServerSchemeDefaultsToHTTP(t *testing.T) {
	t.Setenv("BEAMSYNC_ENABLE_TLS", "")
	if got := ServerScheme(); got != "http" {
		t.Fatalf("ServerScheme = %q, want http", got)
	}
}

func TestServerSchemeUsesHTTPSWhenEnabled(t *testing.T) {
	for _, value := range []string{"1", "true", "yes", "on"} {
		t.Setenv("BEAMSYNC_ENABLE_TLS", value)
		if got := ServerScheme(); got != "https" {
			t.Fatalf("ServerScheme with %q = %q, want https", value, got)
		}
	}
}

func TestGenerateSelfSignedCertificate(t *testing.T) {
	cert, err := generateSelfSignedCertificate()
	if err != nil {
		t.Fatalf("generateSelfSignedCertificate returned error: %v", err)
	}
	if len(cert.Certificate) == 0 {
		t.Fatal("expected certificate chain")
	}
	if cert.PrivateKey == nil {
		t.Fatal("expected private key")
	}
}
