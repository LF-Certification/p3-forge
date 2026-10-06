#!/bin/sh
set -eu
arch=${1:?architecture required}
initramfs=${2:?initramfs required}
out=${3:?output required}
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
case "$arch" in
  amd64) boot=/usr/lib/systemd/boot/efi/systemd-bootx64.efi; efi_suffix=X64;;
  arm64) boot=/usr/lib/systemd/boot/efi/systemd-bootaa64.efi; efi_suffix=AA64;;
  *) echo "unsupported TARGETARCH=$arch" >&2; exit 1;;
esac
kernel=$(find /boot -maxdepth 1 -type f -name 'vmlinuz-*' -print -quit)
[ -s "$kernel" ] || { echo 'no architecture kernel installed' >&2; exit 1; }
[ -s "$boot" ] || { echo "missing systemd-boot fallback: $boot" >&2; exit 1; }
esp_bytes=$(( $(stat -c '%s' "$kernel") + $(stat -c '%s' "$initramfs") + 32*1024*1024 ))
esp_bytes=$(( (esp_bytes + 511) / 512 * 512 ))
raw_bytes=$(( esp_bytes + 2*1024*1024 + 33*512 ))
esp="$work/esp.img"
raw="$work/disk.raw"
truncate -s "$esp_bytes" "$esp"
mkfs.vfat -F 32 -n EFI "$esp" >/dev/null
mmd -i "$esp" ::EFI ::EFI/BOOT ::loader ::debian
mcopy -i "$esp" "$boot" "::EFI/BOOT/BOOT${efi_suffix}.EFI"
mcopy -i "$esp" "$kernel" ::debian/vmlinuz
mcopy -i "$esp" "$initramfs" ::debian/initrd.img
cat >"$work/loader.conf" <<'EOF'
default debian
console-mode max
timeout 0
EOF
cat >"$work/debian.conf" <<'EOF'
title Debian live installer
linux /debian/vmlinuz
initrd /debian/initrd.img
options console=tty0 console=ttyS0,115200n8
EOF
mcopy -i "$esp" "$work/loader.conf" ::loader/loader.conf
mmd -i "$esp" ::loader/entries
mcopy -i "$esp" "$work/debian.conf" ::loader/entries/debian.conf
# sgdisk writes GPT metadata; dd inserts the independently-created ESP without
# mounting either image.  The resulting raw image is converted to qcow2.
truncate -s "$raw_bytes" "$raw"
esp_sectors=$(( (esp_bytes + 511) / 512 ))
sgdisk --clear --new=1:2048:+${esp_sectors}S --typecode=1:ef00 --change-name=1:EFI "$raw" >/dev/null
dd if="$esp" of="$raw" bs=512 seek=2048 conv=notrunc status=none
qemu-img convert -f raw -O qcow2 "$raw" "$out"
