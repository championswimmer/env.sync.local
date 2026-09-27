package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/kardianos/service"

	"envsync/internal/config"
	"envsync/internal/discovery"
	"envsync/internal/identity"
	"envsync/internal/logging"
	"envsync/internal/metadata"
	"envsync/internal/mode"
	"envsync/internal/peer"
	"envsync/internal/secrets"
	syncer "envsync/internal/sync"
	mtlstransport "envsync/internal/transport/mtls"
)

type Options struct {
	Port string
	// PortExplicit is set when the user pinned the port via -p/--port.
	// Explicit ports are never auto-bumped for multi-user coexistence.
	PortExplicit bool
	Daemon       bool
	Quiet        bool
	Service      bool
	SyncInterval time.Duration
}

func Run(opts Options) error {
	if opts.Port == "" {
		opts.Port = config.EnvSyncPort()
	}
	// Two OS users on one machine share the default port: the second
	// user transparently bumps to a free per-user port (persisted).
	// Never bump explicitly pinned ports (-p/--port or ENV_SYNC_PORT).
	if !opts.PortExplicit {
		opts.Port = ensureLocalPort(opts.Port)
	}
	if opts.SyncInterval == 0 {
		opts.SyncInterval = 30 * time.Minute
	}

	if err := secrets.ValidateSecretsFile(config.SecretsFile()); err != nil {
		logging.Log("ERROR", "Invalid secrets file")
		return err
	}

	if opts.Service {
		return runService(opts)
	}
	if opts.Daemon {
		return runDaemon(opts)
	}

	return runServer(opts)
}

func runDaemon(opts Options) error {
	svc, err := newService(opts)
	if err != nil {
		return err
	}
	status, err := svc.Status()
	if err != nil {
		if !errors.Is(err, service.ErrNotInstalled) {
			return err
		}
		if err := svc.Install(); err != nil {
			return err
		}
	} else if status == service.StatusRunning {
		logging.Log("INFO", "Server service already running")
		return nil
	}

	if err := svc.Start(); err != nil {
		return err
	}
	logging.Log("SUCCESS", "Server service started")
	return nil
}

func runServer(opts Options) error {
	currentMode := mode.GetMode()

	switch currentMode {
	case config.ModeDevPlaintextHTTP:
		logging.Log("WARNING", "⚠️  Running in dev-plaintext-http mode — NO SECURITY")
		server, err := buildPlaintextServer(opts)
		if err != nil {
			return err
		}
		return server.ListenAndServe()

	case config.ModeSecurePeer:
		logging.Log("INFO", "Starting secure-peer server (HTTPS + mTLS)")
		server, tlsConfig, err := buildSecureServer(opts)
		if err != nil {
			return err
		}
		server.TLSConfig = tlsConfig
		return server.ListenAndServeTLS("", "")

	default: // trusted-owner-ssh
		server, err := buildPlaintextServer(opts)
		if err != nil {
			return err
		}
		return server.ListenAndServe()
	}
}

func runService(opts Options) error {
	svc, err := newService(opts)
	if err != nil {
		return err
	}
	return svc.Run()
}

func newService(opts Options) (service.Service, error) {
	prg := &serviceProgram{opts: opts}
	svcConfig := &service.Config{
		Name:        "env-sync",
		DisplayName: "env-sync",
		Description: "env-sync server",
		Arguments:   serviceArgs(opts),
		Option:      service.KeyValue{"UserService": true},
	}
	return service.New(prg, svcConfig)
}

func serviceArgs(opts Options) []string {
	args := []string{"serve", "--service"}
	if opts.Port != "" {
		args = append(args, "--port", opts.Port)
	}
	if opts.Quiet {
		args = append(args, "--quiet")
	}
	return args
}

type serviceProgram struct {
	opts       Options
	server     *http.Server
	cancel     context.CancelFunc
	advertiser *discovery.Advertiser
}

func (p *serviceProgram) Start(_ service.Service) error {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	currentMode := mode.GetMode()

	var server *http.Server
	var err error

	switch currentMode {
	case config.ModeSecurePeer:
		var tlsConfig *tls.Config
		server, tlsConfig, err = buildSecureServer(p.opts)
		if err != nil {
			return err
		}
		server.TLSConfig = tlsConfig
		go func() {
			if err := server.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logging.Log("ERROR", fmt.Sprintf("Server stopped: %v", err))
			}
		}()
	default:
		server, err = buildPlaintextServer(p.opts)
		if err != nil {
			return err
		}
		go func() {
			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logging.Log("ERROR", fmt.Sprintf("Server stopped: %v", err))
			}
		}()
	}

	p.server = server

	if adv, err := discovery.StartAdvertiser(p.opts.Port); err == nil {
		p.advertiser = adv
	} else {
		logging.Log("WARN", fmt.Sprintf("Failed to start mDNS advertisement: %v", err))
	}
	go startSyncLoop(ctx, p.opts)
	return nil
}

