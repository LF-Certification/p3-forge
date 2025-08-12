FROM codercom/code-server:4.22.0

# Install necessary packages for SSHFS and SSH
USER root
RUN apt-get update && \
    apt-get install -y \
    openssh-client \
    sshfs \
    fuse3 \
    jq \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Allow non-root users to use FUSE
RUN echo "user_allow_other" >> /etc/fuse.conf

# Create workspace directory
RUN mkdir -p /home/coder/workspace && \
    chown -R coder:coder /home/coder/workspace

# Copy entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Switch back to coder user
USER coder

# Set working directory
WORKDIR /home/coder

# Expose code-server port
EXPOSE 8080

ENTRYPOINT ["/entrypoint.sh"]
