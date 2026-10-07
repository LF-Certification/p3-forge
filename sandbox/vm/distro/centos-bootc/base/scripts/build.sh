#!/usr/bin/env bashp

centosBootcImage="${CENTOS_BOOTC_IMAGE:?must be set}"
install::apt_packages podman uidmap slirp4netns passt nftables

install -d /etc/containers/containers.conf.d
cat >/etc/containers/containers.conf.d/50-sandbox.conf <<'EOF'
[engine]
cgroup_manager = "cgroupfs"
events_logger = "file"
EOF

case "$(uname -m)" in
  x86_64)
    yqArch=amd64
    yqSha256=8e34fc298390875de416e6a4afcb8cabeceb25d9aa8506c1a2f9353cf702ea5f
    ;;
  aarch64)
    yqArch=arm64
    yqSha256=189088da0c6429ec5178dfaab1a114805f6cab0b61b165ab236efedf1d57a71b
    ;;
  *)
    echo "unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

buildContext="$(mktemp -d)"
trap 'rm -rf "${buildContext}"' EXIT

curl --fail --location --silent --show-error \
  "https://github.com/mikefarah/yq/releases/download/v4.54.1/yq_linux_${yqArch}" \
  --output "${buildContext}/yq"
printf '%s  %s\n' "${yqSha256}" "${buildContext}/yq" | sha256sum --check
chmod 0755 "${buildContext}/yq"

cat >"${buildContext}/50-sandbox-sysusers.conf" <<'EOF'
g sudo -
u tux 60000 "Sandbox user" /var/home/tux /bin/bash
m tux sudo
EOF

cat >"${buildContext}/50-sandbox-tmpfiles.conf" <<'EOF'
d /var/home/tux 0700 tux tux -
d /var/sandbox 0755 root root -
EOF

cat >"${buildContext}/90-sandbox-sudoers" <<'EOF'
%sudo ALL=(ALL:ALL) NOPASSWD: ALL
EOF

cat >"${buildContext}/40-sandbox-sshd.conf" <<'EOF'
PasswordAuthentication no
PubkeyAuthentication yes
PermitRootLogin prohibit-password
EOF

cat >"${buildContext}/50-sandbox-containers.conf" <<'EOF'
[engine]
cgroup_manager = "cgroupfs"
events_logger = "file"
EOF

cat >"${buildContext}/sandbox-cloud-final-reset.service" <<'EOF'
[Unit]
Description=Clear completed cloud-final failure state
ConditionPathExists=/mnt/lima-cidata
Wants=cloud-final.service
After=cloud-final.service

[Service]
Type=oneshot
ExecStart=/usr/bin/systemctl reset-failed cloud-final.service
RemainAfterExit=yes

[Install]
WantedBy=cloud-init.target
EOF

cat >"${buildContext}/Containerfile" <<'EOF'
ARG CENTOS_BOOTC_IMAGE
FROM ${CENTOS_BOOTC_IMAGE} AS module-builder

