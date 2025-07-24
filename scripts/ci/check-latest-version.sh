#!/bin/bash
set -e

CURRENT_MAJOR="${1:-}"
IMAGE_NAME="${2:-}"
GITHUB_REPOSITORY="${3:-}"

if [ -z "$CURRENT_MAJOR" ] || [ -z "$IMAGE_NAME" ] || [ -z "$GITHUB_REPOSITORY" ]; then
    echo "Usage: $0 <current-major> <image-name> <github-repository>"
    echo "Example: $0 1 alpine lf-certification/sandbox-images"
    exit 1
fi

if [ -z "$GH_TOKEN" ]; then
    echo "Error: GH_TOKEN environment variable is required"
    exit 1
fi

# Get all tags for this image and find the highest major version
ALL_TAGS=$(gh api repos/$GITHUB_REPOSITORY/git/refs/tags \
  --jq '.[] | select(.ref | test("refs/tags/'$IMAGE_NAME'-v[0-9]+\\.[0-9]+\\.[0-9]+$")) | .ref' \
  | sed 's|refs/tags/'$IMAGE_NAME'-v||' \
  | sort -V)

if [ -z "$ALL_TAGS" ]; then
    echo "is-latest-major=true"
    echo "This is the first release for $IMAGE_NAME"
else
    LATEST_VERSION=$(echo "$ALL_TAGS" | tail -1)
    LATEST_MAJOR=$(echo "$LATEST_VERSION" | cut -d. -f1)

    if [ "$CURRENT_MAJOR" -ge "$LATEST_MAJOR" ]; then
        echo "is-latest-major=true"
        echo "This major version ($CURRENT_MAJOR) is >= latest ($LATEST_MAJOR)"
    else
        echo "is-latest-major=false"
        echo "This major version ($CURRENT_MAJOR) is < latest ($LATEST_MAJOR)"
    fi
fi
