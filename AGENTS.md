# AGENTS.md - Project Guide for LLM Coding Agents

## Project Overview

**env-sync** is a distributed secrets synchronization tool for local networks. It allows multiple machines to sync `.env` style secrets without a central server, using peer-to-peer architecture with mDNS discovery.

**Current Version**: v3.0 - Security Mode Overhaul with Three Operation Modes

## Architecture

### Core Philosophy
- **Distributed**: No master server, all machines are equal
- **Zero Configuration**: New machines auto-discover without touching existing ones (in trusted-owner mode)
- **Local Network Only**: Uses mDNS/Bonjour for discovery
- **Eventually Consistent**: Syncs on shell startup, cron (30min), or manual trigger
- **Explicit Security Models**: Three distinct modes for different trust scenarios

### Three Security Modes (v3.0+)

#### Mode A: `dev-plaintext-http` (Debug Only)
- **Storage**: Plaintext
- **Transport**: Plaintext HTTP
- **Use Case**: Local debugging only
- **Security**: None - displays prominent warnings

#### Mode B: `trusted-owner-ssh` (Same Owner - Default)
- **Storage**: Plaintext (optional AGE encryption)
- **Transport**: SCP/SSH
- **Use Case**: All devices belong to one user, mutually trusted
- **Onboarding**: Zero-touch - new machines need only SSH access to one peer
- **Security**: SSH provides encrypted transport and authentication

#### Mode C: `secure-peer` (Cross-Owner Collaboration)
- **Storage**: AGE encrypted (mandatory)
- **Transport**: HTTPS with mTLS (mutual TLS)
- **Use Case**: Different owners sharing secrets without shell access
- **Onboarding**: Invitation-based with explicit approval
- **Security**: mTLS for authentication, AGE for at-rest encryption, explicit authorization

### Mode-Specific Sync Strategy

**Trusted-Owner Mode:**
1. Discovery: Find peers via mDNS, filter by SSH reachability
2. Fetch: Download secrets via SCP/SSH
3. Compare: Check versions and timestamps
4. Merge: Combine changes (if needed)
5. Save: Store file (plaintext or encrypted based on config)
6. Backup: Always backup before overwriting (keep last 5)

**Secure-Peer Mode:**
1. Discovery: Find peers via mDNS
2. Authentication: Establish mTLS connection
3. Authorization: Check peer approval status
4. Fetch: Download encrypted secrets via HTTPS
5. Decrypt: Decrypt using local AGE private key
6. Compare: Check versions and timestamps
7. Merge: Combine changes (per-key timestamps)
8. Re-encrypt: Encrypt to all known recipient keys
9. Backup: Create backup before modification
10. Update: Replace local file and update metadata

## File Structure

```
env.sync.local/
├── CHANGELOG.md               # Version history
├── README.md                  # User documentation
├── AGENTS.md                  # This file - internal dev documentation
├── docs/                      # Detailed documentation
│   ├── USAGE.md               # Complete usage guide
│   ├── SECURITY-MODES.md      # Security mode details
│   └── INSTALLATION.md        # Installation instructions
├── install.sh                 # Installation script
├── Makefile                   # Build automation
├── src/                       # Go source code (v3.0)
│   ├── cmd/env-sync/          # Main entry point
│   │   └── main.go
│   ├── internal/              # Internal packages
│   │   ├── cli/               # CLI interface and command routing
│   │   ├── mode/              # Mode management (NEW in v3.0)
│   │   ├── sync/              # Sync logic (mode-aware)
│   │   ├── discovery/         # mDNS peer discovery
│   │   ├── crypto/age/        # AGE encryption/decryption
│   │   ├── transport/         # Transport abstraction
│   │   │   ├── ssh/           # SCP transport (trusted-owner mode)
│   │   │   └── https/         # HTTPS+mTLS transport (secure-peer mode)
│   │   ├── server/            # HTTP/HTTPS server
│   │   │   └── secure/        # Secure peer server (mTLS)
│   │   ├── metadata/          # File metadata handling
│   │   ├── secrets/           # Secrets file management
│   │   ├── backup/            # Backup management
│   │   ├── keys/              # Key management (AGE + transport)
│   │   ├── peers/             # Peer registry (NEW in v3.0)
│   │   │   ├── registry.go    # Peer authorization management
│   │   │   └── trust/         # Certificate pinning
│   │   ├── membership/        # Membership events (NEW in v3.0)
│   │   ├── config/            # Configuration
│   │   ├── logging/           # Logging utilities
│   │   └── cron/              # Cron job management
│   ├── go.mod                 # Go module definition
│   └── go.sum                 # Go module checksums
├── target/                    # Build output
│   └── env-sync               # Compiled Go binary
└── tests/                     # Integration tests (BATS)
    ├── bats/                  # BATS test files
    ├── docker/                # Docker test environment
    └── utils/                 # Test utilities
```

