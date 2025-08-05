.PHONY: build push clean help

# Configuration
IMAGE_NAME ?= custom-ubuntu
IMAGE_TAG ?= 24.04
REGISTRY ?=
FULL_IMAGE = $(if $(REGISTRY),$(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG),$(IMAGE_NAME):$(IMAGE_TAG))

help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Environment variables:"
	@echo "  REGISTRY    Container registry (e.g., quay.io/myorg)"
	@echo "  IMAGE_NAME  Image name (default: custom-ubuntu)"
	@echo "  IMAGE_TAG   Image tag (default: 24.04)"
	@echo ""
	@echo "Examples:"
	@echo "  make build"
	@echo "  make push REGISTRY=quay.io/myorg"
	@echo "  make build push REGISTRY=quay.io/myorg IMAGE_TAG=v1.0.0"

build: ## Build the custom containerdisk
	@echo "Building containerdisk..."
	@if [ ! -f overlay-initramfs-script.sh ]; then \
		echo "Error: overlay-initramfs-script.sh not found"; \
		exit 1; \
	fi
	$(if $(REGISTRY),REGISTRY=$(REGISTRY)) ./build.sh
	@echo "Build complete: $(FULL_IMAGE)"

push: ## Push the image to registry (requires REGISTRY)
	@if [ -z "$(REGISTRY)" ]; then \
		echo "Error: REGISTRY must be set to push"; \
		echo "Usage: make push REGISTRY=quay.io/myorg"; \
		exit 1; \
	fi
	@echo "Pushing $(FULL_IMAGE)..."
	podman push $(FULL_IMAGE)
	@echo "Push complete"

build-and-push: build push ## Build and push in one step

clean: ## Clean up build artifacts
	@echo "Cleaning up..."
	@rm -rf build-workspace/
	@podman rmi $(IMAGE_NAME):$(IMAGE_TAG) 2>/dev/null || true
	@if [ -n "$(REGISTRY)" ]; then \
		podman rmi $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG) 2>/dev/null || true; \
	fi
	@echo "Cleanup complete"

test: ## Test the built containerdisk (requires kubectl and virtctl)
	@if [ -z "$(REGISTRY)" ]; then \
		echo "Error: REGISTRY must be set for testing"; \
		echo "Usage: make test REGISTRY=quay.io/myorg"; \
		exit 1; \
	fi
	@echo "Testing containerdisk with virtctl..."
	@echo "Creating test VM..."
	virtctl create vm test-custom-ubuntu \
		--instancetype=u1.medium \
		--preference=ubuntu \
		--volume-import=type:registry,url:docker://$(FULL_IMAGE),size:10Gi \
		--run-strategy=RerunOnFailure
	@echo "Test VM created. Check with: kubectl get vm test-custom-ubuntu"
	@echo "To delete test VM: kubectl delete vm test-custom-ubuntu"

check-requirements: ## Check if all required tools are available
	@echo "Checking requirements..."
	@command -v podman >/dev/null 2>&1 || { echo "Error: podman not found"; exit 1; }
	@command -v qemu-nbd >/dev/null 2>&1 || { echo "Error: qemu-nbd not found"; exit 1; }
	@command -v qemu-img >/dev/null 2>&1 || { echo "Error: qemu-img not found"; exit 1; }
	@sudo -n true 2>/dev/null || { echo "Error: sudo access required"; exit 1; }
	@[ -f overlay-initramfs-script.sh ] || { echo "Error: overlay-initramfs-script.sh not found"; exit 1; }
	@echo "All requirements satisfied"
