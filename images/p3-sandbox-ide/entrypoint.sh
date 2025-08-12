#!/bin/bash
set -e

# Check required environment variables
if [ -z "$TARGET_HOST" ]; then
    echo "Error: TARGET_HOST environment variable is not set."
    exit 1
fi

if [ -z "$TARGET_USER" ]; then
    echo "Error: TARGET_USER environment variable is not set."
    exit 1
fi

if [ -z "$REMOTE_WORKDIR" ]; then
    echo "Error: REMOTE_WORKDIR environment variable is not set."
    exit 1
fi

# Set defaults for optional variables
LOCAL_WORKDIR=${LOCAL_WORKDIR:-"$HOME/workspace"}

echo "Starting IDE container with the following configuration:"
echo "  Target Host: $TARGET_HOST"
echo "  Target User: $TARGET_USER"
echo "  Remote Workspace: $REMOTE_WORKDIR"
echo "  Local Workspace: $LOCAL_WORKDIR"

echo "Debug information:"
echo "  Current user: $(whoami)"
echo "  Current UID: $(id -u)"
echo "  Current GID: $(id -g)"
echo "  Current groups: $(id -G)"
echo "  Home directory: $HOME"
echo "  SSH directory: ~/.ssh"
echo "  SSH directory exists: $([ -d ~/.ssh ] && echo 'yes' || echo 'no')"
if [ -d ~/.ssh ]; then
    echo "  SSH directory permissions: $(ls -ld ~/.ssh)"
    echo "  SSH directory owner: $(stat -c '%U:%G' ~/.ssh 2>/dev/null || stat -f '%Su:%Sg' ~/.ssh 2>/dev/null || echo 'unknown')"
    echo "  SSH directory file permissions: $(ls -al ~/.ssh)"
fi

# Function to test SSH connectivity
test_ssh_connection() {
    echo "Testing SSH connection to $TARGET_USER@$TARGET_HOST..."
    ssh -o ConnectTimeout=10 \
        -o StrictHostKeyChecking=accept-new \
        -o BatchMode=yes \
        -o UserKnownHostsFile=/dev/null \
        "$TARGET_USER@$TARGET_HOST" \
        "echo 'SSH connection successful'"
}

# Alternative approach: Use SSH with rsync for file sync
sync_remote_files() {
    echo "Using rsync for file synchronization..."

    # Initial sync from remote to local
    echo "Syncing remote files to local workspace..."
    rsync -avz --delete \
        -e "ssh -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=/dev/null" \
        "$TARGET_USER@$TARGET_HOST:$REMOTE_WORKDIR/" \
        "$LOCAL_WORKDIR/"

    if [ $? -eq 0 ]; then
        echo "Initial sync completed successfully"

        # Set up periodic sync in background
        start_sync_daemon
        return 0
    else
        echo "Initial sync failed"
        return 1
    fi
}

# Background sync daemon
start_sync_daemon() {
    echo "Starting background sync daemon..."

    # Create sync script
    cat > /tmp/sync_daemon.sh << 'EOF'
#!/bin/bash
SYNC_INTERVAL=${SYNC_INTERVAL:-30}  # seconds
REMOTE_DIR="$1"
LOCAL_DIR="$2"
TARGET="$3"

while true; do
    # Sync remote changes to local
    rsync -avz --delete --timeout=10 \
        -e "ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=/dev/null" \
        "$TARGET:$REMOTE_DIR/" "$LOCAL_DIR/" 2>/dev/null

    # Sync local changes to remote (be careful with --delete here)
    rsync -avz --timeout=10 \
        -e "ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=/dev/null" \
        "$LOCAL_DIR/" "$TARGET:$REMOTE_DIR/" 2>/dev/null

    sleep $SYNC_INTERVAL
done
EOF

    chmod +x /tmp/sync_daemon.sh

    # Start daemon in background
    nohup /tmp/sync_daemon.sh "$REMOTE_WORKDIR" "$LOCAL_WORKDIR" "$TARGET_USER@$TARGET_HOST" > /tmp/sync.log 2>&1 &
    SYNC_PID=$!
    echo $SYNC_PID > /tmp/sync_daemon.pid
    echo "Sync daemon started with PID $SYNC_PID"
    echo "Sync log: /tmp/sync.log"
}

# Function to sync files using rsync instead of SSHFS
mount_sshfs() {
    echo "Syncing remote directory $REMOTE_WORKDIR via rsync..."

    # Ensure mount point exists
    mkdir -p "$LOCAL_WORKDIR"

    # Use rsync-based synchronization
    sync_remote_files
    return $?
}

# Function to cleanup on exit
cleanup() {
    echo "Cleaning up..."

    # Stop sync daemon if running
    if [ -f /tmp/sync_daemon.pid ]; then
        SYNC_PID=$(cat /tmp/sync_daemon.pid)
        if kill -0 $SYNC_PID 2>/dev/null; then
            echo "Stopping sync daemon (PID: $SYNC_PID)..."
            kill $SYNC_PID
            # Final sync before exit
            echo "Performing final sync..."
            rsync -avz --timeout=10 \
                -e "ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=/dev/null" \
                "$LOCAL_WORKDIR/" "$TARGET_USER@$TARGET_HOST:$REMOTE_WORKDIR/" 2>/dev/null
        fi
        rm -f /tmp/sync_daemon.pid
    fi
}

# Set up cleanup trap
trap cleanup EXIT

# Wait for SSH connectivity with retries
MAX_RETRIES=30
RETRY_COUNT=0
while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if test_ssh_connection; then
        echo "SSH connection established successfully"
        break
    else
        RETRY_COUNT=$((RETRY_COUNT + 1))
        echo "SSH connection failed (attempt $RETRY_COUNT/$MAX_RETRIES). Retrying in 5 seconds..."
        sleep 5
    fi
done

if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
    echo "Error: Unable to establish SSH connection after $MAX_RETRIES attempts"
    echo "Please verify:"
    echo "  1. Target host $TARGET_HOST is reachable"
    echo "  2. SSH service is running on the target host"
    echo "  3. SSH keys are properly configured"
    echo "  4. Target user $TARGET_USER exists and has proper permissions"
    exit 1
fi

# Sync remote files with retries
SYNC_RETRIES=5
SYNC_COUNT=0
while [ $SYNC_COUNT -lt $SYNC_RETRIES ]; do
    if mount_sshfs; then
        echo "Remote file sync established successfully"
        break
    else
        SYNC_COUNT=$((SYNC_COUNT + 1))
        echo "Remote sync failed (attempt $SYNC_COUNT/$SYNC_RETRIES). Retrying in 3 seconds..."
        sleep 3
    fi
done

if [ $SYNC_COUNT -eq $SYNC_RETRIES ]; then
    echo "Error: Unable to sync remote files after $SYNC_RETRIES attempts"
    echo "Starting code-server without remote filesystem access..."
    # Continue without remote sync - user can still access local filesystem
fi

# Verify sync
if [ -d "$LOCAL_WORKDIR" ] && [ "$(ls -A $LOCAL_WORKDIR 2>/dev/null)" ]; then
    echo "Remote file sync verification successful"
    echo "Remote workspace content:"
    ls -la "$LOCAL_WORKDIR" | head -10
else
    echo "Warning: Remote file sync not available. Code-server will start with local workspace only."
fi

# Start code-server
echo "Starting code-server..."
exec code-server \
    --bind-addr 0.0.0.0:8080 \
    --auth none \
    --disable-telemetry \
    --disable-update-check \
    --disable-getting-started-override \
    "$LOCAL_WORKDIR"
