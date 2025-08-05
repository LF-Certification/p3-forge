#!/bin/bash
set -euo pipefail

echo "=== Setting up build environment ==="

# Setup caching
echo "=== Setting up build cache ==="

# Create cache key based on script content
SCRIPT_HASH=$(sha256sum overlay-initramfs-script.sh | cut -d' ' -f1)
CACHE_KEY="ubuntu-2404-${SCRIPT_HASH}"

echo "Cache key: $CACHE_KEY"

# Setup cache directories
mkdir -p ~/.cache/podman-images
mkdir -p ~/.cache/qemu-images

echo "✅ Cache setup complete"

# Install required tools
echo "=== Installing required tools ==="
sudo apt-get update
sudo apt-get install -y qemu-utils podman jq libguestfs-tools

sudo modprobe nbd max_part=8
sudo modprobe dm_mod

# Verify tools
qemu-nbd --version
podman --version
jq --version

echo "✅ Tools installed successfully"

# Free up disk space and check resources
echo "=== Initial disk space ==="
df -h
echo ""
echo "=== Available memory ==="
free -h
echo ""
echo "=== Free up space ==="
# Remove unnecessary packages to free space
sudo apt-get autoremove -y
sudo apt-get autoclean
# Remove Docker containers and images if any
docker system prune -f || true
echo ""
echo "=== Post-cleanup disk space ==="
df -h

# Check minimum space requirements (10GB)
AVAILABLE_KB=$(df /tmp --output=avail | tail -1)
REQUIRED_KB=$((10 * 1024 * 1024))  # 10GB in KB
if [ "$AVAILABLE_KB" -lt "$REQUIRED_KB" ]; then
  echo "ERROR: Insufficient disk space. Available: $(($AVAILABLE_KB / 1024 / 1024))GB, Required: 10GB"
  exit 1
fi
echo "✅ Sufficient disk space available: $(($AVAILABLE_KB / 1024 / 1024))GB"
