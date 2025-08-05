#!/bin/bash
set -euo pipefail

echo "=== Modifying disk image with custom initramfs script using qemu-nbd + chroot ==="
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

# Check if required tools are available
if ! command -v qemu-nbd &> /dev/null; then
    echo "ERROR: qemu-nbd not found. Install qemu-utils (e.g., sudo apt install qemu-utils)"
    exit 1
fi

# Load NBD kernel module if not already loaded
if ! lsmod | grep -q nbd; then
    echo "Loading NBD kernel module..."
    sudo modprobe nbd max_part=8
fi

# Find available NBD device
NBD_DEVICE=""
for i in {0..15}; do
    if [ ! -b "/dev/nbd${i}" ]; then
        continue
    fi
    if ! sudo qemu-nbd --list | grep -q "/dev/nbd${i}"; then
        NBD_DEVICE="/dev/nbd${i}"
        break
    fi
done

if [ -z "$NBD_DEVICE" ]; then
    echo "ERROR: No available NBD device found"
    exit 1
fi

echo "Using NBD device: $NBD_DEVICE"

# Setup cleanup function with proper NBD disconnect
cleanup() {
  echo "Cleaning up..."

  # Unmount bind mounts if they exist
  for mount in proc sys dev run; do
    if mountpoint -q "mnt/$mount" 2>/dev/null; then
      echo "Unmounting mnt/$mount..."
      sudo umount "mnt/$mount" || true
    fi
  done

  # Unmount main filesystem if mounted
  if mountpoint -q "mnt" 2>/dev/null; then
    echo "Unmounting mnt..."
    sudo umount mnt || true
  fi

  # Disconnect NBD device if connected
  if [ -n "${NBD_DEVICE:-}" ] && sudo qemu-nbd --list | grep -q "$NBD_DEVICE"; then
    echo "Disconnecting $NBD_DEVICE..."
    sudo qemu-nbd --disconnect "$NBD_DEVICE" || true
  fi

  # Clean up temporary files
  rm -f initramfs-update.log boot-contents.txt modules-contents.txt initramfs-contents.txt 2>/dev/null || true
}
trap cleanup EXIT ERR

# Create mount directory
mkdir -p mnt

# Connect NBD device
echo "Connecting NBD device..."
sudo qemu-nbd --connect="$NBD_DEVICE" base-disk.qcow2

# Wait for device to be ready
sleep 2

# Find the root partition (usually the first or largest partition)
ROOT_PARTITION="${NBD_DEVICE}p1"
if [ ! -b "$ROOT_PARTITION" ]; then
    # Try without partition number if it's not partitioned
    ROOT_PARTITION="$NBD_DEVICE"
fi

echo "Mounting root partition: $ROOT_PARTITION"
sudo mount "$ROOT_PARTITION" mnt/

# Setup chroot environment with bind mounts
echo "Setting up chroot environment..."
sudo mount --bind /proc mnt/proc
sudo mount --bind /sys mnt/sys
sudo mount --bind /dev mnt/dev
sudo mount --bind /run mnt/run

# Modify the image via chroot
echo "Modifying image via chroot..."

# Update package sources
echo "Updating package sources..."
sudo chroot mnt/ /bin/bash -c 'echo "deb http://archive.ubuntu.com/ubuntu noble main universe restricted multiverse" > /etc/apt/sources.list'
sudo chroot mnt/ /bin/bash -c 'apt-get update'

# Clean package cache to free up space
echo "Cleaning package cache..."
sudo chroot mnt/ /bin/bash -c 'apt-get clean'
sudo chroot mnt/ /bin/bash -c 'apt-get autoclean'
sudo chroot mnt/ /bin/bash -c 'rm -rf /var/lib/apt/lists/*'

# Install kernel
echo "Installing linux kernel..."
sudo chroot mnt/ /bin/bash -c 'DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends linux-image-generic'

# Create initramfs scripts directory
echo "Creating initramfs scripts directory..."
sudo mkdir -p mnt/usr/share/initramfs-tools/scripts/init-bottom/

# Copy overlay script
echo "Copying overlay initramfs script..."
sudo cp ../overlay-initramfs-script.sh mnt/usr/share/initramfs-tools/scripts/init-bottom/
sudo chmod +x mnt/usr/share/initramfs-tools/scripts/init-bottom/overlay-initramfs-script.sh

# Update initramfs
echo "Updating initramfs..."
sudo chroot mnt/ /bin/bash -c 'update-initramfs -u -k all -v' 2>&1 | tee initramfs-update.log

# Collect debug information
echo "Collecting debug information..."
sudo chroot mnt/ /bin/bash -c 'ls -la /boot/' > boot-contents.txt
sudo chroot mnt/ /bin/bash -c 'ls -la /lib/modules/' > modules-contents.txt
sudo chroot mnt/ /bin/bash -c 'lsinitramfs /boot/initrd.img-$(uname -r)' > initramfs-contents.txt

echo "✅ Image modified successfully"

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
