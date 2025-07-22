#!/bin/bash
set -euo pipefail

# Create annotated git tag with changelog as message body
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

# Calculate next version using git-cliff (extract just the version number)
# Override output config to get stdout for processing
NEXT_VERSION=$(git cliff --bump --unreleased --include-path "$IMAGE_PATH/**" --tag-pattern "$TAG_PREFIX-v*" --output - 2>/dev/null | grep -oE "$TAG_PREFIX-v[0-9]+\.[0-9]+\.[0-9]+" | head -1 | sed "s/^$TAG_PREFIX-v//")

if [ -z "$NEXT_VERSION" ]; then
    echo "No changes detected for $IMAGE_PATH since last tag"
    exit 1
fi

GIT_TAG="$TAG_PREFIX-v$NEXT_VERSION"

# Generate changelog content and clean it up
# Override output config to get stdout for tag message
CHANGELOG_RAW=$(git cliff --bump --unreleased --include-path "$IMAGE_PATH/**" --tag-pattern "$TAG_PREFIX-v*" --output - 2>/dev/null || echo "")

TAG_MESSAGE="$CHANGELOG_RAW"

# Generate changelog file before tagging (respects output config from cliff.toml)
echo "Generating changelog..."
if [ -f CHANGELOG.md ]; then
    git cliff --bump --tag-pattern "$TAG_PREFIX-v*" --include-path "$IMAGE_PATH/**" --tag "$GIT_TAG" --prepend CHANGELOG.md --output /dev/null 2>/dev/null
else
    git cliff --bump --tag-pattern "$TAG_PREFIX-v*" --include-path "$IMAGE_PATH/**" --tag "$GIT_TAG" --output CHANGELOG.md 2>/dev/null
fi

# Commit changelog if it was updated
if git diff --quiet CHANGELOG.md 2>/dev/null; then
    echo "No changelog changes to commit"
else
    echo "Committing changelog..."
    git add CHANGELOG.md
    git commit -m "chore: update changelog for $GIT_TAG"
fi

# Create annotated tag with cleaned changelog as message
if [ -n "$TAG_MESSAGE" ]; then
    git -c core.commentChar="%" tag -a "$GIT_TAG" -m "$TAG_MESSAGE"
    echo "✅ Created annotated tag $GIT_TAG with changelog message"
else
    git tag -a "$GIT_TAG" -m "Release $GIT_TAG"
    echo "✅ Created annotated tag $GIT_TAG with default message"
fi
