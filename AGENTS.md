# AGENTS.md - Project Guide for LLM Coding Agents

## Project Overview

**env-sync** is a distributed secrets synchronization tool for local networks. It allows multiple machines to sync `.env` style secrets without a central server, using peer-to-peer architecture with mDNS discovery.

## Architecture

### Core Philosophy
- **Distributed**: No master server, all machines are equal
- **Zero Configuration**: New machines auto-discover without touching existing ones
- **Local Network Only**: Uses mDNS/Bonjour for discovery, HTTP for file transfer
- **Eventually Consistent**: Syncs on shell startup, cron (30min), or manual trigger

### Sync Strategy
1. **Discovery**: Find peers via mDNS (`_envsync._tcp` service on port 5739)
2. **Fetch**: Download secrets from discovered peers
3. **Merge**: Compare per-key updated_at timestamps and merge newest values per key
4. **Backup**: Always backup before overwriting (keep last 5)
5. **Update**: Replace local file and update metadata

## File Structure

```
env.sync.local/
├── PLAN.md                    # Detailed implementation plan & roadmap
├── README.md                  # User documentation & installation guide
├── AGENTS.md                  # This file - internal dev documentation
├── install.sh                 # Installation script (system or user)
├── bin/                       # Executable scripts
│   ├── env-sync              # Main CLI entry point
│   ├── env-sync-discover     # mDNS peer discovery tool
│   ├── env-sync-client       # HTTP client for fetching secrets
│   └── env-sync-serve        # HTTP server for serving secrets
└── lib/                       # Shared libraries
    └── common.sh             # Common functions & utilities
```

## File Descriptions

### bin/env-sync (Main CLI)
**Purpose**: Main entry point and command router
**Usage**: `env-sync [command] [options]`

**Commands**:
- `sync` (default): Run sync process
- `serve`: Start HTTP server
- `discover`: Find peers on network
- `status`: Show current status
- `init`: Initialize secrets file
- `restore [n]`: Restore from backup
- `cron`: Manage cron job

**Key Implementation Details**:
- Sources `../lib/common.sh` for shared functions
- Routes to appropriate sub-command functions
- Handles argument parsing for each command
- Exit codes: 0=success, 1=error

### bin/env-sync-discover
**Purpose**: Discover env-sync peers on local network
**Usage**: `env-sync-discover [options]`

**Discovery Methods**:
- **Linux**: Uses `avahi-browse` (avahi-utils package)
- **macOS**: Uses `dns-sd` (built-in)
- **Windows/Fallback**: Scans common hostnames

**Output**:
- One hostname per line (when `--quiet`)
- Formatted list with version info (default)

**Key Implementation Details**:
- Detects OS using `uname -s`
- Service type: `_envsync._tcp`
- Timeout configurable (default: 5 seconds)
- Removes self from results

### bin/env-sync-client
**Purpose**: Fetch and sync secrets from peers using SCP (SSH) by default
**Usage**: `env-sync-client [options] [hostname]`

**Modes**:
- **SCP** (default): Secure copy over SSH - requires SSH keys set up
- **HTTP** (fallback): Insecure HTTP with `--insecure-http` flag

**Options**:
- `--insecure-http`: Use HTTP instead of SCP (shows security warning)
- `-a, --all`: Sync from all discovered peers
- `-f, --force`: Force sync even if local is newer

**Conflict Resolution** (see `is_newer()` in common.sh):
1. Compare timestamps (newer wins)
2. If equal, compare versions (higher wins)
3. If both equal, hostname lexicographic order

**Key Implementation Details**:
- **Default**: Uses `scp` command over SSH (encrypted, authenticated)
- **Fallback**: Uses `curl` for HTTP (plaintext, insecure)
- Displays large security warning when using HTTP mode
- Tests SSH connectivity before attempting SCP
- Always creates backup before overwriting
- Validates fetched files before applying
- Validates checksums

### bin/env-sync-serve
**Purpose**: HTTP server for serving secrets file
**Usage**: `env-sync-serve [options]`

**Endpoints**:
- `GET /health`: JSON status response
- `GET /secrets.env`: Raw secrets file with metadata headers

**HTTP Headers**:
- `X-EnvSync-Version`: Semantic version
- `X-EnvSync-Timestamp`: ISO 8601 timestamp
- `X-EnvSync-Host`: Hostname
- `X-EnvSync-Checksum`: SHA256 checksum

**Key Implementation Details**:
- Uses `nc` (netcat) for HTTP server
- Port 5739 (ENV-SYNC in T9)
- Runs in foreground or daemon mode
- Creates PID file at `~/.config/env-sync/server.pid`

### lib/common.sh
**Purpose**: Shared functions and utilities
**Sourced by**: All other scripts

**Key Functions**:

**File Operations**:
- `init_secrets_file(file)`: Create new secrets file with metadata
- `validate_secrets_file(file)`: Validate file format and checksum
- `update_metadata(file, [version])`: Update timestamps and version
- `create_backup(file)`: Rotate and create backups
- `restore_backup(n)`: Restore from backup number