## Go Implementation (v3.0)

### Main Components

#### cmd/env-sync/main.go
**Purpose**: Entry point for the Go binary
**Key Features**:
- Parses command-line arguments
- Routes to appropriate CLI handlers
- Initializes configuration and logging
- Sets up mode-specific behavior

#### internal/mode/mode.go (NEW)
**Purpose**: Mode management and switching
**Functions**:
- `GetMode()`: Return current operation mode
- `SetMode(mode, options)`: Switch modes with safety checks
- `ValidateMode()`: Ensure mode configuration is consistent
- `SupportsEncryption()`: Check if mode supports/uses encryption
- `RequiresDaemon()`: Check if mode requires background server

**Important**: Mode switching is non-destructive by default. Use `--prune-old-material` for cleanup.

#### internal/peers/registry.go (NEW)
**Purpose**: Peer registry for secure-peer mode
**Functions**:
- `AddPeer(hostname, identity)`: Add peer to registry
- `ApprovePeer(hostname)`: Approve pending peer
- `RevokePeer(hostname)`: Revoke peer access
- `GetPeer(hostname)`: Retrieve peer info
- `ListPeers(filter)`: List peers by status
- `IsAuthorized(hostname)`: Check if peer is approved

**Authorization States**:
- `pending`: Requested access, awaiting approval
- `approved`: Authorized to sync
- `revoked`: Access revoked

#### internal/peers/trust/store.go (NEW)
**Purpose**: Certificate pinning and trust management
**Functions**:
- `PinCertificate(peer, cert)`: Pin peer's certificate
- `GetPinnedCert(peer)`: Retrieve pinned certificate
- `VerifyIdentity(peer, cert)`: Verify peer identity
- `ListPinnedPeers()`: List all pinned peers

**Policy**: No global root CA. Trust is deployment-local via pinned certificates.

#### internal/membership/events.go (NEW)
**Purpose**: Signed membership events for trust propagation
**Functions**:
- `CreateEvent(action, peer, sponsor)`: Create signed event
- `VerifyEvent(event)`: Verify event signature
- `ApplyEvent(event)`: Apply event to local registry
- `GetEventsSince(cursor)`: Get events for catch-up
- `ReplayEvents(events)`: Apply missed events

**Event Types**:
- `peer_joined`: New peer approved
- `peer_revoked`: Peer access revoked
- `membership_sync`: Periodic sync marker

**Security**:
- Signed by sponsor peer
- Monotonic event IDs
- Timestamp + expiry
- Replay protection via cursor

#### internal/transport/ssh/ssh.go
**Purpose**: SCP/SSH transport (trusted-owner mode)
**Functions**:
- `FetchFile(host, remotePath)`: Download file via SCP
- `TestConnection(host)`: Verify SSH connectivity
- `PushFile(host, localPath, remotePath)`: Upload file via SCP

#### internal/transport/https/client.go (NEW)
**Purpose**: HTTPS client with mTLS (secure-peer mode)
**Functions**:
- `FetchFile(host)`: Download via HTTPS with mTLS
- `TestConnection(host)`: Verify TLS and authentication
- `RequestAccess(host, token)`: Send access request
- `GetEvents(host)`: Fetch membership events

#### internal/transport/https/server.go (NEW)
**Purpose**: HTTPS server with mTLS (secure-peer mode)
**Functions**:
- `Start()`: Start HTTPS server
- `HandleSecrets(w, r)`: Serve encrypted secrets
- `HandleMembership(w, r)`: Serve membership events
- `HandlePairing(w, r)`: Handle pairing requests

