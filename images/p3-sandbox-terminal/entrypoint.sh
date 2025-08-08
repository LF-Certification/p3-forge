#!/bin/sh

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

SSH_OPTS="-o StrictHostKeyChecking=accept-new -o BatchMode=yes -o UserKnownHostsFile=/dev/null -t"
SSH_COMMAND="ssh $SSH_OPTS ${TARGET_USER}@${TARGET_HOST}"
exec ttyd -W tmux new-session -A -s remote "while true; do $SSH_COMMAND || echo \"SSH failed with exit code \$?\"; echo \"Reconnecting in $RETRY_INTERVAL seconds...\"; sleep $RETRY_INTERVAL; done"
