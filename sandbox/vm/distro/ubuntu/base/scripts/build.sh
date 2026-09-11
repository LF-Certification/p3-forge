#!/usr/bin/env bashp

# Remove unnecessary packages to reduce image size
export DEBIAN_FRONTEND=noninteractive
apt-get purge -y snapd 'linux-headers-*' 'linux-tools-*' bpftrace 'libllvm*' sosreport landscape-common

p3forge::base

# Ubuntu 26.04 can bring the NIC up before cloud-init applies Lima's set-name,
# leaving systemd-networkd-wait-online blocked on the expected eth0 name.
if grep -Eq '^VERSION_CODENAME="?resolute"?$' /etc/os-release; then
	cat >/etc/netplan/99-p3-eth0-optional.yaml <<'YAML'
network:
  version: 2
  ethernets:
    eth0:
      optional: true
YAML
fi
