package syncer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"envsync/internal/backup"
	"envsync/internal/config"
	"envsync/internal/discovery"
	"envsync/internal/keys"
	"envsync/internal/logging"
	"envsync/internal/metadata"
	"envsync/internal/secrets"
	httptransport "envsync/internal/transport/http"
	sshtransport "envsync/internal/transport/ssh"
)

type Options struct {
	AllPeers     bool
	Force        bool
	ForcePull    bool
	Quiet        bool
	InsecureHTTP bool
	TargetHost   string
}

var (
	fetchFromHostFunc            = fetchFromHost
	ensureRegisteredWithPeerFunc = ensureRegisteredWithPeer
	discoverPeersFunc            = discoverPeers
	testSSHFunc                  = sshtransport.TestSSH
)

func Run(opts Options) error {
	if opts.Quiet {
		_ = os.Setenv("ENV_SYNC_QUIET", "true")
	}

	if opts.InsecureHTTP {
		printHTTPWarning()
	}

	maybeReencryptLocal()

	if opts.TargetHost != "" {
		if !opts.InsecureHTTP {
			if err := testSSHFunc(opts.TargetHost); err != nil {
				logging.Log("ERROR", "Cannot SSH to "+opts.TargetHost)
				logging.Log("INFO", "Ensure SSH keys are set up, or use --insecure-http flag")
				return err
			}
			cachePeerPubkey(opts.TargetHost)
		}
		return syncFromHost(opts.TargetHost, opts.InsecureHTTP, opts.ForcePull)
	}

	if opts.AllPeers {
		logging.Log("INFO", "Syncing from all discovered peers...")
		peers, err := discoverPeers(opts.InsecureHTTP)
		if err != nil || len(peers) == 0 {
			logging.Log("WARN", "No peers discovered")
			return errors.New("no peers")
		}
		success := 0
		for _, peer := range peers {
			if peer == secrets.GetHostname() {
				continue
			}
			if !opts.InsecureHTTP {
				if err := testSSHFunc(peer); err != nil {
					continue
				}
			}
			if err := syncFromHost(peer, opts.InsecureHTTP, opts.ForcePull); err == nil {
				success++
			}
		}
		if success == 0 {
			logging.Log("WARN", "Failed to sync from any peer")
			if !opts.InsecureHTTP {
				logging.Log("INFO", "Tip: Set up SSH keys with: ssh-copy-id hostname.local")
			}
			return errors.New("sync failed")
		}
		logging.Log("SUCCESS", fmt.Sprintf("Synced from %d peer(s)", success))
		return nil
	}

	logging.Log("INFO", fmt.Sprintf("Searching for newest secrets via %s...", formatTransportName(opts.InsecureHTTP)))
	newestHost, err := findNewestPeer(opts.InsecureHTTP)
	if err != nil || newestHost == "" {
		logging.Log("INFO", "No newer secrets found on network")
		if !opts.InsecureHTTP {
			logging.Log("INFO", "To sync without SSH keys, use: env-sync --insecure-http")
		}
		return nil
	}
	logging.Log("INFO", "Newest secrets found on: "+newestHost)
	return syncFromHost(newestHost, opts.InsecureHTTP, false)
}

// ReencryptLocal updates local secrets encryption when new recipients are added.
func ReencryptLocal() {
	maybeReencryptLocal()
}

func formatTransportName(useHTTP bool) string {
	if useHTTP {
		return "HTTP"
	}
	return "SCP/SSH"
}

func printHTTPWarning() {
	if strings.EqualFold(os.Getenv("ENV_SYNC_QUIET"), "true") {
		return
	}
	lines := []string{
		"",
		"╔════════════════════════════════════════════════════════════════════════════╗",
		"║  ⚠️  SECURITY WARNING: USING INSECURE HTTP MODE                          ║",
		"║                                                                            ║",
		"║  You are using the insecure HTTP sync mode. This exposes your secrets:     ║",
		"║  • Transmitted in PLAINTEXT over the network                               ║",
		"║  • Accessible to ANY device on your local network                          ║",
		"║  • No authentication or encryption                                         ║",
		"║                                                                            ║",
		"║  RECOMMENDED: Use SCP mode (default) which requires SSH key authentication ║",
		"║  To use SCP: Remove the --insecure-http flag                               ║",
		"║                                                                            ║",
		"║  Only use HTTP mode if:                                                    ║",
		"║  • SSH keys are not set up between machines                                ║",
		"║  • You are on a completely trusted isolated network                        ║",
		"║  • You are testing/development only                                        ║",
		"╚════════════════════════════════════════════════════════════════════════════╝",
		"",
	}
	fmt.Fprintln(os.Stdout, strings.Join(lines, "\n"))
}

