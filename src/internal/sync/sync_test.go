package syncer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"envsync/internal/config"
)

func writeSSHScript(t *testing.T, dir string, output string) {
	t.Helper()
	sshScript := filepath.Join(dir, "ssh")
	script := "#!/bin/sh\n"
	if output != "" {
		script += "echo \"" + output + "\"\n"
	}
	if err := os.WriteFile(sshScript, []byte(script), 0o700); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func TestCachePeerPubkeyCachesKey(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	pubkey := "age1testpubkey"
	writeSSHScript(t, binDir, pubkey)

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cachePeerPubkey("peer.local")

	if _, err := os.Stat(config.AgeKnownHostsDir()); err != nil {
		t.Fatalf("AgeKnownHostsDir() error = %v", err)
	}

	cachedPath := filepath.Join(config.AgeKnownHostsDir(), "peer.local.pub")
	data, err := os.ReadFile(cachedPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.TrimSpace(string(data)) != pubkey {
		t.Fatalf("Cached pubkey = %q, want %q", strings.TrimSpace(string(data)), pubkey)
	}
}

func TestCachePeerPubkeySkipsInvalidKey(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	writeSSHScript(t, binDir, "not-a-pubkey")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cachePeerPubkey("peer.local")

	cachedPath := filepath.Join(config.AgeKnownHostsDir(), "peer.local.pub")
	if _, err := os.Stat(cachedPath); err == nil {
		t.Fatal("Expected no cached pubkey for invalid key")
	}
}

func TestCachePeerPubkeySkipsEmptyKey(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	writeSSHScript(t, binDir, "")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cachePeerPubkey("peer.local")

	cachedPath := filepath.Join(config.AgeKnownHostsDir(), "peer.local.pub")
	if _, err := os.Stat(cachedPath); err == nil {
		t.Fatal("Expected no cached pubkey for empty key")
	}
}

func TestEnsureRegisteredWithPeerSuccess(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	// Generate a local key so GetLocalPubkey returns something
	if err := os.MkdirAll(config.AgeKeyDir(), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(config.AgePubKeyFile(), []byte("age1testlocalpubkey"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Create a mock ssh script that echoes ENVSYNC_REGISTER_SUCCESS
	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	sshScript := filepath.Join(binDir, "ssh")
	script := "#!/bin/sh\necho 'ENVSYNC_REGISTER_SUCCESS'\n"
	if err := os.WriteFile(sshScript, []byte(script), 0o700); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := ensureRegisteredWithPeer("peer.local")
	if err != nil {
		t.Fatalf("ensureRegisteredWithPeer() error = %v", err)
	}
}

func TestEnsureRegisteredWithPeerNoLocalKey(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	// No key file created, so GetLocalPubkey returns ""
	err := ensureRegisteredWithPeer("peer.local")
	if err == nil {
		t.Fatal("Expected error when no local key exists")
	}
	if !strings.Contains(err.Error(), "no local key found") {
		t.Fatalf("Expected 'no local key found' error, got: %v", err)
	}
}

func TestEnsureRegisteredWithPeerSSHFailure(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	// Generate a local key
	if err := os.MkdirAll(config.AgeKeyDir(), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(config.AgePubKeyFile(), []byte("age1testlocalpubkey"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Create a mock ssh script that fails
	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	sshScript := filepath.Join(binDir, "ssh")
	script := "#!/bin/sh\nexit 1\n"
	if err := os.WriteFile(sshScript, []byte(script), 0o700); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := ensureRegisteredWithPeer("peer.local")
	if err == nil {
		t.Fatal("Expected error when SSH fails")
	}
}

func TestFindNewestPeerRegistersAndRefetchesWhenUndecryptable(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	hostnameScript := filepath.Join(binDir, "hostname")
	if err := os.WriteFile(hostnameScript, []byte("#!/bin/sh\necho hostb.local\n"), 0o700); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Local dummy key files so GetLocalPubkey works and CanDecryptFile checks pass
	if err := os.MkdirAll(config.AgeKeyDir(), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(config.AgeKeyFile(), []byte("dummy-private"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(config.AgePubKeyFile(), []byte("age1hostb"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	originalFetch := fetchFromHostFunc
	originalEnsure := ensureRegisteredWithPeerFunc
	originalDiscover := discoverPeersFunc
	originalTestSSH := testSSHFunc
	defer func() {
		fetchFromHostFunc = originalFetch
		ensureRegisteredWithPeerFunc = originalEnsure
		discoverPeersFunc = originalDiscover
		testSSHFunc = originalTestSSH
	}()

	discoverPeersFunc = func(useHTTP bool) ([]string, error) {
		return []string{"hosta.local"}, nil
	}
	testSSHFunc = func(host string) error { return nil }

	registerCalled := false
	ensureRegisteredWithPeerFunc = func(host string) error {
		registerCalled = true
		return nil
	}

	fetchCount := 0
	fetchFromHostFunc = func(host string, useHTTP bool) (string, error) {
		fetchCount++
		tmp := filepath.Join(tempDir, "remote-fetch.env")
		recipients := "hosta:age1hosta"
		if fetchCount > 1 {
			recipients = recipients + ",hostb:age1hostb"
		}
		content := mockEncryptedContent(host, recipients)
		if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
			return "", err
		}
		return tmp, nil
	}

	newestHost, err := findNewestPeer(false)
	if err != nil {
		t.Fatalf("findNewestPeer() error = %v", err)
	}
	if newestHost != "hosta.local" {
		t.Fatalf("newestHost = %q, want %q", newestHost, "hosta.local")
	}
	if !registerCalled {
		t.Fatalf("expected ensureRegisteredWithPeer to be called")
	}
	if fetchCount < 2 {
		t.Fatalf("expected refetch after registration, got %d fetch(es)", fetchCount)
	}
}

func mockEncryptedContent(host, publicKeys string) string {
	return fmt.Sprintf(`# === ENV_SYNC_METADATA ===
# VERSION: %s
# TIMESTAMP: 2025-01-01T00:00:00Z
# HOST: %s
# ENCRYPTED: true
# PUBLIC_KEYS: %s
# CHECKSUM: 
# === END_METADATA ===

KEY="cipher" # ENVSYNC_UPDATED_AT=2025-01-01T00:00:00Z

# === ENV_SYNC_FOOTER ===
# VERSION: %s
# TIMESTAMP: 2025-01-01T00:00:00Z
# HOST: %s
# === END_FOOTER ===
`, config.Version, host, publicKeys, config.Version, host)
}
