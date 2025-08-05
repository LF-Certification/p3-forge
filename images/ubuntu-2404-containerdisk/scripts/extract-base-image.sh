#!/bin/bash
set -euo pipefail

echo "=== Extracting base containerdisk ==="

# Create workspace
mkdir -p "${WORK_DIR}"
cd "${WORK_DIR}"

echo "Pulling base image: ${BASE_IMAGE}"

# Check if image is in cache
CACHE_FILE="$HOME/.cache/podman-images/$(echo '${BASE_IMAGE}' | tr '/:' '_').tar"
if [ -f "$CACHE_FILE" ]; then
  echo "Loading base image from cache..."
  podman load -i "$CACHE_FILE"
else
  echo "Pulling base image from registry..."
  podman pull "${BASE_IMAGE}"

  # Save to cache for next time
  echo "Saving base image to cache..."
  mkdir -p "$(dirname "$CACHE_FILE")"
  podman save "${BASE_IMAGE}" -o "$CACHE_FILE"
fi

# Extract disk image from base containerdisk
echo "Extracting disk image from containerdisk..."
podman create --name temp-ubuntu "${BASE_IMAGE}"

# List contents to verify structure
echo "Container contents:"
podman export temp-ubuntu | tar -tv | head -20

# Extract the qcow2 disk image
podman cp temp-ubuntu:/disk/ubuntu-24.04.qcow2 ./base-disk.qcow2

# Cleanup temporary container
podman rm temp-ubuntu

# Verify extracted image
if [ ! -f "base-disk.qcow2" ]; then
  echo "ERROR: Failed to extract base-disk.qcow2"
  exit 1
fi

echo "✅ Base disk extracted successfully"
qemu-img info base-disk.qcow2
ls -lh base-disk.qcow2

# Check disk space after extraction
echo ""
echo "=== Disk space after extraction ==="
df -h
