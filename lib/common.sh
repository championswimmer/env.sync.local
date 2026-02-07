#!/bin/bash
# Common functions for env-sync
# This file is sourced by other scripts

set -euo pipefail

# Configuration
ENV_SYNC_VERSION="1.0.0"
ENV_SYNC_PORT="5739"
ENV_SYNC_SERVICE="_envsync._tcp"
ENV_SYNC_INIT_TIMESTAMP="${ENV_SYNC_INIT_TIMESTAMP:-1970-01-01T00:00:00Z}"
ENV_SYNC_KEY_UPDATED_AT_MARKER="ENV_SYNC_UPDATED_AT"
ENV_SYNC_KEY_UPDATED_AT_COMMENT="#${ENV_SYNC_KEY_UPDATED_AT_MARKER}"
ENV_SYNC_KEY_REGEX='[A-Za-z_][A-Za-z0-9_]*'
ENV_SYNC_TIMESTAMP_REGEX='[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z'
SECRETS_FILE="${HOME}/.secrets.env"
CONFIG_DIR="${HOME}/.config/env-sync"
BACKUP_DIR="${CONFIG_DIR}/backups"
LOG_DIR="${CONFIG_DIR}/logs"
MAX_BACKUPS=5

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging function
log() {
    local level="$1"
    shift
    local message="$*"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    mkdir -p "$LOG_DIR"
    echo "[$timestamp] [$level] $message" >> "$LOG_DIR/env-sync.log"
    
    if [[ "${ENV_SYNC_QUIET:-}" != "true" ]]; then
        case "$level" in
            ERROR) echo -e "${RED}ERROR:${NC} $message" >&2 ;;
            WARN)  echo -e "${YELLOW}WARN:${NC} $message" >&2 ;;
            INFO)  echo -e "${BLUE}INFO:${NC} $message" >&2 ;;
            SUCCESS) echo -e "${GREEN}SUCCESS:${NC} $message" >&2 ;;
        esac
    fi
}

# Get hostname
get_hostname() {
    hostname -f 2>/dev/null || hostname
}

# Generate timestamp
get_timestamp() {
    date -u '+%Y-%m-%dT%H:%M:%SZ'
}

# Generate checksum
generate_checksum() {
    local file="$1"
    if [[ -f "$file" ]]; then
        sed 's/^# CHECKSUM: .*/# CHECKSUM: /' "$file" | sha256sum | cut -d' ' -f1
    else
        echo ""
    fi
}

# Extract metadata from secrets file
extract_metadata() {
    local file="$1"
    local key="$2"
    
    if [[ ! -f "$file" ]]; then
        echo ""
        return
    fi
    
    grep "^# $key:" "$file" | head -1 | sed "s/^# $key: //"
}

# Get version from secrets file
get_file_version() {
    local file="$1"
    extract_metadata "$file" "VERSION"
}

# Get timestamp from secrets file
get_file_timestamp() {
    local file="$1"
    extract_metadata "$file" "TIMESTAMP"
}

# Get hostname from secrets file
get_file_host() {
    local file="$1"
    extract_metadata "$file" "HOST"
}

# Get checksum from secrets file
get_file_checksum() {
    local file="$1"
    extract_metadata "$file" "CHECKSUM"
}

# Compare two versions (semantic versioning)
# Returns: 0 if equal, 1 if v1 > v2, 2 if v1 < v2
compare_versions() {
    local v1="$1"
    local v2="$2"
    
    if [[ "$v1" == "$v2" ]]; then
        return 0
    fi
    
    # Use sort -V for version comparison
    local higher=$(printf '%s\n%s\n' "$v1" "$v2" | sort -V | tail -n1)
    
    if [[ "$v1" == "$higher" ]]; then
        return 1  # v1 > v2
    else
        return 2  # v1 < v2
    fi
}