**Endpoints**:
- `GET /v2/health`: Health check
- `GET /v2/secrets`: Fetch encrypted secrets (mTLS required)
- `GET /v2/membership/events`: Membership events (signed)
- `POST /v2/peer/request-access`: Pairing request
- `POST /v2/peer/approve`: Approval webhook

#### internal/crypto/age/age.go
**Purpose**: AGE encryption/decryption
**Functions**:
- `GenerateKey()`: Create new AGE key pair
- `Encrypt(value, recipients)`: Encrypt value to multiple recipients
- `Decrypt(encrypted, privateKey)`: Decrypt value
- `LoadPrivateKey()`: Load private key from disk
- `LoadPublicKey()`: Load public key

#### internal/keys/keys.go
**Purpose**: Key management (AGE + transport identity)
**Functions**:
- `GenerateAGEKeyPair()`: Create AGE key pair
- `GenerateTransportIdentity()`: Create mTLS identity
- `LoadPrivateKey(keyType)`: Load specified key type
- `LoadPublicKey(keyType)`: Load specified public key
- `ImportPeerKey(pubkey, hostname)`: Save peer's public key
- `GetRecipients()`: Get list of all recipients for encryption

#### internal/sync/sync.go
**Purpose**: Core sync logic (mode-aware)
**Functions**:
- `Sync()`: Main sync orchestration (routes by mode)
- `syncTrustedOwner()`: Sync in trusted-owner mode
- `syncSecurePeer()`: Sync in secure-peer mode
- `FetchFromPeers()`: Download from discovered peers
- `CompareVersions()`: Determine newest file
- `MergeSecrets()`: Combine changes from multiple sources

#### internal/cli/cli.go
**Purpose**: Command-line interface
**New Commands (v3.0)**:
- `mode get|set <mode>`: Mode management
- `peer invite`: Create enrollment invitation
- `peer request-access`: Request access to network
- `peer approve <peer>`: Approve pending peer
- `peer revoke <peer>`: Revoke peer access
- `peer list`: List peers
- `peer trust show <peer>`: Show trust details

### Configuration Constants (config.go)

```go
ENV_SYNC_VERSION = "4.0.0"
ENV_SYNC_PORT = "5739"
CONFIG_DIR = "~/.config/env-sync"
SECRETS_FILE = CONFIG_DIR + "/.secrets.env"
BACKUP_DIR = CONFIG_DIR + "/backups"
KEYS_DIR = CONFIG_DIR + "/keys"
PEERS_DIR = CONFIG_DIR + "/peers"
EVENTS_DIR = CONFIG_DIR + "/events"
LOG_DIR = CONFIG_DIR + "/logs"
MAX_BACKUPS = 5

// Mode settings
SYNC_MODE = "trusted-owner-ssh" | "secure-peer" | "dev-plaintext-http"
STORAGE_ENCRYPTION = "plaintext" | "age"
TRANSPORT = "ssh" | "https-mtls" | "http"
```

## Secrets File Format (v3.0)

### Plaintext Mode (trusted-owner)

```bash
# === ENV_SYNC_METADATA ===
# VERSION: 3.0.0
# TIMESTAMP: 2025-02-16T15:30:45Z
# HOST: beelink.local
# MODIFIED: 2025-02-16T15:30:45Z
# ENCRYPTED: false
# MODE: trusted-owner-ssh
# === END_METADATA ===

OPENAI_API_KEY="sk-abc123xyz" # ENVSYNC_UPDATED_AT=2025-02-16T15:30:45Z
DATABASE_URL="postgres://user:pass@localhost/db" # ENVSYNC_UPDATED_AT=2025-02-16T14:20:10Z

# === ENV_SYNC_FOOTER ===
# VERSION: 3.0.0
# TIMESTAMP: 2025-02-16T15:30:45Z
# HOST: beelink.local
# === END_FOOTER ===
```

### Encrypted Mode (secure-peer)

