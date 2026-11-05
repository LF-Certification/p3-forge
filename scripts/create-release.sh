#!/bin/bash
set -euo pipefail

# Create a release for the given image and update the changelog
# Usage: ./scripts/create-release.sh [--dry-run] <image-path>

# Parse arguments
DRY_RUN=false
IMAGE_PATH=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        -*)
            echo "Unknown option: $1" >&2
            exit 1
            ;;
        *)
            if [ -z "$IMAGE_PATH" ]; then
                IMAGE_PATH="$1"
            else
                echo "Error: Multiple image paths provided" >&2
                exit 1
            fi
            shift
            ;;
    esac
done

if [ -z "$IMAGE_PATH" ]; then
    echo "Usage: $0 [--dry-run] <image-path>"
    echo "Example: $0 images/alpine"
    echo "Example: $0 --dry-run images/ui"
    exit 1
fi
TAG_PREFIX=$(echo "$IMAGE_PATH" | sed 's|images/||' | sed 's|/|-|g')
IMAGE_NAME=$(echo "$IMAGE_PATH" | sed 's|images/||' | sed 's|/.*||')
export IMAGE_NAME
# git-cliff exits non-zero when no tag matches the pattern: it proposes a bare
# `0.1.0` and then rejects it against the pattern. A new image has no tag, so
# start it at 0.0.1 under its own prefix.
if [ -z "$(git tag --list "$TAG_PREFIX-v*")" ]; then
    NEXT_VERSION="$TAG_PREFIX-v0.0.1"
else
    NEXT_VERSION=$(git cliff --bumped-version --include-path "$IMAGE_PATH/**" --tag-pattern "$TAG_PREFIX-v*")
fi
if [ -z "$NEXT_VERSION" ]; then
    echo "No changes detected for $IMAGE_PATH since last release"
    exit 1
fi

# If dry-run, show the version and changelog preview
if [ "$DRY_RUN" = true ]; then
    echo "Version: $NEXT_VERSION"
    echo ""
    echo "Changelog preview:"
    echo "=================="
    IMAGE_NAME="$IMAGE_NAME" devbox run -q git-cliff \
      --tag "$NEXT_VERSION" \
      --include-path "$IMAGE_PATH/**" \
      --unreleased \
      --output -
    exit 0
fi

GIT_TAG="$NEXT_VERSION"
CHANGELOG="$IMAGE_PATH/CHANGELOG.md"

# Check if directory exists
if [ ! -d "$IMAGE_PATH" ]; then
    echo "Error: Directory $IMAGE_PATH does not exist"
    exit 1
fi

# Check if there are any uncommitted changes, fail if so
if ! git diff-index --quiet HEAD --; then
    echo "Error: There are uncommitted changes. Please commit or stash them before releasing."
    exit 1
fi

# Update the changelog
IMAGE_NAME="$IMAGE_NAME" devbox run -q git-cliff \
  --tag "$GIT_TAG" \
  --include-path "$IMAGE_PATH/**" \
  --output "$CHANGELOG"
git add "$CHANGELOG"
if ! devbox run -q make runall-pre-commit > /dev/null; then
  git add "$CHANGELOG"
  echo "Pre-commit hooks made changes, continuing..."
fi
git commit -m "chore(release): release $NEXT_VERSION"

echo "Creating git tag: $GIT_TAG"
git tag "$GIT_TAG" -m "Release $GIT_TAG"

echo ""
echo "✅ Updated $CHANGELOG"
echo "✅ Created git tag $NEXT_VERSION"
echo "🚀 Run 'git push origin --follow-tags' to publish this release"
echo ""
