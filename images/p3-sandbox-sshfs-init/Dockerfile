# Basic Dockerfile - customize as needed
FROM alpine:3.19
# OCI labels for package linking and metadata
LABEL org.opencontainers.image.source="https://github.com/lf-certification/sandbox-images"
LABEL org.opencontainers.image.title="p3-sandbox-sshfs-init"
LABEL org.opencontainers.image.description="Container image for p3/sandbox/sshfs/init"

RUN apk update && \
    apk add --no-cache ca-certificates && \
    rm -rf /var/cache/apk/*

RUN addgroup -g 1001 appuser && \
    adduser -D -u 1001 -G appuser appuser

USER appuser
WORKDIR /app

CMD ["/bin/sh"]
