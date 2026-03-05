#!/usr/bin/env bashp

# Full kernel required for 9p mounts (debian cloud image defaults to stripped-down cloud kernel)
arch="$(system::get_arch)"
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y "linux-image-$arch"
apt-get purge -y "linux-image-cloud-$arch"
dpkg --purge --force-depends $(dpkg -l | awk '/linux-image.*cloud/ {print $2}')

p3forge::base
