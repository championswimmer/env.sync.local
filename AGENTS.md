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
2. **Comparison**: Compare version/timestamp to find newest file
3. **Fetch**: Download from peer with newest version
4. **Backup**: Always backup before overwriting (keep last 5)
5. **Update**: Replace local file and update metadata

**Runtime Note**: The Go implementation (`src/cmd/env-sync`) is the default runtime. Legacy Bash lives under `legacy/` and can be forced with `ENV_SYNC_USE_BASH=true` when needed for compatibility or testing.

## File Structure

```
env.sync.local/
├── PLAN.md                    # Detailed implementation plan & roadmap
├── README.md                  # User documentation & installation guide
├── AGENTS.md                  # This file - internal dev documentation
├── install.sh                 # Installation script (builds Go binary)
├── bin/                       # Go shims/symlinks (default entrypoints)
├── legacy/                    # Legacy Bash implementation
│   ├── bin/                   # Legacy CLI scripts
│   └── lib/                   # Legacy shared functions
└── src/                       # Go source code
    └── cmd/env-sync           # Go CLI entrypoint
```

## File Descriptions

### Go CLI (src/cmd/env-sync)
**Purpose**: Primary Go implementation and command router (default runtime)
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
- Command names are derived from argv[0], so shims/symlinks (`env-sync-client`, `env-sync-discover`, etc.) map to one binary
- Lives in `src/cmd/env-sync` with supporting packages under `src/internal`
- Exit codes: 0=success, 1=error

### bin/env-sync (Shim)
**Purpose**: Wrapper that prefers the Go binary and optionally routes to legacy Bash
**Behavior**:
- Locates Go binary from `target/env-sync` in dev or `/usr/local/lib/env-sync/env-sync-go` when installed
- If `ENV_SYNC_USE_BASH=true`, dispatches to `legacy/bin/<command>` using the same argv[0]
- Symlinked entrypoints (`env-sync-client`, `env-sync-discover`, `env-sync-serve`, `env-sync-key`, `env-sync-load`) all point here

### legacy/bin/env-sync (Legacy Bash CLI)
**Purpose**: Legacy Bash entrypoint and command router
**Usage**: `ENV_SYNC_USE_BASH=true env-sync [command] [options]`

**Key Implementation Details**:
- Sources `legacy/lib/common.sh` for shared functions
- Mirrors Go command surface for compatibility
- Exit codes: 0=success, 1=error

### legacy/bin/env-sync-discover
**Purpose**: Discover env-sync peers on local network (legacy Bash)
**Usage**: `legacy/bin/env-sync-discover [options]` or `ENV_SYNC_USE_BASH=true env-sync discover`

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

### legacy/bin/env-sync-client
**Purpose**: Fetch and sync secrets from peers using SCP (SSH) by default (legacy Bash)
**Usage**: `legacy/bin/env-sync-client [options] [hostname]` or `ENV_SYNC_USE_BASH=true env-sync sync`

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

### legacy/bin/env-sync-serve
**Purpose**: HTTP server for serving secrets file (legacy Bash)
**Usage**: `legacy/bin/env-sync-serve [options]`

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

### legacy/lib/common.sh
**Purpose**: Shared functions and utilities
**Sourced by**: All legacy scripts

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
ENV_SYNC_VERSION="2.0.0"    # Tool version
ENV_SYNC_PORT="5739"         # Server port
ENV_SYNC_SERVICE="_envsync._tcp"
SECRETS_FILE="$HOME/.secrets.env"
CONFIG_DIR="$HOME/.config/env-sync"
BACKUP_DIR="$CONFIG_DIR/backups"
LOG_DIR="$CONFIG_DIR/logs"
MAX_BACKUPS=5
```

### install.sh
**Purpose**: Installation script (builds Go binary by default)
**Usage**: `./install.sh [--user]`

**Install Locations**:
- System: `/usr/local/bin/`, `/usr/local/lib/env-sync/`
- User: `~/.local/bin/`, `~/.local/lib/env-sync/`

**Actions**:
1. Builds Go binary from `src/cmd/env-sync`
2. Installs shims to `$BIN_DIR` and Go binary to `$LIB_DIR/env-sync-go`
3. Installs legacy Bash implementation into `$LIB_DIR/legacy` for compatibility/tests
4. Creates symlinks for subcommands (env-sync-client, discover, serve, key, load)
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
OPENAI_API_KEY="sk-xxx"
AWS_ACCESS_KEY_ID="AKIA..."

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
- File permissions: 600 (owner only)

## Dependencies

### Required
- `go` (1.24+) for building the primary binary
- `bash` (v4.0+) for legacy compatibility and test scripts

### Optional (primarily for legacy Bash paths)
- `curl` (HTTP fallback in legacy client)
- `nc` or `netcat` (legacy HTTP server)
- `sha256sum` (legacy checksum generation)
- `date` with `-d/-I` support

### Platform-Specific (Discovery utilities)
- Linux: `avahi-daemon` + `avahi-utils` (`avahi-browse`)
- macOS: `dns-sd` (built-in)
- Windows: WSL2 with Linux dependencies or Bonjour SDK

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
    │   └── Compare version/timestamp
    ├── Find newest version
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
# Test discovery (Go default)
env-sync discover --verbose

# Test server (foreground)
env-sync serve --port 9999

# Test client (dry run)
env-sync sync hostname.local

# Test full sync
env-sync sync -f

# Check status
env-sync status

# View logs
tail -f ~/.config/env-sync/logs/env-sync.log

# Force legacy Bash for comparison
ENV_SYNC_USE_BASH=true env-sync status
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

### Current (v2.0.0)
- ✅ Go implementation is the default runtime (shimmed entrypoints)
- ✅ Legacy Bash implementation preserved under `legacy/` (opt-in via `ENV_SYNC_USE_BASH=true`)
- ✅ SCP/SSH sync (secure by default) with mDNS discovery
- ✅ HTTP server/client fallback (with warnings)
- ✅ Version comparison, backups, cron automation, encryption helpers

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
