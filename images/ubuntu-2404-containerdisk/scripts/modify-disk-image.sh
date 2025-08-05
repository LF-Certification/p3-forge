#!/bin/bash
set -euo pipefail
# Enable verbose debugging
set -x

echo "=== Downloading and modifying Ubuntu 24.04 cloud image ==="
echo "DEBUG: Working directory: $(pwd)"
echo "DEBUG: Environment variables:"
echo " WORK_DIR=${WORK_DIR:-unset}"
echo " USER=$(whoami)"
echo " UID=$(id -u)"
echo " PATH=$PATH"

# Create workspace
mkdir -p "${WORK_DIR}"
cd "${WORK_DIR}"
echo "DEBUG: Changed to work directory: $(pwd)"

# Official Ubuntu 24.04 LTS cloud image URL
CLOUD_IMAGE_URL="https://cloud-images.ubuntu.com/releases/noble/release/ubuntu-24.04-server-cloudimg-amd64.img"
CLOUD_IMAGE_NAME="ubuntu-24.04-server-cloudimg-amd64.img"
CHECKSUM_URL="https://cloud-images.ubuntu.com/releases/noble/release/SHA256SUMS"

# Check if image is in cache
CACHE_DIR="$HOME/.cache/ubuntu-cloud-images"
CACHE_FILE="$CACHE_DIR/$CLOUD_IMAGE_NAME"
mkdir -p "$CACHE_DIR"

if [ -f "$CACHE_FILE" ]; then
  echo "Using cached cloud image: $CACHE_FILE"
  cp "$CACHE_FILE" "./base-disk.qcow2"
else
  echo "Downloading official Ubuntu 24.04 cloud image..."
  echo "URL: $CLOUD_IMAGE_URL"
  # Download the image
  curl -L -o "./base-disk.qcow2" "$CLOUD_IMAGE_URL"
  # Download and verify checksum
  echo "Downloading checksums for verification..."
  curl -L -o "SHA256SUMS" "$CHECKSUM_URL"
  # Verify checksum
  echo "Verifying image integrity..."
  if command -v sha256sum >/dev/null 2>&1; then
    EXPECTED_CHECKSUM=$(grep "$CLOUD_IMAGE_NAME" SHA256SUMS | cut -d' ' -f1)
    ACTUAL_CHECKSUM=$(sha256sum base-disk.qcow2 | cut -d' ' -f1)
    if [ "$EXPECTED_CHECKSUM" = "$ACTUAL_CHECKSUM" ]; then
      echo "✅ Checksum verification passed"
    else
      echo "❌ ERROR: Checksum verification failed"
      echo "Expected: $EXPECTED_CHECKSUM"
      echo "Actual: $ACTUAL_CHECKSUM"
      exit 1
    fi
  else
    echo "⚠️ WARNING: sha256sum not available, skipping checksum verification"
  fi
  # Save to cache for next time
  echo "Saving image to cache..."
  cp "./base-disk.qcow2" "$CACHE_FILE"
  rm -f "SHA256SUMS"
fi

# Verify downloaded image
if [ ! -f "base-disk.qcow2" ]; then
  echo "ERROR: Failed to download base-disk.qcow2"
  exit 1
fi
echo "✅ Ubuntu cloud image downloaded successfully"
qemu-img info base-disk.qcow2
ls -lh base-disk.qcow2

# Verify modification prerequisites
if [ ! -f "../overlay-initramfs-script.sh" ]; then
  echo "ERROR: overlay-initramfs-script.sh not found"
  exit 1
fi

# Install dependencies (for GHA)
echo "Installing dependencies for qemu-nbd..."
sudo apt-get update
sudo apt-get install -y qemu-utils kpartx

# Setup cleanup function
cleanup_nbd() {
  echo "Cleaning up NBD connections..."
  sudo umount mnt/ 2>/dev/null || true
  sudo qemu-nbd --disconnect /dev/nbd0 2>/dev/null || true
}
cleanup_chroot() {
  echo "Cleaning up chroot bind mounts..."
  sudo umount mnt/proc 2>/dev/null || true
  sudo umount mnt/sys 2>/dev/null || true
  sudo umount mnt/dev 2>/dev/null || true
  sudo umount mnt/run 2>/dev/null || true
}
trap 'cleanup_chroot; cleanup_nbd' EXIT ERR

# Create mount directory
mkdir -p mnt

# Connect NBD device
echo "Connecting NBD device..."
sudo modprobe nbd max_part=8
sudo qemu-nbd --connect=/dev/nbd0 base-disk.qcow2 || {
  echo "ERROR: NBD connect failed"
  exit 1
}

# Poll for partitions
echo "Waiting for partition to be ready..."
for i in {1..30}; do
  if [ -b /dev/nbd0p1 ]; then
    echo "✅ Partition /dev/nbd0p1 is ready"
    break
  fi
  echo "Attempt $i/30: Waiting for /dev/nbd0p1..."
  sleep 1
done
if [ ! -b /dev/nbd0p1 ]; then
  echo "ERROR: Partition /dev/nbd0p1 not ready after 30 seconds"
  cleanup_nbd
  exit 1
fi

# Mount the filesystem
echo "Mounting filesystem..."
sudo mount /dev/nbd0p1 mnt/ || {
  echo "ERROR: Mount failed"
  cleanup_nbd
  exit 1
}
echo "✅ Filesystem mounted successfully"

# Verify mount and show filesystem info
mountpoint mnt/
df -h mnt/

# Copy overlay script to initramfs location
echo "Installing overlay initramfs script..."
echo "DEBUG: Source script info:"
ls -la ../overlay-initramfs-script.sh
echo "DEBUG: Target directory info:"
sudo ls -la mnt/usr/share/initramfs-tools/scripts/init-bottom/
sudo cp ../overlay-initramfs-script.sh mnt/usr/share/initramfs-tools/scripts/init-bottom/
sudo chmod +x mnt/usr/share/initramfs-tools/scripts/init-bottom/overlay-initramfs-script.sh

# Verify script installation
echo "Verifying script installation..."
sudo ls -la mnt/usr/share/initramfs-tools/scripts/init-bottom/overlay-initramfs-script.sh
echo "DEBUG: Script content preview:"
sudo head -10 mnt/usr/share/initramfs-tools/scripts/init-bottom/overlay-initramfs-script.sh
