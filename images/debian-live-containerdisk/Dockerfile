# syntax=docker/dockerfile:1.7
ARG BASE_IMAGE=debian:13-slim@sha256:a99cfc517144bc59b1978475ec53b46ecabec7e43635402ee5b77cc54cd1b20a
FROM ${BASE_IMAGE} AS builder
ARG TARGETARCH
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates cloud-init cpio curl dmidecode dmsetup dosfstools e2fsprogs \
    fdisk file gdisk iproute2 iputils-ping isc-dhcp-client kmod linux-image-${TARGETARCH} mtools \
    openssh-server parted psmisc qemu-utils rsync sudo systemd systemd-boot-efi \
    tar tcpdump udev util-linux vim-tiny wget xz-utils gzip bzip2 zstd && \
    apt-get clean && rm -rf /var/lib/apt/lists/* /var/cache/apt/*

COPY scripts/ /build/
RUN /build/make-initramfs.sh /initramfs.cpio.gz && \
    /build/make-disk.sh "$TARGETARCH" /initramfs.cpio.gz /disk.img && \
    qemu-img check /disk.img && \
    rm -rf /build /initramfs.cpio.gz

FROM scratch
COPY --from=builder --chown=107:107 /disk.img /disk/disk.img
