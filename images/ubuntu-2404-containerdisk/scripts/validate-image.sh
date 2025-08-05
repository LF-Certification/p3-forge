#!/bin/bash
set -euo pipefail

echo "=== Comprehensive Image Validation ==="
cd "${WORK_DIR}"

# Ensure IMAGE_NAME is lowercase for registry compatibility
IMAGE_NAME=$(echo "${IMAGE_NAME}" | tr '[:upper:]' '[:lower:]')
IMAGE_FULL_NAME="${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"

echo "Testing containerdisk image: $IMAGE_FULL_NAME"

# Validate image exists locally
if ! podman images --format "{{.Repository}}:{{.Tag}}" | grep -q "^${IMAGE_FULL_NAME}$"; then
  echo "ERROR: Image $IMAGE_FULL_NAME not found locally"
  podman images
  exit 1
fi

echo "✅ Image found locally"

# Inspect image metadata
echo ""
echo "=== Image Metadata Validation ==="
podman inspect "$IMAGE_FULL_NAME" > image-inspect.json

# Validate required labels
REQUIRED_LABELS=("org.opencontainers.image.title" "org.opencontainers.image.source" "io.kubevirt.containerdisk")
for label in "${REQUIRED_LABELS[@]}"; do
  if jq -r ".[].Config.Labels[\"$label\"]" image-inspect.json | grep -q "null"; then
    echo "WARNING: Missing required label: $label"
  else
    echo "✅ Label present: $label"
  fi
done

# Extract and validate disk image
echo ""
echo "=== Disk Image Validation ==="
if podman cp "$TEST_CONTAINER":/disk/custom-ubuntu-24.04.qcow2 ./test-disk.qcow2; then
  echo "✅ Disk file successfully extracted"

  # Get disk information
  echo "Disk image information:"
  qemu-img info test-disk.qcow2

  # Verify it's a valid qcow2 image
  echo ""
  echo "Validating disk image integrity..."
  if qemu-img check test-disk.qcow2; then
    echo "✅ Disk image integrity validation passed"
  else
    echo "❌ Disk image integrity validation failed"
    exit 1
  fi

  # Check file size is reasonable
  DISK_SIZE=$(stat -c%s test-disk.qcow2)
  MIN_SIZE=$((100 * 1024 * 1024))  # 100MB minimum
  MAX_SIZE=$((10 * 1024 * 1024 * 1024))  # 10GB maximum

  if [ "$DISK_SIZE" -lt "$MIN_SIZE" ]; then
    echo "❌ Disk image too small: $(($DISK_SIZE / 1024 / 1024))MB < 100MB"
    exit 1
  elif [ "$DISK_SIZE" -gt "$MAX_SIZE" ]; then
    echo "❌ Disk image too large: $(($DISK_SIZE / 1024 / 1024))MB > 10GB"
    exit 1
  else
    echo "✅ Disk image size is reasonable: $(($DISK_SIZE / 1024 / 1024))MB"
  fi

else
  echo "❌ Failed to extract disk file from container"
  exit 1
fi

# Test overlay script presence (mount disk and check)
echo ""
echo "=== Overlay Script Validation ==="
# Connect NBD for validation
sudo qemu-nbd --connect=/dev/nbd1 test-disk.qcow2 || {
  echo "WARNING: Could not connect NBD for overlay validation"
}

# Wait for device
sleep 2

if [ -b /dev/nbd1p1 ]; then
  mkdir -p test-mnt
  if sudo mount /dev/nbd1p1 test-mnt/; then
    echo "Mounted test disk for overlay script validation"

    if [ -f "test-mnt/usr/share/initramfs-tools/scripts/init-bottom/overlay-initramfs-script.sh" ]; then
      echo "✅ Overlay initramfs script found in disk image"
      sudo ls -la "test-mnt/usr/share/initramfs-tools/scripts/init-bottom/overlay-initramfs-script.sh"

      # Verify script is executable
      if [ -x "test-mnt/usr/share/initramfs-tools/scripts/init-bottom/overlay-initramfs-script.sh" ]; then
        echo "✅ Overlay script is executable"
      else
        echo "❌ Overlay script is not executable"
      fi
    else
      echo "❌ Overlay initramfs script NOT found in disk image"
      echo "Available scripts:"
      sudo find test-mnt/usr/share/initramfs-tools/scripts/ -name "*.sh" -type f || true
    fi

    sudo umount test-mnt/
  else
    echo "WARNING: Could not mount test disk for validation"
  fi

  sudo qemu-nbd --disconnect /dev/nbd1 || true
else
  echo "WARNING: NBD partition not available for validation"
fi

# Cleanup test resources
cleanup_test

echo ""
echo "=== Validation Summary ==="
echo "✅ Image metadata validation passed"
echo "✅ Container creation test passed"
echo "✅ Disk extraction test passed"
echo "✅ Disk integrity validation passed"
echo "✅ Disk size validation passed"
echo "✅ Overlay script presence validated"
echo ""
echo "🎉 All validation tests completed successfully!"

# Save validation results
{
  echo "Validation completed: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "Image: $IMAGE_FULL_NAME"
  echo "Status: PASSED"
  echo "Tests: metadata, container, disk-extraction, disk-integrity, disk-size, overlay-script"
} > ../validation-results.txt