RUN set -eux; \
    kver="$(find /usr/lib/modules -mindepth 1 -maxdepth 1 -type d -printf '%f\n')"; \
    test -n "${kver}"; \
    dnf -y --setopt=install_weak_deps=False install \
      curl elfutils-libelf-devel gcc gzip "kernel-devel-${kver}" make tar; \
    commit=6b07db58401b293c1748b127e445ef3c536f8d27; \
    curl --fail --location --silent --show-error \
      "https://gitlab.com/redhat/centos-stream/src/kernel/centos-stream-10/-/archive/${commit}/centos-stream-10-${commit}.tar.gz" \
      --output /tmp/kernel.tar.gz; \
    printf '%s  %s\n' \
      c811c8bfaabb35b5a3cf5ded94347b2093d71f98e85f1a291d2ffacb45c80837 \
      /tmp/kernel.tar.gz | sha256sum --check; \
    tar -xzf /tmp/kernel.tar.gz -C /tmp; \
    src="/tmp/centos-stream-10-${commit}"; \
    make -C "/usr/src/kernels/${kver}" M="${src}/net/9p" \
      CONFIG_NET_9P=m CONFIG_NET_9P_VIRTIO=m \
      EXTRA_CFLAGS='-DCONFIG_NET_9P_MODULE=1 -DCONFIG_NET_9P_VIRTIO_MODULE=1' \
      modules; \
    make -C "/usr/src/kernels/${kver}" M="${src}/fs/9p" \
      CONFIG_9P_FS=m KBUILD_EXTRA_SYMBOLS="${src}/net/9p/Module.symvers" \
      EXTRA_CFLAGS='-DCONFIG_NET_9P_MODULE=1 -DCONFIG_NET_9P_VIRTIO_MODULE=1 -DCONFIG_9P_FS_MODULE=1' \
      modules; \
    install -d "/out/usr/lib/modules/${kver}/extra/9p"; \
    install -m 0644 \
      "${src}/net/9p/9pnet.ko" \
      "${src}/net/9p/9pnet_virtio.ko" \
      "${src}/fs/9p/9p.ko" \
      "/out/usr/lib/modules/${kver}/extra/9p/"; \
    rm -rf "${src}" /tmp/kernel.tar.gz; \
    dnf clean all; \
    rm -rf /var/cache/dnf /var/log/*

FROM ${CENTOS_BOOTC_IMAGE}

COPY --from=module-builder /out/ /
COPY --chmod=0755 yq /usr/bin/yq
COPY 50-sandbox-sysusers.conf /usr/lib/sysusers.d/50-sandbox.conf
COPY 50-sandbox-tmpfiles.conf /usr/lib/tmpfiles.d/50-sandbox.conf
COPY 90-sandbox-sudoers /etc/sudoers.d/90-sandbox
COPY 40-sandbox-sshd.conf /etc/ssh/sshd_config.d/40-sandbox.conf
COPY 50-sandbox-containers.conf /etc/containers/containers.conf.d/50-sandbox.conf
COPY sandbox-cloud-final-reset.service /usr/lib/systemd/system/sandbox-cloud-final-reset.service

RUN rpm -q openssh-server sudo && \
    dnf -y --setopt=install_weak_deps=False install buildah cloud-init podman && \
    dnf clean all && \
    rm -rf /var/cache/dnf /var/cache/ldconfig /var/lib/dnf /var/lib/cloud \
      /var/lib/rhsm /var/roothome/buildinfo /var/log/* /run/* /tmp/*

RUN visudo -cf /etc/sudoers.d/90-sandbox && \
    chmod 0440 /etc/sudoers.d/90-sandbox && \
    systemctl enable sshd.service sandbox-cloud-final-reset.service && \
    systemctl enable cloud-init-local.service cloud-config.service cloud-final.service && \
    if systemctl cat cloud-init-network.service >/dev/null 2>&1; then \
      systemctl enable cloud-init-network.service; \
    else \
      systemctl enable cloud-init.service; \
    fi && \
    systemctl set-default multi-user.target

RUN set -e; \
    ln -s /var/sandbox /sandbox; \
    ! grep -q '^\[root\]' /usr/lib/ostree/prepare-root.conf; \
    printf '\n[root]\ntransient = true\n' >>/usr/lib/ostree/prepare-root.conf; \
    kver="$(find /usr/lib/modules -mindepth 1 -maxdepth 1 -type d -printf '%f\n')"; \
    depmod "${kver}"; \
    modinfo -k "${kver}" 9p 9pnet 9pnet_virtio; \
    dracut --force --no-hostonly --reproducible \
      "/usr/lib/modules/${kver}/initramfs.img" "${kver}"; \
    rm -rf /var/log/* /var/tmp/* /var/cache/ldconfig /tmp/* /run/*

RUN bootc container lint --fatal-warnings
EOF

podman pull "${centosBootcImage}"
podman build \
  --isolation chroot \
  --build-arg "CENTOS_BOOTC_IMAGE=${centosBootcImage}" \
  --tag localhost/sandbox-centos-bootc:source \
  "${buildContext}"

# The Debian builder filesystem is temporary scratch. Installing over it leaves
# only the generic CentOS bootc deployment in the published distro image.
podman run --rm --privileged --pid=host \
  --volume /var/lib/containers:/var/lib/containers \
  --volume /dev:/dev \
  --volume /:/target \
  --security-opt label=type:unconfined_t \
  localhost/sandbox-centos-bootc:source \
  bootc install to-existing-root \
  --acknowledge-destructive \
  --disable-selinux

podman system reset --force
