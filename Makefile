# Container Image Build System
# Uses git-cliff for automated semantic versioning with namespaced releases

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
setup: install-hooks ## Set up your development environment
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

.PHONY: release
release: ## Prepare a new release of the given image (usage: make release <path>)
	@if [ -z "$(filter-out $@,$(MAKECMDGOALS))" ]; then \
		echo "Usage: make release <image-path>"; \
		echo "Example: make release images/alpine"; \
		exit 1; \
	fi
	$(eval IMAGE_PATH := $(filter-out $@,$(MAKECMDGOALS)))
	@devbox run --quiet -- ./scripts/create-release.sh "$(IMAGE_PATH)"

.PHONY: release-internal
release-internal:
	@devbox run --quiet -- ./scripts/create-release.sh "$(IMAGE_PATH)"

.PHONY: release-dry-run
release-dry-run: ## Show next release version for an image (usage: make release-dry-run <path>)
	@if [ -z "$(filter-out $@,$(MAKECMDGOALS))" ]; then \
		echo "Usage: make release-dry-run <image-path>"; \
		echo "Example: make release-dry-run images/alpine"; \
		exit 1; \
	fi
	$(eval IMAGE_PATH := $(filter-out $@,$(MAKECMDGOALS)))
	@devbox run --quiet -- ./scripts/create-release.sh --dry-run "$(IMAGE_PATH)"

.PHONY: release-dry-run-internal
release-dry-run-internal:
	@devbox run --quiet -- ./scripts/create-release.sh --dry-run "$(IMAGE_PATH)"

.PHONY: runall-pre-commit
runall-pre-commit: ## Run all pre-commit hooks on all files
	pre-commit run --all-files --hook-stage pre-commit

.PHONY: commit-graph
commit-graph: ## Show git commit graph for an image path (usage: make commit-graph <path>)
	@if [ -z "$(filter-out $@,$(MAKECMDGOALS))" ]; then \
		echo "Usage: make commit-graph <image-path>"; \
		echo "Example: make commit-graph images/alpine"; \
		exit 1; \
	fi
	$(eval IMAGE_PATH := $(filter-out $@,$(MAKECMDGOALS)))
	@git log --graph --oneline --decorate --color --format='%C(auto)%h %C(green)%as%C(auto) %C(dim)%an%C(auto)%d %s%C(reset)' -- "$(IMAGE_PATH)"

##@ CI/CD

## Detect which images have changed (usage: make ci-detect-changed-images [BASE_REF=origin/main])
.PHONY: ci-detect-changed-images
ci-detect-changed-images:
	@./scripts/ci/detect-changed-images.sh $(BASE_REF)

## Parse release tag (usage: make ci-parse-tag TAG_NAME=alpine-v1.0.0)
.PHONY: ci-parse-tag
ci-parse-tag:
	@if [ -z "$(TAG_NAME)" ]; then \
		echo "Usage: make ci-parse-tag TAG_NAME=<tag-name>"; \
		echo "Example: make ci-parse-tag TAG_NAME=alpine-v1.0.0"; \
		exit 1; \
	fi
	@./scripts/ci/parse-tag.sh "$(TAG_NAME)"

## Validate image directory and Dockerfile (usage: make ci-validate-image IMAGE_PATH=images/alpine)
.PHONY: ci-validate-image
ci-validate-image:
	@if [ -z "$(IMAGE_PATH)" ]; then \
		echo "Usage: make ci-validate-image IMAGE_PATH=<image-path>"; \
		echo "Example: make ci-validate-image IMAGE_PATH=images/alpine"; \
		exit 1; \
	fi
	@./scripts/ci/validate-image.sh "$(IMAGE_PATH)"

## Check if version is latest major (usage: make ci-check-latest-version CURRENT_MAJOR=1 IMAGE_NAME=alpine GITHUB_REPOSITORY=lf-certification/sandbox-images)
.PHONY: ci-check-latest-version
ci-check-latest-version:
	@if [ -z "$(CURRENT_MAJOR)" ] || [ -z "$(IMAGE_NAME)" ] || [ -z "$(GITHUB_REPOSITORY)" ]; then \
		echo "Usage: make ci-check-latest-version CURRENT_MAJOR=<major> IMAGE_NAME=<name> GITHUB_REPOSITORY=<repo>"; \
		echo "Example: make ci-check-latest-version CURRENT_MAJOR=1 IMAGE_NAME=alpine GITHUB_REPOSITORY=lf-certification/sandbox-images"; \
		exit 1; \
	fi
	@./scripts/ci/check-latest-version.sh "$(CURRENT_MAJOR)" "$(IMAGE_NAME)" "$(GITHUB_REPOSITORY)"

## Generate build summary (usage: make ci-build-summary IMAGE_NAME=alpine IMAGE_PATH=images/alpine VERSION=v1.0.0 GIT_TAG=alpine-v1.0.0 GIT_SHA=abc123 IS_LATEST_MAJOR=true)
.PHONY: ci-build-summary
ci-build-summary:
	@if [ -z "$(IMAGE_NAME)" ] || [ -z "$(IMAGE_PATH)" ] || [ -z "$(VERSION)" ] || [ -z "$(GIT_TAG)" ] || [ -z "$(GIT_SHA)" ] || [ -z "$(IS_LATEST_MAJOR)" ]; then \
		echo "Usage: make ci-build-summary IMAGE_NAME=<name> IMAGE_PATH=<path> VERSION=<version> GIT_TAG=<tag> GIT_SHA=<sha> IS_LATEST_MAJOR=<true|false>"; \
		echo "Example: make ci-build-summary IMAGE_NAME=alpine IMAGE_PATH=images/alpine VERSION=v1.0.0 GIT_TAG=alpine-v1.0.0 GIT_SHA=abc123 IS_LATEST_MAJOR=true"; \
		exit 1; \
	fi
	@./scripts/ci/build-summary.sh "$(IMAGE_NAME)" "$(IMAGE_PATH)" "$(VERSION)" "$(GIT_TAG)" "$(GIT_SHA)" "$(IS_LATEST_MAJOR)" "$(DOCKER_TAGS)" "$(DOCKER_LABELS)"

