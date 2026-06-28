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

const (
	tlsCertificateFileMode os.FileMode = 0o600
	tlsConfigDirMode       os.FileMode = 0o700
)

// TLSEnabled reports whether local HTTPS serving is enabled.
func TLSEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(tlsEnvVar)))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

// ServerScheme returns the URL scheme used by BeamSync local servers.
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

	cert, err := loadOrCreateLocalCertificate(localCertificateHosts())
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

	cert, err := loadOrCreateLocalCertificate(localCertificateHosts())
	if err != nil {
		return nil, false, err
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	return tls.NewListener(listener, config), true, nil
}

func generateSelfSignedCertificate(hosts []string) (tls.Certificate, error) {
	certPEM, keyPEM, err := generateSelfSignedCertificatePEM(hosts)
	if err != nil {
		return tls.Certificate{}, err
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("load generated TLS certificate: %w", err)
	}
	return cert, nil
}

func loadOrCreateLocalCertificate(hosts []string) (tls.Certificate, error) {
	certPath, keyPath, err := localCertificatePaths()
	if err != nil {
		return tls.Certificate{}, err
	}

	if cert, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil {
		_ = os.Chmod(certPath, tlsCertificateFileMode)
		_ = os.Chmod(keyPath, tlsCertificateFileMode)
		return cert, nil
	}

	certPEM, keyPEM, err := generateSelfSignedCertificatePEM(hosts)
	if err != nil {
		return tls.Certificate{}, err
	}

	if err := os.MkdirAll(filepath.Dir(certPath), tlsConfigDirMode); err != nil {
		return tls.Certificate{}, fmt.Errorf("create TLS config directory: %w", err)
	}
	if err := os.WriteFile(certPath, certPEM, tlsCertificateFileMode); err != nil {
		return tls.Certificate{}, fmt.Errorf("write TLS certificate: %w", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, tlsCertificateFileMode); err != nil {
		_ = os.Remove(certPath)
		return tls.Certificate{}, fmt.Errorf("write TLS private key: %w", err)
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("load persisted TLS certificate: %w", err)
	}
	return cert, nil
}

func localCertificatePaths() (string, string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("resolve home directory for TLS certificate: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "beamsync")
	return filepath.Join(configDir, "cert.pem"), filepath.Join(configDir, "key.pem"), nil
}

func generateSelfSignedCertificatePEM(hosts []string) ([]byte, []byte, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate TLS private key: %w", err)
	}

	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialLimit)
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
	}

	for _, host := range hosts {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}
		if ip := net.ParseIP(host); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
			continue
		}
		template.DNSNames = append(template.DNSNames, host)
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

func localCertificateHosts() []string {
	hosts := []string{"localhost", "127.0.0.1", "::1"}
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		hosts = append(hosts, hostname)
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return hosts
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsUnspecified() {
				continue
			}
			hosts = append(hosts, ip.String())
		}
	}
	return dedupeStrings(hosts)
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