# Compare two timestamps (ISO 8601 format)
# Returns: 0 if equal, 1 if t1 > t2, 2 if t1 < t2
compare_timestamps() {
    local t1="$1"
    local t2="$2"
    
    if [[ "$t1" == "$t2" ]]; then
        return 0
    fi
    
    # Convert to seconds since epoch and compare
    local s1=$(date -d "$t1" '+%s' 2>/dev/null || echo "0")
    local s2=$(date -d "$t2" '+%s' 2>/dev/null || echo "0")
    
    if [[ "$s1" -gt "$s2" ]]; then
        return 1  # t1 > t2
    else
        return 2  # t1 < t2
    fi
}

# Determine if file1 is newer than file2
# Returns 0 if file1 is newer, 1 otherwise
is_newer() {
    local file1="$1"
    local file2="$2"
    
    local v1=$(get_file_version "$file1")
    local v2=$(get_file_version "$file2")
    local t1=$(get_file_timestamp "$file1")
    local t2=$(get_file_timestamp "$file2")
    local h1=$(get_file_host "$file1")
    local h2=$(get_file_host "$file2")
    
    # Compare timestamps first
    if [[ -n "$t1" && -n "$t2" ]]; then
        compare_timestamps "$t1" "$t2"
        local ts_result=$?
        if [[ $ts_result -eq 1 ]]; then
            return 0  # file1 is newer
        elif [[ $ts_result -eq 2 ]]; then
            return 1  # file2 is newer
        fi
    fi
    
    # Timestamps equal, compare versions
    if [[ -n "$v1" && -n "$v2" ]]; then
        compare_versions "$v1" "$v2"
        local v_result=$?
        if [[ $v_result -eq 1 ]]; then
            return 0  # file1 is newer
        elif [[ $v_result -eq 2 ]]; then
            return 1  # file2 is newer
        fi
    fi
    
    # Both equal, use hostname as tiebreaker (deterministic)
    if [[ "$h1" < "$h2" ]]; then
        return 0
    else
        return 1
    fi
}

# Create backup of secrets file
create_backup() {
    local file="$1"
    
    if [[ ! -f "$file" ]]; then
        return
    fi
    
    mkdir -p "$BACKUP_DIR"
    
    # Rotate backups
    for i in $(seq $((MAX_BACKUPS - 1)) -1 1); do
        local j=$((i + 1))
        if [[ -f "$BACKUP_DIR/secrets.backup.$i" ]]; then
            mv "$BACKUP_DIR/secrets.backup.$i" "$BACKUP_DIR/secrets.backup.$j"
        fi
    done
    
    # Create new backup
    cp "$file" "$BACKUP_DIR/secrets.backup.1"
    chmod 600 "$BACKUP_DIR/secrets.backup.1"
    
    log "INFO" "Created backup: secrets.backup.1"
}

# Restore from backup
restore_backup() {
    local backup_num="${1:-1}"
    local backup_file="$BACKUP_DIR/secrets.backup.$backup_num"
    
    if [[ ! -f "$backup_file" ]]; then
        log "ERROR" "Backup $backup_num not found"
        return 1
    fi
    
    create_backup "$SECRETS_FILE"
    cp "$backup_file" "$SECRETS_FILE"
    chmod 600 "$SECRETS_FILE"
    log "SUCCESS" "Restored from backup $backup_num"
}

# Initialize new secrets file
init_secrets_file() {
    local file="$1"
    local init_timestamp="${2:-$ENV_SYNC_INIT_TIMESTAMP}"
    local hostname=$(get_hostname)
    local timestamp="$init_timestamp"
    local version="1.0.0"
    
    mkdir -p "$(dirname "$file")"
    
    cat > "$file" << EOF
# === ENV_SYNC_METADATA ===
# VERSION: $version
# TIMESTAMP: $timestamp
# HOST: $hostname
# MODIFIED: $timestamp
# CHECKSUM: 
# === END_METADATA ===

# Add your secrets below this line
# Example:
# OPENAI_API_KEY="sk-..."

# === ENV_SYNC_FOOTER ===
# VERSION: $version
# TIMESTAMP: $timestamp
# HOST: $hostname
# === END_FOOTER ===
EOF
    
    chmod 600 "$file"
    
    # Update checksum
    local checksum=$(generate_checksum "$file")
    sed -i.bak "s/^# CHECKSUM: /# CHECKSUM: $checksum/" "$file"
    rm -f "$file.bak"
    
    log "SUCCESS" "Initialized secrets file: $file"
}

