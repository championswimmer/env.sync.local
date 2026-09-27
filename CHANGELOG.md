# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v4.0.0] - 2026-09-27

### Added

- **Multi-user support on the same device**: two OS users on one machine can now run env-sync side by side
  - Canonical per-user peer IDs (`user@host`) used as mDNS instance name, TLS certificate CN, and peer registry ID
  - Per-user service ports: the second user transparently bumps to a free port and persists it (`~/.config/env-sync/port`)
  - mDNS discovery returns structured peers with per-user ports (`DiscoverPeers`, `PortForPeer`/`PortMap`)
  - Transports (SSH/HTTP/mTLS) accept `host`, `host:port`, and `user@host[:port]` dial strings
  - `ENV_SYNC_USER` / `ENV_SYNC_HOSTNAME` overrides for tests and multi-machine simulation
  - `serve -p/--port` pins an explicit port that is never auto-bumped
- **New `internal/secrets/peerid.go`**: `LocalPeerID`, `NormalizePeerID`, `ParseEndpoint`/`Endpoint` (Dial/SSHDial/SplitDial/IsSelf), mDNS-safe sanitizers

### Changed

- **BREAKING**: peer identity is now `user@host` instead of bare hostname
  - TLS identities, key cache entries, invites, and membership events use the per-user peer ID
  - Legacy bare-hostname peers/identities keep working (matched by hostname fallback in auth and self-detection)
- Sync resolves per-user endpoints with a single shared discovery pass and skips self by peer ID
- `status` and key commands display peer IDs (`user@host`) instead of bare hostnames

## [v3.0.0] - 2025-02-16

### Added

- **Three Security Modes**: Completely restructured security model with explicit operation modes:
  - `dev-plaintext-http`: Debug mode with plaintext storage and HTTP transport
  - `trusted-owner-ssh`: Same-owner devices using SSH/SCP transport (plaintext by default)
  - `secure-peer`: Cross-owner collaboration with HTTPS+mTLS and AGE encryption
- **Secure Peer Server**: New daemon mode with HTTPS+mTLS for cross-owner collaboration
  - Transport identity keypairs for mutual authentication
  - Peer registry with authorization states (pending/approved/revoked)
  - Invitation-based onboarding with enrollment tokens
  - Membership event propagation for offline peer synchronization
- **Mode Management Commands**:
  - `env-sync mode get/set <mode>` - View and change security modes
  - `env-sync peer invite` - Create enrollment invitations
  - `env-sync peer request-access` - Request access to a network
  - `env-sync peer approve/revoke/list` - Manage peer authorizations
- **Non-destructive Mode Switching**: Mode changes preserve existing keys and data by default
- **Peer Trust Store**: Persistent storage of peer identities and certificates

### Changed

- **Default Mode**: Fresh installs default to `trusted-owner-ssh` (backward compatible behavior)
- **Encryption Default**: In `trusted-owner-ssh` mode, storage is now plaintext by default
  - Previous default was encrypted, which provided false sense of security when peers have SSH access
  - Encryption still available with `--encrypted` flag or by switching to `secure-peer` mode
- **Security Model**: Made threat models explicit - no more ambiguous "secure by default" claims
- **API Versioning**: Added `/v2/` endpoints for secure peer operations

### Security

- **Explicit Trust Models**: Each mode has clear, documented security guarantees
- **mTLS for Cross-Owner**: New secure-peer mode uses mutual TLS without requiring SSH trust
- **Certificate Pinning**: Peers pin each other's transport identities (no global root CA)
- **Membership Events**: Signed, append-only log for trust propagation with replay protection
- **Downgrade Protection**: Mode switches require explicit confirmation, especially when leaving secure-peer mode

## [v2.0.0] - 2025-02-08

### Added

- **Complete Go Rewrite**: Migrated from bash scripts to single Go binary
- **Built-in AGE Encryption**: Integrated filippo.io/age library, no external dependency
- **Single Static Binary**: Everything compiled into one executable
- **Improved Performance**: Faster sync operations and better resource usage
- **Service Management**: Native OS service integration (systemd/launchd)
- **Cron Integration**: Built-in cron job management with `env-sync cron` commands

### Changed

- **Architecture**: Moved from shell scripts to Go modules
- **Encryption**: No longer requires external `age` binary
- **Installation**: Simplified to single binary deployment
- **Cross-Platform**: More consistent behavior across Linux, macOS, Windows (WSL2)

### Removed

- **Bash Script Implementation**: All shell scripts replaced with Go code
- **External age Dependency**: Encryption now built-in

## [v1.0.0] - 2025-01-15

### Added

- **AGE Encryption**: Secrets encrypted at rest using AGE encryption
- **Multi-Recipient Encryption**: Each machine has its own key, encrypted to all authorized recipients
- **Transparent Decryption**: Automatic decryption during sync, shell loading, and cron jobs
- **Remote Trigger**: New machines can trigger re-encryption remotely via SSH
- **Zero-Config Addition**: Add new machines without modifying existing ones
- **CLI Secret Management**: Commands to add, remove, list, and show secrets
- **Backup System**: Automatic backups before overwriting (keeps last 5)
- **Force Pull**: Option to forcefully sync from a specific host
- **Version Control**: Built-in versioning and conflict resolution

### Security

- **At-Rest Encryption**: AGE encryption for secrets on disk
- **Multi-Recipient**: Support for encrypting to multiple recipients
- **Key Management**: Automatic key generation and peer key caching
- **SSH Transport**: SCP over SSH for encrypted file transfer

## [v0.1.0] - 2024-12-01

### Added

- **Initial Release**: Basic secrets synchronization for local networks
- **mDNS Discovery**: Automatic peer discovery using mDNS/Bonjour
- **SCP Transport**: File transfer over SSH/SCP
- **HTTP Fallback**: Plaintext HTTP server mode for testing
- **Sync on Startup**: Shell integration for automatic sync
- **Basic CLI**: Simple command-line interface for sync operations
