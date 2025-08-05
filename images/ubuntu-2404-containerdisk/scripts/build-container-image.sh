#!/bin/bash
set -euo pipefail

echo "=== Building containerdisk container image ==="
cd "${WORK_DIR}"

# Verify the modified disk exists
if [ ! -f "base-disk.qcow2" ]; then
  echo "ERROR: Modified disk image not found"
  exit 1
fi

# Create final directory structure
echo "Setting up build directory..."
mkdir -p disk
cp base-disk.qcow2 disk/custom-ubuntu-24.04.qcow2

# Generate build metadata
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
DISK_SIZE=$(qemu-img info --output=json disk/custom-ubuntu-24.04.qcow2 | jq -r '."virtual-size"')
DISK_SIZE_MB=$((DISK_SIZE / 1024 / 1024))

echo "Build metadata:"
echo "  Build date: $BUILD_DATE"
echo "  Disk size: ${DISK_SIZE_MB}MB"
echo "  Git SHA: ${GIT_SHA:-unknown}"

# Create comprehensive Dockerfile with labels
{
  echo "FROM scratch"
  echo "COPY disk/custom-ubuntu-24.04.qcow2 /disk/"
  echo ""
  echo "# Standard OCI labels"
  echo "LABEL org.opencontainers.image.title=\"Custom Ubuntu 24.04 ContainerDisk\""
  echo "LABEL org.opencontainers.image.description=\"Ubuntu 24.04 with overlay initramfs for KubeVirt 2-layer architecture\""
  echo "LABEL org.opencontainers.image.source=\"https://github.com/${GITHUB_REPOSITORY:-unknown}\""
  echo "LABEL org.opencontainers.image.url=\"https://github.com/${GITHUB_REPOSITORY:-unknown}\""
  echo "LABEL org.opencontainers.image.documentation=\"https://github.com/${GITHUB_REPOSITORY:-unknown}/tree/main/images/ubuntu-2404-containerdisk\""
  echo "LABEL org.opencontainers.image.version=\"${IMAGE_TAG}\""
  echo "LABEL org.opencontainers.image.revision=\"${GIT_SHA:-unknown}\""
  echo "LABEL org.opencontainers.image.created=\"$BUILD_DATE\""
  echo "LABEL org.opencontainers.image.licenses=\"Apache-2.0\""
  echo ""
  echo "# Custom metadata labels"
  echo "LABEL io.kubevirt.containerdisk=\"ubuntu-24.04\""
  echo "LABEL io.kubevirt.containerdisk.overlay=\"true\""
  echo "LABEL io.kubevirt.containerdisk.architecture=\"2-layer\""
  echo "LABEL build.base-image=\"ubuntu:24.04-cloud-image\""
  echo "LABEL build.disk-size-mb=\"$DISK_SIZE_MB\""
  echo "LABEL build.workflow-run=\"${GITHUB_RUN_ID:-unknown}\""
  echo "LABEL build.trigger=\"${GITHUB_EVENT_NAME:-manual}\""
} > Dockerfile

echo "Generated Dockerfile:"
cat Dockerfile

# Build and tag image
IMAGE_NAME=$(echo "${IMAGE_NAME}" | tr '[:upper:]' '[:lower:]')
IMAGE_FULL_NAME="${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"
echo "Building image: $IMAGE_FULL_NAME"

podman build -t "$IMAGE_FULL_NAME" . || {
  echo "ERROR: Image build failed"
  exit 1
}

# Also tag as latest on main branch
if [ "${GITHUB_REF:-}" = "refs/heads/main" ]; then
  LATEST_TAG="${REGISTRY}/${IMAGE_NAME}:latest"
  echo "Tagging as latest: $LATEST_TAG"
  podman tag "$IMAGE_FULL_NAME" "$LATEST_TAG"
fi

echo "✅ Image built successfully: $IMAGE_FULL_NAME"

# Show image information
echo ""
echo "=== Image Information ==="
podman images | grep "${IMAGE_NAME}" || true

# Inspect the built image
echo ""
echo "=== Image Inspection ==="
podman inspect "$IMAGE_FULL_NAME" | jq -r '.[] | {Id: .Id, Size: .Size, Labels: .Config.Labels}'

echo ""
echo "=== Final disk space ==="
df -h