# Update secrets file metadata
update_metadata() {
    local file="$1"
    local new_version="${2:-}"
    
    if [[ ! -f "$file" ]]; then
        log "ERROR" "File not found: $file"
        return 1
    fi
    
    local hostname=$(get_hostname)
    local timestamp=$(get_timestamp)
    local current_version=$(get_file_version "$file")
    local version="${new_version:-$current_version}"
    
    # Update header
    sed -i.bak \
        -e "s/^# TIMESTAMP: .*/# TIMESTAMP: $timestamp/" \
        -e "s/^# HOST: .*/# HOST: $hostname/" \
        -e "s/^# MODIFIED: .*/# MODIFIED: $timestamp/" \
        "$file"
    rm -f "$file.bak"
    
    # Update footer
    sed -i.bak \
        -e "s/^# VERSION: .*/# VERSION: $version/" \
        -e "s/^# TIMESTAMP: .*/# TIMESTAMP: $timestamp/" \
        -e "s/^# HOST: .*/# HOST: $hostname/" \
        "$file"
    rm -f "$file.bak"
    
    # Clear checksum before calculating new one
    sed -i.bak "s/^# CHECKSUM: .*/# CHECKSUM: /" "$file"
    rm -f "$file.bak"
    
    # Update checksum
    local checksum=$(generate_checksum "$file")
    sed -i.bak "s/^# CHECKSUM: .*/# CHECKSUM: $checksum/" "$file"
    rm -f "$file.bak"
}

# Validate secrets file
validate_secrets_file() {
    local file="$1"
    
    if [[ ! -f "$file" ]]; then
        log "ERROR" "Secrets file not found: $file"
        return 1
    fi
    
    # Check metadata exists
    if ! grep -q "^# === ENV_SYNC_METADATA ===" "$file"; then
        log "ERROR" "Invalid secrets file: missing metadata header"
        return 1
    fi
    
    if ! grep -q "^# === ENV_SYNC_FOOTER ===" "$file"; then
        log "ERROR" "Invalid secrets file: missing metadata footer"
        return 1
    fi
    
    # Validate checksum
    local stored_checksum=$(get_file_checksum "$file")
    if [[ -n "$stored_checksum" ]]; then
        local current_checksum=$(generate_checksum "$file")
        if [[ "$stored_checksum" != "$current_checksum" ]]; then
            log "WARN" "Checksum mismatch - file may be corrupted"
            return 1
        fi
    fi
    
    return 0
}

# Get secrets content (without metadata)
get_secrets_content() {
    local file="$1"
    
    if [[ ! -f "$file" ]]; then
        return
    fi
    
    if grep -q "^# === END_METADATA ===" "$file"; then
        # Extract content between metadata header and footer
        awk '/^# === END_METADATA ===/{found=1; next} /^# === ENV_SYNC_FOOTER ===/{found=0} found' "$file"
    else
        # Decrypted encrypted content has no metadata
        cat "$file"
    fi
}

# Set secrets content (update file while preserving metadata)
set_secrets_content() {
    local file="$1"
    local content="$2"
    
    if [[ ! -f "$file" ]]; then
        init_secrets_file "$file"
    fi
    
    # Create temporary file
    local tmp_file=$(mktemp)
    
    # Write header
    awk '/^# === ENV_SYNC_METADATA ===/,/^# === END_METADATA ===/' "$file" > "$tmp_file"
    
    # Write content
    echo "" >> "$tmp_file"
    echo "$content" >> "$tmp_file"
    echo "" >> "$tmp_file"
    
    # Write footer
    awk '/^# === ENV_SYNC_FOOTER ===/,/^# === END_FOOTER ===/' "$file" >> "$tmp_file"
    
    # Move temp file to target
    mv "$tmp_file" "$file"
    chmod 600 "$file"
    
    # Update metadata
    update_metadata "$file"
}

