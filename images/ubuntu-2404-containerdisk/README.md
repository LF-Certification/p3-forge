# ContainerDisk Build Scripts

This directory contains the individual scripts that make up the ContainerDisk build process. These scripts were extracted from the GitHub Actions workflow to improve maintainability and enable local development.

# PROOF OF CONCEPT

This is not going to be the final set up in the long run. It's an initial configuration to see if the concept works.

## Scripts Overview

| Script | Purpose | Dependencies |
|--------|---------|--------------|
| `validate-inputs.sh` | Validates environment variables and build inputs | None |
| `setup-environment.sh` | Installs required tools and sets up build environment | sudo, apt-get |
| `setup-registry-auth.sh` | Configures authentication for container registries | podman |
| `extract-base-image.sh` | Extracts base disk image from upstream containerdisk | podman, qemu-img |
| `modify-disk-image.sh` | Mounts disk and installs overlay initramfs script | sudo, qemu-nbd, mount |
| `build-container-image.sh` | Builds new containerdisk with modified disk image | podman, jq |
| `push-image.sh` | Pushes built image to container registry | podman |
| `validate-image.sh` | Comprehensive validation of built image | podman, qemu-img, sudo |
| `generate-summary.sh` | Generates build summary and metadata | podman, jq |

## Environment Variables

The scripts expect the following environment variables to be set:

### Required Variables
- `REGISTRY` - Container registry (e.g., ghcr.io, quay.io)
- `IMAGE_NAME` - Full image name including namespace
- `IMAGE_TAG` - Image tag to use
- `BASE_IMAGE` - Base containerdisk image to modify
- `WORK_DIR` - Build workspace directory

### GitHub Actions Variables
- `GITHUB_TOKEN` - GitHub token for GHCR authentication
- `GITHUB_ACTOR` - GitHub username
- `GITHUB_REPOSITORY` - Repository name
- `GITHUB_REF` - Git reference
- `GITHUB_SHA` - Git commit SHA
- `GITHUB_RUN_ID` - Workflow run ID
- `GITHUB_EVENT_NAME` - Event that triggered workflow
- `GITHUB_OUTPUT` - GitHub Actions output file

### Registry Authentication (Optional)
- `QUAY_USERNAME` / `QUAY_PASSWORD` - Quay.io credentials
- `DOCKERHUB_USERNAME` / `DOCKERHUB_TOKEN` - Docker Hub credentials

## Usage

### Via Makefile (Recommended)
```bash
cd images/ubuntu-2404-containerdisk
make all                    # Complete build process
make build                  # Build without validation
make push                   # Push to registry
make validate-image         # Validate built image
```

### Direct Script Execution
```bash
# Set required environment variables
export REGISTRY=ghcr.io
export IMAGE_NAME=myorg/ubuntu-containerdisk
export IMAGE_TAG=latest
export BASE_IMAGE=quay.io/containerdisks/ubuntu:24.04
export WORK_DIR=build-workspace

# Run scripts in sequence
./scripts/validate-inputs.sh
./scripts/setup-environment.sh
./scripts/extract-base-image.sh
./scripts/modify-disk-image.sh
./scripts/build-container-image.sh
./scripts/validate-image.sh
```

## Script Dependencies

### System Requirements
- Ubuntu/Debian Linux (for package installation)
- sudo access (for NBD operations and package installation)
- 10GB+ free disk space

### Required Tools
- `podman` - Container management
- `qemu-img` - Disk image manipulation
- `qemu-nbd` - Network Block Device
- `jq` - JSON processing
- `git` - Version control (optional, for metadata)

### Kernel Modules
- `nbd` - Network Block Device support
- `dm_mod` - Device mapper support

## Error Handling

All scripts include comprehensive error handling:
- Exit on any command failure (`set -euo pipefail`)
- Cleanup functions for resource management
- Detailed error messages and troubleshooting hints
- Retry logic for network operations

## Security Considerations

- Scripts validate all inputs before processing
- NBD devices are properly cleaned up on exit
- Credentials are passed via environment variables, not arguments
- No hardcoded secrets or sensitive information

## Local Development

For local development and testing:

1. Install required tools (see Makefile `check-requirements` target)
2. Set environment variables appropriately
3. Use `make dev-build` for faster iteration (skips validation)
4. Use `make clean` to remove build artifacts

## CI/CD Integration

The scripts are designed to work in both local and CI environments:
- GitHub Actions integration via the main workflow
- Support for different container registries
- Proper artifact generation for debugging
- Comprehensive logging and status reporting
