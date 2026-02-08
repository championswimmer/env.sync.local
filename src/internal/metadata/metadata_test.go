package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateChecksumMatchesGenerateChecksum(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "secrets.env")
	content := strings.Join([]string{
		"# === ENV_SYNC_METADATA ===",
		"# VERSION: 2.0.0",
		"# TIMESTAMP: 2024-01-01T00:00:00Z",
		"# HOST: example.local",
		"# MODIFIED: 2024-01-01T00:00:00Z",
		"# CHECKSUM: ",
		"# === END_METADATA ===",
		"",
		"FOO=\"bar\"",
		"",
		"# === ENV_SYNC_FOOTER ===",
		"# VERSION: 2.0.0",
		"# TIMESTAMP: 2024-01-01T00:00:00Z",
		"# HOST: example.local",
		"# === END_FOOTER ===",
		"",
	}, "\n")

	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if err := UpdateChecksum(file); err != nil {
		t.Fatalf("UpdateChecksum: %v", err)
	}

	stored := GetFileChecksum(file)
	if stored == "" {
		t.Fatalf("expected checksum to be set")
	}
	if len(stored) != 64 {
		t.Fatalf("expected checksum length 64, got %d", len(stored))
	}

	generated, err := GenerateChecksum(file)
	if err != nil {
		t.Fatalf("GenerateChecksum: %v", err)
	}
	if stored != generated {
		t.Fatalf("checksum mismatch: stored=%s generated=%s", stored, generated)
	}
}

func TestEnsureEncryptedMetadataAndClear(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "secrets.env")
	content := strings.Join([]string{
		"# === ENV_SYNC_METADATA ===",
		"# VERSION: 2.0.0",
		"# TIMESTAMP: 2024-01-01T00:00:00Z",
		"# HOST: oldhost.local",
		"# MODIFIED: 2024-01-01T00:00:00Z",
		"# CHECKSUM: ",
		"# === END_METADATA ===",
		"",
		"FOO=\"bar\"",
		"",
		"# === ENV_SYNC_FOOTER ===",
		"# VERSION: 2.0.0",
		"# TIMESTAMP: 2024-01-01T00:00:00Z",
		"# HOST: oldhost.local",
		"# === END_FOOTER ===",
		"",
	}, "\n")

	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if err := EnsureEncryptedMetadata(file, "newhost.local", "age1aaa,age1bbb"); err != nil {
		t.Fatalf("EnsureEncryptedMetadata: %v", err)
	}
	if err := EnsureEncryptedMetadata(file, "newhost.local", "age1aaa,age1bbb"); err != nil {
		t.Fatalf("EnsureEncryptedMetadata (idempotent): %v", err)
	}

	updated, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	text := string(updated)
	if !strings.Contains(text, "# HOST: newhost.local") {
		t.Fatalf("expected host to be updated")
	}
	if strings.Count(text, "# ENCRYPTED: true") != 1 {
		t.Fatalf("expected single ENCRYPTED line")
	}
	if strings.Count(text, "# RECIPIENTS: age1aaa,age1bbb") != 1 {
		t.Fatalf("expected single RECIPIENTS line")
	}

	if err := ClearEncryptedMetadata(file); err != nil {
		t.Fatalf("ClearEncryptedMetadata: %v", err)
	}

	cleared, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read file after clear: %v", err)
	}
	clearedText := string(cleared)
	if strings.Contains(clearedText, "# ENCRYPTED:") || strings.Contains(clearedText, "# RECIPIENTS:") {
		t.Fatalf("expected encrypted metadata to be removed")
	}
	if !strings.Contains(clearedText, "# HOST: newhost.local") {
		t.Fatalf("expected host to remain updated")
	}
}
