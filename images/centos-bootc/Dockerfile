ARG CENTOS_BOOTC_IMAGE=quay.io/centos-bootc/centos-bootc@sha256:f1f9f8029edbd7de0cb4e1520befebe0dbd5dbd173ff5722f0f95a80087cb3fd
FROM ${CENTOS_BOOTC_IMAGE} AS module-builder

ARG TARGETARCH
RUN set -eux; \
    targetArch="${TARGETARCH:-$(uname -m)}"; \
    case "${targetArch}" in \
      amd64|x86_64) yqArch=amd64; yqSha256=8e34fc298390875de416e6a4afcb8cabeceb25d9aa8506c1a2f9353cf702ea5f ;; \
      arm64|aarch64) yqArch=arm64; yqSha256=189088da0c6429ec5178dfaab1a114805f6cab0b61b165ab236efedf1d57a71b ;; \
      *) echo "unsupported architecture: ${targetArch}" >&2; exit 1 ;; \
    esac; \
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
    install -d "/out/usr/lib/modules/${kver}/extra/9p" /out/usr/bin; \
    install -m 0644 \
      "${src}/net/9p/9pnet.ko" \
      "${src}/net/9p/9pnet_virtio.ko" \
      "${src}/fs/9p/9p.ko" \
      "/out/usr/lib/modules/${kver}/extra/9p/"; \
    curl --fail --location --silent --show-error \
      "https://github.com/mikefarah/yq/releases/download/v4.54.1/yq_linux_${yqArch}" \
      --output /out/usr/bin/yq; \
    printf '%s  %s\n' "${yqSha256}" /out/usr/bin/yq | sha256sum --check; \
    chmod 0755 /out/usr/bin/yq; \
    rm -rf "${src}" /tmp/kernel.tar.gz; \
    dnf clean all; \
    rm -rf /var/cache/dnf /var/log/*

FROM ${CENTOS_BOOTC_IMAGE}

COPY --from=module-builder /out/ /
COPY root/ /

RUN rpm -q openssh-server sudo && \
    dnf -y --setopt=install_weak_deps=False install buildah cloud-init podman && \
    dnf clean all && \
    rm -rf /var/cache/dnf /var/cache/ldconfig /var/lib/dnf /var/lib/cloud \
      /var/lib/rhsm /var/roothome/buildinfo /var/log/* /run/* /tmp/*

RUN visudo -cf /etc/sudoers.d/90-sandbox && \
    chmod 0440 /etc/sudoers.d/90-sandbox && \
    systemctl enable sshd.service && \
    systemctl enable cloud-init-local.service cloud-config.service cloud-final.service && \
    if systemctl cat cloud-init-network.service >/dev/null 2>&1; then \
      systemctl enable cloud-init-network.service; \
    else \
      systemctl enable cloud-init.service; \
    fi && \
    systemctl set-default multi-user.target

RUN set -e; \
    ln -s /var/sandbox /sandbox; \
    if grep -q '^\[root\]' /usr/lib/ostree/prepare-root.conf; then \
      printf '%s\n' 'prepare-root.conf already contains a [root] section' >&2; \
      exit 1; \
    else \
      grepStatus=$?; \
      if [ "${grepStatus}" -ne 1 ]; then \
        printf 'failed to inspect %s (grep status %s)\n' /usr/lib/ostree/prepare-root.conf "${grepStatus}" >&2; \
        exit "${grepStatus}"; \
      fi; \
    fi; \
    printf '\n[root]\ntransient = true\n' >>/usr/lib/ostree/prepare-root.conf; \
    kver="$(find /usr/lib/modules -mindepth 1 -maxdepth 1 -type d -printf '%f\n')"; \
    depmod "${kver}"; \
    modinfo -k "${kver}" 9p 9pnet 9pnet_virtio; \
    dracut --force --no-hostonly --reproducible \
      "/usr/lib/modules/${kver}/initramfs.img" "${kver}"; \
    rm -rf /var/log/* /var/tmp/* /var/cache/ldconfig /tmp/* /run/*

RUN bootc container lint --fatal-warnings
