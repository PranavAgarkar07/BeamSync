package beamsync

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const tlsEnvVar = "BEAMSYNC_ENABLE_TLS"
const tlsCertificateValidity = 10 * 365 * 24 * time.Hour

func TLSEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(tlsEnvVar)))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func ServerScheme() string {
	if TLSEnabled() {
		return "https"
	}
	return "http"
}

func serverTLSFingerprint() (string, error) {
	if !TLSEnabled() {
		return "plaintext-http", nil
	}
	cert, err := loadOrCreateLocalCertificate()
	if err != nil {
		return "", err
	}
	if len(cert.Certificate) == 0 {
		return "", fmt.Errorf("TLS certificate has no DER data")
	}
	fingerprint := sha256.Sum256(cert.Certificate[0])
	return fmt.Sprintf("%x", fingerprint[:]), nil
}

func maybeTLSListener(listener net.Listener) (net.Listener, bool, error) {
	if !TLSEnabled() {
		return listener, false, nil
	}
	cert, err := loadOrCreateLocalCertificate()
	if err != nil {
		return nil, false, err
	}
	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	return tls.NewListener(listener, config), true, nil
}

func loadOrCreateLocalCertificate() (tls.Certificate, error) {
	certPath, keyPath, err := localCertificatePaths()
	if err != nil {
		return tls.Certificate{}, err
	}

	if cert, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil {
		return cert, nil
	}

	certPEM, keyPEM, err := generateSelfSignedCertificatePEM()
	if err != nil {
		return tls.Certificate{}, err
	}

	if err := os.MkdirAll(filepath.Dir(certPath), 0700); err != nil {
		return tls.Certificate{}, fmt.Errorf("create TLS config directory: %w", err)
	}
	if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
		return tls.Certificate{}, fmt.Errorf("write TLS certificate: %w", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		_ = os.Remove(certPath)
		return tls.Certificate{}, fmt.Errorf("write TLS private key: %w", err)
	}

	return tls.X509KeyPair(certPEM, keyPEM)
}

func localCertificatePaths() (string, string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("resolve home directory for TLS certificate: %w", err)
	}
	configDir := filepath.Join(homeDir, ".config", "beamsync")
	return filepath.Join(configDir, "cert.pem"), filepath.Join(configDir, "key.pem"), nil
}

func generateSelfSignedCertificatePEM() ([]byte, []byte, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate TLS private key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("generate TLS serial number: %w", err)
	}

	notBefore := time.Now().Add(-5 * time.Minute)
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: "BeamSync Local Transfer",
		},
		NotBefore:             notBefore,
		NotAfter:              notBefore.Add(tlsCertificateValidity),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		DNSNames:              []string{"localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("create self-signed TLS certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal TLS private key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return certPEM, keyPEM, nil
}
