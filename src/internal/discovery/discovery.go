package discovery

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"envsync/internal/config"
	"envsync/internal/keys"
	"envsync/internal/logging"
	"envsync/internal/secrets"
	httptransport "envsync/internal/transport/http"
	sshtransport "envsync/internal/transport/ssh"
)

type Options struct {
	Timeout     time.Duration
	Quiet       bool
	Verbose     bool
	FilterSSH   bool
	CollectKeys bool
	ShowPubkeys bool
}

// Peer is a discovered env-sync instance. Instance is the raw mDNS
// instance name ("user@host" for new advertisers, bare hostname for
// legacy ones). PeerID is the canonical ID ("user@shorthost" or legacy
// short host). Host is dialable ("beelink.local"), Port is the advertised
// service port ("" means unknown / default).
type Peer struct {
	Instance string
	PeerID   string
	User     string
	Host     string
	Port     string
	Legacy   bool
}

// Dial returns "host" or "host:port" for HTTP/mTLS dialing.
func (p Peer) Dial() string {
	if p.Port != "" {
		return p.Host + ":" + p.Port
	}
	return p.Host
}

// Display returns the human-friendly name: peer ID for multi-user
// instances, dialable host for legacy ones.
func (p Peer) Display() string {
	if p.Legacy {
		return p.Host
	}
	return p.PeerID
}

// Endpoint converts to a secrets.Endpoint for SSH/transport use.
func (p Peer) Endpoint() secrets.Endpoint {
	e := secrets.ParseEndpoint(p.Display())
	if e.Port == "" {
		e.Port = p.Port
	}
	return e
}

// Discover returns display names for backward compatibility.
// New code should prefer DiscoverPeers for structured identity.
func Discover(opts Options) ([]string, error) {
	peers, err := DiscoverPeers(opts)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(peers))
	for _, p := range peers {
		names = append(names, p.Display())
	}
	return names, nil
}

// DiscoverPeers discovers env-sync instances with structured identity
// (user, host, port). Display() values are backward compatible with the
// old Discover() string list.
func DiscoverPeers(opts Options) ([]Peer, error) {
	if !opts.Quiet {
		logging.Log("INFO", fmt.Sprintf("Discovering env-sync peers (timeout: %ds)...", int(opts.Timeout.Seconds())))
	}

	var peers []Peer
	var err error

	switch runtime.GOOS {
	case "linux":
		peers, err = discoverAvahiPeers(opts.Timeout)
	case "darwin":
		peers, err = discoverDnssdPeers(opts.Timeout)
	default:
		peers, err = discoverFallbackPeers(opts.Timeout)
	}

	if err != nil {
		return nil, err
	}

	peers = uniqueSortedPeers(peers)

	if opts.FilterSSH {
		filtered := make([]Peer, 0, len(peers))
		for _, p := range peers {
			if sshtransport.TestSSH(p.Endpoint().SSHDial()) == nil {
				filtered = append(filtered, p)
			}
		}
		peers = filtered
	}

	if opts.Quiet {
		return peers, nil
	}

	if len(peers) == 0 {
		logging.Log("WARN", "No env-sync peers found on local network")
		fmt.Println()
		return peers, nil
	}

	logging.Log("SUCCESS", fmt.Sprintf("Found %d peer(s):", len(peers)))
	for _, p := range peers {
		display := p.Display()
		info := ""
		if p.Port != "" && p.Port != config.DefaultPort {
			info = fmt.Sprintf(" (port: %s)", p.Port)
		}
		if opts.Verbose {
			health, err := FetchHealth(p.Dial())
			if err == nil && health.Version != "" {
				info = info + fmt.Sprintf(" (version: %s)", health.Version)
			}
		}
		if opts.CollectKeys || opts.ShowPubkeys {
			pubkey := fetchPubkey(p.Endpoint().SSHDial())
			if pubkey != "" {
				_ = keys.CachePeerPubkey(p.PeerID, pubkey)
				if opts.ShowPubkeys {
					info = info + fmt.Sprintf(" [pubkey: %s...]", truncate(pubkey, 20))
				}
			}
		}
		fmt.Printf("  - %s%s\n", display, info)
	}

	if opts.CollectKeys {
		logging.Log("INFO", "Collecting public keys from peers...")
		collected := 0
		for _, p := range peers {
			pubkey := fetchPubkey(p.Endpoint().SSHDial())
			if pubkey != "" {
				_ = keys.CachePeerPubkey(p.PeerID, pubkey)
				logging.Log("SUCCESS", "Cached public key from "+p.Display())
				collected++
			} else {
				logging.Log("WARN", "Could not fetch public key from "+p.Display())
			}
		}
		if collected > 0 {
			logging.Log("SUCCESS", fmt.Sprintf("Collected %d public key(s)", collected))
			logging.Log("INFO", "Run 'env-sync' to sync with new recipients")
		}
	}

	return peers, nil
}

