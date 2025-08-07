#!/bin/sh

# Check if TERMINAL_CONFIG is set
if [ -z "$TERMINAL_CONFIG" ]; then
    echo "Error: TERMINAL_CONFIG environment variable is not set."
    exit 1
fi

# Parse the JSON using jq
TARGET_HOST=$(echo "$TERMINAL_CONFIG" | jq -r '.targetHost')
TARGET_USER=$(echo "$TERMINAL_CONFIG" | jq -r '.targetUser')
SSH_USER=$(echo "$TERMINAL_CONFIG" | jq -r '.sshUser')
SSH_IDENTITY_FILE=$(echo "$TERMINAL_CONFIG" | jq -r '.sshIdentityFile')
RETRY_INTERVAL=$(echo "$TERMINAL_CONFIG" | jq -r '.retryInterval // "5"')

# Validate parsed values
if [ -z "$TARGET_HOST" ] || [ -z "$TARGET_USER" ] || [ -z "$SSH_USER" ] || [ -z "$SSH_IDENTITY_FILE" ] || [ "$SSH_USER" = "null" ] || [ "$SSH_IDENTITY_FILE" = "null" ]; then
    echo "Error: Invalid TERMINAL_CONFIG JSON. Must contain non-null 'targetHost', 'targetUser', 'sshUser', and 'sshIdentityFile'."
    exit 1
fi

# Validate SSH identity file
if [ ! -f "$SSH_IDENTITY_FILE" ] || [ ! -r "$SSH_IDENTITY_FILE" ]; then
    echo "Error: SSH identity file '$SSH_IDENTITY_FILE' does not exist or is not readable."
    exit 1
fi

# Build the SSH command, skipping su if sshUser equals targetUser
if [ "$SSH_USER" = "$TARGET_USER" ]; then
    SSH_COMMAND="ssh -i $SSH_IDENTITY_FILE ${SSH_USER}@${TARGET_HOST}"
else
    SSH_COMMAND="ssh -i $SSH_IDENTITY_FILE ${SSH_USER}@${TARGET_HOST} sudo su - ${TARGET_USER}"
fi

# Run ttyd with tmux new-session -A (attach if exists, else create) and the auto-reconnect loop as the session command
exec ttyd -W tmux new-session -A -s remote "while true; do $SSH_COMMAND || echo 'SSH failed with exit code \$?.'; echo 'Reconnecting in $RETRY_INTERVAL seconds...'; sleep $RETRY_INTERVAL; done"
