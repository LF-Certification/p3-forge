#!/bin/bash
set -e

LOCAL_WORKDIR=${LOCAL_WORKDIR:-"$HOME/workspace"}

echo "Starting IDE container..."
echo "Local Workspace: $LOCAL_WORKDIR"

# Ensure workspace directory exists
mkdir -p "$LOCAL_WORKDIR"
echo "Workspace contents:"
ls -la "$LOCAL_WORKDIR" | head -10

echo "Starting code-server..."
exec code-server \
    --bind-addr 0.0.0.0:8080 \
    --auth none \
    --disable-telemetry \
    --disable-update-check \
    --disable-getting-started-override \
    "$LOCAL_WORKDIR"
