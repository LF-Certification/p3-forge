#!/bin/sh

# Initramfs overlay script for 2-layer disk architecture
# This script runs during initramfs init-bottom phase to set up overlay filesystem
#
# Architecture:
#   /dev/vda - ContainerDisk (read-only base image)
#   /dev/vdb - PVC (read-write overlay storage)
#   /dev/vdc - CloudInit (configuration)

# Only run when init has set rootmnt
if [ -z ${rootmnt+x} ]; then
    exit 0
fi

echo "Beginning root overlay setup..."

# Device and mount point configuration
bottomdev=/dev/vda
bottommnt=/media/bottom-mnt
bottomdir=${bottommnt}

topdev=/dev/vdb
topmnt=/media/top-mnt
topdir="${topmnt}/overlay"
workdir="${topmnt}/overlay-workdir"

# Mount the bottom layer (read-only ContainerDisk)
echo "Mounting bottom layer (ContainerDisk)..."
mkdir -p ${bottommnt}
mount ${bottomdev} ${bottommnt}

# Mount the top layer
echo "Formatting top layer..."
mkfs.ext4 ${topdev}
echo "Mounting top layer (PVC overlay)..."
mkdir -p ${topmnt}
mount ${topdev} ${topmnt}

# Create overlay directories on the PVC
mkdir -p ${topdir}
mkdir -p ${workdir}

# Set up the overlay filesystem
echo "Setting up overlay filesystem..."
# Move the current root mount (ContainerDisk) to become the bottom layer
mkdir -p ${bottomdir}
mount -n -o move ${rootmnt} ${bottomdir}

# Create overlay mount with:
# - lowerdir: read-only base from ContainerDisk
# - upperdir: read-write changes on PVC
# - workdir: overlay working directory on PVC
mount -t overlay overlay -olowerdir=${bottomdir},upperdir=${topdir},workdir=${workdir} ${rootmnt}

# Make the constituent mounts visible in the real root filesystem
echo "Making overlay mounts visible in root filesystem..."
mkdir -p "${rootmnt}${bottommnt}"
mount --move ${bottommnt} "${rootmnt}${bottommnt}"

mkdir -p "${rootmnt}${topmnt}"
mount --move ${topmnt} "${rootmnt}${topmnt}"

# Update /etc/fstab to reflect the overlay setup
echo "Configuring /etc/fstab for overlay mounts..."
cp ${rootmnt}/etc/fstab ${rootmnt}/etc/fstab.orig

# Remove old root entry and add overlay mount entries
awk '$2 != "/" {print $0}' ${rootmnt}/etc/fstab.orig > ${rootmnt}/etc/fstab

# Add overlay mount entries based on current mounts
awk '$2 == "'${rootmnt}'" { $2 = "/" ; print $0}' /etc/mtab >> ${rootmnt}/etc/fstab

echo "Root overlay setup complete"
echo "  Bottom layer (read-only): ${bottomdev} -> ${bottommnt}"
echo "  Top layer (read-write):   ${topdev} -> ${topmnt}"
echo "  Overlay root:             / (${bottomdir} + ${topdir})"
