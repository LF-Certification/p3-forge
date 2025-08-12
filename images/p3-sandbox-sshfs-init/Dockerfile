FROM alpine:3.19

# OCI labels for package linking and metadata
LABEL org.opencontainers.image.source="https://github.com/lf-certification/sandbox-images"
LABEL org.opencontainers.image.title="p3-sandbox-sshfs-init"
LABEL org.opencontainers.image.description="SSHFS init container for mounting remote filesystems"

# Install sshfs, openssh-client, and utilities
RUN apk update && \
    apk add --no-cache \
        sshfs \
        openssh-client \
        fuse \
        bash \
        coreutils \
        util-linux \
    && rm -rf /var/cache/apk/*

# Copy the mount script
COPY mount-sshfs.sh /usr/local/bin/mount-sshfs.sh
RUN chmod +x /usr/local/bin/mount-sshfs.sh

# Create directories
RUN mkdir -p /workspace

# Container runs as root to have mount privileges
# The init container security context will handle this
USER root

ENTRYPOINT ["/usr/local/bin/mount-sshfs.sh"]