**Metadata Extraction**:
- `get_file_version(file)`: Extract version from header
- `get_file_timestamp(file)`: Extract timestamp
- `get_file_host(file)`: Extract hostname
- `get_file_checksum(file)`: Extract checksum

**Comparison Functions**:
- `compare_versions(v1, v2)`: Semantic version comparison
- `compare_timestamps(t1, t2)`: ISO 8601 timestamp comparison
- `is_newer(file1, file2)`: Determine if file1 is newer

**Utilities**:
- `log(level, message)`: Logging with colors and log file
- `get_hostname()`: Get system hostname
- `get_timestamp()`: Get current ISO 8601 timestamp
- `generate_checksum(file)`: SHA256 checksum

**Configuration Variables**:
```bash
ENV_SYNC_VERSION="1.0.0"    # Tool version
ENV_SYNC_PORT="5739"         # Server port
ENV_SYNC_SERVICE="_envsync._tcp"
SECRETS_FILE="$HOME/.secrets.env"
CONFIG_DIR="$HOME/.config/env-sync"
BACKUP_DIR="$CONFIG_DIR/backups"
LOG_DIR="$CONFIG_DIR/logs"
MAX_BACKUPS=5
```

### install.sh
**Purpose**: Installation script
**Usage**: `./install.sh [--user]`

**Install Locations**:
- System: `/usr/local/bin/`, `/usr/local/lib/env-sync/`
- User: `~/.local/bin/`, `~/.local/lib/env-sync/`

**Actions**:
1. Check dependencies (curl, nc, avahi-utils/dns-sd)
2. Create directories
3. Copy binaries and libraries
4. Make scripts executable
5. Verify installation

## Secrets File Format

**Location**: `~/.secrets.env`

**Structure**:
```bash
# === ENV_SYNC_METADATA ===
# VERSION: 1.2.3
# TIMESTAMP: 2025-02-07T15:30:45Z
# HOST: beelink.local
# MODIFIED: 2025-02-07T15:30:45Z
# CHECKSUM: sha256:abc123...
# === END_METADATA ===

# Your secrets here
OPENAI_API_KEY="sk-xxx" #ENV_SYNC_UPDATED_AT 2026-02-07T15:30:45Z
AWS_ACCESS_KEY_ID="AKIA..." #ENV_SYNC_UPDATED_AT 2026-02-07T15:32:10Z

# === ENV_SYNC_FOOTER ===
# VERSION: 1.2.3
# TIMESTAMP: 2025-02-07T15:30:45Z
# HOST: beelink.local
# === END_FOOTER ===
```

**Important Notes**:
- Metadata in header AND footer (for validation)
- Checksum calculated over entire file
- Version uses semantic versioning
- Timestamps in ISO 8601 UTC format
- Per-key timestamps stored inline as `#ENV_SYNC_UPDATED_AT <timestamp>` comments
- File permissions: 600 (owner only)

## Dependencies

### Required
- `bash` (v4.0+)
- `curl` (for HTTP requests)
- `nc` or `netcat` (for HTTP server)
- `sha256sum` (for checksums)
- `date` (with -d/-I support)

### Platform-Specific

**Linux**:
- `avahi-daemon` (running)
- `avahi-utils` (avahi-browse)
- Optional: `nss-mdns` (for .local resolution)

**macOS**:
- Built-in: `dns-sd` (no additional packages)

**Windows**:
- WSL2 with Linux dependencies
- Or: Bonjour SDK for Windows

## Data Flow

### Discovery Flow
```
env-sync-discover
├── Detect OS (Linux/macOS/Windows)
├── Platform-specific discovery
│   ├── Linux: avahi-browse _envsync._tcp
│   ├── macOS: dns-sd -B _envsync._tcp
│   └── Fallback: Scan common hostnames
├── Parse results
├── Filter self
└── Output sorted unique hostnames
```

### Sync Flow
```
env-sync (or cron/shell trigger)
└── env-sync-client
    ├── Discover peers (or use specific host)
    ├── For each peer:
    │   ├── Fetch /secrets.env via HTTP
    │   ├── Validate file format
    │   └── Merge keys based on per-key updated_at timestamps
    ├── Create backup
    ├── Replace local file
    └── Update metadata
```

### Server Flow
```
env-sync serve -d
└── env-sync-serve
    ├── Check port availability
    ├── Create PID file
    ├── Start netcat loop
    └── For each request:
        ├── Parse HTTP request
        ├── Route to handler
        │   ├── /health → JSON status
        │   └── /secrets.env → File with headers
        └── Return HTTP response
```

## Triggers & Automation

### 1. Shell Startup (Manual Setup)
Add to `~/.bashrc` or `~/.zshrc`:
```bash
if command -v env-sync &> /dev/null; then
    (env-sync --quiet &)
fi
```

### 2. Cron Job
```bash
env-sync cron --install    # Creates: */30 * * * * /usr/local/bin/env-sync --quiet
env-sync cron --remove     # Remove cron job
env-sync cron --show       # Show current job
```

