#!/bin/bash
set -euo pipefail

echo "=== Modifying disk image with overlay initramfs script ==="

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Configuration - ensure WORK_DIR is absolute
if [[ "${WORK_DIR:-build-workspace}" = /* ]]; then
    WORK_DIR="${WORK_DIR:-build-workspace}"
else
    WORK_DIR="${PROJECT_DIR}/${WORK_DIR:-build-workspace}"
fi
NBD_DEVICE="/dev/nbd0"
MOUNT_POINT="${WORK_DIR}/disk-mount"
OVERLAY_SCRIPT="${PROJECT_DIR}/overlay-initramfs-script.sh"
INITRAMFS_SCRIPT_DIR="usr/share/initramfs-tools/scripts/init-bottom"
SCRIPT_NAME="overlay-setup"

# Validate inputs
if [ ! -f "${WORK_DIR}/base-disk.qcow2" ]; then
    echo "ERROR: Base disk image not found at ${WORK_DIR}/base-disk.qcow2"
    echo "Run the download-disk-image target first"
    exit 1
fi

if [ ! -f "$OVERLAY_SCRIPT" ]; then
    echo "ERROR: Overlay initramfs script not found at $OVERLAY_SCRIPT"
    exit 1
fi

# Check if running as root or with sudo capabilities
if [ "$EUID" -ne 0 ] && ! sudo -n true 2>/dev/null; then
    echo "ERROR: This script requires sudo access for NBD operations"
    echo "Please run with sudo or ensure passwordless sudo is configured"
    exit 1
fi

# Global cleanup function
cleanup() {
    local exit_code=$?

    echo "Performing cleanup..."

    # Unmount filesystem if mounted
    if mountpoint -q "$MOUNT_POINT" 2>/dev/null; then
        echo "Unmounting filesystem..."
        sudo umount "$MOUNT_POINT" 2>/dev/null || true
    fi

    # Remove mount point
    if [ -d "$MOUNT_POINT" ]; then
        rmdir "$MOUNT_POINT" 2>/dev/null || true
    fi

    # Disconnect NBD device if connected
    if [ -b "$NBD_DEVICE" ]; then
        echo "Disconnecting NBD device..."
        sudo qemu-nbd --disconnect "$NBD_DEVICE" 2>/dev/null || true

        # Wait for device to be fully disconnected
        local timeout=10
        while [ $timeout -gt 0 ] && [ -b "${NBD_DEVICE}p1" ]; do
            sleep 1
            ((timeout--))
        done
    fi

    if [ $exit_code -ne 0 ]; then
        echo "ERROR: Disk modification failed (exit code: $exit_code)"
    fi

    exit $exit_code
}
trap cleanup EXIT

# Load NBD kernel module if not loaded
if ! lsmod | grep -q "^nbd "; then
    echo "Loading NBD kernel module..."
    sudo modprobe nbd max_part=8
fi

# Check if NBD device is available
if [ ! -b "$NBD_DEVICE" ]; then
    echo "ERROR: NBD device $NBD_DEVICE not available"
    echo "Ensure NBD kernel module is loaded: sudo modprobe nbd max_part=8"
    exit 1
fi

# Disconnect NBD device if already in use
if [ -b "${NBD_DEVICE}p1" ]; then
    echo "NBD device appears to be in use, disconnecting..."
    sudo qemu-nbd --disconnect "$NBD_DEVICE" || true
    sleep 2
fi

# Connect QCOW2 image to NBD device
echo "Connecting QCOW2 image to NBD device..."
sudo qemu-nbd --connect="$NBD_DEVICE" "${WORK_DIR}/base-disk.qcow2"

# Wait for device partitions to appear
echo "Waiting for device partitions..."
timeout=30
while [ $timeout -gt 0 ] && [ ! -b "${NBD_DEVICE}p1" ]; do
    sleep 1
    ((timeout--))
done

if [ ! -b "${NBD_DEVICE}p1" ]; then
    echo "ERROR: Device partition ${NBD_DEVICE}p1 did not appear"
    echo "Available NBD devices:"
    ls -la /dev/nbd* || true
    exit 1
fi

# Show partition information
echo "Partition information:"
sudo fdisk -l "$NBD_DEVICE" || true

# Create mount point and mount the filesystem
echo "Creating mount point and mounting filesystem..."
mkdir -p "$MOUNT_POINT"
sudo mount "${NBD_DEVICE}p1" "$MOUNT_POINT"

# Verify mount was successful
if ! mountpoint -q "$MOUNT_POINT"; then
    echo "ERROR: Failed to mount filesystem"
    exit 1
fi

echo "✅ Filesystem mounted successfully"

# Show filesystem information
echo "Filesystem information:"
df -h "$MOUNT_POINT"
echo ""
echo "Root directory contents:"
sudo ls -la "$MOUNT_POINT/" | head -10

# Create initramfs scripts directory if it doesn't exist
INITRAMFS_DIR="${MOUNT_POINT}/${INITRAMFS_SCRIPT_DIR}"
echo "Creating initramfs script directory..."
sudo mkdir -p "$INITRAMFS_DIR"

# Install the overlay initramfs script
echo "Installing overlay initramfs script..."
SCRIPT_PATH="${INITRAMFS_DIR}/${SCRIPT_NAME}"
sudo cp "$OVERLAY_SCRIPT" "$SCRIPT_PATH"
sudo chmod +x "$SCRIPT_PATH"

# Verify script was installed correctly
if [ ! -f "$SCRIPT_PATH" ]; then
    echo "ERROR: Failed to install overlay script"
    exit 1
fi

echo "✅ Overlay script installed at: ${INITRAMFS_SCRIPT_DIR}/${SCRIPT_NAME}"

# Show script content for verification
echo "Installed script content (first 10 lines):"
sudo head -10 "$SCRIPT_PATH"

# Update initramfs to include our script
echo "Updating initramfs..."
sudo chroot "$MOUNT_POINT" /bin/bash -c "
    export DEBIAN_FRONTEND=noninteractive
    if command -v update-initramfs >/dev/null 2>&1; then
        update-initramfs -u -k all
        echo '✅ Initramfs updated successfully'
    else
        echo '⚠️  Warning: update-initramfs not found, manual initramfs update may be required'
    fi
"

# Verify the modifications
echo ""
echo "=== Modification Summary ==="
echo "Script installed: ${INITRAMFS_SCRIPT_DIR}/${SCRIPT_NAME}"
echo "Script size: $(sudo stat -c%s "$SCRIPT_PATH") bytes"
echo "Script permissions: $(sudo stat -c%a "$SCRIPT_PATH")"

# Show some key directories to verify structure
echo ""
echo "Key directories in modified image:"
sudo ls -la "${MOUNT_POINT}/usr/share/initramfs-tools/scripts/" || echo "initramfs-tools not found"
sudo ls -la "${MOUNT_POINT}/boot/" | head -5 || echo "boot directory listing failed"

# Sync filesystem changes
echo "Syncing filesystem changes..."
sudo sync

# Unmount filesystem
echo "Unmounting filesystem..."
sudo umount "$MOUNT_POINT"
rmdir "$MOUNT_POINT"

# Disconnect NBD device
echo "Disconnecting NBD device..."
sudo qemu-nbd --disconnect "$NBD_DEVICE"

# Wait for disconnect to complete
sleep 2

# Verify the modified image
echo "Verifying modified image..."
if ! qemu-img check "${WORK_DIR}/base-disk.qcow2" >/dev/null 2>&1; then
    echo "ERROR: Modified image failed integrity check"
    exit 1
fi

# Show final image information
echo ""
echo "=== Modified Image Information ==="
qemu-img info "${WORK_DIR}/base-disk.qcow2"
echo ""
echo "Image size on disk: $(du -h "${WORK_DIR}/base-disk.qcow2" | cut -f1)"

echo "✅ Disk image modification completed successfully"
echo "Modified image ready for containerization at: ${WORK_DIR}/base-disk.qcow2"
