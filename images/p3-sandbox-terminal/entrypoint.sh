#!/bin/sh
set -e

if [ -z "$TERMINAL_CONFIG" ]; then
    echo "Error: TERMINAL_CONFIG environment variable is not set."
    exit 1
fi

TARGET_HOST=$(echo "$TERMINAL_CONFIG" | jq -r '.targetHost')
TARGET_USER=$(echo "$TERMINAL_CONFIG" | jq -r '.targetUser')
RETRY_INTERVAL=$(echo "$TERMINAL_CONFIG" | jq -r '.retryInterval // "5"')

# Validate parsed values
if [ -z "$TARGET_HOST" ] || [ -z "$TARGET_USER" ]; then
    echo "Error: Invalid TERMINAL_CONFIG JSON. Must contain non-null 'targetHost' and 'targetUser'."
    exit 1
fi

if ! expr "$RETRY_INTERVAL" : '^[0-9]\+$' >/dev/null; then
    echo "Error: retryInterval must be a positive integer."
    exit 1
fi

echo "Starting Terminal container with the following configuration:"
echo "  Target Host: $TARGET_HOST"
echo "  Target User: $TARGET_USER"
echo "  Retry Interval: $RETRY_INTERVAL seconds"

echo "Debug information:"
echo "  Current user: $(whoami)"
echo "  Current UID: $(id -u)"
echo "  Current GID: $(id -g)"
echo "  Current groups: $(id -G)"
echo "  Home directory: $HOME"
echo "  SSH directory: ~/.ssh"
echo "  SSH directory exists: $([ -d ~/.ssh ] && echo 'yes' || echo 'no')"

# Create SSH directory if it doesn't exist and set proper permissions
mkdir -p ~/.ssh

if [ -d ~/.ssh ]; then
    echo "  SSH directory permissions: $(ls -ld ~/.ssh)"
    echo "  SSH directory owner: $(stat -c '%U:%G' ~/.ssh 2>/dev/null || stat -f '%Su:%Sg' ~/.ssh 2>/dev/null || echo 'unknown')"
    echo "  SSH directory file permissions: $(ls -al ~/.ssh)"
fi

SSH_OPTS="-o StrictHostKeyChecking=accept-new -o BatchMode=yes -o UserKnownHostsFile=/dev/null -t"
SSH_COMMAND="ssh $SSH_OPTS ${TARGET_USER}@${TARGET_HOST}"

echo "Starting ttyd with auto-reconnecting SSH session..."
echo "SSH command: $SSH_COMMAND"

exec ttyd -W tmux new-session -A -s remote "while true; do $SSH_COMMAND || echo \"SSH failed with exit code \$?\"; echo \"Reconnecting in $RETRY_INTERVAL seconds...\"; sleep $RETRY_INTERVAL; done"
