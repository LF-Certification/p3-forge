#!/bin/bash
set -euo pipefail

# Configuration
BASE_IMAGE="quay.io/containerdisks/ubuntu:24.04"
CUSTOM_IMAGE_NAME="custom-ubuntu"
CUSTOM_IMAGE_TAG="24.04"
REGISTRY="${REGISTRY:-}"
WORK_DIR="${PWD}/build-workspace"
SCRIPT_PATH="overlay-initramfs-script.sh"
DOCKER_CREDS_PATH="${DOCKER_CREDS_PATH:-/mnt/docker-creds}"

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
    log_info "Cleaning up..."

    # Unmount if mounted
    if mountpoint -q "${WORK_DIR}/mnt" 2>/dev/null; then
        sudo umount "${WORK_DIR}/mnt" || true
    fi

    # Disconnect NBD device
    if [ -b /dev/nbd0 ]; then
        sudo qemu-nbd --disconnect /dev/nbd0 || true
    fi

    # Remove temp containers
    podman rm temp-ubuntu 2>/dev/null || true

    log_info "Cleanup complete"
}

trap cleanup EXIT

setup_docker_credentials() {
    if [ -n "$REGISTRY" ] && [ -d "$DOCKER_CREDS_PATH" ]; then
        log_info "Setting up Docker credentials..."

        # Check for different credential formats
        if [ -f "$DOCKER_CREDS_PATH/.dockerconfigjson" ]; then
            # Kubernetes docker-registry secret format
            mkdir -p ~/.docker
            cp "$DOCKER_CREDS_PATH/.dockerconfigjson" ~/.docker/config.json
            log_info "Using Kubernetes docker-registry secret"
        elif [ -f "$DOCKER_CREDS_PATH/config.json" ]; then
            # Direct config.json mount
            mkdir -p ~/.docker
            cp "$DOCKER_CREDS_PATH/config.json" ~/.docker/config.json
            log_info "Using direct config.json"
        elif [ -f "$DOCKER_CREDS_PATH/username" ] && [ -f "$DOCKER_CREDS_PATH/password" ]; then
            # Username/password files
            local username=$(cat "$DOCKER_CREDS_PATH/username")
            local password=$(cat "$DOCKER_CREDS_PATH/password")
            local registry_host=$(echo "$REGISTRY" | cut -d'/' -f1)

            echo "$password" | podman login --username "$username" --password-stdin "$registry_host"
            log_info "Logged in using username/password files"
        else
            log_warn "Docker credentials found but format not recognized"
            log_warn "Available files: $(ls -la $DOCKER_CREDS_PATH)"
        fi
    elif [ -n "$REGISTRY" ]; then
        log_warn "Registry specified but no credentials found at $DOCKER_CREDS_PATH"
        log_warn "Push may fail if registry requires authentication"
    fi
}

check_requirements() {
    log_info "Checking requirements..."

    # Check if running as root or with sudo access
    if ! sudo -n true 2>/dev/null; then
        log_error "This script requires sudo access"
        exit 1
    fi

    # Check required tools
    for tool in podman qemu-nbd qemu-img; do
        if ! command -v "$tool" &> /dev/null; then
            log_error "$tool is not installed"
            exit 1
        fi
    done

    # Check if overlay script exists
    if [ ! -f "$SCRIPT_PATH" ]; then
        log_error "Overlay script not found: $SCRIPT_PATH"
        exit 1
    fi

    # Load required kernel modules
    sudo modprobe nbd max_part=8 || {
        log_error "Failed to load NBD module"
        exit 1
    }

    sudo modprobe dm-mod || {
        log_error "Failed to load device mapper module"
        exit 1
    }

    log_info "Requirements check passed"
}

prepare_workspace() {
    log_info "Preparing workspace..."

    # Create work directory
    mkdir -p "${WORK_DIR}"/{mnt,disk}
    cd "${WORK_DIR}"

    log_info "Workspace ready at ${WORK_DIR}"
}

extract_base_image() {
    log_info "Extracting base image..."

    # Pull base image
    podman pull "$BASE_IMAGE"

    # Extract disk image
    podman create --name temp-ubuntu "$BASE_IMAGE"
    podman cp temp-ubuntu:/disk/ubuntu-24.04.qcow2 ./base-disk.qcow2
    podman rm temp-ubuntu

    log_info "Base image extracted to base-disk.qcow2"
}

modify_disk_image() {
    log_info "Modifying disk image..."

    # Connect NBD device
    sudo qemu-nbd --connect=/dev/nbd0 base-disk.qcow2

    # Wait for device to be ready
    sleep 2

    # Mount the filesystem
    sudo mount /dev/nbd0p1 mnt/

    # Copy overlay script
    sudo cp "../${SCRIPT_PATH}" mnt/usr/share/initramfs-tools/scripts/init-bottom/
    sudo chmod +x mnt/usr/share/initramfs-tools/scripts/init-bottom/overlay-initramfs-script.sh

    log_info "Overlay script installed, rebuilding initramfs..."

    # Rebuild initramfs
    sudo chroot mnt/ /bin/bash -c "update-initramfs -u"

    # Unmount and disconnect
    sudo umount mnt/
    sudo qemu-nbd --disconnect /dev/nbd0

    log_info "Disk image modification complete"
}

build_containerdisk() {
    log_info "Building containerdisk..."

    # Copy modified disk to final location
    cp base-disk.qcow2 disk/custom-ubuntu-24.04.qcow2

    # Create Dockerfile
    cat > Dockerfile << 'EOF'
FROM scratch
COPY disk/custom-ubuntu-24.04.qcow2 /disk/
EOF

    # Build container image
    local image_tag="${CUSTOM_IMAGE_NAME}:${CUSTOM_IMAGE_TAG}"
    podman build -t "$image_tag" .

    log_info "Containerdisk built: $image_tag"

    # Tag and push if registry specified
    if [ -n "$REGISTRY" ]; then
        local registry_tag="${REGISTRY}/${image_tag}"
        podman tag "$image_tag" "$registry_tag"

        log_info "Pushing to registry..."
        podman push "$registry_tag"
        log_info "Pushed to registry: $registry_tag"
    fi
}

main() {
    log_info "Starting containerdisk build process..."

    check_requirements
    setup_docker_credentials
    prepare_workspace
    extract_base_image
    modify_disk_image
    build_containerdisk

    log_info "Containerdisk build complete!"

    if [ -n "$REGISTRY" ]; then
        log_info "Image available at: ${REGISTRY}/${CUSTOM_IMAGE_NAME}:${CUSTOM_IMAGE_TAG}"
    else
        log_info "Local image built: ${CUSTOM_IMAGE_NAME}:${CUSTOM_IMAGE_TAG}"
        log_warn "Set REGISTRY environment variable to push to a registry"
    fi
}

# Show usage if help requested
if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
    cat << 'EOF'
Usage: ./build.sh

Environment variables:
  REGISTRY           - Container registry to push to (optional)
                      Example: REGISTRY=quay.io/myorg ./build.sh
  DOCKER_CREDS_PATH  - Path to mounted Docker credentials (default: /mnt/docker-creds)
                      Example: DOCKER_CREDS_PATH=/mnt/my-creds ./build.sh

Requirements:
  - sudo access
  - podman, qemu-nbd, qemu-img installed
  - overlay-initramfs-script.sh in current directory
  - NBD and device mapper kernel modules available

The script will:
1. Extract the base Ubuntu 24.04 containerdisk
2. Inject the overlay initramfs script
3. Rebuild the initramfs
4. Create a new containerdisk image
5. Optionally push to registry if REGISTRY is set
EOF
    exit 0
fi

main "$@"
