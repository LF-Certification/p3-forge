#!/usr/bin/env bashp

centosBootcImage="${CENTOS_BOOTC_IMAGE:?must be set}"
install::apt_packages podman uidmap slirp4netns passt nftables

install -d /etc/containers/containers.conf.d
cat >/etc/containers/containers.conf.d/50-sandbox.conf <<'EOF'
[engine]
cgroup_manager = "cgroupfs"
events_logger = "file"
EOF

podman pull "${centosBootcImage}"

# The Debian builder filesystem is temporary scratch. Installing over it leaves
# only the customized CentOS bootc deployment in the published distro image.
podman run --rm --privileged --pid=host \
  --volume /var/lib/containers:/var/lib/containers \
  --volume /dev:/dev \
  --volume /:/target \
  --security-opt label=type:unconfined_t \
  "${centosBootcImage}" \
  bootc install to-existing-root \
  --acknowledge-destructive \
  --disable-selinux

podman system reset --force
