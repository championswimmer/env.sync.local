package syncer

import (
	"encoding/json"
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
	"envsync/internal/identity"
	"envsync/internal/keys"
	"envsync/internal/logging"
	"envsync/internal/metadata"
	"envsync/internal/mode"
	"envsync/internal/peer"
	"envsync/internal/secrets"
	httptransport "envsync/internal/transport/http"
	mtlstransport "envsync/internal/transport/mtls"
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

	currentMode := mode.GetMode()

	// Mode overrides InsecureHTTP flag
	switch currentMode {
	case config.ModeDevPlaintextHTTP:
		opts.InsecureHTTP = true
		printHTTPWarning()
	case config.ModeSecurePeer:
		opts.InsecureHTTP = false
		return runSecurePeerSync(opts)
	default: // trusted-owner-ssh
		if opts.InsecureHTTP {
			printHTTPWarning()
		}
	}

	maybeReencryptLocal()

	if opts.TargetHost != "" {
		ep := resolveEndpoint(opts.TargetHost)
		if ep.IsSelf() {
			logging.Log("INFO", "Target is self, nothing to sync")
			return nil
		}
		dial := ep.SSHDial()
		if opts.InsecureHTTP {
			dial = ep.Dial("")
		}
		if !opts.InsecureHTTP {
			if err := testSSHFunc(dial); err != nil {
				logging.Log("ERROR", "Cannot SSH to "+dial)
				logging.Log("INFO", "Ensure SSH keys are set up, or switch mode: env-sync mode set dev-plaintext-http")
				return err
			}
			cachePeerPubkeyFor(ep)
		}
		return syncFromHost(dial, opts.InsecureHTTP, opts.ForcePull)
	}

	if opts.AllPeers {
		logging.Log("INFO", "Syncing from all discovered peers...")
		peers, err := discoverPeers(opts.InsecureHTTP)
		if err != nil || len(peers) == 0 {
			logging.Log("WARN", "No peers discovered")
			return errors.New("no peers")
		}
		success := 0
		eps := resolveEndpoints(peers)
		for i := range peers {
			ep := eps[i]
			if ep.IsSelf() {
				continue
			}
			dial := ep.Dial("")
			if !opts.InsecureHTTP {
				dial = ep.SSHDial()
				if err := testSSHFunc(dial); err != nil {
					continue
				}
			}
			if err := syncFromHost(dial, opts.InsecureHTTP, opts.ForcePull); err == nil {
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
			logging.Log("INFO", "To sync without SSH: env-sync mode set dev-plaintext-http --yes")
		}
		return nil
	}
	logging.Log("INFO", "Newest secrets found on: "+newestHost)
	return syncFromHost(newestHost, opts.InsecureHTTP, false)
}

// runSecurePeerSync handles Mode C (secure-peer) synchronization via mTLS.
func runSecurePeerSync(opts Options) error {
	logging.Log("INFO", "Syncing in secure-peer mode (mTLS)...")

	// Ensure local identity exists (per-user "user@host" CN).
	if _, err := identity.EnsureIdentity(secrets.LocalPeerID()); err != nil {
		return fmt.Errorf("failed to ensure transport identity: %w", err)
	}

	maybeReencryptLocal()

	// Discover peers
	peers, err := discoverPeersFunc(false)
	if err != nil || len(peers) == 0 {
		logging.Log("INFO", "No peers discovered")
		return nil
	}

	// Load peer registry
	reg, err := peer.LoadRegistry()
	if err != nil {
		return fmt.Errorf("failed to load peer registry: %w", err)
	}

	// Sync membership events from all discovered peers
	// This helps us learn about other peers and their trust relationships.
	// One shared discovery pass resolves per-user ports (two users on one
	// host advertise different ports).
	eps := resolveEndpoints(peers)
	for i, p := range peers {
		ep := eps[i]
		if ep.IsSelf() {
			continue
		}
		// Try to sync from any peer - the server will reject if we're not authorized
		if err := syncMembershipEvents(ep.Dial(""), ep.PeerID); err != nil {
			logging.Log("DEBUG", fmt.Sprintf("Failed to sync membership events from %s: %v", p, err))
		}
	}

	// Reload registry after syncing events
	reg, err = peer.LoadRegistry()
	if err != nil {
		return fmt.Errorf("failed to reload peer registry: %w", err)
	}

	// Fetch secrets from all discovered peers (server will reject unauthorized)
	// Try each peer and merge the newest secrets
	var newestContent []byte
	var newestHost string
	var newestTime time.Time

	for i, p := range peers {
		ep := eps[i]
		if ep.IsSelf() {
			continue
		}

		// Get peer info from registry if available
		peerID := ep.PeerID
		if registered, err := reg.GetPeer(ep.PeerID); err == nil {
			peerID = registered.ID
		} else if registered, err := reg.GetPeerByHostname(ep.Host); err == nil {
			peerID = registered.ID
		}

		data, err := mtlstransport.FetchSecrets(ep.Dial(""), peerID)
		if err != nil {
			logging.Log("DEBUG", fmt.Sprintf("Failed to fetch secrets from %s: %v", p, err))
			continue
		}

		// Write to temp file and validate
		tmpFile, err := os.CreateTemp("", "env-sync-secure-")
		if err != nil {
			continue
		}
		tmpPath := tmpFile.Name()
		_ = tmpFile.Close()
		if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
			_ = os.Remove(tmpPath)
			continue
		}

		if err := secrets.ValidateSecretsFile(tmpPath); err != nil {
			logging.Log("DEBUG", "Invalid secrets from "+p)
			_ = os.Remove(tmpPath)
			continue
		}

		// Check if decryptable, request re-encryption if not
		if keys.IsFileEncrypted(tmpPath) && !keys.CanDecryptFile(tmpPath) {
			logging.Log("INFO", "Cannot decrypt secrets from "+p+", requesting re-encryption...")
			agePubkey := keys.GetLocalPubkey()
			if agePubkey != "" && peerID != "" {
				if err := mtlstransport.RequestReencrypt(ep.Dial(""), peerID, agePubkey); err != nil {
					logging.Log("DEBUG", "Re-encryption request failed: "+err.Error())
				}
			}
			_ = os.Remove(tmpPath)
			continue
		}

		// Check if this is newer than what we have
		remoteTime := metadata.GetFileTimestamp(tmpPath)
		if newestContent == nil || (newestTime.IsZero() && remoteTime != "") {
			newestContent, _ = os.ReadFile(tmpPath)
			newestHost = p
			if remoteTime != "" {
				newestTime, _ = time.Parse(time.RFC3339, remoteTime)
			}
		} else if !newestTime.IsZero() && remoteTime != "" {
			remoteTimeParsed, err := time.Parse(time.RFC3339, remoteTime)
			if err == nil && remoteTimeParsed.After(newestTime) {
				newestContent, _ = os.ReadFile(tmpPath)
				newestHost = p
				newestTime = remoteTimeParsed
			}
		}
	}

	// If we got newer content from a peer, merge it with local
	if newestContent != nil {
		tmpFile, err := os.CreateTemp("", "env-sync-secure-")
		if err != nil {
			return nil
		}
		tmpPath := tmpFile.Name()
		_ = tmpFile.Close()
		if err := os.WriteFile(tmpPath, newestContent, 0o600); err != nil {
			_ = os.Remove(tmpPath)
			return nil
		}

		if _, err := os.Stat(config.SecretsFile()); err != nil {
			if err := copyFile(tmpPath, config.SecretsFile()); err != nil {
				_ = os.Remove(tmpPath)
				return nil
			}
			logging.Log("SUCCESS", "Synced secrets from "+newestHost+" (mTLS)")
		} else {
			_ = backup.CreateBackup(config.SecretsFile())
			localContent, _ := secrets.GetSecretsContent(config.SecretsFile())
			remoteContent, _ := secrets.GetSecretsContent(tmpPath)
			merged := secrets.MergeSecretsContent(localContent, remoteContent)
			if err := secrets.SetSecretsContent(config.SecretsFile(), merged); err != nil {
				_ = os.Remove(tmpPath)
				return nil
			}
			logging.Log("SUCCESS", "Merged secrets from "+newestHost+" (mTLS)")
		}
		_ = os.Remove(tmpPath)
	}

	return nil
}