func discoverPeers(useHTTP bool) ([]string, error) {
	opts := discovery.Options{Timeout: 5 * time.Second, Quiet: true}
	if !useHTTP {
		opts.FilterSSH = true
	}
	return discovery.Discover(opts)
}

func fetchFromHost(host string, useHTTP bool) (string, error) {
	tmpFile, err := os.CreateTemp("", "env-sync-remote")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()

	if useHTTP {
		logging.Log("INFO", fmt.Sprintf("Fetching from %s via HTTP (INSECURE)...", host))
		data, err := httptransport.FetchSecrets(host)
		if err != nil {
			logging.Log("WARN", "Failed to fetch from "+host+" via HTTP")
			_ = os.Remove(tmpPath)
			return "", err
		}
		if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
			_ = os.Remove(tmpPath)
			return "", err
		}
	} else {
		logging.Log("INFO", fmt.Sprintf("Fetching from %s via SCP...", host))
		if err := sshtransport.FetchViaSCP(host, config.SecretsFile(), tmpPath); err != nil {
			logging.Log("WARN", "Failed to fetch from "+host+" via SCP (SSH key may not be set up)")
			_ = os.Remove(tmpPath)
			return "", err
		}
	}

	if err := secrets.ValidateSecretsFile(tmpPath); err != nil {
		logging.Log("WARN", "Invalid secrets file from "+host)
		_ = os.Remove(tmpPath)
		return "", err
	}

	return tmpPath, nil
}

func syncFromHost(host string, useHTTP bool, forcePull bool) error {
	remoteFile, err := fetchRemoteWithRegistration(host, useHTTP)
	if err != nil {
		return err
	}

	defer os.Remove(remoteFile)

	// Extract and cache public keys from remote file
	if err := keys.CachePublicKeysFromFile(remoteFile); err != nil {
		logging.Log("WARN", "Failed to cache public keys from remote file")
	}

	// After caching new public keys, re-encrypt local file if needed
	// to add the new recipients to our file's PUBLIC_KEYS metadata
	maybeReencryptLocal()

	if _, err := os.Stat(config.SecretsFile()); err != nil {
		logging.Log("INFO", "No local secrets file found, copying from "+host)
		if err := copyFile(remoteFile, config.SecretsFile()); err != nil {
			return err
		}
		_ = os.Chmod(config.SecretsFile(), 0o600)
		logging.Log("SUCCESS", "Synced secrets from "+host)
		return nil
	}

	_ = backup.CreateBackup(config.SecretsFile())

	if forcePull {
		// Force pull: completely overwrite local file with remote file
		logging.Log("INFO", "Force pulling all secrets from "+host+" (overwriting local)")
		remoteContent, err := secrets.GetSecretsContent(remoteFile)
		if err != nil {
			return err
		}
		if err := secrets.SetSecretsContent(config.SecretsFile(), remoteContent); err != nil {
			return err
		}
		if err := refreshPublicKeysMetadata(config.SecretsFile()); err != nil {
			logging.Log("WARN", "Failed to update PUBLIC_KEYS metadata: "+err.Error())
		}
		logging.Log("SUCCESS", "Force pulled secrets from "+host)
		return nil
	}

	localContent, _ := secrets.GetSecretsContent(config.SecretsFile())
	remoteContent, _ := secrets.GetSecretsContent(remoteFile)
	merged := secrets.MergeSecretsContent(localContent, remoteContent)

	if err := secrets.SetSecretsContent(config.SecretsFile(), merged); err != nil {
		return err
	}
	if err := refreshPublicKeysMetadata(config.SecretsFile()); err != nil {
		logging.Log("WARN", "Failed to update PUBLIC_KEYS metadata: "+err.Error())
	}

	logging.Log("SUCCESS", "Synced and merged secrets from "+host)
	return nil
}

