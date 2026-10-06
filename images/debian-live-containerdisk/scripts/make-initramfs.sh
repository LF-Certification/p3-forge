#!/bin/sh
set -eu

out=${1:?output required}
stage=$(mktemp -d)
chmod 0755 "$stage"
trap 'rm -rf "$stage"' EXIT
# The initramfs is the complete installed Debian userspace.  No source disk is
# needed after the kernel and this archive have been read by firmware.
for path in bin etc home lib lib64 media mnt opt root sbin srv usr var; do
    [ -e "/$path" ] && cp -a "/$path" "$stage/"
done
mkdir -p "$stage/dev" "$stage/proc" "$stage/sys" "$stage/run" "$stage/tmp"

mkdir -p "$stage/etc/systemd/network"
cat >"$stage/etc/systemd/network/10-sandbox.link" <<'EOF'
[Match]
Driver=virtio_net

[Link]
Name=eth0
EOF
chmod 1777 "$stage/tmp"

ln -s /lib/systemd/systemd "$stage/init"

cat >"$stage/etc/systemd/system/sandbox-dhcp.service" <<'EOF'
[Unit]
Description=Configure the sandbox build network
Wants=systemd-udev-settle.service
After=systemd-udev-settle.service
Before=network-online.target sandbox-cloud-init.service ssh.service

[Service]
Type=oneshot
ExecStart=/sbin/dhclient -4 -1 -v
RemainAfterExit=yes
EOF

# Run cloud-init's stages directly after deterministic DHCP configuration. The
# distribution's socket-based systemd generator assumes an installed root and
# can wait forever for its network-stage trigger in an all-RAM root.
rm -f "$stage/usr/lib/systemd/system-generators/cloud-init-generator"
rm -rf "$stage/etc/systemd/system/cloud-init.target.wants" "$stage/var/lib/cloud"
mkdir -p "$stage/etc/cloud/cloud.cfg.d" "$stage/var/lib/cloud"
cat >"$stage/etc/cloud/cloud.cfg.d/99-sandbox.cfg" <<'EOF'
datasource_list: [ NoCloud, None ]
network:
  config: disabled
ssh_pwauth: false
preserve_hostname: false
manage_etc_hosts: true
EOF
cat >"$stage/etc/systemd/system/sandbox-cloud-init.service" <<'EOF'
[Unit]
Description=Apply Lima NoCloud configuration
Requires=sandbox-dhcp.service
After=sandbox-dhcp.service
Before=ssh.service

[Service]
Type=oneshot
ExecStart=/usr/bin/cloud-init init --local
ExecStart=/usr/bin/cloud-init init
ExecStart=/usr/bin/cloud-init modules --mode=config
ExecStart=/usr/bin/cloud-init modules --mode=final
RemainAfterExit=yes
EOF
mkdir -p "$stage/etc/systemd/system/multi-user.target.wants" \
    "$stage/etc/systemd/system/network-online.target.wants" \
    "$stage/etc/systemd/system/ssh.service.d"
cat >"$stage/etc/systemd/system/ssh.service.d/10-sandbox.conf" <<'EOF'
[Unit]
Requires=sandbox-cloud-init.service
After=sandbox-cloud-init.service

[Service]
ExecStartPre=
ExecStartPre=/usr/bin/ssh-keygen -A
ExecStartPre=/usr/sbin/sshd -t
EOF
ln -sf /etc/systemd/system/sandbox-dhcp.service \
    "$stage/etc/systemd/system/network-online.target.wants/sandbox-dhcp.service"
ln -sf /etc/systemd/system/sandbox-cloud-init.service \
    "$stage/etc/systemd/system/multi-user.target.wants/sandbox-cloud-init.service"
ln -sf /lib/systemd/system/ssh.service \
    "$stage/etc/systemd/system/multi-user.target.wants/ssh.service"
rm -f "$stage/etc/ssh/ssh_host_"*
: >"$stage/etc/machine-id"

# package state and host resolver data are not part of the published archive.
rm -rf "$stage/var/lib/apt/lists" "$stage/var/cache/apt" "$stage/var/log"/*
rm -f "$stage/etc/resolv.conf"
: >"$stage/etc/resolv.conf"
(cd "$stage" && find . -xdev -print0 | cpio -0 -o -H newc -C 512 --quiet) | gzip -9 > "$out"
