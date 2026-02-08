#!/bin/bash
# Entrypoint script for env-sync test containers
# Note: We don't use 'set -e' because we want to handle errors gracefully

USER_NAME="envsync"
USER_HOME="/home/envsync"
SSH_WORK_DIR="/tmp/ssh-work"
AVAHI_SERVICE_DIR="/etc/avahi/services"
AVAHI_SERVICE_FILE="${AVAHI_SERVICE_DIR}/env-sync.service"
KNOWN_HOSTS_FILE="${USER_HOME}/.ssh/known_hosts"
SSH_CONFIG_FILE="${USER_HOME}/.ssh/config"
KEYSCAN_TIMEOUT="${ENV_SYNC_KEYSCAN_TIMEOUT:-1}"
KEYSCAN_ATTEMPTS="${ENV_SYNC_KEYSCAN_ATTEMPTS:-3}"
LEGACY_BIN_DIR="/usr/local/lib/env-sync/legacy/bin"

echo "[entrypoint] Starting env-sync test container: $HOSTNAME"

if [ -n "${ENV_SYNC_USE_BASH:-}" ]; then
    export PATH="${LEGACY_BIN_DIR}:${PATH}"
    echo "[entrypoint] ENV_SYNC_USE_BASH set; using legacy scripts from ${LEGACY_BIN_DIR}"
fi

# Create a writable SSH directory since the mounted one is read-only
echo "[entrypoint] Setting up writable SSH directory..."
mkdir -p ${SSH_WORK_DIR}
chmod 700 ${SSH_WORK_DIR}

# Copy SSH keys from the mounted location to the writable location
if [ -f /mnt/ssh-keys/id_ed25519 ]; then
    cp /mnt/ssh-keys/id_ed25519 ${SSH_WORK_DIR}/id_ed25519
    cp /mnt/ssh-keys/id_ed25519.pub ${SSH_WORK_DIR}/id_ed25519.pub
    cp /mnt/ssh-keys/authorized_keys ${SSH_WORK_DIR}/authorized_keys
    
    chown -R ${USER_NAME}:${USER_NAME} ${SSH_WORK_DIR}
    chmod 600 ${SSH_WORK_DIR}/id_ed25519
    chmod 644 ${SSH_WORK_DIR}/id_ed25519.pub
    chmod 644 ${SSH_WORK_DIR}/authorized_keys
    
    echo "[entrypoint] SSH keys copied to writable location: ${SSH_WORK_DIR}"
fi

# Create .ssh directory for the user and copy keys there
mkdir -p ${USER_HOME}/.ssh
chown ${USER_NAME}:${USER_NAME} ${USER_HOME}/.ssh
chmod 700 ${USER_HOME}/.ssh

# Copy keys to user's .ssh directory
cp ${SSH_WORK_DIR}/id_ed25519 ${USER_HOME}/.ssh/id_ed25519 2>/dev/null || true
cp ${SSH_WORK_DIR}/id_ed25519.pub ${USER_HOME}/.ssh/id_ed25519.pub 2>/dev/null || true
cp ${SSH_WORK_DIR}/authorized_keys ${USER_HOME}/.ssh/authorized_keys 2>/dev/null || true

# Ensure known_hosts exists
touch ${KNOWN_HOSTS_FILE} 2>/dev/null || true

chown -R ${USER_NAME}:${USER_NAME} ${USER_HOME}/.ssh
chmod 600 ${USER_HOME}/.ssh/id_ed25519 2>/dev/null || true
chmod 644 ${USER_HOME}/.ssh/id_ed25519.pub 2>/dev/null || true
chmod 644 ${USER_HOME}/.ssh/authorized_keys 2>/dev/null || true
chmod 600 ${KNOWN_HOSTS_FILE} 2>/dev/null || true

# Set SSH client config to reduce noisy output in tests
cat > ${SSH_CONFIG_FILE} << 'EOF'
Host *
    StrictHostKeyChecking no
    LogLevel ERROR
EOF
chown ${USER_NAME}:${USER_NAME} ${SSH_CONFIG_FILE} 2>/dev/null || true
chmod 600 ${SSH_CONFIG_FILE} 2>/dev/null || true

