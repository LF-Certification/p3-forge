#!/bin/bash
set -euo pipefail

echo "=== Modifying disk image with custom initramfs script using virt-customize ==="
echo "DEBUG: Working directory: $(pwd)"
echo "DEBUG: Environment variables:"
echo " WORK_DIR=${WORK_DIR:-unset}"
echo " USER=$(whoami)"
echo " UID=$(id -u)"
echo " PATH=$PATH"

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

# Check if virt-customize is available
if ! command -v virt-customize &> /dev/null; then
    echo "ERROR: virt-customize not found. Install libguestfs-tools (e.g., sudo apt install libguestfs-tools)"
    exit 1
fi

# Setup cleanup function (though virt-customize doesn't require manual mounts)
cleanup() {
  echo "Cleaning up any temporary files..."
  rm -f initramfs-update.log boot-contents.txt modules-contents.txt initramfs-contents.txt 2>/dev/null || true
}
trap cleanup EXIT ERR

# Use virt-customize to modify the image
echo "Modifying image with virt-customize..."

virt-customize -a base-disk.qcow2 \
  --run-command 'echo "deb http://archive.ubuntu.com/ubuntu noble main universe restricted multiverse" > /etc/apt/sources.list' \
  --run-command 'apt-get update' \
  --install linux-image-generic \
  --mkdir /usr/share/initramfs-tools/scripts/init-bottom/ \
  --copy-in ../overlay-initramfs-script.sh:/usr/share/initramfs-tools/scripts/init-bottom/ \
  --run-command 'chmod +x /usr/share/initramfs-tools/scripts/init-bottom/overlay-initramfs-script.sh' \
  --run-command 'update-initramfs -u -k all -v > /initramfs-update.log 2>&1' \
  --run-command 'ls -la /boot/ > /boot-contents.txt' \
  --run-command 'ls -la /lib/modules/ > /modules-contents.txt' \
  --run-command 'lsinitramfs /boot/initrd.img-$(uname -r) > /initramfs-contents.txt' \
  --selinux-relabel

echo "✅ Image modified successfully"

# Extract debug logs from the image for verification
echo "DEBUG: Extracting logs and contents..."

virt-cat -a base-disk.qcow2 /initramfs-update.log > initramfs-update.log
virt-cat -a base-disk.qcow2 /boot-contents.txt > boot-contents.txt
virt-cat -a base-disk.qcow2 /modules-contents.txt > modules-contents.txt
virt-cat -a base-disk.qcow2 /initramfs-contents.txt > initramfs-contents.txt

# Display debug information
echo "DEBUG: Initramfs update log contents:"
cat initramfs-update.log || echo "No log found"

echo "DEBUG: /boot/ contents:"
cat boot-contents.txt || echo "No contents found"

echo "DEBUG: /lib/modules/ contents:"
cat modules-contents.txt || echo "No contents found"

echo "DEBUG: Initramfs contents preview (first 20 lines):"
head -20 initramfs-contents.txt || echo "No contents found"

echo "DEBUG: Searching for overlay script in initramfs:"
grep -i "overlay-initramfs-script" initramfs-contents.txt || echo "WARNING: overlay-initramfs-script.sh not found in initramfs"

# Final verification of modified image
echo "Final image verification:"
qemu-img info base-disk.qcow2
ls -lh base-disk.qcow2

# Check disk space
echo ""
echo "=== Disk space after modification ==="
df -h

echo "✅ Disk image modification completed successfully"
