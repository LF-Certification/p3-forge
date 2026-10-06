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

# The sidecar runs as uid 1000: ssh needs a passwd entry for it, and
# fusermount3 needs user_allow_other for a non-root `-o allow_other` mount.
RUN addgroup -g 1000 coder && \
    adduser -D -u 1000 -G coder -h /home/coder -s /bin/bash coder && \
    echo "user_allow_other" >> /etc/fuse.conf

# Copy the mount script
COPY mount-sshfs.sh /usr/local/bin/mount-sshfs.sh
RUN chmod +x /usr/local/bin/mount-sshfs.sh

# Container runs as root to have mount privileges
# The init container security context will handle this
USER root

ENTRYPOINT ["/usr/local/bin/mount-sshfs.sh"]
