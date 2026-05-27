package beamsync

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"strings"
	"time"
)

const tlsEnvVar = "BEAMSYNC_ENABLE_TLS"

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

func maybeTLSListener(listener net.Listener) (net.Listener, bool, error) {
	if !TLSEnabled() {
		return listener, false, nil
	}

	cert, err := generateSelfSignedCertificate(localCertificateHosts())
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
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate TLS private key: %w", err)
	}

	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate TLS serial number: %w", err)
	}

	notBefore := time.Now().Add(-5 * time.Minute)
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: "BeamSync Local Transfer",
		},
		NotBefore:             notBefore,
		NotAfter:              notBefore.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
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
		return tls.Certificate{}, fmt.Errorf("create self-signed TLS certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("load generated TLS certificate: %w", err)
	}
	return cert, nil
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
