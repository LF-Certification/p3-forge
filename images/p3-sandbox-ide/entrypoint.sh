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
    if [ -f "$LOCAL_WORKDIR/.sshfs-status" ]; then
        echo "Using SSHFS-mounted remote workspace"
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
