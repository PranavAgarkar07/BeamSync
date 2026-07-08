package beamsync

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
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
	if _, ok := parsed.PublicKey.(*ecdsa.PublicKey); !ok {
		t.Fatalf("certificate public key type = %T, want *ecdsa.PublicKey", parsed.PublicKey)
	}
	if validity := parsed.NotAfter.Sub(parsed.NotBefore); validity < 9*365*24*time.Hour {
		t.Fatalf("certificate validity = %s, want about 10 years", validity)
	}
}

func TestLoadOrCreateLocalCertificatePersistsSecureECDSAFiles(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	cert, err := loadOrCreateLocalCertificate([]string{"localhost"})
	if err != nil {
		t.Fatalf("loadOrCreateLocalCertificate returned error: %v", err)
	}
	if len(cert.Certificate) == 0 {
		t.Fatal("persisted certificate has no DER chain")
	}

	certPath := filepath.Join(homeDir, ".config", "beamsync", "cert.pem")
	keyPath := filepath.Join(homeDir, ".config", "beamsync", "key.pem")
	assertFileMode(t, certPath, tlsCertificateFileMode)
	assertFileMode(t, keyPath, tlsCertificateFileMode)

	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("failed to read persisted key: %v", err)
	}
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil || keyBlock.Type != "EC PRIVATE KEY" {
		t.Fatalf("key PEM type = %v, want EC PRIVATE KEY", keyBlock)
	}
	if _, err := x509.ParseECPrivateKey(keyBlock.Bytes); err != nil {
		t.Fatalf("persisted key is not a valid ECDSA key: %v", err)
	}

	certPEMBefore, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("failed to read persisted cert: %v", err)
	}
	keyPEMBefore := append([]byte(nil), keyPEM...)

	if _, err := loadOrCreateLocalCertificate([]string{"localhost"}); err != nil {
		t.Fatalf("second loadOrCreateLocalCertificate returned error: %v", err)
	}

	certPEMAfter, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("failed to read persisted cert after reload: %v", err)
	}
	keyPEMAfter, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("failed to read persisted key after reload: %v", err)
	}
	if string(certPEMBefore) != string(certPEMAfter) {
		t.Fatal("certificate was regenerated instead of reused")
	}
	if string(keyPEMBefore) != string(keyPEMAfter) {
		t.Fatal("private key was regenerated instead of reused")
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

func TestCertificateHostFilteringKeepsOnlyRelevantLANIPs(t *testing.T) {
	cases := []struct {
		name string
		ip   string
		want bool
	}{
		{name: "loopback ipv4", ip: "127.0.0.1", want: true},
		{name: "private class a", ip: "10.2.3.4", want: true},
		{name: "private class b", ip: "172.20.1.9", want: true},
		{name: "private class c", ip: "192.168.1.20", want: true},
		{name: "link local ipv6", ip: "fe80::1", want: true},
		{name: "public ipv4", ip: "8.8.8.8", want: false},
		{name: "carrier nat", ip: "100.64.0.1", want: false},
		{name: "public ipv6", ip: "2001:4860:4860::8888", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isRelevantCertificateIP(net.ParseIP(tc.ip))
			if got != tc.want {
				t.Fatalf("isRelevantCertificateIP(%s) = %v, want %v", tc.ip, got, tc.want)
			}
		})
	}
}

func TestCertificateHostFilteringSkipsVirtualInterfaces(t *testing.T) {
	for _, name := range []string{"docker0", "br-1234", "vethabc", "tun0", "tap1", "wg0", "vboxnet0", "Hyper-V Adapter", "tailscale0"} {
		t.Run(name, func(t *testing.T) {
			if !shouldSkipCertificateInterface(name) {
				t.Fatalf("shouldSkipCertificateInterface(%q) = false, want true", name)
			}
		})
	}

	for _, name := range []string{"eth0", "en0", "wlan0", "Wi-Fi"} {
		t.Run(name, func(t *testing.T) {
			if shouldSkipCertificateInterface(name) {
				t.Fatalf("shouldSkipCertificateInterface(%q) = true, want false", name)
			}
		})
	}
}

func assertFileMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %o, want %o", path, got, want)
	}
}