func (p *serviceProgram) Stop(_ service.Service) error {
	if p.cancel != nil {
		p.cancel()
	}
	if p.advertiser != nil {
		_ = p.advertiser.Stop()
	}
	if p.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return p.server.Shutdown(ctx)
}

func startSyncLoop(ctx context.Context, opts Options) {
	syncInterval := opts.SyncInterval
	if syncInterval <= 0 {
		syncInterval = 30 * time.Minute
	}

	runSync := func() {
		if _, err := os.Stat(config.SecretsFile()); err != nil {
			logging.Log("WARN", "Skipping sync: secrets file not found")
			return
		}
		syncOpts := syncer.Options{Quiet: true}
		if err := syncer.Run(syncOpts); err != nil {
			logging.Log("WARN", fmt.Sprintf("Background sync failed: %v", err))
		}
	}

	runSync()
	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runSync()
		}
	}
}

// buildPlaintextServer creates an HTTP server for dev-plaintext-http and trusted-owner-ssh modes.
func buildPlaintextServer(opts Options) (*http.Server, error) {
	if err := checkPort(opts.Port); err != nil {
		logging.Log("WARN", fmt.Sprintf("Port %s is already in use", opts.Port))
		return nil, err
	}

	if err := os.MkdirAll(config.ConfigDir(), 0o700); err == nil {
		_ = os.WriteFile(config.ServerPidFile(), []byte(fmt.Sprintf("%d", os.Getpid())), 0o600)
	}

	currentMode := mode.GetMode()
	mux := http.NewServeMux()

	// v1 endpoints (kept for trusted-owner-ssh compatibility)
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/secrets.env", secretsHandler)

	// v2 endpoints (available in all modes)
	mux.HandleFunc("/v2/health", v2HealthHandler)

	if currentMode == config.ModeDevPlaintextHTTP {
		// In dev mode, v2/secrets is accessible without auth (with warning)
		mux.HandleFunc("/v2/secrets", secretsHandler)
	}

	mux.HandleFunc("/", notFoundHandler)

	logging.Log("INFO", fmt.Sprintf("Starting env-sync server on port %s (mode: %s)", opts.Port, currentMode))
	logging.Log("SUCCESS", fmt.Sprintf("Server listening on http://0.0.0.0:%s", opts.Port))
	logging.Log("INFO", "Endpoints:")
	logging.Log("INFO", "  - GET /health         - Health check")
	logging.Log("INFO", "  - GET /v2/health      - Health check (v2)")
	logging.Log("INFO", "  - GET /secrets.env    - Get secrets file")

	server := &http.Server{Addr: ":" + opts.Port, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	return server, nil
}

// buildSecureServer creates an HTTPS+mTLS server for secure-peer mode.
func buildSecureServer(opts Options) (*http.Server, *tls.Config, error) {
	if err := checkPort(opts.Port); err != nil {
		logging.Log("WARN", fmt.Sprintf("Port %s is already in use", opts.Port))
		return nil, nil, err
	}

	if err := os.MkdirAll(config.ConfigDir(), 0o700); err == nil {
		_ = os.WriteFile(config.ServerPidFile(), []byte(fmt.Sprintf("%d", os.Getpid())), 0o600)
	}

	// Ensure transport identity exists. The identity is per-user
	// ("user@host" CN) so two OS users on one machine differ.
	if _, err := identity.EnsureIdentity(secrets.LocalPeerID()); err != nil {
		return nil, nil, fmt.Errorf("failed to ensure transport identity: %w", err)
	}

	tlsConfig, err := mtlstransport.NewServerTLSConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create TLS config: %w", err)
	}

	mux := http.NewServeMux()

	// v2 endpoints only for secure mode
	mux.HandleFunc("/v2/health", v2HealthHandler)
	mux.HandleFunc("/v2/secrets", v2SecretsHandler)
	mux.HandleFunc("/v2/membership/events", v2MembershipEventsHandler)
	mux.HandleFunc("/v2/peer/request-access", v2PeerRequestAccessHandler)
	mux.HandleFunc("/v2/peer/approve", v2PeerApproveHandler)
	mux.HandleFunc("/v2/secrets/request-reencrypt", v2RequestReencryptHandler)
	mux.HandleFunc("/", notFoundHandler)

	logging.Log("INFO", fmt.Sprintf("Starting env-sync secure server on port %s (mode: secure-peer)", opts.Port))
	logging.Log("SUCCESS", fmt.Sprintf("Server listening on https://0.0.0.0:%s (mTLS)", opts.Port))
	logging.Log("INFO", "v2 Endpoints (mTLS required):")
	logging.Log("INFO", "  - GET  /v2/health                  - Health check")
	logging.Log("INFO", "  - GET  /v2/secrets                 - Get encrypted secrets")
	logging.Log("INFO", "  - GET  /v2/membership/events       - Membership events")
	logging.Log("INFO", "  - POST /v2/peer/request-access     - Request peer access")
	logging.Log("INFO", "  - POST /v2/peer/approve            - Approve peer")
	logging.Log("INFO", "  - POST /v2/secrets/request-reencrypt - Request re-encryption")

	id, _ := identity.LoadIdentity()
	if id != nil {
		logging.Log("INFO", "Transport fingerprint: "+identity.Fingerprint(id.Certificate))
	}

	server := &http.Server{Addr: ":" + opts.Port, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	return server, tlsConfig, nil
}

// ---- v1 Handlers (unchanged) ----

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	version := metadata.GetFileVersion(config.SecretsFile())
	timestamp := metadata.GetFileTimestamp(config.SecretsFile())
	host := metadata.GetFileHost(config.SecretsFile())
	response := fmt.Sprintf(`{"status": "ok", "version": "%s", "timestamp": "%s", "host": "%s"}`, version, timestamp, host)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(response)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(response))
}

func secretsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	content, err := os.ReadFile(config.SecretsFile())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	version := metadata.GetFileVersion(config.SecretsFile())
	timestamp := metadata.GetFileTimestamp(config.SecretsFile())
	host := metadata.GetFileHost(config.SecretsFile())
	checksum := metadata.GetFileChecksum(config.SecretsFile())
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
	w.Header().Set("X-EnvSync-Version", version)
	w.Header().Set("X-EnvSync-Timestamp", timestamp)
	w.Header().Set("X-EnvSync-Host", host)
	w.Header().Set("X-EnvSync-Checksum", checksum)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

// ---- v2 Handlers ----

func v2HealthHandler(w http.ResponseWriter, _ *http.Request) {
	version := metadata.GetFileVersion(config.SecretsFile())
	timestamp := metadata.GetFileTimestamp(config.SecretsFile())
	host := metadata.GetFileHost(config.SecretsFile())
	currentMode := mode.GetMode()

	response := map[string]string{
		"status":    "ok",
		"version":   version,
		"timestamp": timestamp,
		"host":      host,
		"mode":      string(currentMode),
	}

	data, _ := json.Marshal(response)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func v2SecretsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// In secure-peer mode, check client cert authorization
	// Accept if: the peer is in our registry as approved, OR we are in the peer's registry as approved
	// (i.e., we approved them, so they should be able to fetch from us)
	peerID := extractPeerID(r)
	if peerID == "" {
		jsonError(w, http.StatusForbidden, "peer identity required")
		return
	}

	reg, err := peer.LoadRegistry()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to load peer registry")
		return
	}

	if !isPeerAllowed(reg, peerID, peer.CapRead) {
		jsonError(w, http.StatusForbidden, "peer not authorized to read secrets")
		return
	}

	content, err := os.ReadFile(config.SecretsFile())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
	w.Header().Set("X-EnvSync-Version", metadata.GetFileVersion(config.SecretsFile()))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func v2MembershipEventsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	sinceStr := r.URL.Query().Get("since")
	var since uint64
	if sinceStr != "" {
		var err error
		since, err = strconv.ParseUint(sinceStr, 10, 64)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "invalid 'since' parameter")
			return
		}
	}

	log, err := peer.LoadMembershipLog()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to load membership log")
		return
	}

	events := peer.GetEventsSince(log, since)
	data, _ := json.Marshal(events)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func v2PeerRequestAccessHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	var req struct {
		Token       string `json:"token"`
		PeerID      string `json:"peer_id"`
		Hostname    string `json:"hostname"`
		Fingerprint string `json:"fingerprint"`
		AGEPubkey   string `json:"age_pubkey"`
		CertPEM     string `json:"cert_pem"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	// Validate invite token
	invite, err := peer.ValidateInvite(req.Token)
	if err != nil {
		jsonError(w, http.StatusForbidden, err.Error())
		return
	}

	// Save the peer's certificate
	if req.CertPEM != "" {
		_ = identity.SaveTrustedCert(req.PeerID, []byte(req.CertPEM))
	}

	// Add peer to registry
	reg, err := peer.LoadRegistry()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to load registry")
		return
	}

	newPeer := peer.Peer{
		ID:                   req.PeerID,
		Hostname:             req.Hostname,
		TransportFingerprint: req.Fingerprint,
		AGEPubkey:            req.AGEPubkey,
		State:                peer.StatePending,
	}

	if err := reg.AddPeer(newPeer); err != nil {
		// If peer already exists, update it
		_ = reg.UpdatePeer(req.PeerID, func(p *peer.Peer) {
			p.TransportFingerprint = req.Fingerprint
			p.AGEPubkey = req.AGEPubkey
			p.Hostname = req.Hostname
		})
	}

	if err := peer.SaveRegistry(reg); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to save registry")
		return
	}

	_ = peer.MarkInviteUsed(invite.Token)

	logging.Log("INFO", fmt.Sprintf("Peer access request received from %s (%s)", req.Hostname, req.PeerID))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"pending","message":"access request received, awaiting approval"}`))
}

