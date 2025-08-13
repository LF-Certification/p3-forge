#!/bin/bash
set -e

if [ -z "$LOCAL_WORKDIR" ]; then
    echo "ERROR: LOCAL_WORKDIR environment variable is required"
    exit 1
fi

echo "Starting IDE container..."
echo "Local Workspace: $LOCAL_WORKDIR"

# Wait for workspace directory to exist
echo "Waiting for workspace directory to be available..."
while [ ! -d "$LOCAL_WORKDIR" ]; do
    echo "Waiting for $LOCAL_WORKDIR to exist..."
    sleep 1
done
echo "Workspace contents:"
ls -la "$LOCAL_WORKDIR" | head -10

echo "Starting code-server..."
exec code-server --config=/etc/code-server.yaml "$LOCAL_WORKDIR"
