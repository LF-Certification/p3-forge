FROM nginxinc/nginx-unprivileged:alpine
# OCI labels for package linking and metadata
LABEL org.opencontainers.image.source="https://github.com/lf-certification/sandbox-images"
LABEL org.opencontainers.image.title="p3-sandbox-instructions-server"
LABEL org.opencontainers.image.description="Container image for p3/sandbox/instructions/server"

# Switch to root to set up permissions
USER root

# Copy entrypoint script
COPY entrypoint.sh /entrypoint.sh

# Set up permissions for UID 1000
# The unprivileged image already has /tmp paths configured for nginx temp files
RUN chmod +x /entrypoint.sh && \
    chown -R 1000:0 /usr/share/nginx/html && \
    chmod -R g+rwX /usr/share/nginx/html && \
    chown -R 1000:0 /var/cache/nginx && \
    chmod -R g+rwX /var/cache/nginx && \
    chown -R 1000:0 /etc/nginx && \
    chmod -R g+rwX /etc/nginx

# Switch to UID 1000
USER 1000

EXPOSE 8080

ENTRYPOINT ["/entrypoint.sh"]
