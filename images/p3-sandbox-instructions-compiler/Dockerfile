# Hugo-based markdown compiler for static site generation
FROM alpine:3.19

# OCI labels for package linking and metadata
LABEL org.opencontainers.image.source="https://github.com/lf-certification/sandbox-images"
LABEL org.opencontainers.image.title="p3-sandbox-instructions-compiler"
LABEL org.opencontainers.image.description="Container image for compiling markdown to static sites using Hugo"

# Install dependencies and Hugo
RUN apk update && \
    apk add --no-cache \
        ca-certificates \
        git \
        curl \
        libc6-compat && \
    rm -rf /var/cache/apk/*

# Install Hugo extended version for SCSS/SASS support
ARG HUGO_VERSION=0.134.2
RUN curl -L https://github.com/gohugoio/hugo/releases/download/v${HUGO_VERSION}/hugo_extended_${HUGO_VERSION}_linux-amd64.tar.gz | tar -xz -C /tmp && \
    mv /tmp/hugo /usr/local/bin/hugo && \
    chmod +x /usr/local/bin/hugo && \
    rm -rf /tmp/*

# Create non-root user
RUN addgroup -g 1001 appuser && \
    adduser -D -u 1001 -G appuser appuser

# Copy entrypoint script
COPY --chown=appuser:appuser entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

USER appuser
WORKDIR /app

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