func syncMembershipEvents(host string, peerID string) error {
	log, err := peer.LoadMembershipLog()
	if err != nil {
		return err
	}

	eventsData, err := mtlstransport.FetchMembershipEvents(host, peerID, log.LastEvent)
	if err != nil {
		return err
	}

	var events []peer.MembershipEvent
	if err := json.Unmarshal(eventsData, &events); err != nil {
		return fmt.Errorf("failed to parse membership events: %w", err)
	}

	for _, event := range events {
		if err := peer.AppendEvent(log, &event); err != nil {
			logging.Log("WARN", fmt.Sprintf("Skipping event %d: %v", event.EventID, err))
			continue
		}
	}

	if len(events) > 0 {
		if err := peer.SaveMembershipLog(log); err != nil {
			return err
		}
		// Apply to registry
		reg, err := peer.LoadRegistry()
		if err != nil {
			return err
		}
		_, _ = peer.ApplyEvents(reg, log, 0)
		_ = peer.SaveRegistry(reg)
		logging.Log("INFO", fmt.Sprintf("Applied %d membership events from %s", len(events), host))
	}

	return nil
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

// resolveEndpoint parses a peer name ("host", "host:port",
// "user@host[:port]") into an Endpoint and fills in the discovered
// service port when the name did not carry one explicitly.
// NOTE: single-peer lookup does a fresh discovery pass; for loops over
// many peers prefer resolveEndpoints (one shared pass).
func resolveEndpoint(name string) secrets.Endpoint {
	ep := secrets.ParseEndpoint(name)
	if ep.Port != "" {
		return ep
	}
	if port := discovery.PortForPeer(ep.PeerID); port != "" {
		ep.Port = port
	}
	return ep
}

// resolveEndpoints parses many peer names with a single shared discovery
// pass for port resolution (avoids one mDNS round-trip per peer).
func resolveEndpoints(names []string) []secrets.Endpoint {
	eps := make([]secrets.Endpoint, 0, len(names))
	need := []string{}
	for _, n := range names {
		ep := secrets.ParseEndpoint(n)
		eps = append(eps, ep)
		if ep.Port == "" {
			need = append(need, n)
		}
	}
	if len(need) == 0 {
		return eps
	}
	ports := discovery.PortMap(need)
	for i := range eps {
		if eps[i].Port == "" {
			if port, ok := ports[names[i]]; ok {
				eps[i].Port = port
			}
		}
	}
	return eps
}

// cachePeerPubkeyFor fetches a peer's AGE pubkey over SSH and caches it
// under the canonical peer ID.
func cachePeerPubkeyFor(ep secrets.Endpoint) {
	pubkey := discovery.FetchPubkey(ep.SSHDial())
	if pubkey == "" {
		logging.Log("WARN", "Could not fetch public key from "+ep.Raw)
		return
	}
	if !keys.ValidatePubkey(pubkey) {
		logging.Log("WARN", "Invalid public key fetched from "+ep.Raw)
		return
	}
	id := ep.PeerID
	if id == "" {
		id = ep.Host
	}
	if err := keys.CachePeerPubkey(id, pubkey); err != nil {
		logging.Log("WARN", "Failed to cache public key from "+ep.Raw+": "+err.Error())
		return
	}
	logging.Log("SUCCESS", "Cached public key from "+ep.Raw)
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

	eps := resolveEndpoints(peers)
	for i := range peers {
		peer := peers[i]
		ep := eps[i]
		if ep.IsSelf() {
			continue
		}
		// Port-aware dial: two users on one host advertise different ports.
		dial := ep.Dial("")
		if !useHTTP {
			dial = ep.SSHDial()
			if err := testSSHFunc(dial); err != nil {
				logging.Log("DEBUG", "Cannot SSH to "+dial+" (skipping)")
				continue
			}
		}
		remoteFile, err := fetchRemoteWithRegistration(dial, useHTTP)
		if err != nil {
			continue
		}
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
	// Register under the per-user peer ID so two OS users on one machine
	// don't overwrite each other's cached keys on the peer.
	localPeerID := secrets.LocalPeerID()

	logging.Log("INFO", "Registering public key with "+host+" and triggering re-encryption...")
	if err := sshtransport.RegisterPubkeyWithPeer(host, localPubkey, localPeerID); err != nil {
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
