#!/bin/bash
set -euo pipefail

echo "=== Downloading official Ubuntu cloud image ==="

# Create workspace
mkdir -p "${WORK_DIR}"
cd "${WORK_DIR}"

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
      echo "Actual:   $ACTUAL_CHECKSUM"
      exit 1
    fi
  else
    echo "⚠️  WARNING: sha256sum not available, skipping checksum verification"
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

# Check disk space after download
echo ""
echo "=== Disk space after download ==="
df -h
