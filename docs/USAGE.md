# Usage Guide

Complete guide for using env-sync in all operation modes.

## Table of Contents

- [Quick Commands](#quick-commands)
- [Mode Management](#mode-management)
- [Peer Management](#peer-management)
- [Secret Management](#secret-management)
- [Sync Operations](#sync-operations)
- [Service Management](#service-management)
- [Shell Integration](#shell-integration)
- [Troubleshooting](#troubleshooting)

## Quick Commands

```bash
# Sync with peers (works in all modes)
env-sync

# Check current status
env-sync status

# Discover peers on network
env-sync discover

# Show help
env-sync --help
```

## Mode Management

env-sync operates in three distinct security modes. See [SECURITY-MODES.md](./SECURITY-MODES.md) for detailed security information.

### View Current Mode

```bash
env-sync mode get
```

### Switch Modes

```bash
# Switch to trusted-owner-ssh mode (default)
env-sync mode set trusted-owner-ssh

# Switch to secure-peer mode (for cross-owner collaboration)
env-sync mode set secure-peer

# Switch to dev mode (insecure, for debugging only)
env-sync mode set dev-plaintext-http

# Force mode switch without confirmation (non-interactive)
env-sync mode set secure-peer --yes

# Switch and clean up old mode's data
env-sync mode set trusted-owner-ssh --prune-old-material --yes
```

**Important**: Mode switches are non-destructive by default. Your keys, secrets, and peer data are preserved. Use `--prune-old-material` to clean up.

## Peer Management

### Secure-Peer Mode

In `secure-peer` mode, peers must be explicitly authorized before they can sync secrets.

#### Invite a New Peer

On an existing trusted peer:

```bash
# Create an enrollment invitation
env-sync peer invite

# Create invitation that expires in 1 hour
env-sync peer invite --expires 1h

# Create invitation with description
env-sync peer invite --expires 30m --description "John's laptop"
```

The command outputs:
- Enrollment token
- Hostname to connect to
- Transport fingerprint (for verification)

#### Join as a New Peer

On the new machine:

```bash
# 1. Set mode to secure-peer
env-sync mode set secure-peer

# 2. Request access using the token from invitation (peer IDs are user@host)
env-sync peer request alice@hostname.local <token>

# 3. Wait for approval, then sync
env-sync
```

#### Approve Pending Peers

```bash
# List pending requests
env-sync peer list --pending

# Approve a peer (peer IDs are user@host)
env-sync peer approve bob@new-host.local

# Revoke a peer's access
env-sync peer revoke mallory@compromised-host.local

# List all peers
env-sync peer list
```

#### View Peer Trust Information

```bash
# Show trust details for a specific peer (peer ID: user@host)
env-sync peer trust show alice@hostname.local

# List all trusted fingerprints
env-sync peer trust list
```

### Trusted-Owner-SSH Mode

In `trusted-owner-ssh` mode, any peer with SSH access can sync. No explicit approval needed.

```bash
# Discover peers (also filters by SSH reachability)
env-sync discover

# Collect public keys from discovered peers
env-sync discover --collect-keys
```

### Multiple Users on One Machine (v4.0+)

Peer IDs are per-user `user@host` (username + hostname, overridable via
`ENV_SYNC_USER` / `ENV_SYNC_HOSTNAME`). Each OS user gets their own port:
default `5739`, else `ENV_SYNC_PORT`, else `~/.config/env-sync/port`
(auto-bumped when taken). `serve -p/--port` or `ENV_SYNC_PORT` pins an
explicit port that is never bumped. Endpoints accept `host`, `host:port`, or
`user@host[:port]`; `user@host` auto-resolves the right port when reachable.
Legacy bare-hostname peers keep working via fallback.

## Secret Management

Manage your secrets with these commands. All commands work in both encrypted and plaintext modes.

### Add Secrets

```bash
# Add a new secret
env-sync add OPENAI_API_KEY="sk-abc123xyz"

# Add secret with spaces (use quotes)
env-sync add DATABASE_URL="postgres://user:pass@localhost/db"

# Add multiple secrets
env-sync add API_KEY="value1"
env-sync add AWS_ACCESS_KEY="AKIA..."
```

**Features:**
- Updates existing key if it already exists
- Automatically creates backup before modification
- Updates metadata (timestamp, checksum)
- Properly handles quotes in values

### Remove Secrets

```bash
# Remove a secret
env-sync remove OPENAI_API_KEY

# Safe to run - warns if key doesn't exist
env-sync remove NONEXISTENT_KEY
```

### List Secrets

```bash
# List all secret keys (values hidden)
env-sync list

# Output example:
# Secrets keys:
#   • OPENAI_API_KEY
#   • DATABASE_URL
#   • AWS_ACCESS_KEY
#
# Total: 3 keys
```

### View Secret Values

```bash
# Show value of a specific key
env-sync show OPENAI_API_KEY

# Output: sk-abc123xyz
```

### Load Secrets for Shell

```bash
# Output secrets as export statements
env-sync load

# Typical usage in shell profile
eval "$(env-sync load 2>/dev/null)"
```

## Sync Operations

### Automatic Sync

```bash
# Sync with all discovered peers
env-sync

# Same as:
env-sync sync
```

### Sync from Specific Host

```bash
# Sync from a specific peer only (peer IDs are user@host; port optional)
env-sync sync alice@hostname.local
```

### Force Pull

Forcefully overwrite local secrets with those from a specific host:

```bash
# Force pull from specific peer
env-sync sync --force-pull alice@hostname.local
```

**Use when:**
- You want to reset local secrets to match a trusted source
- You've made incorrect local changes and want to revert
- You want exact consistency with a specific machine

**Warning:** Local changes will be lost. A backup is created automatically.

### Dry Run

```bash
# Preview what would happen without making changes
env-sync sync --dry-run
```

## Service Management

The background service handles mDNS advertising and periodic sync.

### Start Service

```bash
# Start as background service
env-sync serve -d
```

### Stop Service

```bash
env-sync service stop
```

### Restart Service

```bash
env-sync service restart
```

### Check Service Status

```bash
# Native OS commands
systemctl --user status env-sync      # Linux
launchctl print gui/$(id -u)/env-sync # macOS
```

### Uninstall Service

```bash
env-sync service uninstall
```

## Shell Integration

Add to your shell profile (`~/.bashrc`, `~/.zshrc`, etc.):

```bash
# Auto-load secrets on shell startup (decrypts automatically if encrypted)
eval "$(env-sync load 2>/dev/null)"

# Auto-sync in background on shell startup
if command -v env-sync &> /dev/null; then
    (env-sync --quiet &)
fi
```

## Cron Jobs

### Install Cron Job

```bash
# Install with default 30-minute interval
env-sync cron --install

# Install with custom interval (minutes)
env-sync cron --install --interval 10
env-sync cron --install --interval 60
```

### View Cron Job

```bash
env-sync cron --show
```

### Remove Cron Job

```bash
env-sync cron --remove
```

## Backups

Backups are created automatically before any modification.

### View Backups

```bash
ls ~/.config/env-sync/backups/
```

### Restore from Backup

```bash
# Restore from most recent backup
env-sync restore

# Restore from specific backup (1-5, where 1 is most recent)
env-sync restore 1
env-sync restore 3
```

## Troubleshooting

### Cannot Decrypt (Not in Recipient List)

```bash
# In trusted-owner-ssh mode
env-sync key request-access --trigger alice@hostname.local

# In secure-peer mode
# Ensure you've been approved by a trusted peer
env-sync peer list
```

### SSH Connection Fails (Trusted-Owner Mode)

```bash
# Test SSH connectivity (user = remote OS login)
ssh -v user@hostname.local

# Copy SSH key again
ssh-copy-id user@hostname.local
```

### Sync Not Working

```bash
# Check status
env-sync status

# View logs
tail -f ~/.config/env-sync/logs/env-sync.log

# Test discovery
env-sync discover
```

### Certificate/Trust Issues (Secure-Peer Mode)

```bash
# View peer trust status (peer ID: user@host)
env-sync peer trust show alice@hostname.local

# Check for pending approvals
env-sync peer list --pending

# Verify your transport identity
env-sync key show --transport
```

### Mode Switch Issues

```bash
# View current mode
env-sync mode get

# Check if old mode material exists
ls ~/.config/env-sync/

# Force mode switch with cleanup
env-sync mode set trusted-owner-ssh --prune-old-material --yes
```

## File Locations

```
~/.config/env-sync/
├── config                      # Config file
├── .secrets.env                # Secrets (encrypted or plaintext)
├── .secrets.env.backup.1..5    # Backups
├── keys/
│   ├── age_key                 # AGE private key (secure-peer mode)
│   ├── age_key.pub             # AGE public key
│   ├── transport_key           # mTLS transport private key
│   ├── transport_cert.pem      # mTLS transport certificate
│   ├── known_hosts/            # Cached peer pubkeys
│   └── cache/
│       └── pubkey_cache.json
├── peers/
│   ├── registry.json           # Peer registry (secure-peer mode)
│   └── trust_store/            # Pinned peer certificates
├── events/
│   └── membership.log          # Signed membership events
└── logs/
    └── env-sync.log            # Application logs
```