```bash
# === ENV_SYNC_METADATA ===
# VERSION: 3.0.0
# TIMESTAMP: 2025-02-16T15:30:45Z
# HOST: beelink.local
# MODIFIED: 2025-02-16T15:30:45Z
# ENCRYPTED: true
# MODE: secure-peer
# RECIPIENTS: beelink.local:age1xyz...,mbp16.local:age1abc...,nuc.local:age1def...
# === END_METADATA ===

OPENAI_API_KEY="YWdlLWVuY3J5cHRpb24ub3JnL3YxCi0+..." # ENVSYNC_UPDATED_AT=2025-02-16T15:30:45Z
DATABASE_URL="YWdlLWVuY3J5cHRpb24ub3JnL3YxCi0+..." # ENVSYNC_UPDATED_AT=2025-02-16T14:20:10Z

# === ENV_SYNC_FOOTER ===
# VERSION: 3.0.0
# TIMESTAMP: 2025-02-16T15:30:45Z
# HOST: beelink.local
# === END_FOOTER ===
```

**Important Notes**:
- Mode stored in metadata
- Recipients tracked per-mode
- Values individually encrypted (keys/metadata plaintext)
- Timestamps for granular merging
- File permissions: 600

## Data Flow

### Mode Switch Flow

```
env-sync mode set <new-mode>
└── mode.SetMode()
    ├── Check current mode
    ├── Validate prerequisites for new mode
    ├── Prompt for confirmation (if security downgrade)
    ├── Update config file
    ├── If --prune-old-material:
    │   ├── Backup old mode data
    │   └── Remove old mode artifacts
    └── Initialize new mode (generate keys if needed)
```

### Trusted-Owner Sync Flow

```
env-sync sync
└── sync.syncTrustedOwner()
    ├── discovery.DiscoverPeers()
    ├── Filter peers by SSH reachability
    ├── For each peer:
    │   ├── transport/ssh.FetchFile()
    │   ├── metadata.Compare()
    │   └── secrets.Read()
    ├── Find newest version
    ├── If remote is newer:
    │   ├── backup.Create()
    │   ├── Merge secrets
    │   ├── secrets.Write()
    │   └── metadata.Update()
    └── Return status
```

### Secure-Peer Sync Flow

```
env-sync sync
└── sync.syncSecurePeer()
    ├── discovery.DiscoverPeers()
    ├── For each peer:
    │   ├── peers.IsAuthorized(peer)
    │   ├── transport/https.FetchFile()
    │   │   └── mTLS handshake
    │   ├── metadata.Compare()
    │   └── secrets.Read() and decrypt
    ├── Find newest version
    ├── Fetch missed membership events
    ├── Apply new events to registry
    ├── If remote is newer:
    │   ├── backup.Create()
    │   ├── Merge secrets (per-key timestamps)
    │   ├── crypto/age.Encrypt() to all recipients
    │   ├── secrets.Write()
    │   └── metadata.Update()
    └── Return status
```

### Secure Peer Invitation Flow

```
# On existing peer (Host A)
env-sync peer invite
└── peers.CreateInvitation()
    ├── Generate enrollment token
    ├── Set expiry
    ├── Return: token, hostname, fingerprint

# On new machine (Host B)
env-sync peer request-access --to HostA --token <token>
└── transport/https.RequestAccess()
    ├── Load transport identity
    ├── Connect to HostA (mTLS)
    ├── Send token + identity + AGE pubkey
    └── Host A marks as pending

# On Host A (approval)
env-sync peer approve HostB
└── peers.ApprovePeer()
    ├── Update peer status to approved
    ├── Pin HostB certificate
    ├── Create membership event
    ├── Sign event
    └── Replicate to other peers

# Host B syncs
env-sync sync
└── Now authorized, can decrypt secrets
```

### Membership Event Propagation Flow

```
HostA approves HostC
└── membership.CreateEvent(peer_joined, HostC, HostA)
    ├── Generate event ID (monotonic)
    ├── Sign with HostA transport key
    └── Replicate to online peers

HostB (online) receives event
└── membership.VerifyEvent()
    ├── Verify signature (HostA signed)
    ├── Check event ID > last applied
    ├── Check timestamp not expired
    └── Apply to registry

HostD (offline during approval) comes online
└── sync.syncSecurePeer()
    ├── Fetch events from any peer
    ├── membership.GetEventsSince(lastCursor)
    ├── Verify and apply missed events
    └── Now knows about HostC
```

