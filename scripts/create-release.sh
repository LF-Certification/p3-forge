#!/bin/bash
set -euo pipefail

# Create a release for the given path and update its changelog.
# Usage: ./scripts/create-release.sh [--dry-run] <path>
#
# Tag form depends on the path:
#   images/<name>  -> <name>-vX.Y.Z      (container image; see VERSIONING.md)
#   <other path>   -> <path>/vX.Y.Z      (Go module, e.g. cli/lfed/v0.1.0)
# The second form is what `go get module@vX.Y.Z` needs for a nested module.
# A first release has no tag to bump from; it uses $INITIAL_VERSION (default 0.1.0).

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
NEXT_VERSION=""
case "$IMAGE_PATH" in
    images/*)
        TAG_PREFIX="$(echo "$IMAGE_PATH" | sed 's|images/||' | sed 's|/|-|g')-v"
        IMAGE_NAME=$(echo "$IMAGE_PATH" | sed 's|images/||' | sed 's|/.*||')
        ;;
    *)
        TAG_PREFIX="${IMAGE_PATH%/}/v"
        IMAGE_NAME=$(basename "$IMAGE_PATH")
        if [ -z "$(git tag -l "${TAG_PREFIX}*")" ]; then
            NEXT_VERSION="${TAG_PREFIX}${INITIAL_VERSION:-0.1.0}"
        fi
        ;;
esac
export IMAGE_NAME
if [ -z "$NEXT_VERSION" ]; then
    NEXT_VERSION=$(git cliff --bumped-version --include-path "$IMAGE_PATH/**" --tag-pattern "${TAG_PREFIX}*")
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
