#!/usr/bin/env bash
set -euo pipefail

readonly BLUEFIN_IMAGE="ghcr.io/projectbluefin/bluefin-lts@sha256:727e3f36eede8eca74bdbf76a258743b9b05681c1216232ea5bc54388869efda"
readonly YQ_VERSION="v4.47.1"
readonly DISK_SIZE="32G"
readonly OUTPUT_IMAGE="${1:?usage: build.sh <output-image>}"

case "$(uname -m)" in
  x86_64)
    arch=amd64
    yq_sha256=0fb28c6680193c41b364193d0c0fc4a03177aecde51cfc04d506b1517158c2fb
    ;;
  aarch64)
    arch=arm64
    yq_sha256=b7f7c991abe262b0c6f96bbcb362f8b35429cefd59c8b4c2daa4811f1e9df599
    ;;
  *)
    echo "unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
work_dir="$(mktemp -d)"
trap 'rm -rf "${work_dir}"' EXIT

cp "${script_dir}/Containerfile" "${script_dir}"/*.conf "${script_dir}/90-sandbox-sudoers" "${work_dir}/"
curl --fail --location --silent --show-error \
  "https://github.com/mikefarah/yq/releases/download/${YQ_VERSION}/yq_linux_${arch}" \
  --output "${work_dir}/yq"
printf '%s  %s\n' "${yq_sha256}" "${work_dir}/yq" | sha256sum --check
chmod 0755 "${work_dir}/yq"

podman pull "${BLUEFIN_IMAGE}"
podman build \
  --build-arg "BLUEFIN_IMAGE=${BLUEFIN_IMAGE}" \
  --file "${work_dir}/Containerfile" \
  --tag localhost/sandbox-bluefin-lts:source \
  "${work_dir}"

mkdir -p "${work_dir}/output"
truncate -s "${DISK_SIZE}" "${work_dir}/output/disk.raw"
podman run --rm --privileged --pid=host \
  --security-opt label=type:unconfined_t \
  -e BOOTC_SETENFORCE0_FALLBACK=1 \
  -v /dev:/dev \
  -v "${work_dir}/output:/output" \
  localhost/sandbox-bluefin-lts:source \
  bootc install to-disk \
    --via-loopback \
    --filesystem xfs \
    --generic-image \
    /output/disk.raw

qemu-img convert -f raw -O qcow2 -c \
  "${work_dir}/output/disk.raw" \
  "${work_dir}/output/disk.img"
qemu-img info "${work_dir}/output/disk.img"
rm "${work_dir}/output/disk.raw"

cat >"${work_dir}/Containerfile.containerdisk" <<'EOF'
FROM scratch
LABEL org.opencontainers.image.source="https://github.com/LF-Certification/p3-forge"
LABEL org.opencontainers.image.title="sandbox-vm-bluefin-lts"
COPY --chown=107:107 output/disk.img /disk/disk.img
EOF

podman build \
  --file "${work_dir}/Containerfile.containerdisk" \
  --tag "${OUTPUT_IMAGE}" \
  "${work_dir}"
podman push "${OUTPUT_IMAGE}"
