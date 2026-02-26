#!/usr/bin/env bashp

# Full kernel required for 9p mounts (debian cloud image defaults to stripped-down cloud kernel)
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y linux-image-amd64
apt-get purge -y linux-image-cloud-amd64
dpkg --purge --force-depends $(dpkg -l | awk '/linux-image.*cloud/ {print $2}')

p3forge::base
