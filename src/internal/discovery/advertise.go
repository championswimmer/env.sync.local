package discovery

import (
	"io"
	"os/exec"
	"runtime"

	"envsync/internal/config"
	"envsync/internal/logging"
	"envsync/internal/secrets"
)

// Advertiser keeps an mDNS advertisement process running.
type Advertiser struct {
	cmd *exec.Cmd
}

// StartAdvertiser announces env-sync on the local network via mDNS/Bonjour.
// The instance name is per-user ("user@host") so two OS users on the same
// machine advertise distinct instances and can run side by side.
// Missing system binaries are treated as non-fatal and will skip advertising.
func StartAdvertiser(port string) (*Advertiser, error) {
	if port == "" {
		port = config.EnvSyncPort()
	}

	name := secrets.LocalInstanceName()
	switch runtime.GOOS {
	case "linux":
		return startAvahiAdvertiser(name, port)
	case "darwin":
		return startDnssdAdvertiser(name, port)
	default:
		logging.Log("WARN", "mDNS advertising is not supported on this platform")
		return nil, nil
	}
}

// Stop terminates the underlying advertisement process.
func (a *Advertiser) Stop() error {
	if a == nil || a.cmd == nil || a.cmd.Process == nil {
		return nil
	}
	_ = a.cmd.Process.Kill()
	_, _ = a.cmd.Process.Wait()
	return nil
}

func startAvahiAdvertiser(name, port string) (*Advertiser, error) {
	if _, err := exec.LookPath("avahi-publish-service"); err != nil {
		logging.Log("WARN", "avahi-publish-service not found; skipping mDNS advertisement")
		return nil, nil
	}
	args := []string{"avahi-publish-service", "-s", name, config.Service, port}
	logging.LogCommand(args...)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	logging.Log("INFO", "mDNS advertisement started via avahi-publish-service")
	return &Advertiser{cmd: cmd}, nil
}

func startDnssdAdvertiser(name, port string) (*Advertiser, error) {
	if _, err := exec.LookPath("dns-sd"); err != nil {
		logging.Log("WARN", "dns-sd not found; skipping mDNS advertisement")
		return nil, nil
	}
	args := []string{"dns-sd", "-R", name, config.Service, "local.", port}
	logging.LogCommand(args...)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	logging.Log("INFO", "mDNS advertisement started via dns-sd")
	return &Advertiser{cmd: cmd}, nil
}