# Extract per-key updated-at timestamp from inline comment
extract_key_updated_at_timestamp() {
    local line="$1"
    local ts_regex="$ENV_SYNC_TIMESTAMP_REGEX"
    if [[ "$line" =~ [[:space:]]#${ENV_SYNC_KEY_UPDATED_AT_MARKER}[[:space:]]+(${ts_regex}) ]]; then
        echo "${BASH_REMATCH[1]}"
    fi
}

# Strip inline updated-at comment from a line
strip_key_updated_at_comment() {
    local line="$1"
    local ts_regex="$ENV_SYNC_TIMESTAMP_REGEX"
    echo "$line" | sed -E "s/[[:space:]]#${ENV_SYNC_KEY_UPDATED_AT_MARKER}[[:space:]]+${ts_regex}[[:space:]]*$//"
}

# Append per-key updated-at comment to a line
append_key_updated_at_comment() {
    local line="$1"
    local timestamp="$2"
    local clean_line
    clean_line=$(strip_key_updated_at_comment "$line")
    echo "${clean_line} ${ENV_SYNC_KEY_UPDATED_AT_COMMENT} ${timestamp}"
}

# Merge secrets content from two files based on per-key timestamps.
# Writes merged content (without metadata header/footer) to output_file.
# Params: base_file base_fallback_ts incoming_file incoming_fallback_ts output_file [force_override]
merge_secrets_content() {
    local base_file="$1"
    local base_fallback_ts="${2:-}"
    local incoming_file="$3"
    local incoming_fallback_ts="${4:-}"
    local output_file="$5"
    local force_override="${6:-false}"
    local key_regex="$ENV_SYNC_KEY_REGEX"

    [[ -z "$base_fallback_ts" ]] && base_fallback_ts="$ENV_SYNC_INIT_TIMESTAMP"
    [[ -z "$incoming_fallback_ts" ]] && incoming_fallback_ts="$ENV_SYNC_INIT_TIMESTAMP"

    local base_content
    base_content=$(get_secrets_content "$base_file")
    local incoming_content
    incoming_content=$(get_secrets_content "$incoming_file")

    local lines=()
    if [[ -n "$base_content" ]]; then
        mapfile -t lines < <(printf '%s\n' "$base_content")
    fi

    declare -A base_key_index=()
    declare -A base_key_ts=()
    declare -A base_key_by_index=()

    for i in "${!lines[@]}"; do
        local line="${lines[$i]}"
        if [[ "$line" =~ ^export[[:space:]]+(${key_regex})= ]]; then
            local key="${BASH_REMATCH[1]}"
            local updated_at
            updated_at=$(extract_key_updated_at_timestamp "$line")
            lines[$i]=$(strip_key_updated_at_comment "$line")
            base_key_index["$key"]="$i"
            base_key_by_index["$i"]="$key"
            base_key_ts["$key"]="${updated_at:-$base_fallback_ts}"
            continue
        fi
        if [[ "$line" =~ ^(${key_regex})= ]]; then
            local key="${BASH_REMATCH[1]}"
            local updated_at
            updated_at=$(extract_key_updated_at_timestamp "$line")
            lines[$i]=$(strip_key_updated_at_comment "$line")
            base_key_index["$key"]="$i"
            base_key_by_index["$i"]="$key"
            base_key_ts["$key"]="${updated_at:-$base_fallback_ts}"
        fi
    done

    declare -A incoming_lines=()
    declare -A incoming_ts=()

    if [[ -n "$incoming_content" ]]; then
        while IFS= read -r line; do
            if [[ "$line" =~ ^export[[:space:]]+(${key_regex})= ]]; then
                local key="${BASH_REMATCH[1]}"
                local updated_at
                updated_at=$(extract_key_updated_at_timestamp "$line")
                incoming_lines["$key"]=$(strip_key_updated_at_comment "$line")
                incoming_ts["$key"]="${updated_at:-$incoming_fallback_ts}"
                continue
            fi
            if [[ "$line" =~ ^(${key_regex})= ]]; then
                local key="${BASH_REMATCH[1]}"
                local updated_at
                updated_at=$(extract_key_updated_at_timestamp "$line")
                incoming_lines["$key"]=$(strip_key_updated_at_comment "$line")
                incoming_ts["$key"]="${updated_at:-$incoming_fallback_ts}"
            fi
        done < <(printf '%s\n' "$incoming_content")
    fi

    for key in "${!incoming_lines[@]}"; do
        local remote_ts="${incoming_ts[$key]:-$incoming_fallback_ts}"
        if [[ -z "${base_key_index[$key]+x}" ]]; then
            if [[ ${#lines[@]} -gt 0 ]]; then
                local last_index=$(( ${#lines[@]} - 1 ))
                if [[ -n "${lines[$last_index]}" ]]; then
                    lines+=("")
                fi
            fi
            lines+=("${incoming_lines[$key]}")
            base_key_index["$key"]=$(( ${#lines[@]} - 1 ))
            base_key_by_index["${base_key_index[$key]}"]="$key"
            base_key_ts["$key"]="$remote_ts"
            continue
        fi

        if [[ "$force_override" == "true" ]]; then
            lines[${base_key_index[$key]}]="${incoming_lines[$key]}"
            base_key_ts["$key"]="$remote_ts"
            continue
        fi

        local local_ts="${base_key_ts[$key]:-$base_fallback_ts}"
        local ts_result=0
        set +e
        compare_timestamps "$remote_ts" "$local_ts"
        ts_result=$?
        set -e
        if [[ $ts_result -eq 1 ]]; then
            lines[${base_key_index[$key]}]="${incoming_lines[$key]}"
            base_key_ts["$key"]="$remote_ts"
        fi
    done

    local output_lines=()
    for i in "${!lines[@]}"; do
        local line="${lines[$i]}"
        if [[ -n "${base_key_by_index[$i]+x}" ]]; then
            local key="${base_key_by_index[$i]}"
            local ts="${base_key_ts[$key]:-$base_fallback_ts}"
            output_lines+=("$(append_key_updated_at_comment "$line" "$ts")")
        else
            output_lines+=("$line")
        fi
    done

    if [[ ${#output_lines[@]} -eq 0 ]]; then
        : > "$output_file"
    else
        printf '%s\n' "${output_lines[@]}" > "$output_file"
    fi
    return 0
}

# AGE Key Management Functions
# ============================

# Directory for AGE keys
AGE_KEY_DIR="$CONFIG_DIR/keys"
AGE_KEY_FILE="$AGE_KEY_DIR/age_key"
AGE_PUBKEY_FILE="$AGE_KEY_DIR/age_key.pub"
AGE_CACHE_DIR="$AGE_KEY_DIR/cache"
AGE_KNOWN_HOSTS_DIR="$AGE_KEY_DIR/known_hosts"

# Initialize AGE directories
init_age_dirs() {
    mkdir -p "$AGE_KEY_DIR"
    mkdir -p "$AGE_CACHE_DIR"
    mkdir -p "$AGE_KNOWN_HOSTS_DIR"
    chmod 700 "$AGE_KEY_DIR"
}

# Check if age is installed
check_age_installed() {
    if ! command -v age >/dev/null 2>&1; then
        log "ERROR" "age is not installed. Install with: brew install age (macOS) or apt install age (Linux)"
        return 1
    fi
    if ! command -v age-keygen >/dev/null 2>&1; then
        log "ERROR" "age-keygen is not installed. Install age package."
        return 1
    fi
    return 0
}

# Generate AGE key pair
generate_age_key() {
    init_age_dirs
    
    if [[ -f "$AGE_KEY_FILE" ]]; then
        log "WARN" "AGE key already exists at $AGE_KEY_FILE"
        return 1
    fi
    
    # Generate key
    age-keygen -o "$AGE_KEY_FILE" 2>/dev/null
    chmod 600 "$AGE_KEY_FILE"
    
    # Extract public key
    local pubkey=$(age-keygen -y "$AGE_KEY_FILE" 2>/dev/null)
    echo "$pubkey" > "$AGE_PUBKEY_FILE"
    chmod 644 "$AGE_PUBKEY_FILE"
    
    log "SUCCESS" "Generated AGE key pair"
    log "INFO" "Public key: $pubkey"
    
    return 0
}

# Get local AGE public key
get_local_age_pubkey() {
    if [[ -f "$AGE_PUBKEY_FILE" ]]; then
        cat "$AGE_PUBKEY_FILE"
    else
        echo ""
    fi
}

# Get local AGE private key path
get_local_age_key() {
    echo "$AGE_KEY_FILE"
}

# Check if file is encrypted
is_file_encrypted() {
    local file="$1"
    if [[ ! -f "$file" ]]; then
        return 1
    fi
    
    # Check metadata flag
    if grep -q "^# ENCRYPTED: true" "$file"; then
        return 0
    fi
    
    return 1
}

# Get list of recipients from encrypted file
get_recipients_from_file() {
    local file="$1"
    if ! is_file_encrypted "$file"; then
        echo ""
        return
    fi
    
    local recipients=$(extract_metadata "$file" "RECIPIENTS")
    echo "$recipients"
}

# Check if local machine can decrypt file
can_decrypt_file() {
    local file="$1"
    
    if ! is_file_encrypted "$file"; then
        return 0  # Not encrypted = can "decrypt" (nothing to do)
    fi
    
    if [[ ! -f "$AGE_KEY_FILE" ]]; then
        return 1  # No private key
    fi
    
    local local_pubkey=$(get_local_age_pubkey)
    local recipients=$(get_recipients_from_file "$file")
    
    # Check if local pubkey is in recipients
    if [[ "$recipients" == *"$local_pubkey"* ]]; then
        return 0
    fi
    
    return 1
}

# Decrypt secrets file to plaintext
decrypt_secrets_file() {
    local input_file="$1"
    local output_file="${2:-}"
    
    if ! is_file_encrypted "$input_file"; then
        # Not encrypted, just copy
        if [[ -n "$output_file" ]]; then
            cp "$input_file" "$output_file"
        fi
        return 0
    fi
    
    if ! can_decrypt_file "$input_file"; then
        log "ERROR" "Cannot decrypt file - not in recipient list"
        return 1
    fi
    
    # Extract encrypted block
    local encrypted_block=$(sed -n '/-----BEGIN AGE ENCRYPTED FILE-----/,/-----END AGE ENCRYPTED FILE-----/p' "$input_file" | grep -v '^-----')
    
    if [[ -z "$encrypted_block" ]]; then
        log "ERROR" "No encrypted data found in file"
        return 1
    fi
    
    # Decode base64 and decrypt
    if [[ -n "$output_file" ]]; then
        echo "$encrypted_block" | base64 -d | age -d -i "$AGE_KEY_FILE" -o "$output_file" 2>/dev/null
    else
        echo "$encrypted_block" | base64 -d | age -d -i "$AGE_KEY_FILE" 2>/dev/null
    fi
    
    return $?
}

# Encrypt secrets content to multiple recipients
encrypt_secrets_content() {
    local content="$1"
    shift
    local recipients=("$@")  # Array of recipient pubkeys
    
    if [[ ${#recipients[@]} -eq 0 ]]; then
        log "ERROR" "No recipients specified for encryption"
        return 1
    fi
    
    # Build age recipient arguments
    local age_args=()
    for recipient in "${recipients[@]}"; do
        age_args+=("-r" "$recipient")
    done
    
    # Encrypt content
    local encrypted
    encrypted=$(echo "$content" | age "${age_args[@]}" 2>/dev/null | base64)
    
    if [[ -z "$encrypted" ]]; then
        log "ERROR" "Encryption failed"
        return 1
    fi
    
    echo "$encrypted"
}

# Save encrypted secrets file
save_encrypted_secrets() {
    local output_file="$1"
    local encrypted_content="$2"
    local recipients="$3"  # Comma-separated list
    local timestamp_override="${4:-}"
    
    local hostname=$(get_hostname)
    local timestamp="${timestamp_override:-$(get_timestamp)}"
    local version=$(get_file_version "$output_file" 2>/dev/null || echo "1.0.0")
    
    cat > "$output_file" << EOF
# === ENV_SYNC_METADATA ===
# VERSION: $version
# TIMESTAMP: $timestamp
# HOST: $hostname
# MODIFIED: $timestamp
# ENCRYPTED: true
# RECIPIENTS: $recipients
# CHECKSUM: 
# === END_METADATA ===

-----BEGIN AGE ENCRYPTED FILE-----
$encrypted_content
-----END AGE ENCRYPTED FILE-----

# === ENV_SYNC_FOOTER ===
# VERSION: $version
# TIMESTAMP: $timestamp
# HOST: $hostname
# === END_FOOTER ===
EOF
    
    chmod 600 "$output_file"
    
    # Update checksum
    local checksum=$(generate_checksum "$output_file")
    sed -i.bak "s/^# CHECKSUM: .*/# CHECKSUM: $checksum/" "$output_file"
    rm -f "$output_file.bak"
}

# Get all known recipient pubkeys from cache
get_all_known_recipients() {
    local recipients=()
    
    # Add local pubkey
    local local_pubkey=$(get_local_age_pubkey)
    if [[ -n "$local_pubkey" ]]; then
        recipients+=("$local_pubkey")
    fi
    
    # Add cached pubkeys from known_hosts
    if [[ -d "$AGE_KNOWN_HOSTS_DIR" ]]; then
        for pubkey_file in "$AGE_KNOWN_HOSTS_DIR"/*.pub; do
            if [[ -f "$pubkey_file" ]]; then
                local pubkey=$(cat "$pubkey_file")
                if [[ -n "$pubkey" && ! " ${recipients[@]} " =~ " ${pubkey} " ]]; then
                    recipients+=("$pubkey")
                fi
            fi
        done
    fi
    
    printf '%s\n' "${recipients[@]}"
}

# Cache a peer's public key
cache_peer_pubkey() {
    local hostname="$1"
    local pubkey="$2"
    
    init_age_dirs
    
    # Save to known_hosts
    echo "$pubkey" > "$AGE_KNOWN_HOSTS_DIR/$hostname.pub"
    chmod 644 "$AGE_KNOWN_HOSTS_DIR/$hostname.pub"
    
    # Update cache metadata
    local cache_file="$AGE_CACHE_DIR/pubkey_cache.json"
    local timestamp=$(get_timestamp)
    
    # Create cache if doesn't exist
    if [[ ! -f "$cache_file" ]]; then
        echo '{}' > "$cache_file"
    fi
    
    # Update cache using jq if available, otherwise use sed
    if command -v jq >/dev/null 2>&1; then
        local temp_cache=$(mktemp)
        jq --arg host "$hostname" \
           --arg key "$pubkey" \
           --arg ts "$timestamp" \
           --arg first "$(jq -r ".[$hostname].first_seen // \"$timestamp\"" "$cache_file" 2>/dev/null || echo "$timestamp")" \
           '.[$host] = {"pubkey": $key, "last_seen": $ts, "first_seen": $first}' \
           "$cache_file" > "$temp_cache"
        mv "$temp_cache" "$cache_file"
    else
        # Simple sed-based update (jq not available)
        log "DEBUG" "jq not available, skipping cache update"
    fi
    
    chmod 600 "$cache_file"
}

# Get cached pubkey for a hostname
get_cached_pubkey() {
    local hostname="$1"
    local pubkey_file="$AGE_KNOWN_HOSTS_DIR/$hostname.pub"
    
    if [[ -f "$pubkey_file" ]]; then
        cat "$pubkey_file"
    else
        echo ""
    fi
}

# List all cached peers
list_cached_peers() {
    if [[ ! -d "$AGE_KNOWN_HOSTS_DIR" ]]; then
        return
    fi
    
    for pubkey_file in "$AGE_KNOWN_HOSTS_DIR"/*.pub; do
        if [[ -f "$pubkey_file" ]]; then
            local hostname=$(basename "$pubkey_file" .pub)
            local pubkey=$(head -1 "$pubkey_file")
            echo "$hostname: $pubkey"
        fi
    done
}

# Remove peer from recipients
remove_peer_pubkey() {
    local hostname="$1"
    local pubkey_file="$AGE_KNOWN_HOSTS_DIR/$hostname.pub"
    
    if [[ -f "$pubkey_file" ]]; then
        rm -f "$pubkey_file"
        log "INFO" "Removed pubkey for $hostname"
    fi
}
