package beamsync

import (
	"crypto/x509"
	"net"
	"testing"
)

func TestTLSEnabledReadsEnvironment(t *testing.T) {
	t.Setenv(tlsEnvVar, "true")
	if !TLSEnabled() {
		t.Fatal("TLSEnabled() = false, want true")
	}
	if got := ServerScheme(); got != "https" {
		t.Fatalf("ServerScheme() = %q, want https", got)
	}

	t.Setenv(tlsEnvVar, "false")
	if TLSEnabled() {
		t.Fatal("TLSEnabled() = true, want false")
	}
	if got := ServerScheme(); got != "http" {
		t.Fatalf("ServerScheme() = %q, want http", got)
	}
}

func TestGenerateSelfSignedCertificateIncludesHosts(t *testing.T) {
	cert, err := generateSelfSignedCertificate([]string{"localhost", "127.0.0.1"})
	if err != nil {
		t.Fatalf("generateSelfSignedCertificate returned error: %v", err)
	}
	if len(cert.Certificate) == 0 {
		t.Fatal("generated certificate has no DER chain")
	}

	parsed, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("failed to parse generated certificate: %v", err)
	}

	if err := parsed.VerifyHostname("localhost"); err != nil {
		t.Fatalf("certificate does not verify for localhost: %v", err)
	}
	if err := parsed.VerifyHostname("127.0.0.1"); err != nil {
		t.Fatalf("certificate does not verify for 127.0.0.1: %v", err)
	}
}

func TestMaybeTLSListenerWrapsOnlyWhenEnabled(t *testing.T) {
	plain, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer plain.Close()

	t.Setenv(tlsEnvVar, "")
	wrapped, enabled, err := maybeTLSListener(plain)
	if err != nil {
		t.Fatalf("maybeTLSListener disabled returned error: %v", err)
	}
	if enabled || wrapped != plain {
		t.Fatal("disabled TLS should return original listener")
	}

	secure, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create TLS listener: %v", err)
	}
	defer secure.Close()

	t.Setenv(tlsEnvVar, "1")
	tlsWrapped, enabled, err := maybeTLSListener(secure)
	if err != nil {
		t.Fatalf("maybeTLSListener enabled returned error: %v", err)
	}
	if !enabled {
		t.Fatal("enabled TLS should report enabled=true")
	}
	if tlsWrapped == secure {
		t.Fatal("enabled TLS should wrap the listener")
	}
}
