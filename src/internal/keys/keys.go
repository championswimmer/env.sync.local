package keys

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"envsync/internal/config"
	"envsync/internal/crypto/age"
	"envsync/internal/logging"
	"envsync/internal/metadata"
	"envsync/internal/secrets"
)

type cacheEntry struct {
	Pubkey    string `json:"pubkey"`
	LastSeen  string `json:"last_seen"`
	FirstSeen string `json:"first_seen"`
}

func InitDirs() error {
	if err := os.MkdirAll(config.AgeKeyDir(), 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(config.AgeCacheDir(), 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(config.AgeKnownHostsDir(), 0o700); err != nil {
		return err
	}
	return nil
}

func CheckAgeInstalled() error {
	return age.CheckInstalled()
}

func GenerateKey() error {
	if err := InitDirs(); err != nil {
		return err
	}
	return age.GenerateKey()
}

func GetLocalPubkey() string {
	return age.GetLocalPubkey()
}

func GetLocalKeyPath() string {
	return config.AgeKeyFile()
}

func IsFileEncrypted(file string) bool {
	content, err := os.ReadFile(file)
	if err != nil {
		return false
	}
	return strings.Contains(string(content), "# ENCRYPTED: true")
}

func GetRecipientsFromFile(file string) string {
	if !IsFileEncrypted(file) {
		return ""
	}
	return metadata.ExtractMetadata(file, "RECIPIENTS")
}

func CanDecryptFile(file string) bool {
	if !IsFileEncrypted(file) {
		return true
	}
	if _, err := os.Stat(config.AgeKeyFile()); err != nil {
		return false
	}
	localPubkey := GetLocalPubkey()
	recipients := GetRecipientsFromFile(file)
	return strings.Contains(recipients, localPubkey)
}

func DecryptSecretsFile(inputFile string, outputFile string) error {
	text, err := secrets.GetSecretsContent(inputFile)
	if err != nil {
		return err
	}
	if strings.Contains(text, "ENVSYNC_UPDATED_AT=") {
		decrypted := make([]string, 0)
		failedKeys := make([]string, 0)
		linePattern := regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)="(.*)"\s*#`)
		for _, line := range strings.Split(text, "\n") {
			if matches := linePattern.FindStringSubmatch(line); len(matches) > 0 {
				dec, err := age.DecryptValue(matches[2])
				if err == nil {
					decrypted = append(decrypted, fmt.Sprintf("%s=\"%s\"", matches[1], dec))
				} else {
					failedKeys = append(failedKeys, matches[1])
				}
			} else if strings.TrimSpace(line) != "" {
				decrypted = append(decrypted, line)
			}
		}
		if len(failedKeys) > 0 {
			logging.Log("ERROR", fmt.Sprintf("Failed to decrypt %d secret(s): %s", len(failedKeys), strings.Join(failedKeys, ", ")))
			return errors.New("failed to decrypt secrets")
		}
		output := strings.Join(decrypted, "\n")
		if outputFile != "" {
			return os.WriteFile(outputFile, []byte(output), 0o600)
		}
		fmt.Println(output)
		return nil
	}

	if !IsFileEncrypted(inputFile) {
		if outputFile != "" {
			return copyFile(inputFile, outputFile)
		}
		fmt.Print(text)
		return nil
	}

	if !CanDecryptFile(inputFile) {
		logging.Log("ERROR", "Cannot decrypt file - not in recipient list")
		return errors.New("cannot decrypt")
	}

	logging.Log("ERROR", "Legacy file format detected (full file encryption). Please re-initialize.")
	return errors.New("legacy encryption")
}

func EncryptValue(value string, recipients []string) (string, error) {
	return age.EncryptValue(value, recipients)
}

func DecryptValue(value string) (string, error) {
	return age.DecryptValue(value)
}

func CachePeerPubkey(hostname string, pubkey string) error {
	if err := InitDirs(); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(config.AgeKnownHostsDir(), hostname+".pub"), []byte(pubkey), 0o644); err != nil {
		return err
	}

	cacheFile := filepath.Join(config.AgeCacheDir(), "pubkey_cache.json")
	now := nowTimestamp()
	cache := map[string]cacheEntry{}

	if data, err := os.ReadFile(cacheFile); err == nil {
		_ = json.Unmarshal(data, &cache)
	}

	entry := cache[hostname]
	firstSeen := entry.FirstSeen
	if firstSeen == "" {
		firstSeen = now
	}
	cache[hostname] = cacheEntry{Pubkey: pubkey, LastSeen: now, FirstSeen: firstSeen}

	payload, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(cacheFile, payload, 0o600); err != nil {
		return err
	}
	return nil
}

func GetCachedPubkey(hostname string) string {
	data, err := os.ReadFile(filepath.Join(config.AgeKnownHostsDir(), hostname+".pub"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func ListCachedPeers() []string {
	entries := []string{}
	files, _ := filepath.Glob(filepath.Join(config.AgeKnownHostsDir(), "*.pub"))
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		hostname := strings.TrimSuffix(filepath.Base(file), ".pub")
		entries = append(entries, fmt.Sprintf("%s: %s", hostname, strings.TrimSpace(string(data))))
	}
	sort.Strings(entries)
	return entries
}

func RemovePeerPubkey(hostname string) error {
	file := filepath.Join(config.AgeKnownHostsDir(), hostname+".pub")
	if _, err := os.Stat(file); err == nil {
		_ = os.Remove(file)
		logging.Log("INFO", "Removed pubkey for "+hostname)
	}
	return nil
}

func GetAllKnownRecipients() []string {
	recipients := []string{}
	local := GetLocalPubkey()
	if local != "" {
		recipients = append(recipients, local)
	}
	files, _ := filepath.Glob(filepath.Join(config.AgeKnownHostsDir(), "*.pub"))
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		pubkey := strings.TrimSpace(string(data))
		if pubkey == "" {
			continue
		}
		if !contains(recipients, pubkey) {
			recipients = append(recipients, pubkey)
		}
	}
	return recipients
}

func ValidatePubkey(pubkey string) bool {
	return regexp.MustCompile(`^age1[0-9a-z]+$`).MatchString(pubkey)
}

func nowTimestamp() string {
	return timeNowUTC().Format("2006-01-02T15:04:05Z")
}

func timeNowUTC() time.Time {
	return time.Now().UTC()
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}
