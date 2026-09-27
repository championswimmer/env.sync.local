package config

import (
	"os"
	"path/filepath"
	"strings"
)

// SyncMode represents the operation mode for env-sync.
type SyncMode string

const (
	Version              = "4.0.0"
	DefaultPort          = "5739"
	Service              = "_envsync._tcp"
	DefaultInitTimestamp = "1970-01-01T00:00:00Z"

	// Operation modes
	ModeDevPlaintextHTTP SyncMode = "dev-plaintext-http"
	ModeTrustedOwnerSSH  SyncMode = "trusted-owner-ssh"
	ModeSecurePeer       SyncMode = "secure-peer"

	DefaultMode = ModeTrustedOwnerSSH
)

var (
	verboseMode bool
)

// SetVerbose sets the global verbose mode
func SetVerbose(enabled bool) {
	verboseMode = enabled
	if enabled {
		_ = os.Setenv("ENV_SYNC_VERBOSE", "true")
	}
}

// IsVerbose returns whether verbose mode is enabled
func IsVerbose() bool {
	if verboseMode {
		return true
	}
	return strings.EqualFold(os.Getenv("ENV_SYNC_VERBOSE"), "true")
}

func HomeDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	return "."
}

func EnvSyncPort() string {
	if port := os.Getenv("ENV_SYNC_PORT"); port != "" {
		return port
	}
	return LocalPort()
}

// PortFile persists a per-user port choice so two OS users on the same
// machine can run servers side by side without colliding.
func PortFile() string {
	return filepath.Join(ConfigDir(), "port")
}

// ExplicitPort reports whether the port was explicitly pinned via
// ENV_SYNC_PORT. Explicit ports are never auto-bumped.
func ExplicitPort() bool {
	return os.Getenv("ENV_SYNC_PORT") != ""
}

// LocalPort returns this user's service port: explicit env override,
// else the persisted per-user choice, else the default.
func LocalPort() string {
	if data, err := os.ReadFile(PortFile()); err == nil {
		if port := strings.TrimSpace(string(data)); isValidPort(port) {
			return port
		}
	}
	return DefaultPort
}

// SetLocalPort persists this user's service port choice.
func SetLocalPort(port string) error {
	if !isValidPort(port) {
		return &portError{port}
	}
	if err := os.MkdirAll(ConfigDir(), 0o700); err != nil {
		return err
	}
	return os.WriteFile(PortFile(), []byte(port+"\n"), 0o600)
}

type portError struct{ port string }

func (e *portError) Error() string { return "invalid port: " + e.port }

func isValidPort(port string) bool {
	if port == "" || len(port) > 5 {
		return false
	}
	n := 0
	for _, r := range port {
		if r < '0' || r > '9' {
			return false
		}
		n = n*10 + int(r-'0')
	}
	return n > 0 && n <= 65535
}

func InitTimestamp() string {
	if ts := os.Getenv("ENV_SYNC_INIT_TIMESTAMP"); ts != "" {
		return ts
	}
	return DefaultInitTimestamp
}

func SecretsFile() string {
	return filepath.Join(HomeDir(), ".secrets.env")
}

func ConfigDir() string {
	return filepath.Join(HomeDir(), ".config", "env-sync")
}

func BackupDir() string {
	return filepath.Join(ConfigDir(), "backups")
}

func LogDir() string {
	return filepath.Join(ConfigDir(), "logs")
}

func AgeKeyDir() string {
	return filepath.Join(ConfigDir(), "keys")
}

func AgeKeyFile() string {
	return filepath.Join(AgeKeyDir(), "age_key")
}

func AgePubKeyFile() string {
	return filepath.Join(AgeKeyDir(), "age_key.pub")
}

func AgeCacheDir() string {
	return filepath.Join(AgeKeyDir(), "cache")
}

func AgeKnownHostsDir() string {
	return filepath.Join(AgeKeyDir(), "known_hosts")
}

func RequestsDir() string {
	return filepath.Join(ConfigDir(), "requests")
}

func ServerPidFile() string {
	return filepath.Join(ConfigDir(), "server.pid")
}

func ModeConfigFile() string {
	return filepath.Join(ConfigDir(), "mode.conf")
}

func TLSDir() string {
	return filepath.Join(ConfigDir(), "tls")
}

func TLSKeyFile() string {
	return filepath.Join(TLSDir(), "transport.key")
}

func TLSCertFile() string {
	return filepath.Join(TLSDir(), "transport.crt")
}

func PeerRegistryDir() string {
	return filepath.Join(ConfigDir(), "peers")
}

func PeerRegistryFile() string {
	return filepath.Join(PeerRegistryDir(), "registry.json")
}

func InviteDir() string {
	return filepath.Join(PeerRegistryDir(), "invites")
}

func MembershipEventsFile() string {
	return filepath.Join(PeerRegistryDir(), "membership_events.json")
}

func TrustedCertsDir() string {
	return filepath.Join(TLSDir(), "trusted")
}

// ValidSyncModes returns all valid sync modes.
func ValidSyncModes() []SyncMode {
	return []SyncMode{ModeDevPlaintextHTTP, ModeTrustedOwnerSSH, ModeSecurePeer}
}

// IsValidSyncMode checks if a mode string is valid.
func IsValidSyncMode(mode string) bool {
	for _, m := range ValidSyncModes() {
		if string(m) == mode {
			return true
		}
	}
	return false
}
