#!/bin/bash
set -euo pipefail

# CI-optimized build script for GitHub Actions
# This is a streamlined version of build.sh optimized for CI environments

# Configuration
BASE_IMAGE="quay.io/containerdisks/ubuntu:24.04"
CUSTOM_IMAGE_NAME="${IMAGE_NAME:-custom-ubuntu}"
CUSTOM_IMAGE_TAG="${IMAGE_TAG:-24.04}"
REGISTRY="${REGISTRY:-}"
SCRIPT_PATH="overlay-initramfs-script.sh"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

cleanup() {
    log_info "Cleaning up CI environment..."

    # Unmount if mounted
    if mountpoint -q "./mnt" 2>/dev/null; then
        sudo umount "./mnt" || true
    fi

    # Disconnect NBD device
    if [ -b /dev/nbd0 ]; then
        sudo qemu-nbd --disconnect /dev/nbd0 || true
    fi

    # Remove temp containers
    podman rm temp-ubuntu 2>/dev/null || true
    podman rm test-container 2>/dev/null || true

    log_info "CI cleanup complete"
}

trap cleanup EXIT

check_ci_requirements() {
    log_info "Checking CI requirements..."

    # Check required tools (should be pre-installed in CI)
    for tool in podman qemu-nbd qemu-img; do
        if ! command -v "$tool" &> /dev/null; then
            log_error "$tool is not available in CI environment"
            exit 1
        fi
    done

    # Check if overlay script exists
    if [ ! -f "$SCRIPT_PATH" ]; then
        log_error "Overlay script not found: $SCRIPT_PATH"
        exit 1
    fi

    # Verify NBD module is loaded (should be pre-loaded in CI)
    if [ ! -b /dev/nbd0 ]; then
        log_error "NBD device not available. Module may not be loaded."
        exit 1
    fi

    # Check available disk space
    available_space=$(df . | awk 'NR==2 {print $4}')
    if [ "$available_space" -lt 5000000 ]; then  # 5GB in KB
        log_warn "Low disk space detected: $(df -h . | awk 'NR==2 {print $4}') available"
    fi

    log_info "CI requirements check passed"
}

extract_base_image() {
    log_info "Extracting base image in CI..."

    # Pull base image with retry logic
    local max_retries=3
    local retry=0

    while [ $retry -lt $max_retries ]; do
        if podman pull "$BASE_IMAGE"; then
            break
        else
            retry=$((retry + 1))
            log_warn "Pull attempt $retry failed, retrying..."
            sleep 5
        fi
    done

    if [ $retry -eq $max_retries ]; then
        log_error "Failed to pull base image after $max_retries attempts"
        exit 1
    fi

    # Extract disk image
    podman create --name temp-ubuntu "$BASE_IMAGE"
    podman cp temp-ubuntu:/disk/ubuntu-24.04.qcow2 ./base-disk.qcow2
    podman rm temp-ubuntu

    # Verify extracted image
    qemu-img info base-disk.qcow2

    log_info "Base image extracted successfully"
}

modify_disk_image() {
    log_info "Modifying disk image in CI environment..."

    # Create mount directory
    mkdir -p mnt

    # Connect NBD device with timeout
    if ! timeout 30 sudo qemu-nbd --connect=/dev/nbd0 base-disk.qcow2; then
        log_error "Failed to connect NBD device"
        exit 1
    fi

    # Wait for device to be ready with verification
    local wait_count=0
    while [ ! -b /dev/nbd0p1 ] && [ $wait_count -lt 10 ]; do
        sleep 1
        wait_count=$((wait_count + 1))
    done

    if [ ! -b /dev/nbd0p1 ]; then
        log_error "NBD partition device not ready"
        exit 1
    fi

    # Mount the filesystem
    sudo mount /dev/nbd0p1 mnt/

    # Verify mount and required directories
    if [ ! -d "mnt/usr/share/initramfs-tools/scripts/init-bottom" ]; then
        log_error "Expected initramfs directory not found in mounted image"
        exit 1
    fi

    # Copy overlay script
    sudo cp "$SCRIPT_PATH" mnt/usr/share/initramfs-tools/scripts/init-bottom/
    sudo chmod +x mnt/usr/share/initramfs-tools/scripts/init-bottom/overlay-initramfs-script.sh

    log_info "Overlay script installed, rebuilding initramfs..."

    # Rebuild initramfs with proper error handling
    if ! sudo chroot mnt/ /bin/bash -c "update-initramfs -u"; then
        log_error "Failed to rebuild initramfs"
        # Show some debug info
        sudo ls -la mnt/usr/share/initramfs-tools/scripts/init-bottom/
        exit 1
    fi

    # Verify the script was added
    if sudo ls mnt/usr/share/initramfs-tools/scripts/init-bottom/overlay-initramfs-script.sh > /dev/null 2>&1; then
        log_info "Overlay script successfully added to initramfs"
    else
        log_error "Overlay script not found after installation"
        exit 1
    fi

    # Unmount and disconnect
    sudo umount mnt/
    sudo qemu-nbd --disconnect /dev/nbd0

    log_info "Disk image modification complete"
}

