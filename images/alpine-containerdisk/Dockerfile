FROM alpine AS builder

ARG ALPINE_VERSION=3.23
ARG TARGETARCH

RUN set -ex; \
    case "${TARGETARCH}" in \
        amd64) ARCH="x86_64" ;; \
        arm64) ARCH="aarch64" ;; \
        *) echo "Unsupported architecture: ${TARGETARCH}" && exit 1 ;; \
    esac; \
    BASE_URL="https://dl-cdn.alpinelinux.org/alpine/v${ALPINE_VERSION}/releases/cloud"; \
    LATEST=$(wget -qO- "${BASE_URL}/" | grep -oE "nocloud_alpine-[0-9.]+-${ARCH}-bios-tiny-r[0-9]+\.qcow2" | sort -V | tail -1); \
    wget -O /disk.qcow2 "${BASE_URL}/${LATEST}"

FROM scratch
COPY --from=builder --chown=107:107 /disk.qcow2 /disk/disk.qcow2