// peerFromInstance builds a Peer from a raw mDNS instance name and the
// resolved host/port. Instances containing "@" are multi-user
// ("user@host"); anything else is a legacy bare-hostname advertisement.
func peerFromInstance(instance, host, port string) Peer {
	instance = strings.TrimSpace(instance)
	if at := strings.LastIndex(instance, "@"); at >= 0 {
		ep := secrets.ParseEndpoint(instance)
		return Peer{
			Instance: instance,
			PeerID:   ep.PeerID,
			User:     ep.User,
			Host:     ep.Host,
			Port:     port,
			Legacy:   false,
		}
	}
	short := secrets.ShortHost(instance)
	dialHost := secrets.DialHost(short)
	if h := strings.TrimSpace(host); h != "" {
		// Prefer the SRV target when the resolver gave us one.
		dialHost = secrets.DialHost(secrets.ShortHost(h))
	}
	return Peer{
		Instance: instance,
		PeerID:   short,
		User:     "",
		Host:     dialHost,
		Port:     port,
		Legacy:   true,
	}
}

// isSelfPeer reports whether a discovered peer is this user on this machine.
// New-style instances compare by full peer ID. Legacy bare-hostname
// advertisements can only be self if host AND port both match, so a second
// user on the same host running a legacy server on another port is kept.
func isSelfPeer(p Peer) bool {
	if p.PeerID == secrets.LocalPeerID() {
		return true
	}
	if p.Legacy {
		localHost := secrets.DialHost(secrets.ShortHost(sshtransport.Hostname()))
		if !strings.EqualFold(p.Host, localHost) {
			return false
		}
		if p.Port == "" || p.Port == config.EnvSyncPort() {
			return true
		}
	}
	return false
}

func uniqueSortedPeers(peers []Peer) []Peer {
	seen := map[string]bool{}
	unique := []Peer{}
	for _, p := range peers {
		key := p.PeerID + "\x00" + p.Port
		if p.PeerID == "" || seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, p)
	}
	sort.Slice(unique, func(i, j int) bool {
		if unique[i].PeerID == unique[j].PeerID {
			return unique[i].Port < unique[j].Port
		}
		return unique[i].PeerID < unique[j].PeerID
	})
	return unique
}

func discoverAvahiPeers(timeout time.Duration) ([]Peer, error) {
	if _, err := exec.LookPath("avahi-browse"); err != nil {
		logging.Log("ERROR", "avahi-browse not found. Install with: sudo apt-get install avahi-utils")
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := []string{"avahi-browse", "-t", "-r", "-p", config.Service}
	logging.LogCommand(args...)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	output, _ := cmd.Output()
	return parseAvahiPeers(string(output)), nil
}

// parseAvahiPeers parses `avahi-browse -t -r -p` output.
// Resolved lines look like:
//
//	=;eth0;IPv4;arnav@beelink;_envsync._tcp;local;beelink.local;192.168.1.5;5740;...
func parseAvahiPeers(output string) []Peer {
	peers := []Peer{}
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "=") {
			continue
		}
		fields := strings.Split(line, ";")
		if len(fields) < 7 {
			continue
		}
		instance := strings.TrimSpace(fields[3])
		if instance == "" {
			continue
		}
		host := strings.TrimSuffix(strings.TrimSpace(fields[6]), ".")
		port := ""
		if len(fields) > 8 {
			if p := strings.TrimSpace(fields[8]); p != "" && isPort(p) {
				port = p
			}
		}
		p := peerFromInstance(instance, host, port)
		if isSelfPeer(p) {
			continue
		}
		peers = append(peers, p)
	}
	return peers
}

func discoverDnssdPeers(timeout time.Duration) ([]Peer, error) {
	if _, err := exec.LookPath("dns-sd"); err != nil {
		logging.Log("ERROR", "dns-sd not found. This should be built into macOS.")
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := []string{"dns-sd", "-B", config.Service, "local."}
	logging.LogCommand(args...)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)

	// dns-sd -B is a monitoring command that doesn't exit on its own.
	// We need to read output as it arrives, not wait for the process to exit.
	// Use StdoutPipe to read streaming output before the context timeout kills the process.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Read output in a channel to handle timeout properly
	outputChan := make(chan string, 1)
	go func() {
		var builder strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				builder.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		outputChan <- builder.String()
	}()

	// Wait for either the output to be collected or context timeout
	var output string
	select {
	case output = <-outputChan:
		// Output collected successfully, process should have exited
	case <-ctx.Done():
		// Timeout reached - context will kill the process
		// Try to get partial output if available (short timeout)
		select {
		case output = <-outputChan:
		case <-time.After(100 * time.Millisecond):
			// Give up after a short wait
		}
	}

	// Always wait for the process to clean up
	_ = cmd.Wait()

	instances := parseDnssdInstances(output)
	return resolveDnssdPeers(instances, timeout), nil
}

