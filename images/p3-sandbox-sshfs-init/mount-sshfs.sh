#!/bin/bash
set -e

# SSHFS Init Container Mount Script
# This script mounts a remote filesystem via SSHFS in a privileged init container
# The mounted filesystem is then accessible to the main container via shared volume

echo "Starting SSHFS init container..."

# Validate required environment variables
if [ -z "$TARGET_HOST" ]; then
    echo "ERROR: TARGET_HOST environment variable is required"
    exit 1
fi

if [ -z "$TARGET_USER" ]; then
    echo "ERROR: TARGET_USER environment variable is required"
    exit 1
fi

if [ -z "$REMOTE_WORKDIR" ]; then
    echo "ERROR: REMOTE_WORKDIR environment variable is required"
    exit 1
fi

# Set defaults
MOUNT_POINT=${MOUNT_POINT:-"/workspace"}
SSH_KEY_PATH=${SSH_KEY_PATH:-"/home/coder/.ssh/id_rsa"}
SSH_CONFIG_PATH=${SSH_CONFIG_PATH:-"/home/coder/.ssh/config"}
MAX_RETRIES=${MAX_RETRIES:-10}
RETRY_DELAY=${RETRY_DELAY:-5}

echo "Configuration:"
echo "  Target: $TARGET_USER@$TARGET_HOST:$REMOTE_WORKDIR"
echo "  Mount Point: $MOUNT_POINT"
echo "  SSH Key: $SSH_KEY_PATH"
echo "  SSH Config: $SSH_CONFIG_PATH"

# Validate SSH key exists and has proper permissions
if [ ! -f "$SSH_KEY_PATH" ]; then
    echo "ERROR: SSH key not found at $SSH_KEY_PATH"
    exit 1
fi

# Set proper SSH key permissions (important for SSHFS)
chmod 600 "$SSH_KEY_PATH"

# Validate mount point exists
if [ ! -d "$MOUNT_POINT" ]; then
    echo "ERROR: Mount point $MOUNT_POINT does not exist"
    exit 1
fi

# Test SSH connectivity first
echo "Testing SSH connectivity..."
retry_count=0
while [ $retry_count -lt $MAX_RETRIES ]; do
    if ssh -o ConnectTimeout=10 \
           -o StrictHostKeyChecking=no \
           -o UserKnownHostsFile=/dev/null \
           -o BatchMode=yes \
           -i "$SSH_KEY_PATH" \
           "$TARGET_USER@$TARGET_HOST" \
           "echo 'SSH connection successful'" 2>/dev/null; then
        echo "SSH connection established successfully"
        break
    else
        retry_count=$((retry_count + 1))
        echo "SSH connection attempt $retry_count/$MAX_RETRIES failed. Retrying in $RETRY_DELAY seconds..."
        if [ $retry_count -eq $MAX_RETRIES ]; then
            echo "ERROR: Unable to establish SSH connection after $MAX_RETRIES attempts"
            echo "Please verify:"
            echo "  1. Target host $TARGET_HOST is reachable"
            echo "  2. SSH service is running on target host"
            echo "  3. SSH key is valid and authorized"
            echo "  4. Target user $TARGET_USER exists"
            exit 1
        fi
        sleep $RETRY_DELAY
    fi
done

# Verify remote directory exists
echo "Verifying remote directory $REMOTE_WORKDIR exists..."
if ! ssh -o ConnectTimeout=10 \
         -o StrictHostKeyChecking=no \
         -o UserKnownHostsFile=/dev/null \
         -o BatchMode=yes \
         -i "$SSH_KEY_PATH" \
         "$TARGET_USER@$TARGET_HOST" \
         "test -d '$REMOTE_WORKDIR'" 2>/dev/null; then
    echo "WARNING: Remote directory $REMOTE_WORKDIR does not exist. Creating it..."
    if ! ssh -o ConnectTimeout=10 \
             -o StrictHostKeyChecking=no \
             -o UserKnownHostsFile=/dev/null \
             -o BatchMode=yes \
             -i "$SSH_KEY_PATH" \
             "$TARGET_USER@$TARGET_HOST" \
             "mkdir -p '$REMOTE_WORKDIR'" 2>/dev/null; then
        echo "ERROR: Failed to create remote directory $REMOTE_WORKDIR"
        exit 1
    fi
    echo "Remote directory created successfully"
fi

# Mount the remote filesystem using SSHFS
echo "Mounting remote filesystem via SSHFS..."
retry_count=0
while [ $retry_count -lt $MAX_RETRIES ]; do
    if sshfs -o StrictHostKeyChecking=no \
             -o UserKnownHostsFile=/dev/null \
             -o IdentityFile="$SSH_KEY_PATH" \
             -o allow_other \
             -o default_permissions \
             -o uid=1000 \
             -o gid=1000 \
             -o umask=022 \
             -o reconnect \
             -o ServerAliveInterval=15 \
             -o ServerAliveCountMax=3 \
             "$TARGET_USER@$TARGET_HOST:$REMOTE_WORKDIR" \
             "$MOUNT_POINT" 2>/dev/null; then
        echo "SSHFS mount successful"
        break
    else
        retry_count=$((retry_count + 1))
        echo "SSHFS mount attempt $retry_count/$MAX_RETRIES failed. Retrying in $RETRY_DELAY seconds..."
        if [ $retry_count -eq $MAX_RETRIES ]; then
            echo "ERROR: Unable to mount remote filesystem after $MAX_RETRIES attempts"
            exit 1
        fi
        sleep $RETRY_DELAY
    fi
done

# Verify mount was successful
if mount | grep -q "$MOUNT_POINT"; then
    echo "SSHFS mount verified successfully"
    echo "Mount details:"
    mount | grep "$MOUNT_POINT"

    # List contents to verify access
    echo "Remote directory contents:"
    ls -la "$MOUNT_POINT" | head -10
else
    echo "ERROR: SSHFS mount verification failed"
    exit 1
fi

# Create a marker file to indicate successful mount
echo "sshfs-mounted" > "$MOUNT_POINT/.sshfs-status"
echo "SUCCESS: Remote filesystem mounted successfully at $MOUNT_POINT"

# Keep the container running briefly to ensure mount is stable
echo "Waiting 5 seconds to ensure mount stability..."
sleep 5

echo "SSHFS init container completed successfully"
