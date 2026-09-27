package identity

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"envsync/internal/config"
	"envsync/internal/logging"
	"envsync/internal/secrets"
)

// TransportIdentity holds the TLS transport keypair and certificate.
type TransportIdentity struct {
	Key         *ecdsa.PrivateKey
	Certificate *x509.Certificate
	CertPEM     []byte
	KeyPEM      []byte
}

// GenerateTransportIdentity creates a new ECDSA P-256 keypair and self-signed X.509 certificate.
// peerID is the per-user peer ID ("user@host"); it becomes the certificate
// CN so two OS users on the same machine have distinct identities.
// Legacy identities used a bare hostname CN and keep working.
func GenerateTransportIdentity(peerID string) (*TransportIdentity, error) {
	if peerID == "" {
		return nil, errors.New("peer ID is required")
	}
	dnsNames := []string{peerID}
	if ep := secrets.ParseEndpoint(peerID); !ep.Legacy {
		dnsNames = []string{peerID, ep.Host}
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   peerID,
			Organization: []string{"env-sync"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour), // 10 years
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return &TransportIdentity{
		Key:         key,
		Certificate: cert,
		CertPEM:     certPEM,
		KeyPEM:      keyPEM,
	}, nil
}

// SaveIdentity writes the transport identity to disk.
func SaveIdentity(id *TransportIdentity) error {
	if err := os.MkdirAll(config.TLSDir(), 0o700); err != nil {
		return err
	}

	if err := os.WriteFile(config.TLSKeyFile(), id.KeyPEM, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(config.TLSCertFile(), id.CertPEM, 0o644); err != nil {
		return err
	}

	logging.Log("SUCCESS", "Generated TLS transport identity")
	logging.Log("INFO", "Fingerprint: "+Fingerprint(id.Certificate))
	return nil
}

// LoadIdentity reads the transport identity from disk.
func LoadIdentity() (*TransportIdentity, error) {
	keyPEM, err := os.ReadFile(config.TLSKeyFile())
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}
	certPEM, err := os.ReadFile(config.TLSCertFile())
	if err != nil {
		return nil, fmt.Errorf("failed to read cert file: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, errors.New("failed to decode key PEM")
	}
	key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse key: %w", err)
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, errors.New("failed to decode cert PEM")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cert: %w", err)
	}

	return &TransportIdentity{
		Key:         key,
		Certificate: cert,
		CertPEM:     certPEM,
		KeyPEM:      keyPEM,
	}, nil
}

// IdentityExists checks if a transport identity exists on disk.
func IdentityExists() bool {
	_, err1 := os.Stat(config.TLSKeyFile())
	_, err2 := os.Stat(config.TLSCertFile())
	return err1 == nil && err2 == nil
}

// EnsureIdentity ensures a transport identity exists, generating one if needed.
func EnsureIdentity(hostname string) (*TransportIdentity, error) {
	if IdentityExists() {
		return LoadIdentity()
	}
	id, err := GenerateTransportIdentity(hostname)
	if err != nil {
		return nil, err
	}
	if err := SaveIdentity(id); err != nil {
		return nil, err
	}
	return id, nil
}

// Fingerprint returns the SHA-256 fingerprint of a certificate.
func Fingerprint(cert *x509.Certificate) string {
	hash := sha256.Sum256(cert.Raw)
	hexStr := hex.EncodeToString(hash[:])
	// Format as colon-separated pairs
	parts := make([]string, 0, len(hexStr)/2)
	for i := 0; i < len(hexStr); i += 2 {
		parts = append(parts, hexStr[i:i+2])
	}
	return strings.Join(parts, ":")
}

// SaveTrustedCert saves a peer's certificate to the trusted certs directory.
func SaveTrustedCert(peerID string, certPEM []byte) error {
	if err := os.MkdirAll(config.TrustedCertsDir(), 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(config.TrustedCertsDir(), peerID+".crt"), certPEM, 0o644)
}

// LoadTrustedCert loads a peer's certificate from the trusted certs directory.
func LoadTrustedCert(peerID string) (*x509.Certificate, error) {
	certPEM, err := os.ReadFile(filepath.Join(config.TrustedCertsDir(), peerID+".crt"))
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, errors.New("failed to decode cert PEM")
	}
	return x509.ParseCertificate(block.Bytes)
}
