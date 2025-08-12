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

# Set defaults for optional variables
WORKSPACE_DIR=${WORKSPACE_DIR:-"/home/$TARGET_USER"}
PASSWORD=${PASSWORD:-"password"}
SSHFS_MOUNT_POINT="/home/coder/workspace"

echo "Starting IDE container with the following configuration:"
echo "  Target Host: $TARGET_HOST"
echo "  Target User: $TARGET_USER"
echo "  Remote Workspace: $WORKSPACE_DIR"
echo "  Local Mount Point: $SSHFS_MOUNT_POINT"

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
fi

# Create SSH directory if it doesn't exist
mkdir -p ~/.ssh

# Fix ownership and permissions using sudo (needed for Kubernetes-mounted files)
sudo chown -R coder:coder ~/.ssh
sudo chmod 700 ~/.ssh

# Set proper permissions for SSH key if it exists
if [ -f ~/.ssh/id_rsa ]; then
    chmod 600 ~/.ssh/id_rsa
    echo "Found SSH private key at ~/.ssh/id_rsa"
else
    echo "Warning: No SSH private key found at ~/.ssh/id_rsa"
fi

# Set proper permissions for SSH config if it exists
if [ -f ~/.ssh/config ]; then
    chmod 600 ~/.ssh/config
    echo "Found SSH config at ~/.ssh/config"
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

# Function to mount SSHFS
mount_sshfs() {
    echo "Mounting remote directory $WORKSPACE_DIR via SSHFS..."

    # Ensure mount point exists and is empty
    mkdir -p "$SSHFS_MOUNT_POINT"

    # Check if already mounted
    if mountpoint -q "$SSHFS_MOUNT_POINT"; then
        echo "SSHFS already mounted at $SSHFS_MOUNT_POINT"
        return 0
    fi

    # Mount with SSHFS
    sshfs -o reconnect,ServerAliveInterval=15,ServerAliveCountMax=3 \
          -o StrictHostKeyChecking=accept-new \
          -o UserKnownHostsFile=/dev/null \
          -o allow_other \
          "$TARGET_USER@$TARGET_HOST:$WORKSPACE_DIR" \
          "$SSHFS_MOUNT_POINT"

    if [ $? -eq 0 ]; then
        echo "Successfully mounted $TARGET_USER@$TARGET_HOST:$WORKSPACE_DIR to $SSHFS_MOUNT_POINT"
        return 0
    else
        echo "Failed to mount SSHFS"
        return 1
    fi
}

# Function to cleanup on exit
cleanup() {
    echo "Cleaning up..."
    if mountpoint -q "$SSHFS_MOUNT_POINT"; then
        echo "Unmounting SSHFS..."
        fusermount -u "$SSHFS_MOUNT_POINT" || umount "$SSHFS_MOUNT_POINT" || true
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

# Mount SSHFS with retries
MOUNT_RETRIES=5
MOUNT_COUNT=0
while [ $MOUNT_COUNT -lt $MOUNT_RETRIES ]; do
    if mount_sshfs; then
        echo "SSHFS mounted successfully"
        break
    else
        MOUNT_COUNT=$((MOUNT_COUNT + 1))
        echo "SSHFS mount failed (attempt $MOUNT_COUNT/$MOUNT_RETRIES). Retrying in 3 seconds..."
        sleep 3
    fi
done

if [ $MOUNT_COUNT -eq $MOUNT_RETRIES ]; then
    echo "Error: Unable to mount SSHFS after $MOUNT_RETRIES attempts"
    echo "Starting code-server without remote filesystem access..."
    # Continue without SSHFS mount - user can still access local filesystem
fi

# Verify mount
if mountpoint -q "$SSHFS_MOUNT_POINT"; then
    echo "SSHFS mount verification successful"
    echo "Remote workspace content:"
    ls -la "$SSHFS_MOUNT_POINT" | head -10
else
    echo "Warning: SSHFS mount not available. Code-server will start with local workspace only."
fi

# Start code-server
echo "Starting code-server..."
exec code-server \
    --bind-addr 0.0.0.0:8080 \
    --auth password \
    --password "$PASSWORD" \
    --disable-telemetry \
    "$SSHFS_MOUNT_POINT"