echo "[entrypoint] SSH directory set up at ${USER_HOME}/.ssh"

# Generate host SSH keys if not present
if [ ! -f /etc/ssh/ssh_host_ed25519_key ]; then
    echo "[entrypoint] Generating SSH host keys..."
    ssh-keygen -t ed25519 -f /etc/ssh/ssh_host_ed25519_key -N "" 2>/dev/null || true
fi
if [ ! -f /etc/ssh/ssh_host_rsa_key ]; then
    ssh-keygen -t rsa -b 4096 -f /etc/ssh/ssh_host_rsa_key -N "" 2>/dev/null || true
fi

# Start SSH daemon
echo "[entrypoint] Starting SSH daemon..."
/usr/sbin/sshd

# Start D-Bus (required for avahi on some systems)
if [ -d /var/run/dbus ]; then
    rm -rf /var/run/dbus/* 2>/dev/null || true
fi
mkdir -p /var/run/dbus 2>/dev/null || true
dbus-daemon --system --fork 2>/dev/null || echo "[entrypoint] D-Bus note: non-fatal"

# Register env-sync mDNS service for discovery
echo "[entrypoint] Registering env-sync mDNS service..."
mkdir -p ${AVAHI_SERVICE_DIR} 2>/dev/null || true
cat > ${AVAHI_SERVICE_FILE} << EOF
<?xml version="1.0" standalone='no'?><!--*-nxml-*-->
<!DOCTYPE service-group SYSTEM "avahi-service.dtd">
<service-group>
  <name replace-wildcards="yes">%h</name>
  <service>
    <type>_envsync._tcp</type>
    <port>${ENV_SYNC_PORT:-5739}</port>
  </service>
</service-group>
EOF

# Start Avahi for mDNS
echo "[entrypoint] Starting Avahi daemon..."
avahi-daemon --daemonize --no-drop-root 2>/dev/null || avahi-daemon --daemonize 2>/dev/null || echo "[entrypoint] Avahi warning: could not start"

# Wait for services to be ready
echo "[entrypoint] Waiting for services to start..."
sleep 3

# Test SSH is working
echo "[entrypoint] Testing SSH daemon..."
if pgrep -x sshd > /dev/null 2>&1; then
    echo "[entrypoint] SSH daemon is running"
else
    echo "[entrypoint] WARNING: SSH daemon may not be running"
fi

# Test mDNS is working
echo "[entrypoint] Testing Avahi daemon..."
if pgrep -x avahi-daemon > /dev/null 2>&1; then
    echo "[entrypoint] Avahi daemon is running"
else
    echo "[entrypoint] WARNING: Avahi daemon may not be running"
fi

# Populate known_hosts with other containers (as envsync user)
echo "[entrypoint] Populating SSH known_hosts..."
for host in alpha.local beta.local gamma.local; do
    if [ "$host" != "$HOSTNAME" ]; then
        for ((attempt=1; attempt<=KEYSCAN_ATTEMPTS; attempt++)); do
            scan_output=$(su - ${USER_NAME} -c "ssh-keyscan -T ${KEYSCAN_TIMEOUT} -H $host 2>/dev/null" || true)
            if [ -n "$scan_output" ]; then
                echo "$scan_output" >> ${KNOWN_HOSTS_FILE}
                break
            fi
            sleep 1
        done
    fi
done
chown ${USER_NAME}:${USER_NAME} ${KNOWN_HOSTS_FILE} 2>/dev/null || true
chmod 600 ${KNOWN_HOSTS_FILE} 2>/dev/null || true

echo "[entrypoint] Container is ready: $HOSTNAME"

# If no command provided or command is 'bash', keep container running as envsync user
if [ $# -eq 0 ] || [ "$1" = "bash" ] || [ "$1" = "/bin/bash" ]; then
    echo "[entrypoint] Keeping container running as ${USER_NAME}..."
    su - ${USER_NAME} -c "tail -f /dev/null"
else
    echo "[entrypoint] Executing command as ${USER_NAME}: $@"
    su - ${USER_NAME} -c "$@"
fi