## Dependencies

### Required

- Go 1.24 or later (for building)
- `avahi-browse` (Linux - for mDNS discovery)
- `dns-sd` (macOS - built-in, for mDNS discovery)

### Go Modules

- `filippo.io/age` - AGE encryption library
- `golang.org/x/crypto` - Cryptographic primitives
- `github.com/kardianos/service` - Cross-platform service management
- Standard library: `crypto/tls`, `net/http` for mTLS

## Building and Testing

### Build Commands

```bash
# Build Go binary
make build

# Run Go unit tests
make test

# Install to /usr/local/bin
sudo make install

# Install using install.sh
./install.sh --user
```

### Testing

```bash
# Run all integration tests
./tests/test-dockers.sh

# Run Go unit tests
cd src && go test ./...

# Run tests for specific mode
cd src && go test ./internal/mode/...
cd src && go test ./internal/peers/...
cd src && go test ./internal/membership/...
```

### Test Environment
- Uses Docker containers to simulate multiple machines
- BATS (Bash Automated Testing System) for integration tests
- Three-mode test matrix (dev, trusted-owner, secure-peer)

## Development Guidelines

### Adding a New Mode

1. Add mode constant in `internal/config/config.go`
2. Implement mode handler in `internal/mode/handlers/<mode>.go`
3. Add transport support in `internal/transport/<mode>/`
4. Update sync logic in `internal/sync/sync.go`
5. Add CLI commands in `internal/cli/cli.go`
6. Write tests in `tests/bats/<mode>/`
7. Update documentation

### Adding Peer Management Features

1. Update peer struct in `internal/peers/types.go`
2. Add registry methods in `internal/peers/registry.go`
3. Update transport handlers in `internal/transport/https/server.go`
4. Add CLI commands in `internal/cli/peer_commands.go`
5. Add tests

### Code Style Guidelines

### Go Best Practices
- Follow standard Go conventions (gofmt, go vet)
- Use standard library where possible
- Error handling: explicit error returns
- Package organization: internal/ for private code
- Constants in SCREAMING_SNAKE_CASE
- Functions and variables in camelCase
- Exported names start with capital letter

### Error Handling
- Always check and handle errors explicitly
- Use custom error types for domain-specific errors
- Log errors with context
- Exit with non-zero on failure
- Wrap errors with context using `fmt.Errorf("...: %w", err)`

### Security Best Practices
- Never log secrets or private keys
- Validate all inputs
- Use constant-time comparison for sensitive data
- Pin certificates in secure-peer mode (no TOFU in production)
- Validate membership event signatures
- Check event freshness and ordering

### Logging
- Use internal/logging package
- Levels: ERROR, WARN, INFO, DEBUG
- Respect quiet mode flags
- Log to both console and file
- Never log sensitive data

## Contributing

When making changes to v3.0:
1. Update relevant documentation (README, AGENTS.md, docs/)
2. Test on both Linux and macOS if possible
3. Test all three modes
4. Follow Go code style guidelines
5. Add unit tests for new functionality
6. Add integration tests for mode-specific features
7. Run full test matrix before submitting
8. Update version number if needed
9. Update CHANGELOG.md

## Resources

- Go: https://go.dev/
- AGE: https://age-encryption.org/
- filippo.io/age: https://pkg.go.dev/filippo.io/age
- TLS 1.3: https://www.rfc-editor.org/rfc/rfc8446
- X.509 PKI: https://www.rfc-editor.org/rfc/rfc5280
- mDNS: https://www.ietf.org/rfc/rfc6762.txt
- DNS-SD: https://www.ietf.org/rfc/rfc6763.txt
- Semantic Versioning: https://semver.org/

## Questions?

For implementation questions:
1. Check `docs/SECURITY-MODES.md` for security mode details
2. Check `docs/USAGE.md` for command reference
3. Review this file (AGENTS.md) for technical architecture
4. Look at Go source code in `src/` for current implementation
5. Check test files in `tests/` for usage examples
