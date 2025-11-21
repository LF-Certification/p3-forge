FROM alpine:3.21
# OCI labels for package linking and metadata
LABEL org.opencontainers.image.source="https://github.com/lf-certification/p3-forge"
LABEL org.opencontainers.image.title="p3-sandbox-lifecycle-base"
LABEL org.opencontainers.image.description="Base image for sandbox lifecycle hook images with bash, ssh, and yq runtime"

RUN apk add --no-cache openssh-client=9.9_p2-r0 bash=5.2.37-r0 && \
    wget -q https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64 -O /usr/local/bin/yq && \
    chmod a+x /usr/local/bin/yq

# Create the hooks directory
RUN mkdir -p /var/lib/p3-platform/hooks

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

WORKDIR /var/lib/p3-platform/hooks

ENTRYPOINT ["/entrypoint.sh"]
