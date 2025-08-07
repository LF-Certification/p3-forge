#!/bin/sh

if [ -z "$TERMINAL_CONFIG" ]; then
    echo "Error: TERMINAL_CONFIG environment variable is not set."
    exit 1
fi

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

if ! expr "$RETRY_INTERVAL" : '^[0-9]\+$' >/dev/null; then
    echo "Error: retryInterval must be a positive integer."
    exit 1
fi

if [ ! -f "$SSH_IDENTITY_FILE" ] || [ ! -r "$SSH_IDENTITY_FILE" ]; then
    echo "Error: SSH identity file '$SSH_IDENTITY_FILE' does not exist or is not readable."
    exit 1
fi

chmod 600 "$SSH_IDENTITY_FILE"

SSH_OPTS="-i $SSH_IDENTITY_FILE -o StrictHostKeyChecking=accept-new -o BatchMode=yes -o UserKnownHostsFile=/dev/null -t"
if [ "$SSH_USER" = "$TARGET_USER" ]; then
    SSH_COMMAND="ssh $SSH_OPTS ${SSH_USER}@${TARGET_HOST}"
else
    SSH_COMMAND="ssh $SSH_OPTS ${SSH_USER}@${TARGET_HOST} 'sudo sh -c \"mkdir -p ~${TARGET_USER}/.ssh && cp ~/.ssh/authorized_keys ~${TARGET_USER}/.ssh/ && chown -R ${TARGET_USER}:${TARGET_USER} ~${TARGET_USER}/.ssh && chmod 700 ~${TARGET_USER}/.ssh && chmod 600 ~${TARGET_USER}/.ssh/authorized_keys\" 2>/dev/null' ; ssh $SSH_OPTS ${TARGET_USER}@${TARGET_HOST}"
fi

exec ttyd -W tmux new-session -A -s remote "while true; do $SSH_COMMAND || echo \"SSH failed with exit code \$?\"; echo \"Reconnecting in $RETRY_INTERVAL seconds...\"; sleep $RETRY_INTERVAL; done"
