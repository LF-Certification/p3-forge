FROM alpine:3.21
# OCI labels for package linking and metadata
LABEL org.opencontainers.image.source="https://github.com/lf-certification/p3-forge"
LABEL org.opencontainers.image.title="p3-sandbox-instructions-base"
LABEL org.opencontainers.image.description="Base image for sandbox instruction images with bash runtime"

RUN apk add --no-cache bash=5.2.37-r0

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