build_containerdisk() {
    log_info "Building containerdisk in CI..."

    # Create final directory structure
    mkdir -p disk
    cp base-disk.qcow2 disk/custom-ubuntu-24.04.qcow2

    # Create optimized Dockerfile for CI
    cat > Dockerfile << EOF
FROM scratch
COPY disk/custom-ubuntu-24.04.qcow2 /disk/
LABEL org.opencontainers.image.title="Custom Ubuntu 24.04 ContainerDisk"
LABEL org.opencontainers.image.description="Ubuntu 24.04 with overlay initramfs for KubeVirt"
LABEL org.opencontainers.image.source="https://github.com/${GITHUB_REPOSITORY:-local}"
LABEL org.opencontainers.image.revision="${GITHUB_SHA:-local-build}"
LABEL org.opencontainers.image.created="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
EOF

    # Build container image
    local image_tag="${CUSTOM_IMAGE_NAME}:${CUSTOM_IMAGE_TAG}"
    podman build -t "$image_tag" .

    log_info "Containerdisk built: $image_tag"

    # Tag for registry if specified
    if [ -n "$REGISTRY" ]; then
        local registry_tag="${REGISTRY}/${image_tag}"
        podman tag "$image_tag" "$registry_tag"
        log_info "Tagged for registry: $registry_tag"
    fi
}

validate_image() {
    log_info "Validating built containerdisk..."

    local image_tag="${CUSTOM_IMAGE_NAME}:${CUSTOM_IMAGE_TAG}"
    if [ -n "$REGISTRY" ]; then
        image_tag="${REGISTRY}/${image_tag}"
    fi

    # Create test container to verify structure
    podman create --name test-container "$image_tag"

    # Verify disk file exists and extract it
    if podman cp test-container:/disk/custom-ubuntu-24.04.qcow2 ./test-disk.qcow2; then
        log_info "Disk file successfully extracted for validation"
        qemu-img info test-disk.qcow2

        # Verify it's a valid qcow2 image
        if qemu-img check test-disk.qcow2; then
            log_info "✅ Disk image validation passed"
        else
            log_error "❌ Disk image validation failed"
            exit 1
        fi

        # Check file size is reasonable (should be > 500MB)
        local file_size=$(stat -f%z test-disk.qcow2 2>/dev/null || stat -c%s test-disk.qcow2 2>/dev/null || echo "0")
        if [ "$file_size" -gt 500000000 ]; then
            log_info "✅ Disk image size validation passed: $(numfmt --to=iec $file_size)"
        else
            log_warn "⚠️  Disk image seems small: $(numfmt --to=iec $file_size)"
        fi
    else
        log_error "❌ Failed to extract disk file for validation"
        exit 1
    fi

    # Cleanup validation artifacts
    podman rm test-container
    rm -f test-disk.qcow2

    log_info "✅ Image validation completed successfully"
}

main() {
    log_info "Starting CI containerdisk build process..."

    check_ci_requirements
    extract_base_image
    modify_disk_image
    build_containerdisk
    validate_image

    log_info "🎉 CI containerdisk build completed successfully!"

    if [ -n "$REGISTRY" ]; then
        log_info "Image ready for push: ${REGISTRY}/${CUSTOM_IMAGE_NAME}:${CUSTOM_IMAGE_TAG}"
    else
        log_info "Local image built: ${CUSTOM_IMAGE_NAME}:${CUSTOM_IMAGE_TAG}"
    fi
}

# Show usage if help requested
if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
    cat << 'EOF'
Usage: ./build-ci.sh

CI-optimized build script for GitHub Actions

Environment variables:
  REGISTRY      - Container registry prefix (e.g., ghcr.io/owner)
  IMAGE_NAME    - Custom image name (default: custom-ubuntu)
  IMAGE_TAG     - Image tag (default: 24.04)
  GITHUB_REPOSITORY - GitHub repository name (for labels)
  GITHUB_SHA    - Git commit SHA (for labels)

Requirements (pre-installed in CI):
  - podman, qemu-nbd, qemu-img
  - NBD kernel module loaded
  - overlay-initramfs-script.sh in current directory
  - Sufficient disk space (5GB+)

This script is optimized for CI environments with:
  - Enhanced error handling and timeouts
  - Disk space monitoring
  - Retry logic for network operations
  - Comprehensive validation
  - Proper cleanup on exit
EOF
    exit 0
fi

main "$@"