### 3. Manual
```bash
env-sync                   # Foreground sync with output
env-sync --quiet          # Silent mode
```

## Adding New Machines

**Key Design**: Adding machine N+1 requires ZERO changes to existing N machines.

**Process**:
1. Install env-sync on new machine
2. Run `env-sync init`
3. Run `env-sync serve -d`
4. (Optional) Run `env-sync` once to get existing secrets

**Auto-Discovery**: Existing machines will discover the new one on their next sync cycle (within 30 minutes, or immediately if they run sync).

## Testing

### Manual Testing Commands
```bash
# Test discovery
./bin/env-sync-discover --verbose

# Test server (foreground)
./bin/env-sync-serve --port 9999

# Test client (dry run)
./bin/env-sync-client hostname.local

# Test full sync
./bin/env-sync sync -f

# Check status
./bin/env-sync status

# View logs
tail -f ~/.config/env-sync/logs/env-sync.log
```

### Validation Checklist
- [ ] Init creates valid secrets file
- [ ] Server starts and responds to /health
- [ ] Discovery finds other peers
- [ ] Sync downloads from peer
- [ ] Backup created before overwrite
- [ ] Version comparison works correctly
- [ ] Checksum validation works
- [ ] Cron job installs and runs
- [ ] All error cases handled gracefully

## Common Issues & Solutions

### Issue: No peers found
**Debug**:
```bash
# Check if avahi is running (Linux)
sudo systemctl status avahi-daemon

# Browse all mDNS services
avahi-browse -a  # Linux
dns-sd -B _services._dns-sd._udp  # macOS

# Test specific host
curl http://hostname.local:5739/health
```

### Issue: Permission denied
**Fix**:
```bash
chmod 600 ~/.secrets.env
chmod 700 ~/.config/env-sync
```

### Issue: Port already in use
**Fix**:
```bash
# Check what's using port
lsof -i :5739

# Use different port
env-sync serve -p 5740
export ENV_SYNC_PORT=5740
```

## Version History & Roadmap

### Current (v1.0.0)
- ✅ Core sync functionality
- ✅ SCP/SSH sync (secure by default)
- ✅ mDNS discovery (Linux/macOS)
- ✅ HTTP server/client (insecure fallback)
- ✅ Version comparison
- ✅ Backup system
- ✅ Cron automation
- ✅ Security warnings for HTTP mode

### Future Enhancements
- [ ] Native Windows support (no WSL)
- [ ] Encryption (age/sops)
- [ ] Web UI for management
- [ ] Selective sync (whitelist/blacklist)
- [ ] Conflict resolution UI
- [ ] Docker container support
- [ ] REST API for programmatic access

## Security Considerations

### SCP Mode (Default - Secure)
- ✅ Encrypted transmission via SSH
- ✅ Requires SSH key authentication
- ✅ File permissions: 600
- ✅ Accessible only to authorized machines

### HTTP Mode (Fallback - Insecure)
- ❌ Secrets transmitted in plaintext
- ❌ Accessible to any device on local network
- ❌ No authentication required
- ❌ Displays security warning when used

### Storage
- Secrets stored in plaintext (file permissions: 600)
- Metadata in headers/footers is also plaintext

### Future (Encryption)
- Encrypt values while keeping metadata plaintext
- Support for age/sops encryption
- Key management via age keys
- Optional authentication tokens

### Recommendations
- Always use SCP mode (default)
- Set up SSH keys between all machines
- Only use HTTP mode on completely trusted networks
- Use firewall to block port 5739 externally if using HTTP mode
- Regular backups

## Code Style Guidelines

### Bash Best Practices
- Always use `set -euo pipefail`
- Quote all variables: `"$variable"`
- Use `[[ ]]` for conditionals (not `[ ]`)
- Use `$()` for command substitution (not backticks)
- Functions should be lowercase with underscores
- Constants are UPPERCASE
- Indent with 4 spaces

### Error Handling
- Always check if commands exist: `command -v cmd >/dev/null 2>&1`
- Use `|| true` for commands that might fail
- Log errors with `log ERROR`
- Exit with non-zero on failure

### Logging
- Use `log` function for all output
- Levels: ERROR, WARN, INFO, SUCCESS
- Respect `ENV_SYNC_QUIET` flag
- Log to both console and file

## Contributing

When making changes:
1. Update relevant documentation (README, AGENTS.md)
2. Test on both Linux and macOS if possible
3. Maintain backward compatibility
4. Follow existing code style
5. Add error handling for edge cases
6. Update version number if needed

## Resources

- mDNS: https://www.ietf.org/rfc/rfc6762.txt
- DNS-SD: https://www.ietf.org/rfc/rfc6763.txt
- Avahi: https://www.avahi.org/
- Bonjour: https://developer.apple.com/bonjour/
- Semantic Versioning: https://semver.org/

## Questions?

For implementation questions:
1. Check PLAN.md for design decisions
2. Check README.md for usage
3. Review this file for technical details
4. Look at existing code for patterns
