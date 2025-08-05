#!/bin/bash
set -euo pipefail

# Enable verbose debugging
set -x

echo "=== Modifying disk image with custom initramfs script ==="
echo "DEBUG: Working directory: $(pwd)"
echo "DEBUG: Environment variables:"
echo "  WORK_DIR=${WORK_DIR:-unset}"
echo "  USER=$(whoami)"
echo "  UID=$(id -u)"
echo "  PATH=$PATH"

cd "${WORK_DIR}"
echo "DEBUG: Changed to work directory: $(pwd)"

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

# Show current initramfs hooks for debugging
echo "Current initramfs scripts:"
sudo find mnt/usr/share/initramfs-tools/scripts/ -name "*.sh" -type f || true

# Rebuild initramfs with comprehensive error handling
echo "Rebuilding initramfs..."
echo "DEBUG: Checking chroot environment:"
sudo chroot mnt/ /bin/sh -c "ls -la /usr/share/initramfs-tools/scripts/init-bottom/ | grep overlay || echo 'No overlay script found'"

# Bind mount essential directories for chroot environment
echo "DEBUG: Setting up chroot bind mounts..."
sudo mount --bind /proc mnt/proc
sudo mount --bind /sys mnt/sys
sudo mount --bind /dev mnt/dev
sudo mount --bind /run mnt/run

# Setup cleanup for bind mounts
cleanup_chroot() {
  echo "Cleaning up chroot bind mounts..."
  sudo umount mnt/proc 2>/dev/null || true
  sudo umount mnt/sys 2>/dev/null || true
  sudo umount mnt/dev 2>/dev/null || true
  sudo umount mnt/run 2>/dev/null || true
}
trap 'cleanup_chroot; cleanup_nbd' EXIT ERR

echo "DEBUG: Checking available kernels after bind mounts:"
sudo chroot mnt/ /bin/sh -c "ls -la /boot/vmlinuz-* || echo 'No kernels in /boot'"
echo "DEBUG: Checking /lib/modules:"
sudo chroot mnt/ /bin/sh -c "ls -la /lib/modules/ || echo 'No modules directory'"
echo "DEBUG: Checking current initramfs files before update:"
sudo chroot mnt/ /bin/sh -c "ls -la /boot/initrd.img-* || echo 'No initramfs files found'"

echo "DEBUG: Running update-initramfs with verbose output:"
sudo chroot mnt/ /bin/sh -c "update-initramfs -u -v" 2>&1 | tee initramfs-update.log || {
  echo "ERROR: Failed to rebuild initramfs"
  echo "DEBUG: initramfs update log:"
  cat initramfs-update.log || true
  echo "DEBUG: Checking for any error files:"
  sudo find mnt/var/log -name "*initramfs*" -o -name "*kernel*" | head -10 | xargs sudo ls -la || true
  cleanup_chroot
  sudo umount mnt/ || true
  sudo qemu-nbd --disconnect /dev/nbd0 || true
  exit 1
}

echo "✅ Initramfs rebuilt successfully"
echo "DEBUG: initramfs update log contents:"
cat initramfs-update.log || true

# Show final verification
echo "Final verification of script installation:"
sudo ls -la mnt/usr/share/initramfs-tools/scripts/init-bottom/

# Check if initramfs was actually updated
echo "Checking initramfs modification time:"
sudo find mnt/boot -name "initrd.img-*" -exec ls -la {} \;

# Verify the overlay script is included in the initramfs
echo "Verifying overlay script is included in initramfs:"
INITRD_FILE=$(sudo find mnt/boot -name "initrd.img-*" | head -1)
if [ -n "$INITRD_FILE" ]; then
  echo "DEBUG: Found initramfs file: $INITRD_FILE"
  echo "DEBUG: File info:"
  sudo ls -la "$INITRD_FILE"
  echo "DEBUG: Checking initramfs contents:"
  sudo chroot mnt/ /bin/sh -c "lsinitramfs ${INITRD_FILE#mnt} | head -20"
  echo "DEBUG: Searching for overlay-related files:"
  sudo chroot mnt/ /bin/sh -c "lsinitramfs ${INITRD_FILE#mnt} | grep -i overlay || echo 'No overlay files found in initramfs'"
  echo "DEBUG: Searching specifically for our script:"
  sudo chroot mnt/ /bin/sh -c "lsinitramfs ${INITRD_FILE#mnt} | grep overlay-initramfs-script || echo 'WARNING: overlay-initramfs-script.sh not found in initramfs'"
  echo "DEBUG: All init-bottom scripts in initramfs:"
  sudo chroot mnt/ /bin/sh -c "lsinitramfs ${INITRD_FILE#mnt} | grep scripts/init-bottom/ || echo 'No init-bottom scripts found'"
else
  echo "WARNING: No initramfs file found"
  echo "DEBUG: Available files in /boot:"
  sudo ls -la mnt/boot/ || true
fi

# Cleanup bind mounts before unmounting
cleanup_chroot

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
