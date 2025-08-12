#!/bin/bash
set -e

echo "Starting SSHFS container..."

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
SSHFS_UID=${SSHFS_UID:-1000}
SSHFS_GID=${SSHFS_GID:-1000}

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

# Check SSH key permissions (should already be 600 from projected volume)
key_perms=$(stat -c '%a' "$SSH_KEY_PATH" 2>/dev/null || stat -f '%Mp%Lp' "$SSH_KEY_PATH" 2>/dev/null || echo "unknown")
echo "SSH key permissions: $key_perms"
if [ "$key_perms" != "600" ] && [ "$key_perms" != "unknown" ]; then
    echo "WARNING: SSH key permissions are $key_perms, expected 600"
    # Try to fix permissions if possible (will fail on read-only filesystem)
    if ! chmod 600 "$SSH_KEY_PATH" 2>/dev/null; then
        echo "INFO: Cannot change SSH key permissions (read-only filesystem), continuing anyway"
    fi
fi

# Create mount point directory if it doesn't exist
if [ ! -d "$MOUNT_POINT" ]; then
    echo "Creating mount point directory: $MOUNT_POINT"
    mkdir -p "$MOUNT_POINT"
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

sshfs_cmd=(sshfs
    -o StrictHostKeyChecking=no
    -o UserKnownHostsFile=/dev/null
    -o IdentityFile="$SSH_KEY_PATH"
    -o allow_other
    -o default_permissions
    -o uid=$SSHFS_UID
    -o gid=$SSHFS_GID
    -o reconnect
    -o ServerAliveInterval=15
    -o ServerAliveCountMax=3
    -o compression=yes
    -o ssh_command="ssh -t $TARGET_USER@$TARGET_HOST sudo -n"
)

# Mount the remote filesystem using SSHFS
echo "Mounting remote filesystem via SSHFS..."
retry_count=0
while [ $retry_count -lt $MAX_RETRIES ]; do
    if "${sshfs_cmd[@]}" \
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

echo "SUCCESS: Remote filesystem mounted successfully at $MOUNT_POINT"

# Ensure we exit cleanly when the cluster kills the pod
cleanup() {
    echo "Received SIGTERM, unmounting SSHFS..."
    retry_count=0
    while [ $retry_count -lt 3 ]; do
        if mount | grep -q "[[:space:]]$MOUNT_POINT[[:space:]]"; then
            if umount -f "$MOUNT_POINT" 2>/tmp/umount_error; then
                echo "SSHFS unmounted successfully"
                break
            else
                retry_count=$((retry_count + 1))
                echo "Failed to unmount SSHFS, retry $retry_count/3: $(cat /tmp/umount_error)"
                sleep 2
            fi
        else
            echo "No SSHFS mount found at $MOUNT_POINT"
            break
        fi
    done
    if [ $retry_count -eq 3 ]; then
        echo "Failed to unmount SSHFS after retries"
    fi
    echo "Exiting..."
    exit 0
}
trap cleanup SIGTERM SIGINT

# Create a monitoring loop to ensure mount stays active
while true; do
    # Check if mount is still active
    if ! mount | grep -q "$MOUNT_POINT"; then
        echo "SSHFS mount lost, attempting to remount..."

        # Attempt to remount
        if "${sshfs_cmd[@]}" \
           "$TARGET_USER@$TARGET_HOST:$REMOTE_WORKDIR" \
           "$MOUNT_POINT" 2>/dev/null; then
            echo "SSHFS remount successful"
        else
            echo "SSHFS remount failed, will retry..."
        fi
    fi

    # Check every 30 seconds
    sleep 30
done