func findNewestPeer(useHTTP bool) (string, error) {
	newestHost := ""
	newestFile := ""
	newestIsLocal := false

	if _, err := os.Stat(config.SecretsFile()); err == nil {
		newestFile = config.SecretsFile()
		newestIsLocal = true
	}

	peers, err := discoverPeersFunc(useHTTP)
	if err != nil {
		logging.Log("WARN", "No peers discovered")
		return "", err
	}

	for _, peer := range peers {
		if peer == secrets.GetHostname() {
			continue
		}
		if !useHTTP {
			if err := testSSHFunc(peer); err != nil {
				logging.Log("DEBUG", "Cannot SSH to "+peer+" (skipping)")
				continue
			}
		}
		remoteFile, err := fetchRemoteWithRegistration(peer, useHTTP)
		if err != nil {
			continue
		}
		if keys.IsFileEncrypted(remoteFile) && !keys.CanDecryptFile(remoteFile) {
			if !useHTTP {
				// Auto-register with peer so they re-encrypt with our public key. HTTP mode cannot auto-register because it cannot push our public key to the peer for re-encryption.
				logging.Log("INFO", "Cannot decrypt secrets from "+peer+", registering and requesting re-encryption...")
				// Keep reference to the originally fetched file so we can clean it up even if remoteFile is reassigned
				originalEncryptedFile := remoteFile
				// Local cleanup helper keeps temp-file handling scoped to this registration path
				cleanupTempFile := func(path string) {
					if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
						logging.Log("DEBUG", "Failed to remove temp file from "+peer+" ("+path+"): "+err.Error())
					}
				}
				if err := ensureRegisteredWithPeer(peer); err != nil {
					logging.Log("WARN", "Failed to register with "+peer+": "+err.Error())
					cleanupTempFile(originalEncryptedFile)
					continue
				}
				refetchedFile, err := fetchFromHost(peer, useHTTP)
				if err != nil {
					logging.Log("WARN", "Failed to re-fetch secrets from "+peer+" after registration: "+err.Error())
					cleanupTempFile(originalEncryptedFile)
					continue
				}
				if keys.IsFileEncrypted(refetchedFile) && !keys.CanDecryptFile(refetchedFile) {
					logging.Log("WARN", "Still cannot decrypt secrets from "+peer+" after registration. Waiting for peer to sync and finish re-encrypting with our key.")
					// Clean up both fetched copies before skipping this peer
					cleanupTempFile(refetchedFile)
					cleanupTempFile(originalEncryptedFile)
					continue
				}
				// Drop the original encrypted copy now that we have a decryptable replacement
				cleanupTempFile(originalEncryptedFile)
				remoteFile = refetchedFile
				logging.Log("SUCCESS", "Registered with "+peer+" and can decrypt re-encrypted secrets")
			} else {
				logging.Log("DEBUG", "Cannot decrypt file from "+peer+" (skipping)")
				_ = os.Remove(remoteFile)
				continue
			}
		}
		// Cache any new public keys discovered and re-encrypt local file if needed
		if err := keys.CachePublicKeysFromFile(remoteFile); err != nil && config.IsVerbose() {
			logging.Log("DEBUG", "Failed to cache public keys from "+peer+": "+err.Error())
		}
		maybeReencryptLocal()
		if newestFile == "" || secrets.IsNewer(remoteFile, newestFile) {
			newestHost = peer
			if newestFile != "" && !newestIsLocal {
				_ = os.Remove(newestFile)
			}
			newestFile = remoteFile
			newestIsLocal = false
		} else {
			_ = os.Remove(remoteFile)
		}
	}

	if newestHost == "" {
		if newestFile != "" && !newestIsLocal {
			_ = os.Remove(newestFile)
		}
		if !useHTTP {
			logging.Log("WARN", "No valid peers found via SSH")
			logging.Log("INFO", "Tip: Ensure SSH keys are set up between machines")
			logging.Log("INFO", "     Or use --insecure-http flag (not recommended)")
		}
		return "", errors.New("no peers")
	}

	if newestFile != "" && !newestIsLocal {
		_ = os.Remove(newestFile)
	}
	return newestHost, nil
}

func maybeReencryptLocal() {
	if _, err := os.Stat(config.SecretsFile()); err != nil {
		return
	}
	if !keys.IsFileEncrypted(config.SecretsFile()) {
		return
	}
	if !keys.CanDecryptFile(config.SecretsFile()) {
		return
	}

	recipientsInFile := keys.GetRecipientsFromFile(config.SecretsFile())
	allRecipients := keys.GetAllKnownRecipients()
	if len(allRecipients) == 0 {
		return
	}

	missing := false
	for _, recipient := range allRecipients {
		if !keys.RecipientsContain(recipientsInFile, recipient) {
			missing = true
			break
		}
	}
	if !missing {
		return
	}

	tempFile, err := os.CreateTemp("", "env-sync-reencrypt")
	if err != nil {
		return
	}
	tempPath := tempFile.Name()
	_ = tempFile.Close()

	if err := reencryptSecrets(config.SecretsFile(), tempPath); err != nil {
		_ = os.Remove(tempPath)
		logging.Log("ERROR", "Failed to re-encrypt secrets")
		return
	}
	_ = os.Rename(tempPath, config.SecretsFile())
	_ = os.Chmod(config.SecretsFile(), 0o600)
	logging.Log("SUCCESS", "Re-encrypted local secrets with updated recipients")
}

