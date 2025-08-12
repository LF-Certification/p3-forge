FROM codercom/code-server:4.22.0

# Install necessary packages for SSH and rsync
USER root
RUN apt-get update && \
    apt-get install -y \
    openssh-client \
    rsync \
    jq \
    curl \
    && rm -rf /var/lib/apt/lists/*


# Copy entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Expose code-server port
EXPOSE 8080

ENTRYPOINT ["/entrypoint.sh"]
