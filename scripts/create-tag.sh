#!/bin/bash
set -euo pipefail

# Tag a release for the given image and update the changelog
# Usage: ./scripts/create-tag.sh <image-path>

if [ $# -ne 1 ]; then
    # Detect if called via make
    if [[ "${MAKELEVEL:-}" ]]; then
        echo "Usage: make tag <image-path>"
        echo "Example: make tag images/alpine"
        echo "Example: make tag images/ui"
    else
        echo "Usage: $0 <image-path>"
        echo "Example: $0 images/alpine"
        echo "Example: $0 images/ui"
    fi
    exit 1
fi

IMAGE_PATH="$1"
TAG_PREFIX=$(echo "$IMAGE_PATH" | sed 's|images/||' | sed 's|/|-|g')
NEXT_VERSION=$(git cliff --bumped-version --include-path "$IMAGE_PATH/**" --tag-pattern "$TAG_PREFIX-v*")
if [ -z "$NEXT_VERSION" ]; then
    echo "No changes detected for $IMAGE_PATH since last tag"
    exit 1
fi
GIT_TAG="$NEXT_VERSION"
CHANGELOG="CHANGELOG.md"

# Check if directory exists
if [ ! -d "$IMAGE_PATH" ]; then
    echo "Error: Directory $IMAGE_PATH does not exist"
    exit 1
fi

# Check if there are any uncommitted changes, fail if so
if ! git diff-index --quiet HEAD --; then
    echo "Error: There are uncommitted changes. Please commit or stash them before tagging."
    exit 1
fi

# Ensure changelog file exists
touch "$CHANGELOG"

# Update the changelog
  devbox run -q git-cliff \
      --unreleased \
      --tag "$GIT_TAG" \
      --prepend "$CHANGELOG" \
      --include-path "$IMAGE_PATH/**"
git add "$CHANGELOG"
if ! devbox run -q make run-hooks > /dev/null; then
  git add "$CHANGELOG"
  echo "Pre-commit hooks made changes, continuing..."
fi
git commit -m "chore: update "$CHANGELOG""

echo "Creating git tag: $GIT_TAG"
git tag "$GIT_TAG" -m "Tag $GIT_TAG"

echo ""
echo "✅ Updated $CHANGELOG"
echo "✅ Tagged $NEXT_VERSION"
echo "🚀 Push upstream to publish this version"
echo ""
