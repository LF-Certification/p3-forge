#!/bin/bash
set -euo pipefail

echo "=== Downloading Ubuntu 24.04 cloud image ==="

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Configuration
UBUNTU_BASE_URL="https://cloud-images.ubuntu.com/noble/current"
IMAGE_FILENAME="noble-server-cloudimg-amd64.img"
CHECKSUM_FILENAME="SHA256SUMS"
CACHE_DIR="$HOME/.cache/qemu-images"

# Ensure WORK_DIR is absolute
if [[ "${WORK_DIR:-build-workspace}" = /* ]]; then
    WORK_DIR="${WORK_DIR:-build-workspace}"
else
    WORK_DIR="${PROJECT_DIR}/${WORK_DIR:-build-workspace}"
fi

# Create necessary directories
mkdir -p "${CACHE_DIR}"
mkdir -p "${WORK_DIR}"

# URLs for download
IMAGE_URL="${UBUNTU_BASE_URL}/${IMAGE_FILENAME}"
CHECKSUM_URL="${UBUNTU_BASE_URL}/${CHECKSUM_FILENAME}"

# File paths
CACHED_IMAGE="${CACHE_DIR}/${IMAGE_FILENAME}"
CACHED_CHECKSUM="${CACHE_DIR}/${CHECKSUM_FILENAME}"
WORKSPACE_IMAGE="${WORK_DIR}/base-disk.qcow2"

# Cleanup function
cleanup() {
    local exit_code=$?
    if [ $exit_code -ne 0 ]; then
        echo "ERROR: Download failed, cleaning up partial files..."
        rm -f "${CACHED_IMAGE}.tmp" "${CACHED_CHECKSUM}.tmp"
    fi
    exit $exit_code
}
trap cleanup EXIT

# Download function with retry logic
download_with_retry() {
    local url="$1"
    local output="$2"
    local max_attempts=3
    local attempt=1

    while [ $attempt -le $max_attempts ]; do
        echo "Downloading $url (attempt $attempt/$max_attempts)..."

        if curl -L --fail --show-error --progress-bar \
               --connect-timeout 30 --max-time 1800 \
               -o "${output}.tmp" "$url"; then
            mv "${output}.tmp" "$output"
            echo "✅ Downloaded successfully: $(basename "$output")"
            return 0
        else
            echo "⚠️  Download attempt $attempt failed"
            rm -f "${output}.tmp"
            if [ $attempt -lt $max_attempts ]; then
                echo "Retrying in 5 seconds..."
                sleep 5
            fi
            ((attempt++))
        fi
    done

    echo "ERROR: Failed to download after $max_attempts attempts"
    return 1
}

# Check if we need to download the checksum file
if [ ! -f "${CACHED_CHECKSUM}" ] || [ $(($(date +%s) - $(stat -c %Y "${CACHED_CHECKSUM}" 2>/dev/null || echo 0))) -gt 86400 ]; then
    echo "Downloading checksums (daily refresh)..."
    download_with_retry "$CHECKSUM_URL" "$CACHED_CHECKSUM"
fi

# Extract expected checksum for our image
EXPECTED_CHECKSUM=$(grep "${IMAGE_FILENAME}" "${CACHED_CHECKSUM}" | awk '{print $1}')
if [ -z "$EXPECTED_CHECKSUM" ]; then
    echo "ERROR: Could not find checksum for ${IMAGE_FILENAME}"
    exit 1
fi
echo "Expected SHA256: $EXPECTED_CHECKSUM"

# Check if cached image exists and is valid
if [ -f "$CACHED_IMAGE" ]; then
    echo "Found cached image, verifying integrity..."
    ACTUAL_CHECKSUM=$(sha256sum "$CACHED_IMAGE" | awk '{print $1}')

    if [ "$ACTUAL_CHECKSUM" = "$EXPECTED_CHECKSUM" ]; then
        echo "✅ Cached image is valid, using cached version"
        IMAGE_SOURCE="cache"
    else
        echo "❌ Cached image checksum mismatch, re-downloading..."
        rm -f "$CACHED_IMAGE"
        IMAGE_SOURCE="download"
    fi
else
    echo "No cached image found, downloading..."
    IMAGE_SOURCE="download"
fi

# Download image if needed
if [ "$IMAGE_SOURCE" = "download" ]; then
    # Check available disk space (need ~2GB for download + workspace copy)
    AVAILABLE_KB=$(df "${CACHE_DIR}" --output=avail | tail -1)
    REQUIRED_KB=$((3 * 1024 * 1024))  # 3GB in KB for safety

    if [ "$AVAILABLE_KB" -lt "$REQUIRED_KB" ]; then
        echo "ERROR: Insufficient disk space. Available: $(($AVAILABLE_KB / 1024 / 1024))GB, Required: 3GB"
        exit 1
    fi

    download_with_retry "$IMAGE_URL" "$CACHED_IMAGE"

    # Verify downloaded image
    echo "Verifying downloaded image integrity..."
    ACTUAL_CHECKSUM=$(sha256sum "$CACHED_IMAGE" | awk '{print $1}')

    if [ "$ACTUAL_CHECKSUM" != "$EXPECTED_CHECKSUM" ]; then
        echo "ERROR: Downloaded image checksum verification failed!"
        echo "  Expected: $EXPECTED_CHECKSUM"
        echo "  Actual:   $ACTUAL_CHECKSUM"
        rm -f "$CACHED_IMAGE"
        exit 1
    fi

    echo "✅ Image integrity verified successfully"
fi

# Copy to workspace
echo "Copying image to workspace..."
cp "$CACHED_IMAGE" "$WORKSPACE_IMAGE"

# Show image information
echo ""
echo "=== Image Information ==="
echo "Source: $IMAGE_SOURCE"
echo "Size: $(du -h "$WORKSPACE_IMAGE" | cut -f1)"
echo "Virtual size: $(qemu-img info "$WORKSPACE_IMAGE" | grep 'virtual size' | cut -d'(' -f2 | cut -d' ' -f1-2)"
echo "Format: $(qemu-img info "$WORKSPACE_IMAGE" | grep 'file format' | cut -d: -f2 | xargs)"
echo "Path: $WORKSPACE_IMAGE"

# Verify the copied image works with qemu-img
if ! qemu-img check "$WORKSPACE_IMAGE" >/dev/null 2>&1; then
    echo "ERROR: Workspace image failed qemu-img check"
    exit 1
fi

echo "✅ Ubuntu 24.04 cloud image ready for modification"
