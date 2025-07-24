#!/bin/bash
set -euo pipefail

# Initialize a new image directory with Dockerfile and initial tag
# Usage: ./scripts/init-image.sh <image-path> [version]

if [ $# -lt 1 ] || [ $# -gt 2 ]; then
    # Detect if called via make
    if [[ "${MAKELEVEL:-}" ]]; then
        echo "Usage: make init-image <image-path> [version]"
        echo "Example: make init-image images/alpine v0.1.0"
        echo "Example: make init-image images/myapp v1.0.0"
    else
        echo "Usage: $0 <image-path> [version]"
        echo "Example: $0 images/alpine v0.1.0"
        echo "Example: $0 images/myapp v1.0.0"
    fi
    exit 1
fi

IMAGE_PATH="$1"
VERSION="${2:-v0.1.0}"
TAG_PREFIX=$(echo "$IMAGE_PATH" | sed 's|images/||' | sed 's|/|-|g')
IMAGE_NAME=$(echo "$TAG_PREFIX" | sed 's|-|/|g')
CHANGELOG="$IMAGE_PATH/CHANGELOG.md"

# Validate that the path is a direct subdirectory of images/
if [[ ! "$IMAGE_PATH" =~ ^images/[^/]+/?$ ]]; then
    echo "Error: Image path must be a direct subdirectory of images/"
    echo "Valid format: images/<image-name>"
    echo "Examples: images/alpine, images/ui, images/nginx"
    echo "Invalid: $IMAGE_PATH"
    exit 1
fi

# Check if directory already exists
if [ -d "$IMAGE_PATH" ]; then
    echo "Error: Directory $IMAGE_PATH already exists"
    exit 1
fi

# Check if tags already exist
if git tag -l | grep -q "$TAG_PREFIX-v"; then
    echo "Error: Tags for $TAG_PREFIX already exist:"
    git tag -l | grep "$TAG_PREFIX-v"
    exit 1
fi

echo "Creating directory: $IMAGE_PATH"
mkdir -p "$IMAGE_PATH"

DOCKERFILE_LABELS=$(cat << EOF
# OCI labels for package linking and metadata
LABEL org.opencontainers.image.source="https://github.com/lf-certification/sandbox-images"
LABEL org.opencontainers.image.title="$TAG_PREFIX"
LABEL org.opencontainers.image.description="Container image for $IMAGE_NAME"
EOF
)

# Create basic Dockerfile
echo "Creating basic Dockerfile..."
cat > "$IMAGE_PATH/Dockerfile" << EOF
# Basic Dockerfile - customize as needed
FROM alpine:3.19
$DOCKERFILE_LABELS

RUN apk update && \\
    apk add --no-cache ca-certificates && \\
    rm -rf /var/cache/apk/*

RUN addgroup -g 1001 appuser && \\
    adduser -D -u 1001 -G appuser appuser

USER appuser
WORKDIR /app

CMD ["/bin/sh"]
EOF

echo "Creating initial commit for $IMAGE_PATH"
git add "$IMAGE_PATH/"
git commit -m "feat: initialize $IMAGE_PATH"

echo "Updating $CHANGELOG..."
cat > "$CHANGELOG" << EOF
# Changelog
EOF
devbox run -q git-cliff \
    --unreleased \
    --prepend "$CHANGELOG" \
    --tag "$TAG_PREFIX-$VERSION" \
    --include-path "$IMAGE_PATH/**"
git add "$CHANGELOG"
if ! devbox run -q make runall-pre-commit > /dev/null; then
  git add "$CHANGELOG"
  echo "Pre-commit hooks made changes, continuing..."
fi
git commit -m "chore(release): release $TAG_PREFIX-$VERSION"

echo "Creating git tag: $TAG_PREFIX-$VERSION"
git tag "$TAG_PREFIX-$VERSION" -m "Tag $TAG_PREFIX-$VERSION"

echo ""
echo "✅ Initialized $IMAGE_PATH with tag $TAG_PREFIX-$VERSION"
echo "📝 Edit $IMAGE_PATH/Dockerfile to customize the image"
echo "🚀 Use 'make release $IMAGE_PATH' for future releases"
echo "🚀 Run 'git push origin --follow-tags' to publish this release"
echo ""
