# Stage 1: Extract and resize the QCOW2 image
FROM ubuntu:24.04 AS builder

RUN apt-get update && apt-get install -y qemu-utils skopeo tar jq

RUN skopeo copy docker://quay.io/containerdisks/ubuntu:24.04 oci-archive:ubuntu-image.tar

RUN mkdir -p /image && tar -xf ubuntu-image.tar -C /image

RUN cd /image && \
    MANIFEST_HASH=$(jq -r '.manifests[0].digest' index.json | cut -d: -f2) && \
    LAYER_HASH=$(jq -r '.layers[0].digest' blobs/sha256/$MANIFEST_HASH | cut -d: -f2) && \
    tar -xzf blobs/sha256/$LAYER_HASH -C .

RUN DISK_FILE=$(find /image -type f -name 'disk.img' -print -quit) && \
    if [ -z "$DISK_FILE" ]; then \
        echo "Error: No disk.img file found in /image"; \
        ls -lR /image; \
        exit 1; \
    fi && \
    qemu-img info "$DISK_FILE" | grep -q "file format: qcow2" || { echo "Error: Disk image is not in QCOW2 format"; exit 1; } && \
    mv "$DISK_FILE" /original.qcow2

RUN qemu-img resize -f qcow2 /original.qcow2 16G

# Stage 2: Create the final containerdisk image
FROM scratch
COPY --from=builder --chown=107:107 /original.qcow2 /disk/disk.img
