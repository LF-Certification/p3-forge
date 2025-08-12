FROM tsl0922/ttyd:1.7.7

# Install necessary packages
USER root
RUN apt-get update -y && \
    apt-get install -y \
    tmux \
    openssh-client \
    jq \
    && rm -rf /var/lib/apt/lists/*

# Create user directory for UID 1000
RUN mkdir -p /home/user/.ssh && \
    chown -R 1000:1000 /home/user

# Configure tmux for the user
RUN echo "set -g status off" > /home/user/.tmux.conf && \
    chown 1000:1000 /home/user/.tmux.conf

# Copy entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Switch to UID 1000 and set HOME
USER 1000
ENV HOME=/home/user

# Set working directory
WORKDIR /home/user

ENTRYPOINT ["/entrypoint.sh"]
