#!/bin/bash
set -e

TAG_NAME="${1:-}"
if [ -z "$TAG_NAME" ]; then
    echo "Usage: $0 <tag-name>"
    echo "Example: $0 alpine-v1.0.0"
    exit 1
fi

echo "Processing tag: $TAG_NAME"

# Extract image name and version from tag (e.g., p3-sandbox-test-image3-v1.0.0 -> p3-sandbox-test-image3, v1.0.0)
if [[ $TAG_NAME =~ ^(.+)-v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
    IMAGE_NAME="${BASH_REMATCH[1]}"
    MAJOR="${BASH_REMATCH[2]}"
    MINOR="${BASH_REMATCH[3]}"
    PATCH="${BASH_REMATCH[4]}"
    VERSION="v${MAJOR}.${MINOR}.${PATCH}"
    IMAGE_PATH="images/${IMAGE_NAME}"

    echo "image-name=$IMAGE_NAME"
    echo "image-path=$IMAGE_PATH"
    echo "version=$VERSION"
    echo "major=$MAJOR"
    echo "minor=$MINOR"
    echo "patch=$PATCH"

    echo "Parsed tag successfully:"
    echo "  Image: $IMAGE_NAME"
    echo "  Path: $IMAGE_PATH"
    echo "  Version: $VERSION"
else
    echo "Error: Invalid tag format. Expected format: <image>-v<major>.<minor>.<patch>"
    exit 1
fi
