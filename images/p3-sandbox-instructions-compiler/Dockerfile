# Hugo-based markdown compiler for static site generation
FROM debian:bookworm-slim

# OCI labels for package linking and metadata
LABEL org.opencontainers.image.source="https://github.com/lf-certification/sandbox-images"
LABEL org.opencontainers.image.title="p3-sandbox-instructions-compiler"
LABEL org.opencontainers.image.description="Container image for compiling markdown to static sites using Hugo"

# Install dependencies and Hugo
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        git \
        curl && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Install Hugo extended version for SCSS/SASS support
ARG HUGO_VERSION=0.134.2
RUN curl -L https://github.com/gohugoio/hugo/releases/download/v${HUGO_VERSION}/hugo_extended_${HUGO_VERSION}_linux-amd64.tar.gz | tar -xz -C /tmp && \
    mv /tmp/hugo /usr/local/bin/hugo && \
    chmod +x /usr/local/bin/hugo && \
    rm -rf /tmp/*

# Create non-root user
RUN groupadd -g 1001 appuser && \
    useradd -u 1001 -g appuser -m appuser

# Copy entrypoint script
COPY --chown=appuser:appuser entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

USER appuser
WORKDIR /app

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
