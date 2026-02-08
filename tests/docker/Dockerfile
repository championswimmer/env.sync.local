FROM ubuntu:22.04

# Prevent interactive prompts during build
ENV DEBIAN_FRONTEND=noninteractive

# Install dependencies
RUN apt-get update && apt-get install -y \
    bash \
    curl \
    openssh-server \
    avahi-daemon \
    avahi-utils \
    libnss-mdns \
    age \
    jq \
    netcat \
    iputils-ping \
    sudo \
    && rm -rf /var/lib/apt/lists/*

# Create envsync user
RUN useradd -m -s /bin/bash -u 1000 envsync && \
    echo 'envsync:envsync' | chpasswd && \
    usermod -aG sudo envsync

# Configure SSH
RUN mkdir -p /var/run/sshd
RUN sed -i 's/#PermitRootLogin prohibit-password/PermitRootLogin no/' /etc/ssh/sshd_config
RUN sed -i 's/#PasswordAuthentication yes/PasswordAuthentication yes/' /etc/ssh/sshd_config
RUN sed -i 's/PermitRootLogin prohibit-password/PermitRootLogin no/' /etc/ssh/sshd_config

# Allow envsync user to run commands without password (for tests)
RUN echo 'envsync ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/envsync && \
    chmod 440 /etc/sudoers.d/envsync

# Configure Avahi for mDNS (enable D-Bus)
RUN sed -i 's/#enable-dbus=yes/enable-dbus=yes/' /etc/avahi/avahi-daemon.conf
RUN sed -i 's/hosts:.*files dns/hosts: files mdns_minimal [NOTFOUND=return] dns mdns/' /etc/nsswitch.conf

# Copy env-sync binaries (built from source)
ARG ENV_SYNC_GO_BIN=target/env-sync
COPY ${ENV_SYNC_GO_BIN} /usr/local/bin/env-sync
RUN ln -sf env-sync /usr/local/bin/env-sync-discover && \
    ln -sf env-sync /usr/local/bin/env-sync-client && \
    ln -sf env-sync /usr/local/bin/env-sync-serve && \
    ln -sf env-sync /usr/local/bin/env-sync-key && \
    ln -sf env-sync /usr/local/bin/env-sync-load

RUN mkdir -p /usr/local/lib/env-sync/legacy/bin /usr/local/lib/env-sync/legacy/lib
COPY legacy/bin/ /usr/local/lib/env-sync/legacy/bin/
COPY legacy/lib/ /usr/local/lib/env-sync/legacy/lib/

# Make scripts executable
RUN chmod +x /usr/local/bin/env-sync /usr/local/lib/env-sync/legacy/bin/env-sync*

# Create env-sync directories for envsync user
RUN mkdir -p /home/envsync/.config/env-sync/keys/known_hosts && \
    mkdir -p /home/envsync/.config/env-sync/logs && \
    mkdir -p /home/envsync/.ssh && \
    chmod 700 /home/envsync/.ssh && \
    chmod 755 /home/envsync/.config && \
    chmod 700 /home/envsync/.config/env-sync && \
    chmod 700 /home/envsync/.config/env-sync/keys && \
    chown -R envsync:envsync /home/envsync

# Entrypoint script
COPY tests/docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
CMD ["bash"]