// parseDnssdInstances extracts raw instance names from `dns-sd -B` output.
// dns-sd -B output format:
//
//	Timestamp     A/R    Flags  if Domain               Service Type         Instance Name
//	3:26:33.519  Add        3  14 local.               _envsync._tcp.       arnav@beelink
//
// NOTE: the instance name is the last field but may itself contain spaces,
// so this takes everything after the service-type column instead of just
// the last whitespace-separated token.
func parseDnssdInstances(output string) []string {
	lines := strings.Split(output, "\n")
	instances := []string{}
	seen := make(map[string]bool)

	for _, line := range lines {
		if line == "" || strings.Contains(line, "Timestamp") || strings.Contains(line, "STARTING") || strings.Contains(line, "DATE:") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}

		if fields[1] != "Add" {
			continue
		}

		serviceType := strings.TrimSuffix(fields[5], ".")
		if serviceType != config.Service {
			continue
		}

		// Instance name is everything after the service type column.
		idx := strings.Index(line, fields[5])
		if idx < 0 {
			continue
		}
		instanceName := strings.TrimSpace(line[idx+len(fields[5]):])
		if instanceName == "" || seen[instanceName] {
			continue
		}
		seen[instanceName] = true
		instances = append(instances, instanceName)
	}

	return instances
}

// parseDnssdPeers keeps the legacy []string behavior (display names) for
// existing callers/tests. Resolution is best-effort: ports are omitted.
func parseDnssdPeers(output string) []string {
	peers := []Peer{}
	for _, instance := range parseDnssdInstances(output) {
		p := peerFromInstance(instance, "", "")
		if isSelfPeer(p) {
			continue
		}
		peers = append(peers, p)
	}
	peers = uniqueSortedPeers(peers)
	names := make([]string, 0, len(peers))
	for _, p := range peers {
		names = append(names, p.Display())
	}
	return names
}

// resolveDnssdPeers resolves each browsed instance via `dns-sd -L` to learn
// its SRV host/port. Unresolvable instances are kept with empty port
// (callers fall back to the default port). Self is filtered after
// resolution so legacy same-host/different-port peers survive.
func resolveDnssdPeers(instances []string, timeout time.Duration) []Peer {
	if len(instances) == 0 {
		return []Peer{}
	}

	perLookup := 2 * time.Second
	if timeout < perLookup {
		perLookup = timeout
	}

	type result struct {
		idx  int
		peer Peer
	}
	results := make([]result, len(instances))
	var wg sync.WaitGroup
	for i, instance := range instances {
		wg.Add(1)
		go func(i int, instance string) {
			defer wg.Done()
			host, port := resolveDnssdInstance(instance, perLookup)
			results[i] = result{i, peerFromInstance(instance, host, port)}
		}(i, instance)
	}
	wg.Wait()

	peers := make([]Peer, 0, len(instances))
	for _, r := range results {
		if isSelfPeer(r.peer) {
			continue
		}
		peers = append(peers, r.peer)
	}
	return peers
}

