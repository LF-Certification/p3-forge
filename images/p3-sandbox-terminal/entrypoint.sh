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
    SSH_COMMAND="ssh -i $SSH_IDENTITY_FILE ${SSH_USER}@${TARGET_HOST} su - ${TARGET_USER}"
fi

# Check if a tmux session named 'remote' exists
if tmux has-session -t remote 2>/dev/null; then
    # If it exists, attach to the session
    exec ttyd tmux attach-session -t remote
else
    # If it doesn't exist, create a new session with the SSH command and auto-reconnect loop
    exec ttyd tmux new-session -s remote "while true; do $SSH_COMMAND || echo 'SSH failed with exit code $?.'; echo 'Reconnecting in $RETRY_INTERVAL seconds...'; sleep $RETRY_INTERVAL; done"
fi