func cachePeerPubkey(host string) {
	pubkey := discovery.FetchPubkey(host)
	if pubkey == "" {
		logging.Log("WARN", "Could not fetch public key from "+host)
		return
	}
	if !keys.ValidatePubkey(pubkey) {
		logging.Log("WARN", "Invalid public key fetched from "+host)
		return
	}
	if err := keys.CachePeerPubkey(host, pubkey); err != nil {
		logging.Log("WARN", "Failed to cache public key from "+host+": "+err.Error())
		return
	}
	logging.Log("SUCCESS", "Cached public key from "+host)
}

func ensureRegisteredWithPeer(host string) error {
	localPubkey := keys.GetLocalPubkey()
	if localPubkey == "" {
		return errors.New("no local key found - generate with: env-sync init --encrypted")
	}
	localHostname := secrets.GetHostname()

	logging.Log("INFO", "Registering public key with "+host+" and triggering re-encryption...")
	if err := sshtransport.RegisterPubkeyWithPeer(host, localPubkey, localHostname); err != nil {
		return fmt.Errorf("failed to register with %s: %w", host, err)
	}
	logging.Log("SUCCESS", "Registered with "+host)
	return nil
}

func reencryptSecrets(inputFile, outputFile string) error {
	recipients := keys.GetAllKnownRecipients()
	if len(recipients) == 0 {
		logging.Log("WARN", "No recipients found - keeping as plaintext")
		return copyFile(inputFile, outputFile)
	}

	content, err := secrets.GetSecretsContent(inputFile)
	if err != nil {
		return err
	}

	var newLines []string
	linePattern := regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)="(.*)"\s*#.*ENVSYNC_UPDATED_AT=(.*)`)
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			newLines = append(newLines, line)
			continue
		}
		matches := linePattern.FindStringSubmatch(line)
		if len(matches) == 0 {
			newLines = append(newLines, line)
			continue
		}
		decrypted, err := keys.DecryptValue(matches[2])
		if err != nil {
			logging.Log("WARN", "Failed to decrypt "+matches[1]+" during re-encryption (skipping re-encryption for this key)")
			newLines = append(newLines, line)
			continue
		}
		encrypted, err := keys.EncryptValue(decrypted, recipients)
		if err != nil {
			logging.Log("ERROR", "Failed to re-encrypt "+matches[1])
			return err
		}
		newLines = append(newLines, fmt.Sprintf("%s=\"%s\" # ENVSYNC_UPDATED_AT=%s", matches[1], encrypted, matches[3]))
	}

	if err := secrets.SetSecretsContent(outputFile, strings.TrimSpace(strings.Join(newLines, "\n"))); err != nil {
		return err
	}

	if err := metadata.EnsureEncryptedMetadata(outputFile, secrets.GetHostname()); err != nil {
		return err
	}

	// Add PUBLIC_KEYS metadata
	publicKeys := keys.GetAllKnownPublicKeys()
	if err := metadata.EnsurePublicKeysMetadata(outputFile, publicKeys); err != nil {
		logging.Log("WARN", "Failed to update PUBLIC_KEYS metadata")
	}

	return secrets.UpdateMetadata(outputFile, "")
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}

func refreshPublicKeysMetadata(file string) error {
	if !keys.IsFileEncrypted(file) {
		return nil
	}
	publicKeys := keys.GetAllKnownPublicKeys()
	return metadata.EnsurePublicKeysMetadata(file, publicKeys)
}

func fetchRemoteWithRegistration(host string, useHTTP bool) (string, error) {
	remoteFile, err := fetchFromHostFunc(host, useHTTP)
	if err != nil {
		return "", err
	}

	// If the remote file is encrypted and we can't decrypt it, auto-register
	// with the peer so it re-encrypts secrets with our key, then re-fetch
	if !useHTTP && keys.IsFileEncrypted(remoteFile) && !keys.CanDecryptFile(remoteFile) {
		_ = os.Remove(remoteFile)
		logging.Log("INFO", "Local key not in recipients list on "+host+", registering and triggering re-encryption...")
		if err := ensureRegisteredWithPeerFunc(host); err != nil {
			return "", fmt.Errorf("cannot decrypt secrets from %s and failed to register: %w", host, err)
		}
		remoteFile, err = fetchFromHostFunc(host, useHTTP)
		if err != nil {
			return "", fmt.Errorf("failed to re-fetch secrets from %s after registration: %w", host, err)
		}
		if keys.IsFileEncrypted(remoteFile) && !keys.CanDecryptFile(remoteFile) {
			_ = os.Remove(remoteFile)
			return "", errors.New("still cannot decrypt secrets from " + host + " after registration")
		}
		logging.Log("SUCCESS", "Registered with "+host+" and re-fetched secrets")
	}

	return remoteFile, nil
}
