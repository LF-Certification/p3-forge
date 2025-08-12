#!/bin/bash
set -e

# Set defaults for workspace
LOCAL_WORKDIR=${LOCAL_WORKDIR:-"$HOME/workspace"}

echo "Starting IDE container..."
echo "  Local Workspace: $LOCAL_WORKDIR"

# Ensure workspace directory exists
mkdir -p "$LOCAL_WORKDIR"

# Check if workspace is available (either SSHFS mounted or local)
if [ -d "$LOCAL_WORKDIR" ]; then
    echo "Workspace available at: $LOCAL_WORKDIR"

    # Check if SSHFS mount is available from sidecar
    if [ -f "$LOCAL_WORKDIR/.sshfs-status" ] && [ -f "$LOCAL_WORKDIR/.sshfs-info" ]; then
        echo "SSHFS sidecar detected, setting up remote workspace access..."

        # Get the remote mount path from sidecar
        REMOTE_PATH=$(grep "remote-path:" "$LOCAL_WORKDIR/.sshfs-info" | cut -d' ' -f2)

        if [ -d "$REMOTE_PATH" ]; then
            echo "Creating symlinks to SSHFS-mounted remote workspace at $REMOTE_PATH"

            # Create symlinks for all remote files/directories
            for item in "$REMOTE_PATH"/* "$REMOTE_PATH"/.*; do
                # Skip . and .. and non-existent globs
                if [ "$item" = "$REMOTE_PATH/*" ] || [ "$item" = "$REMOTE_PATH/.*" ]; then
                    continue
                fi
                basename_item=$(basename "$item")
                if [ "$basename_item" != "." ] && [ "$basename_item" != ".." ]; then
                    # Remove local file/dir if it exists to avoid conflicts
                    rm -rf "$LOCAL_WORKDIR/$basename_item" 2>/dev/null
                    # Create symlink
                    ln -sf "$item" "$LOCAL_WORKDIR/$basename_item"
                fi
            done

            echo "Using SSHFS-mounted remote workspace via symlinks"
        else
            echo "Warning: SSHFS mount path $REMOTE_PATH not found"
        fi
    else
        echo "Using local workspace"
    fi

    # Show workspace contents if available
    if [ "$(ls -A $LOCAL_WORKDIR 2>/dev/null)" ]; then
        echo "Workspace contents:"
        ls -la "$LOCAL_WORKDIR" | head -10
    else
        echo "Workspace is empty - ready for development"
    fi
else
    echo "Warning: Unable to access workspace directory"
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