## Detect which images have changed in JSON format (usage: make ci-detect-changed-images-json [BASE_REF=origin/main])
.PHONY: ci-detect-changed-images-json
ci-detect-changed-images-json:
	@./scripts/ci/detect-changed-images.sh $(BASE_REF) --json

## Build dev image (usage: make ci-build-dev <path>)
.PHONY: ci-build-dev
ci-build-dev:
	@if [ -z "$(filter-out $@,$(MAKECMDGOALS))" ]; then \
		echo "Usage: make ci-build-dev <image-path>"; \
		echo "Example: make ci-build-dev images/alpine"; \
		exit 1; \
	fi
	$(eval IMAGE_PATH := $(filter-out $@,$(MAKECMDGOALS)))
	@./scripts/ci/build-dev-image.sh "$(IMAGE_PATH)"

## Build and push dev image (usage: make ci-build-dev-push <path>)
.PHONY: ci-build-dev-push
ci-build-dev-push:
	@if [ -z "$(filter-out $@,$(MAKECMDGOALS))" ]; then \
		echo "Usage: make ci-build-dev-push <image-path>"; \
		echo "Example: make ci-build-dev-push images/alpine"; \
		exit 1; \
	fi
	$(eval IMAGE_PATH := $(filter-out $@,$(MAKECMDGOALS)))
	@./scripts/ci/build-dev-image.sh "$(IMAGE_PATH)" --push

## Build dev images for all changed directories
.PHONY: ci-build-changed-dev
ci-build-changed-dev:
	@CHANGED_IMAGES=$$(./scripts/ci/detect-changed-images.sh --json | jq -r '.changed_images[]' 2>/dev/null || true); \
	if [ -z "$$CHANGED_IMAGES" ]; then \
		echo "No changed images to build"; \
		exit 0; \
	fi; \
	echo "Building changed images:"; \
	for image in $$CHANGED_IMAGES; do \
		echo "  $$image"; \
		./scripts/ci/build-dev-image.sh "$$image"; \
	done

## Build and push dev images for all changed directories
.PHONY: ci-build-changed-dev-push
ci-build-changed-dev-push:
	@CHANGED_IMAGES=$$(./scripts/ci/detect-changed-images.sh --json | jq -r '.changed_images[]' 2>/dev/null || true); \
	if [ -z "$$CHANGED_IMAGES" ]; then \
		echo "No changed images to build"; \
		exit 0; \
	fi; \
	echo "Building and pushing changed images:"; \
	for image in $$CHANGED_IMAGES; do \
		echo "  $$image"; \
		./scripts/ci/build-dev-image.sh "$$image" --push; \
	done

## Dynamic Image Targets (for tab completion)

# Extract image names from paths (e.g., images/alpine -> alpine)
IMAGE_NAMES := $(notdir $(IMAGE_DIRS))

# Define template for release targets
define RELEASE_TEMPLATE
release-$(1): ## Release $(1) image
	@$$(MAKE) release-internal IMAGE_PATH=images/$(1)
endef

# Define template for release-dry-run targets
define RELEASE_DRY_RUN_TEMPLATE
release-dry-run-$(1): ## Dry run release for $(1) image
	@$$(MAKE) release-dry-run-internal IMAGE_PATH=images/$(1)
endef

# Define template for commit-graph targets
define COMMIT_GRAPH_TEMPLATE
commit-graph-$(1): ## Show git commit graph for $(1) image
	@git log --graph --oneline --decorate --color --format='%C(auto)%h %C(green)%as%C(auto) %C(dim)%an%C(auto)%d %s%C(reset)' -- images/$(1)
endef

# Generate dynamic release targets (e.g., release-alpine, release-ui)
$(foreach name,$(IMAGE_NAMES),$(eval $(call RELEASE_TEMPLATE,$(name))))

# Generate dynamic release-dry-run targets (e.g., release-dry-run-alpine, release-dry-run-ui)
$(foreach name,$(IMAGE_NAMES),$(eval $(call RELEASE_DRY_RUN_TEMPLATE,$(name))))

# Generate dynamic commit-graph targets (e.g., commit-graph-alpine, commit-graph-ui)
$(foreach name,$(IMAGE_NAMES),$(eval $(call COMMIT_GRAPH_TEMPLATE,$(name))))

# Mark all dynamic targets as PHONY
.PHONY: $(addprefix release-,$(IMAGE_NAMES)) $(addprefix release-dry-run-,$(IMAGE_NAMES)) $(addprefix commit-graph-,$(IMAGE_NAMES))

.PHONY: list-dynamic-targets
list-dynamic-targets: ## List all dynamically generated targets
	@echo "Dynamic release targets:"
	@for name in $(IMAGE_NAMES); do \
		echo "  release-$$name"; \
	done
	@echo
	@echo "Dynamic release-dry-run targets:"
	@for name in $(IMAGE_NAMES); do \
		echo "  release-dry-run-$$name"; \
	done
	@echo
	@echo "Dynamic commit-graph targets:"
	@for name in $(IMAGE_NAMES); do \
		echo "  commit-graph-$$name"; \
	done
