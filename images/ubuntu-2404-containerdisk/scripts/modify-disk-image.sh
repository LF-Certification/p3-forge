#!/bin/bash
set -euo pipefail

echo "=== Modifying disk image with custom initramfs script ==="
cd "${WORK_DIR}"

# Verify prerequisites
if [ ! -f "base-disk.qcow2" ]; then
  echo "ERROR: base-disk.qcow2 not found"
  exit 1
fi

if [ ! -f "../overlay-initramfs-script.sh" ]; then
  echo "ERROR: overlay-initramfs-script.sh not found"
  exit 1
fi

# Setup cleanup function
cleanup_nbd() {
  echo "Cleaning up NBD connections..."
  sudo umount mnt/ 2>/dev/null || true
  sudo qemu-nbd --disconnect /dev/nbd0 2>/dev/null || true
}
trap cleanup_nbd EXIT ERR

# Create mount directory
mkdir -p mnt

# Connect NBD device with error handling
echo "Connecting NBD device..."
sudo qemu-nbd --connect=/dev/nbd0 base-disk.qcow2 || {
  echo "ERROR: NBD connect failed"
  exit 1
}

# Poll for partitions with better error handling
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
  sudo qemu-nbd --disconnect /dev/nbd0 || true
  exit 1
fi

# Mount the filesystem with error handling
echo "Mounting filesystem..."
sudo mount /dev/nbd0p1 mnt/ || {
  echo "ERROR: Mount failed"
  sudo qemu-nbd --disconnect /dev/nbd0 || true
  exit 1
}

echo "✅ Filesystem mounted successfully"

# Verify mount and show filesystem info
mountpoint mnt/
df -h mnt/

# Copy overlay script to initramfs location
echo "Installing overlay initramfs script..."
sudo cp ../overlay-initramfs-script.sh mnt/usr/share/initramfs-tools/scripts/init-bottom/
sudo chmod +x mnt/usr/share/initramfs-tools/scripts/init-bottom/overlay-initramfs-script.sh

# Verify script installation
echo "Verifying script installation..."
sudo ls -la mnt/usr/share/initramfs-tools/scripts/init-bottom/overlay-initramfs-script.sh

# Show current initramfs hooks for debugging
echo "Current initramfs scripts:"
sudo find mnt/usr/share/initramfs-tools/scripts/ -name "*.sh" -type f || true

# Rebuild initramfs with comprehensive error handling
echo "Rebuilding initramfs..."
sudo chroot mnt/ /bin/sh -c "update-initramfs -u" || {
  echo "ERROR: Failed to rebuild initramfs"
  sudo umount mnt/ || true
  sudo qemu-nbd --disconnect /dev/nbd0 || true
  exit 1
}

echo "✅ Initramfs rebuilt successfully"

# Show final verification
echo "Final verification of script installation:"
sudo ls -la mnt/usr/share/initramfs-tools/scripts/init-bottom/

# Check if initramfs was actually updated
echo "Checking initramfs modification time:"
sudo find mnt/boot -name "initrd.img-*" -exec ls -la {} \;

# Unmount and disconnect with verification
echo "Unmounting filesystem..."
sudo umount mnt/ || {
  echo "ERROR: Unmount failed"
  exit 1
}

echo "Disconnecting NBD device..."
sudo qemu-nbd --disconnect /dev/nbd0 || {
  echo "ERROR: NBD disconnect failed"
  exit 1
}

echo "✅ Disk image modification completed successfully"

# Final verification of modified image
echo "Final image verification:"
qemu-img info base-disk.qcow2
ls -lh base-disk.qcow2

# Check disk space
echo ""
echo "=== Disk space after modification ==="
df -h
