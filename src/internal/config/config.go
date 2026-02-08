package config

import (
	"os"
	"path/filepath"
)

const (
	Version              = "2.0.0"
	DefaultPort          = "5739"
	Service              = "_envsync._tcp"
	DefaultInitTimestamp = "1970-01-01T00:00:00Z"
)

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
	return DefaultPort
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
