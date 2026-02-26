#!/usr/bin/env bashp

# Remove unnecessary packages to reduce image size
export DEBIAN_FRONTEND=noninteractive
apt-get purge -y snapd 'linux-headers-*' 'linux-tools-*' bpftrace 'libllvm*' sosreport landscape-common

p3forge::base
