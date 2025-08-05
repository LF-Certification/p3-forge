#!/bin/bash
set -euo pipefail

echo "=== Build Summary Report ==="
cd "${WORK_DIR}"

# Gather build information
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
# Ensure IMAGE_NAME is lowercase for registry compatibility
IMAGE_NAME=$(echo "${IMAGE_NAME}" | tr '[:upper:]' '[:lower:]')
IMAGE_FULL_NAME="${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"

echo "## ContainerDisk Build Summary"
echo "**Build Date:** $BUILD_DATE"
echo "**Repository:** ${GITHUB_REPOSITORY:-unknown}"
echo "**Branch:** ${GITHUB_REF_NAME:-unknown}"
echo "**Commit:** ${GIT_SHA:-unknown}"
echo "**Trigger:** ${GITHUB_EVENT_NAME:-manual}"
echo "**Workflow Run:** ${GITHUB_RUN_ID:-unknown}"
echo ""

echo "### Image Details"
echo "**Registry:** ${REGISTRY}"
echo "**Image Name:** ${IMAGE_NAME}"
echo "**Image Tag:** ${IMAGE_TAG}"
echo "**Full Image URL:** \`$IMAGE_FULL_NAME\`"
echo ""

echo "### Base Image Information"
echo "**Base Image:** ${BASE_IMAGE}"
echo ""

# Disk image information if available
if [ -f "disk/custom-ubuntu-24.04.qcow2" ]; then
  echo "### Disk Image Details"
  DISK_INFO=$(qemu-img info --output=json disk/custom-ubuntu-24.04.qcow2)
  VIRTUAL_SIZE=$(echo "$DISK_INFO" | jq -r '."virtual-size"')
  ACTUAL_SIZE=$(echo "$DISK_INFO" | jq -r '."actual-size"')
  FORMAT=$(echo "$DISK_INFO" | jq -r '.format')

  echo "**Format:** $FORMAT"
  echo "**Virtual Size:** $((VIRTUAL_SIZE / 1024 / 1024)) MB"
  echo "**Actual Size:** $((ACTUAL_SIZE / 1024 / 1024)) MB"
  echo ""
fi

# Container image information if available
if podman images --format "{{.Repository}}:{{.Tag}}" | grep -q "^${IMAGE_FULL_NAME}$"; then
  echo "### Container Image Details"
  IMAGE_SIZE=$(podman images --format "{{.Size}}" "$IMAGE_FULL_NAME")
  IMAGE_ID=$(podman images --format "{{.ID}}" "$IMAGE_FULL_NAME")
  echo "**Image ID:** $IMAGE_ID"
  echo "**Container Size:** $IMAGE_SIZE"
  echo ""

  echo "### Image Labels"
  podman inspect "$IMAGE_FULL_NAME" | jq -r '.[].Config.Labels | to_entries[] | "**" + .key + ":** " + .value' | sort
  echo ""
fi

# System resource usage
echo "### Build Environment"
echo "**Runner OS:** ${RUNNER_OS:-$(uname -s)}"
echo "**Architecture:** $(uname -m)"
echo ""

echo "### Final Resource Usage"
echo "\`\`\`"
df -h | head -10
echo "\`\`\`"
echo ""

echo "### Usage Instructions"
echo "To use this ContainerDisk in a KubeVirt VM:"
echo "\`\`\`bash"
echo "virtctl create vm my-vm \\\\"
echo "  --instancetype=u1.medium \\\\"
echo "  --preference=ubuntu \\\\"
echo "  --volume-import=type:registry,url:docker://$IMAGE_FULL_NAME,size:10Gi \\\\"
echo "  --volume-pvc=pvc-name:my-overlay-pvc,size:20Gi"
echo "\`\`\`"
echo ""

# Save summary to file for artifact upload
{
  echo "# ContainerDisk Build Summary"
  echo "Generated: $BUILD_DATE"
  echo ""
  echo "## Build Information"
  echo "- Repository: ${GITHUB_REPOSITORY:-unknown}"
  echo "- Branch: ${GITHUB_REF_NAME:-unknown}"
  echo "- Commit: ${GIT_SHA:-unknown}"
  echo "- Trigger: ${GITHUB_EVENT_NAME:-manual}"
  echo "- Workflow Run: https://github.com/${GITHUB_REPOSITORY:-unknown}/actions/runs/${GITHUB_RUN_ID:-unknown}"
  echo ""
  echo "## Image Details"
  echo "- Full Image: $IMAGE_FULL_NAME"
  echo "- Base Image: ${BASE_IMAGE}"
  echo "- Registry: ${REGISTRY}"
} > ../build-summary.md

echo "✅ Build summary generated and saved to build-summary.md"
