package mtlstransport

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"envsync/internal/config"
	"envsync/internal/identity"
	"envsync/internal/secrets"
)

// NewServerTLSConfig creates a TLS configuration for the mTLS server.
// Uses RequestClientCert so the peer enrollment endpoint can accept unauthenticated connections.
// Individual handlers enforce mTLS via extractPeerID checks.
func NewServerTLSConfig() (*tls.Config, error) {
	id, err := identity.LoadIdentity()
	if err != nil {
		return nil, fmt.Errorf("failed to load server identity: %w", err)
	}

	cert, err := tls.X509KeyPair(id.CertPEM, id.KeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to create key pair: %w", err)
	}

	pool := loadTrustedCertPool()

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequestClientCert,
		ClientCAs:    pool,
		MinVersion:   tls.VersionTLS13,
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			return verifyPeerCert(rawCerts)
		},
	}
	return tlsConfig, nil
}

// NewClientTLSConfig creates a TLS configuration for the mTLS client.
// It uses the local transport identity as the client certificate
// and pins trust to specific peer certificates.
func NewClientTLSConfig(peerID string) (*tls.Config, error) {
	id, err := identity.LoadIdentity()
	if err != nil {
		return nil, fmt.Errorf("failed to load client identity: %w", err)
	}

	cert, err := tls.X509KeyPair(id.CertPEM, id.KeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to create key pair: %w", err)
	}

	pool := x509.NewCertPool()
	peerCert, err := identity.LoadTrustedCert(peerID)
	if err == nil {
		pool.AddCert(peerCert)
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            pool,
		MinVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true, // Custom verification via VerifyPeerCertificate
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return errors.New("no server certificate presented")
			}
			serverCert, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return fmt.Errorf("failed to parse server cert: %w", err)
			}
			expectedFP := pinnedFingerprint(peerID)
			actualFP := identity.Fingerprint(serverCert)
			if expectedFP != "" && expectedFP != actualFP {
				return fmt.Errorf("server certificate fingerprint mismatch: expected %s, got %s", expectedFP, actualFP)
			}
			return nil
		},
	}
	return tlsConfig, nil
}

// HealthResponse matches the v2 health endpoint response.
type HealthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
	Host      string `json:"host"`
	Mode      string `json:"mode"`
}

// dialURL splits a "host[:port]" / "user@host[:port]" dial string and
// builds an https URL on the peer's port (defaulting to the local port).
// The SSH-style user part is stripped: it never goes on the wire for mTLS.
func dialURL(dial, path string) string {
	host, port := secrets.SplitDial(dial)
	if port == "" {
		port = config.EnvSyncPort()
	}
	return fmt.Sprintf("https://%s:%s%s", host, port, path)
}

// FetchHealth fetches health from a peer via mTLS.
// dial accepts "host", "host:port", or "user@host[:port]".
func FetchHealth(dial string, peerID string) (HealthResponse, error) {
	client, err := newMTLSClient(peerID)
	if err != nil {
		return HealthResponse{}, err
	}

	url := dialURL(dial, "/v2/health")
	resp, err := client.Get(url)
	if err != nil {
		return HealthResponse{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return HealthResponse{}, err
	}

	var health HealthResponse
	if err := json.Unmarshal(body, &health); err != nil {
		return HealthResponse{}, err
	}
	return health, nil
}

// FetchSecrets fetches secrets from a peer via mTLS.
// dial accepts "host", "host:port", or "user@host[:port]".
func FetchSecrets(dial string, peerID string) ([]byte, error) {
	client, err := newMTLSClient(peerID)
	if err != nil {
		return nil, err
	}

	url := dialURL(dial, "/v2/secrets")
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return nil, errors.New("access denied: peer not authorized")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// FetchMembershipEvents fetches membership events from a peer via mTLS.
func FetchMembershipEvents(dial string, peerID string, sinceEventID uint64) ([]byte, error) {
	client, err := newMTLSClient(peerID)
	if err != nil {
		return nil, err
	}

	url := dialURL(dial, fmt.Sprintf("/v2/membership/events?since=%d", sinceEventID))
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// RequestReencrypt asks a peer to re-encrypt secrets including our AGE pubkey.
func RequestReencrypt(dial string, peerID string, agePubkey string) error {
	client, err := newMTLSClient(peerID)
	if err != nil {
		return err
	}

	url := dialURL(dial, "/v2/secrets/request-reencrypt")
	payload, _ := json.Marshal(map[string]string{"age_pubkey": agePubkey})
	resp, err := client.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("re-encrypt request failed (%s): %s", resp.Status, string(body))
	}
	return nil
}

// RequestAccess sends a peer access request with our identity.
// dial accepts "host", "host:port", or "user@host[:port]".
func RequestAccess(dial string, token string, peerID string, hostname string, fingerprint string, agePubkey string, certPEM []byte) error {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS13,
	}

	if identity.IdentityExists() {
		id, err := identity.LoadIdentity()
		if err == nil {
			cert, err := tls.X509KeyPair(id.CertPEM, id.KeyPEM)
			if err == nil {
				tlsConfig.Certificates = []tls.Certificate{cert}
			}
		}
	}

	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
	}

	payload, _ := json.Marshal(map[string]string{
		"token":       token,
		"peer_id":     peerID,
		"hostname":    hostname,
		"fingerprint": fingerprint,
		"age_pubkey":  agePubkey,
		"cert_pem":    string(certPEM),
	})

	url := dialURL(dial, "/v2/peer/request-access")
	resp, err := client.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("access request failed (%s): %s", resp.Status, string(body))
	}
	return nil
}

func newMTLSClient(peerID string) (*http.Client, error) {
	tlsConfig, err := NewClientTLSConfig(peerID)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
	}, nil
}

func verifyPeerCert(rawCerts [][]byte) error {
	// Always allow at TLS level. With RequestClientCert, the client may or may
	// not present a certificate. Trust decisions are made per-handler:
	//  - /v2/peer/request-access accepts unauthenticated connections (validates invite token)
	//  - All other v2 endpoints require a trusted client cert via extractPeerID
	return nil
}

func loadTrustedCertPool() *x509.CertPool {
	pool := x509.NewCertPool()
	trustedDir := config.TrustedCertsDir()
	files, _ := filepath.Glob(filepath.Join(trustedDir, "*.crt"))
	for _, file := range files {
		certPEM, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		pool.AppendCertsFromPEM(certPEM)
	}
	return pool
}

func pinnedFingerprint(peerID string) string {
	cert, err := identity.LoadTrustedCert(peerID)
	if err != nil {
		return ""
	}
	return identity.Fingerprint(cert)
}
