# Container Image Build System
# Uses git-cliff for automated semantic versioning with namespaced tags

# Configuration
SHELL := /bin/bash
REGISTRY ?= ghcr.io/lf-certification

# Auto-discover all image directories
IMAGE_DIRS := $(shell find images -name Dockerfile -exec dirname {} \; | sort)

.DEFAULT_GOAL := help

##@ General & Setup

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: setup
setup: install-hooks ## Set up development environment (install pre-commit hooks)
	@echo "Setting up development environment..."
	pre-commit install
	pre-commit install --hook-type commit-msg
	@echo "✅ Development environment ready!"

.PHONY: install-hooks
install-hooks:
	pre-commit install
	pre-commit install --hook-type commit-msg
	@echo "✅ Pre-commit hooks installed"

##@ Development

.PHONY: init-image
init-image: ## Initialize an image directory (usage: make init-image <path> [version])
	@./scripts/init-image.sh $(filter-out $@,$(MAKECMDGOALS))

.PHONY: build
build: ## Build specific image (usage: make build <path>)
	@if [ -z "$(filter-out $@,$(MAKECMDGOALS))" ]; then \
		echo "Usage: make build <image-path>"; \
		echo "Example: make build images/alpine"; \
		exit 1; \
	fi
	$(eval IMAGE_PATH := $(filter-out $@,$(MAKECMDGOALS)))
	$(eval IMAGE_NAME := $(shell echo "$(IMAGE_PATH)" | sed 's|images/||' | sed 's|/|-|g'))
	@echo "Building $(REGISTRY)/$(IMAGE_NAME)..."
	docker build -t $(REGISTRY)/$(IMAGE_NAME):dev $(IMAGE_PATH)

.PHONY: run-hooks
run-hooks: ## Run all pre-commit hooks on all files
	pre-commit run --all-files --hook-stage pre-commit

.PHONY: tag
tag: ## Tag a specific image with changelog message (usage: make tag <path>)
	@if [ -z "$(filter-out $@,$(MAKECMDGOALS))" ]; then \
		echo "Usage: make tag <image-path>"; \
		echo "Example: make tag images/alpine"; \
		exit 1; \
	fi
	$(eval IMAGE_PATH := $(filter-out $@,$(MAKECMDGOALS)))
	@devbox run --quiet -- ./scripts/create-tag.sh "$(IMAGE_PATH)"

.PHONY: tag-internal
tag-internal:
	@devbox run --quiet -- ./scripts/create-tag.sh "$(IMAGE_PATH)"

.PHONY: preview-next-tag
preview-next-tag: ## Preview next tag for an image (usage: make preview-next-tag <path>)
	@if [ -z "$(filter-out $@,$(MAKECMDGOALS))" ]; then \
		echo "Usage: make preview-next-tag <image-path>"; \
		echo "Example: make preview-next-tag images/alpine"; \
		exit 1; \
	fi
	$(eval IMAGE_PATH := $(filter-out $@,$(MAKECMDGOALS)))
	$(eval TAG_PREFIX := $(shell echo "$(IMAGE_PATH)" | sed 's|images/||' | sed 's|/|-|g'))
	@echo "Next tag for $(IMAGE_PATH):"
	@git cliff --bump --unreleased --include-path "$(IMAGE_PATH)/**" --tag-pattern "$(TAG_PREFIX)-v*" 2>/dev/null || echo "No changes since last tag"

## CI/CD

## Detect which images have changed (usage: make ci-detect-changed-images [BASE_REF=origin/main])
.PHONY: ci-detect-changed-images
ci-detect-changed-images:
	@./scripts/detect-changed-images.sh $(BASE_REF)

## Detect which images have changed in JSON format (usage: make ci-detect-changed-images-json [BASE_REF=origin/main])
.PHONY: ci-detect-changed-images-json
ci-detect-changed-images-json:
	@./scripts/detect-changed-images.sh $(BASE_REF) --json

## Build dev image (usage: make ci-build-dev <path>)
.PHONY: ci-build-dev
ci-build-dev:
	@if [ -z "$(filter-out $@,$(MAKECMDGOALS))" ]; then \
		echo "Usage: make ci-build-dev <image-path>"; \
		echo "Example: make ci-build-dev images/alpine"; \
		exit 1; \
	fi
	$(eval IMAGE_PATH := $(filter-out $@,$(MAKECMDGOALS)))
	@./scripts/build-dev-image.sh "$(IMAGE_PATH)"

## Build and push dev image (usage: make ci-build-dev-push <path>)
.PHONY: ci-build-dev-push
ci-build-dev-push:
	@if [ -z "$(filter-out $@,$(MAKECMDGOALS))" ]; then \
		echo "Usage: make ci-build-dev-push <image-path>"; \
		echo "Example: make ci-build-dev-push images/alpine"; \
		exit 1; \
	fi
	$(eval IMAGE_PATH := $(filter-out $@,$(MAKECMDGOALS)))
	@./scripts/build-dev-image.sh "$(IMAGE_PATH)" --push

## Build dev images for all changed directories
.PHONY: ci-build-changed-dev
ci-build-changed-dev:
	@CHANGED_IMAGES=$$(./scripts/detect-changed-images.sh --json | jq -r '.changed_images[]' 2>/dev/null || true); \
	if [ -z "$$CHANGED_IMAGES" ]; then \
		echo "No changed images to build"; \
		exit 0; \
	fi; \
	echo "Building changed images:"; \
	for image in $$CHANGED_IMAGES; do \
		echo "  $$image"; \
		./scripts/build-dev-image.sh "$$image"; \
	done

## Build and push dev images for all changed directories
.PHONY: ci-build-changed-dev-push
ci-build-changed-dev-push:
	@CHANGED_IMAGES=$$(./scripts/detect-changed-images.sh --json | jq -r '.changed_images[]' 2>/dev/null || true); \
	if [ -z "$$CHANGED_IMAGES" ]; then \
		echo "No changed images to build"; \
		exit 0; \
	fi; \
	echo "Building and pushing changed images:"; \
	for image in $$CHANGED_IMAGES; do \
		echo "  $$image"; \
		./scripts/build-dev-image.sh "$$image" --push; \
	done

## Dynamic Image Targets (for tab completion)

# Generate dynamic build targets for each image directory
$(foreach dir,$(IMAGE_DIRS),$(eval build-$(subst /,-,$(subst images/,,$(dir))): ; @$(MAKE) build $(dir)))

# Generate dynamic tag targets for each image directory
$(foreach dir,$(IMAGE_DIRS),$(eval tag-$(subst /,-,$(subst images/,,$(dir))): ; @IMAGE_PATH="$(dir)" $(MAKE) --no-print-directory tag-internal))

# Generate dynamic preview targets for each image directory
$(foreach dir,$(IMAGE_DIRS),$(eval preview-$(subst /,-,$(subst images/,,$(dir))): ; @$(MAKE) preview-next-tag $(dir)))

# List all dynamic targets for reference
.PHONY: list-dynamic-targets
list-dynamic-targets:
	@echo "Build targets:"
	@$(foreach dir,$(IMAGE_DIRS),echo "  build-$(subst /,-,$(subst images/,,$(dir)))";)
	@echo "Tag targets:"
	@$(foreach dir,$(IMAGE_DIRS),echo "  tag-$(subst /,-,$(subst images/,,$(dir)))";)
	@echo "Preview targets:"
	@$(foreach dir,$(IMAGE_DIRS),echo "  preview-$(subst /,-,$(subst images/,,$(dir)))";)

# Prevent make from treating arguments as targets
%:
	@:
