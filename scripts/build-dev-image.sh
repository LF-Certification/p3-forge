#!/bin/bash
set -euo pipefail

# Build a dev image with proper tagging
# Usage: ./scripts/build-dev-image.sh <image-path> [--push]

if [ $# -lt 1 ]; then
    echo "Usage: $0 <image-path> [--push]"
    echo ""
    echo "Build a dev image with proper dev tagging"
    echo ""
    echo "Arguments:"
    echo "  image-path  Path to image directory (e.g., images/alpine)"
    echo "  --push      Push to registry after building"
    echo ""
    echo "Environment variables:"
    echo "  REGISTRY         Container registry (default: ghcr.io)"
    echo "  REGISTRY_OWNER   Registry owner (default: lf-certification)"
        echo "  BRANCH_NAME      Branch name for tagging (auto-detected if not set)"
    echo "  SHORT_SHA        Short SHA for tagging (auto-detected if not set)"
    exit 1
fi

IMAGE_PATH="$1"
PUSH_IMAGE=false

# Parse arguments
for arg in "$@"; do
    case $arg in
        --push)
            PUSH_IMAGE=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 <image-path> [--push]"
            exit 0
            ;;
    esac
done

# Configuration with defaults
REGISTRY="${REGISTRY:-ghcr.io}"
REGISTRY_OWNER="${REGISTRY_OWNER:-lf-certification}"
# Convert registry owner to lowercase for container registry compatibility
REGISTRY_OWNER=$(echo "$REGISTRY_OWNER" | tr '[:upper:]' '[:lower:]')

# Validate image path exists
if [ ! -d "$IMAGE_PATH" ]; then
    echo "Error: Image path '$IMAGE_PATH' does not exist"
    exit 1
fi

if [ ! -f "$IMAGE_PATH/Dockerfile" ]; then
    echo "Error: No Dockerfile found in '$IMAGE_PATH'"
    exit 1
fi

# Generate image name from path (images/alpine -> alpine)
IMAGE_NAME=$(echo "$IMAGE_PATH" | sed 's|images/||' | sed 's|/|-|g')
FULL_IMAGE_NAME="${REGISTRY}/${REGISTRY_OWNER}/${IMAGE_NAME}"

# Get git information for tagging
BRANCH_NAME="${BRANCH_NAME:-$(git rev-parse --abbrev-ref HEAD | sed 's|/|-|g')}"
SHORT_SHA="${SHORT_SHA:-$(git rev-parse --short HEAD)}"

# Generate dev tags
DEV_TAG="${FULL_IMAGE_NAME}:dev-${BRANCH_NAME}-${SHORT_SHA}"
BRANCH_LATEST_TAG="${FULL_IMAGE_NAME}:dev-${BRANCH_NAME}-latest"

echo "Building dev image..."
echo "  Source: $IMAGE_PATH"
echo "  Image: $IMAGE_NAME"
echo "  Branch: $BRANCH_NAME"
echo "  SHA: $SHORT_SHA"
echo ""

# Build the image with multiple tags
TAGS="--tag $DEV_TAG --tag $BRANCH_LATEST_TAG"

# Add dev-latest tag only for main branch
if [ "$BRANCH_NAME" = "main" ]; then
    DEV_LATEST_TAG="${FULL_IMAGE_NAME}:dev-latest"
    TAGS="$TAGS --tag $DEV_LATEST_TAG"
    echo "  Tags: dev-${BRANCH_NAME}-${SHORT_SHA}, dev-${BRANCH_NAME}-latest, dev-latest"
else
    echo "  Tags: dev-${BRANCH_NAME}-${SHORT_SHA}, dev-${BRANCH_NAME}-latest"
fi

echo ""

# Get repository URL for linking package to repo (enables repository permission inheritance)
REPO_URL=$(git config --get remote.origin.url 2>/dev/null || echo "")
if [ -n "$REPO_URL" ]; then
    # Convert SSH URL to HTTPS format for GitHub
    REPO_URL=$(echo "$REPO_URL" | sed 's|git@github.com:|https://github.com/|' | sed 's|\.git$||')
fi

# Build the image
docker build $TAGS \
    --label "org.opencontainers.image.source=${REPO_URL}" \
    --label "org.opencontainers.image.revision=$(git rev-parse HEAD)" \
    --label "org.opencontainers.image.created=$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
    --label "org.opencontainers.image.description=Dev build of ${IMAGE_NAME} from ${BRANCH_NAME}" \
    --label "org.opencontainers.image.licenses=Proprietary" \
    --label "dev.lf-certification.image.path=$IMAGE_PATH" \
    --label "dev.lf-certification.build.branch=$BRANCH_NAME" \
    "$IMAGE_PATH"

echo "✅ Build complete!"

# Push if requested
if [ "$PUSH_IMAGE" = true ]; then
    echo ""
    echo "Pushing to registry..."
    docker push "$DEV_TAG"
    docker push "$BRANCH_LATEST_TAG"

    if [ "$BRANCH_NAME" = "main" ]; then
        docker push "$DEV_LATEST_TAG"
    fi

    echo "✅ Push complete!"
    echo ""
    echo "Published tags:"
    echo "  $DEV_TAG"
    echo "  $BRANCH_LATEST_TAG"
    if [ "$BRANCH_NAME" = "main" ]; then
        echo "  $DEV_LATEST_TAG"
    fi
fi
