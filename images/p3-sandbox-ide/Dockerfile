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


# Create user directory and workspace for UID 1000
RUN mkdir -p /home/user/.ssh /home/user/workspace && \
    chown -R 1000:1000 /home/user

# Copy entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Switch to UID 1000 and set HOME
USER 1000
ENV HOME=/home/user

# Set working directory
WORKDIR /home/user

# Expose code-server port
EXPOSE 8080

ENTRYPOINT ["/entrypoint.sh"]
