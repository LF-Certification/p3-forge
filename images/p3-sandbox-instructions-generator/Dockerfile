# MkDocs-based markdown compiler for static site generation
FROM python:3.12-slim-bookworm

# OCI labels for package linking and metadata
LABEL org.opencontainers.image.source="https://github.com/lf-certification/sandbox-images"
LABEL org.opencontainers.image.title="p3-sandbox-instructions-generator"
LABEL org.opencontainers.image.description="Container image for compiling markdown to static sites using MkDocs"

# Install dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        git && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Install Poetry
RUN pip install --no-cache-dir poetry==1.8.3

# Copy dependency files
COPY pyproject.toml poetry.lock ./

# Install dependencies from pyproject.toml
RUN poetry config virtualenvs.create false && \
    poetry install --no-interaction --no-ansi --no-root

# Create non-root user
RUN groupadd -g 1001 appuser && \
    useradd -u 1001 -g appuser -m appuser

# Copy MkDocs config and overrides
COPY --chown=appuser:appuser mkdocs /mkdocs

# Copy entrypoint script
COPY --chown=appuser:appuser entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

USER appuser
WORKDIR /app

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