func v2PeerApproveHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	var req struct {
		PeerID string `json:"peer_id"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	reg, err := peer.LoadRegistry()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to load registry")
		return
	}

	if err := reg.ApprovePeer(req.PeerID); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := peer.SaveRegistry(reg); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to save registry")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"approved"}`))
}

func v2RequestReencryptHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	peerID := extractPeerID(r)
	if peerID == "" {
		jsonError(w, http.StatusForbidden, "peer identity required")
		return
	}

	reg, err := peer.LoadRegistry()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to load registry")
		return
	}

	if !isPeerAllowed(reg, peerID, peer.CapRequestReencrypt) {
		jsonError(w, http.StatusForbidden, "peer not authorized to request re-encryption")
		return
	}

	// Trigger background re-encryption
	go func() {
		syncer.ReencryptLocal()
		logging.Log("INFO", "Re-encryption completed for request from "+peerID)
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"accepted","message":"re-encryption queued"}`))
}

// extractPeerID extracts the peer ID from a client certificate CommonName.
// New identities use "user@host" CNs; legacy ones use a bare hostname.
func extractPeerID(r *http.Request) string {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return ""
	}
	return r.TLS.PeerCertificates[0].Subject.CommonName
}

// isPeerAllowed reports whether peerID may act with the given capability.
// Accepts the peer when it is approved with the capability, or when it is
// otherwise known-approved in the registry (lenient path that predates
// capabilities). Legacy bare-hostname CNs are matched by hostname as well
// as by ID so old clients keep working.
func isPeerAllowed(reg *peer.Registry, peerID string, cap peer.Capability) bool {
	if peerID == "" || reg == nil {
		return false
	}
	if reg.IsAuthorized(peerID, cap) {
		return true
	}
	if p, err := reg.GetPeer(peerID); err == nil && p.State == peer.StateApproved {
		return true
	}
	if p, err := reg.GetPeerByHostname(peerID); err == nil && p.State == peer.StateApproved {
		return true
	}
	return false
}

// ensureLocalPort returns a free port for this user. Explicitly pinned
// ports (ENV_SYNC_PORT) are never bumped. Otherwise, if the wanted port
// is occupied (e.g. another OS user on this machine), the next free port
// is picked and persisted as this user's port.
func ensureLocalPort(want string) string {
	if want == "" {
		want = config.EnvSyncPort()
	}
	if config.ExplicitPort() {
		return want
	}
	if checkPort(want) == nil {
		return want
	}
	base := 0
	fmt.Sscanf(want, "%d", &base)
	for p := base + 1; p <= base+100 && p <= 65535; p++ {
		candidate := fmt.Sprintf("%d", p)
		if checkPort(candidate) == nil {
			if err := config.SetLocalPort(candidate); err == nil {
				logging.Log("INFO", fmt.Sprintf("Port %s in use, using per-user port %s", want, candidate))
			}
			return candidate
		}
	}
	return want
}

func jsonError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	data, _ := json.Marshal(map[string]string{"error": message})
	_, _ = w.Write(data)
}

func notFoundHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte("Not Found"))
}

func checkPort(port string) error {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}
	_ = ln.Close()
	return nil
}
