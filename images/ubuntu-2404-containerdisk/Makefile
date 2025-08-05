.PHONY: all build push clean help validate-inputs setup-environment setup-registry-auth extract-base-image modify-disk-image build-container-image push-image validate-image generate-summary check-requirements

##@ Build Targets

# Configuration
export REGISTRY ?= ghcr.io
export IMAGE_NAME ?= lf-certification/sandbox-images/ubuntu-2404-containerdisk
export IMAGE_TAG ?= latest
export WORK_DIR ?= build-workspace

# GitHub-like environment variables for local builds
export GITHUB_REPOSITORY ?= lf-certification/sandbox-images
export GITHUB_REF ?= refs/heads/main
export GITHUB_REF_NAME ?= main
export GITHUB_SHA ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
export GIT_SHA ?= $(GITHUB_SHA)
export GITHUB_RUN_ID ?= local
export GITHUB_EVENT_NAME ?= manual
export GITHUB_ACTOR ?= $(shell whoami)
export GITHUB_TOKEN ?=
export GITHUB_OUTPUT ?= /dev/null
export RUNNER_OS ?= $(shell uname -s)

# Registry credentials (optional)
export QUAY_USERNAME ?=
export QUAY_PASSWORD ?=
export DOCKERHUB_USERNAME ?=
export DOCKERHUB_TOKEN ?=

.PHONY: help
help: ## Show this help message
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
	@echo ""
	@echo "Environment variables:"
	@echo "  REGISTRY         Container registry (default: ghcr.io)"
	@echo "  IMAGE_NAME       Image name (default: lf-certification/sandbox-images/ubuntu-2404-containerdisk)"
	@echo "  IMAGE_TAG        Image tag (default: latest)"
	@echo "  WORK_DIR         Build workspace directory (default: build-workspace)"
	@echo "  GITHUB_TOKEN     GitHub token for GHCR authentication"
	@echo "  QUAY_USERNAME    Quay.io username"
	@echo "  QUAY_PASSWORD    Quay.io password"
	@echo "  DOCKERHUB_USERNAME Docker Hub username"
	@echo "  DOCKERHUB_TOKEN  Docker Hub token"
	@echo ""
	@echo "Examples:"
	@echo "  make all"
	@echo "  make build"
	@echo "  make push REGISTRY=quay.io/myorg"
	@echo "  make all REGISTRY=ghcr.io/myorg IMAGE_TAG=v1.0.0"

##@ Primary Targets

all: check-requirements validate-inputs setup-environment extract-base-image modify-disk-image build-container-image validate-image generate-summary ## Build complete containerdisk image

build: check-requirements validate-inputs setup-environment extract-base-image modify-disk-image build-container-image ## Build containerdisk image without validation

push: setup-registry-auth push-image ## Push the image to registry

build-and-push: build push ## Build and push in one step

##@ Individual Build Steps

validate-inputs: ## Validate build inputs and environment variables
	@echo "=== Validating Build Inputs ==="
	@./scripts/validate-inputs.sh

setup-environment: ## Setup build environment and install dependencies
	@echo "=== Setting Up Build Environment ==="
	@./scripts/setup-environment.sh

setup-registry-auth: ## Setup registry authentication
	@echo "=== Setting Up Registry Authentication ==="
	@./scripts/setup-registry-auth.sh

extract-base-image: ## Extract base containerdisk image
	@echo "=== Extracting Base Image ==="
	@./scripts/extract-base-image.sh

modify-disk-image: ## Modify disk image with overlay initramfs script
	@echo "=== Modifying Disk Image ==="
	@./scripts/modify-disk-image.sh

build-container-image: ## Build container image from modified disk
	@echo "=== Building Container Image ==="
	@./scripts/build-container-image.sh

push-image: ## Push container image to registry
	@echo "=== Pushing Container Image ==="
	@./scripts/push-image.sh

validate-image: ## Validate the built container image
	@echo "=== Validating Container Image ==="
	@./scripts/validate-image.sh

generate-summary: ## Generate build summary report
	@echo "=== Generating Build Summary ==="
	@./scripts/generate-summary.sh

##@ Utility Targets

check-requirements: ## Check if all required tools are available
	@echo "=== Checking Requirements ==="
	@echo "Checking for required tools..."
	@command -v podman >/dev/null 2>&1 || { echo "ERROR: podman is required but not installed"; exit 1; }
	@command -v qemu-img >/dev/null 2>&1 || { echo "ERROR: qemu-img is required but not installed"; exit 1; }
	@command -v jq >/dev/null 2>&1 || { echo "ERROR: jq is required but not installed"; exit 1; }
	@command -v git >/dev/null 2>&1 || { echo "WARNING: git is recommended for proper metadata"; }
	@echo "✅ All required tools are available"
	@echo ""
	@echo "Configuration:"
	@echo "  Registry: $(REGISTRY)"
	@echo "  Image: $(IMAGE_NAME):$(IMAGE_TAG)"
	@echo "  Work Directory: $(WORK_DIR)"

clean: ## Clean up build artifacts and workspace
	@echo "=== Cleaning Up Build Artifacts ==="
	@rm -rf $(WORK_DIR)
	@rm -f build-summary.md validation-results.txt
	@echo "✅ Cleanup completed"

clean-images: ## Remove built container images
	@echo "=== Cleaning Up Container Images ==="
	@podman rmi $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG) 2>/dev/null || true
	@if [ "$(IMAGE_TAG)" != "latest" ]; then \
		podman rmi $(REGISTRY)/$(IMAGE_NAME):latest 2>/dev/null || true; \
	fi
	@echo "✅ Container images cleaned"

clean-all: clean clean-images ## Clean everything including images

##@ Development and Testing

dev-build: ## Quick development build (skips validation)
	@$(MAKE) validate-inputs setup-environment extract-base-image modify-disk-image build-container-image

test-image: validate-image ## Test the built containerdisk image

show-config: ## Show current configuration
	@echo "Build Configuration:"
	@echo "  REGISTRY=$(REGISTRY)"
	@echo "  IMAGE_NAME=$(IMAGE_NAME)"
	@echo "  IMAGE_TAG=$(IMAGE_TAG)"
	@echo "  WORK_DIR=$(WORK_DIR)"
	@echo "  Full Image: $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)"
	@echo ""
	@echo "Environment:"
	@echo "  GITHUB_REPOSITORY=$(GITHUB_REPOSITORY)"
	@echo "  GITHUB_REF_NAME=$(GITHUB_REF_NAME)"
	@echo "  GITHUB_SHA=$(GITHUB_SHA)"
	@echo "  RUNNER_OS=$(RUNNER_OS)"

##@ CI/CD Integration

ci-build: ## CI build target (all steps including validation)
	@$(MAKE) all

ci-push: ## CI push target with authentication
	@$(MAKE) setup-registry-auth push-image

# Ensure scripts are executable
$(shell chmod +x scripts/*.sh)

# Default target
.DEFAULT_GOAL := help
