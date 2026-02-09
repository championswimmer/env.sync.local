package keys

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	agecrypto "filippo.io/age"

	"envsync/internal/config"
	"envsync/internal/crypto/age"
	"envsync/internal/metadata"
	"envsync/internal/secrets"
)

func TestDecryptSecretsFileFailsWhenPrivateKeyIsInvalid(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.MkdirAll(config.AgeKeyDir(), 0o700); err != nil {
		t.Fatalf("failed to create key dir: %v", err)
	}

	identity, err := agecrypto.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}
	recipient := identity.Recipient().String()

	if err := os.WriteFile(config.AgePubKeyFile(), []byte(recipient), 0o644); err != nil {
		t.Fatalf("failed to write pubkey file: %v", err)
	}
	if err := os.WriteFile(config.AgeKeyFile(), []byte("invalid-age-key"), 0o600); err != nil {
		t.Fatalf("failed to write private key file: %v", err)
	}

	if err := secrets.InitSecretsFile(config.SecretsFile(), config.InitTimestamp()); err != nil {
		t.Fatalf("InitSecretsFile: %v", err)
	}
	if err := metadata.EnsureEncryptedMetadata(config.SecretsFile(), "test.local", recipient); err != nil {
		t.Fatalf("EnsureEncryptedMetadata: %v", err)
	}

	encrypted, err := age.EncryptValue("super-secret", []string{recipient})
	if err != nil {
		t.Fatalf("EncryptValue: %v", err)
	}
	content := fmt.Sprintf(`TEST_KEY="%s" # ENVSYNC_UPDATED_AT=%s`, encrypted, secrets.GetTimestamp())
	if err := secrets.SetSecretsContent(config.SecretsFile(), content); err != nil {
		t.Fatalf("SetSecretsContent: %v", err)
	}

	outputFile := filepath.Join(home, "decrypted.env")
	if err := DecryptSecretsFile(config.SecretsFile(), outputFile); err == nil {
		t.Fatalf("expected decryption to fail with invalid private key")
	}

	if _, err := os.Stat(outputFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no decrypted output file, got err=%v", err)
	}
}