// resolveDnssdInstance runs `dns-sd -L <instance> _envsync._tcp local.`
// and parses the SRV host/port. Returns ("", "") on failure.
func resolveDnssdInstance(instance string, timeout time.Duration) (string, string) {
	if _, err := exec.LookPath("dns-sd"); err != nil {
		return "", ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := []string{"dns-sd", "-L", instance, config.Service, "local."}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	output, _ := cmd.Output()
	return parseDnssdLookup(string(output))
}

var (
	// e.g. "beelink.local.:5739" or "beelink.local:5739"
	reachableAt = regexp.MustCompile(`can be reached at\s+(\S+?):(\d+)\b`)
	// e.g. "Host: beelink.local  Port: 5740"
	hostPortLine = regexp.MustCompile(`(?mi)^\s*host\s*:\s*(\S+)\s+port\s*:\s*(\d+)`)
)

func parseDnssdLookup(output string) (string, string) {
	if m := reachableAt.FindStringSubmatch(output); len(m) == 3 {
		return strings.TrimSuffix(m[1], "."), m[2]
	}
	if m := hostPortLine.FindStringSubmatch(output); len(m) == 3 {
		return strings.TrimSuffix(m[1], "."), m[2]
	}
	return "", ""
}

func discoverFallbackPeers(timeout time.Duration) ([]Peer, error) {
	logging.Log("WARN", "Using fallback discovery method (limited functionality)")

	commonHosts := []string{
		"beelink.local",
		"mbp16.local",
		"razer.local",
		"macbook.local",
		"macbook-pro.local",
		"macbook-air.local",
		"imac.local",
		"mac-mini.local",
		"ubuntu.local",
		"debian.local",
		"fedora.local",
		"linux.local",
		"raspberrypi.local",
		"pi.local",
	}

	peers := []Peer{}
	for _, host := range commonHosts {
		if host == sshtransport.Hostname() {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		pingArgs := []string{"ping", "-c", "1", "-W", "1", host}
		logging.LogCommand(pingArgs...)
		ping := exec.CommandContext(ctx, pingArgs[0], pingArgs[1:]...)
		_ = ping.Run()
		cancel()
		health, err := FetchHealth(host)
		if err == nil && health.Status == "ok" {
			p := peerFromInstance(secrets.ShortHost(host), host, "")
			if !isSelfPeer(p) {
				peers = append(peers, p)
			}
		}
	}
	return peers, nil
}

type HealthResponse = httptransport.HealthResponse

func FetchHealth(host string) (HealthResponse, error) {
	return httptransport.FetchHealth(host)
}

func FetchPubkey(host string) string {
	return fetchPubkey(host)
}

func fetchPubkey(host string) string {
	args := []string{"ssh", "-n", "-o", "ConnectTimeout=3", "-o", "StrictHostKeyChecking=" + sshtransport.HostKeyCheckingMode(), host, "cat ~/.config/env-sync/keys/age_key.pub"}
	logging.LogCommand(args...)
	cmd := exec.Command(args[0], args[1:]...)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// PortMap returns advertised ports for the given peer IDs/hosts from a
// single discovery pass. Keys are canonical peer IDs ("user@shorthost")
// plus dialable hosts for legacy entries. Missing entries mean unknown.
func PortMap(ids []string) map[string]string {
	peers, err := DiscoverPeers(Options{Timeout: 5 * time.Second, Quiet: true})
	if err != nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(ids))
	byPeer := make(map[string]string, len(peers))
	byHost := make(map[string]string, len(peers))
	for _, p := range peers {
		if p.PeerID != "" && p.Port != "" {
			if _, ok := byPeer[p.PeerID]; !ok {
				byPeer[p.PeerID] = p.Port
			}
		}
		if p.Host != "" && p.Port != "" {
			if _, ok := byHost[p.Host]; !ok {
				byHost[p.Host] = p.Port
			}
		}
	}
	for _, id := range ids {
		want := secrets.ParseEndpoint(id)
		if want.Legacy {
			if port, ok := byHost[want.Host]; ok {
				out[id] = port
			}
			continue
		}
		if port, ok := byPeer[want.PeerID]; ok {
			out[id] = port
		}
	}
	return out
}

// PortForPeer returns the advertised port for a peer ID ("user@host") or
// legacy host from a fresh discovery pass. Empty string means unknown.
func PortForPeer(id string) string {
	peers, err := DiscoverPeers(Options{Timeout: 5 * time.Second, Quiet: true})
	if err != nil {
		return ""
	}
	want := secrets.ParseEndpoint(id)
	for _, p := range peers {
		if want.Legacy {
			if strings.EqualFold(p.Host, want.Host) {
				return p.Port
			}
			continue
		}
		if p.PeerID == want.PeerID {
			return p.Port
		}
	}
	return ""
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	unique := []string{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		unique = append(unique, value)
	}
	sort.Strings(unique)
	return unique
}

func isPort(s string) bool {
	n, err := strconv.Atoi(s)
	if err != nil {
		return false
	}
	return n > 0 && n <= 65535
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}

func ParseOptions(args []string) (Options, []string, error) {
	opts := Options{Timeout: 5 * time.Second}
	remaining := []string{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-t", "--timeout":
			if i+1 >= len(args) {
				return opts, remaining, errors.New("timeout required")
			}
			duration, err := time.ParseDuration(args[i+1] + "s")
			if err != nil {
				return opts, remaining, err
			}
			opts.Timeout = duration
			i++
		case "-q", "--quiet":
			opts.Quiet = true
		case "-v", "--verbose":
			opts.Verbose = true
		case "--ssh":
			opts.FilterSSH = true
		case "--collect-keys":
			opts.CollectKeys = true
		case "--pubkeys":
			opts.ShowPubkeys = true
		case "-h", "--help":
			return opts, remaining, errors.New("help")
		default:
			if strings.HasPrefix(args[i], "-") {
				return opts, remaining, fmt.Errorf("unknown option: %s", args[i])
			}
			remaining = append(remaining, args[i])
		}
	}
	return opts, remaining, nil
}
