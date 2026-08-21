#!/usr/bin/env bashp

bluefin_image="${BLUEFIN_IMAGE:?must be set}"
install::apt_packages podman uidmap slirp4netns passt nftables

# Non-interactive SSH sessions have no systemd user bus, so use cgroupfs for
# rootful and rootless Podman during the image build.
mkdir -p /etc/containers/containers.conf.d
cat >/etc/containers/containers.conf.d/50-sandbox.conf <<'EOF'
[engine]
cgroup_manager = "cgroupfs"
events_logger = "file"
EOF

case "$(uname -m)" in
  x86_64)
    yq_arch=amd64
    yq_sha256=0fb28c6680193c41b364193d0c0fc4a03177aecde51cfc04d506b1517158c2fb
    ;;
  aarch64)
    yq_arch=arm64
    yq_sha256=b7f7c991abe262b0c6f96bbcb362f8b35429cefd59c8b4c2daa4811f1e9df599
    ;;
  *)
    echo "unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

build_context="$(mktemp -d)"
trap 'rm -rf "${build_context}"' EXIT

curl --fail --location --silent --show-error \
  "https://github.com/mikefarah/yq/releases/download/v4.47.1/yq_linux_${yq_arch}" \
  --output "${build_context}/yq"
printf '%s  %s\n' "${yq_sha256}" "${build_context}/yq" | sha256sum --check
chmod 0755 "${build_context}/yq"

cat >"${build_context}/50-sandbox-sysusers.conf" <<'EOF'
g sudo -
u tux 60000 "Sandbox user" /var/home/tux /bin/bash
m tux sudo
EOF

cat >"${build_context}/50-sandbox-tmpfiles.conf" <<'EOF'
d /var/home/tux 0700 tux tux -
EOF

cat >"${build_context}/90-sandbox-sudoers" <<'EOF'
__omp_magic("sudo", "ALL=(ALL:ALL) NOPASSWD: ALL")
EOF

cat >"${build_context}/40-sandbox-sshd.conf" <<'EOF'
PasswordAuthentication no
PubkeyAuthentication yes
PermitRootLogin prohibit-password
EOF

cat >"${build_context}/Containerfile" <<'EOF'
ARG BLUEFIN_IMAGE
FROM ${BLUEFIN_IMAGE}

RUN rpm -q openssh-server sudo && \
    dnf -y --setopt=install_weak_deps=False install cloud-init && \
    dnf clean all && \
    rm -rf /var/cache/dnf

COPY --chmod=0755 yq /usr/bin/yq
COPY 50-sandbox-sysusers.conf /usr/lib/sysusers.d/50-sandbox.conf
COPY 50-sandbox-tmpfiles.conf /usr/lib/tmpfiles.d/50-sandbox.conf
COPY 90-sandbox-sudoers /etc/sudoers.d/90-sandbox
COPY 40-sandbox-sshd.conf /etc/ssh/sshd_config.d/40-sandbox.conf

RUN chmod 0440 /etc/sudoers.d/90-sandbox && \
    systemctl enable sshd.service && \
    systemctl enable cloud-init-local.service cloud-config.service cloud-final.service && \
    if systemctl cat cloud-init-network.service >/dev/null 2>&1; then \
      systemctl enable cloud-init-network.service; \
    else \
      systemctl enable cloud-init.service; \
    fi && \
    systemctl set-default multi-user.target

RUN bootc container lint --fatal-warnings
EOF

podman pull "${bluefin_image}"
podman build \
  --build-arg "BLUEFIN_IMAGE=${bluefin_image}" \
  --tag localhost/sandbox-bluefin-lts:source \
  "${build_context}"

# Replace the disposable builder OS on disk. The published p3-forge artifact
# therefore boots Bluefin from its first consumer-visible start.
podman run --rm --privileged --pid=host \
  -v /var/lib/containers:/var/lib/containers \
  -v /dev:/dev \
  -v /:/target \
  --security-opt label=type:unconfined_t \
  localhost/sandbox-bluefin-lts:source \
  bootc install to-existing-root \
    --acknowledge-destructive \
    --disable-selinux

# The deployment is fully imported into ostree; discard the builder's duplicate
# container storage before the VM disk is compressed and published.
podman system reset --force
