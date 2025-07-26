#!/bin/bash
set -e

IMAGE_NAME="${1:-}"
IMAGE_PATH="${2:-}"
VERSION="${3:-}"
GIT_TAG="${4:-}"
GIT_SHA="${5:-}"
IS_LATEST_MAJOR="${6:-}"
DOCKER_TAGS="${7:-}"
DOCKER_LABELS="${8:-}"

if [ -z "$IMAGE_NAME" ] || [ -z "$IMAGE_PATH" ] || [ -z "$VERSION" ] || [ -z "$GIT_TAG" ] || [ -z "$GIT_SHA" ] || [ -z "$IS_LATEST_MAJOR" ]; then
    echo "Usage: $0 <image-name> <image-path> <version> <git-tag> <git-sha> <is-latest-major> [docker-tags] [docker-labels]"
    echo "Example: $0 alpine images/alpine v1.0.0 alpine-v1.0.0 abc123 true"
    exit 1
fi

OUTPUT_FILE="${GITHUB_STEP_SUMMARY:-/dev/stdout}"

{
    echo "## 🚀 Release Build Complete"
    echo ""
    echo "**Image:** \`$IMAGE_NAME\`"
    echo "**Source:** \`$IMAGE_PATH\`"
    echo "**Version:** \`$VERSION\`"
    echo "**Git Tag:** \`$GIT_TAG\`"
    echo "**Commit:** \`$GIT_SHA\`"
    echo "**Latest Tag Updated:** \`$IS_LATEST_MAJOR\`"
    echo ""

    if [ -n "$DOCKER_TAGS" ]; then
        echo "### 📦 Published Tags:"
        echo "\`\`\`"
        printf '%s\n' "$DOCKER_TAGS" | tr ',' '\n'
        echo "\`\`\`"
        echo ""
    fi

    if [ -n "$DOCKER_LABELS" ]; then
        echo "### 🏷️ Container Labels:"
        echo "\`\`\`"
        printf '%s\n' "$DOCKER_LABELS" | tr ',' '\n'
        echo "\`\`\`"
        echo ""
    fi

    if [ "$IS_LATEST_MAJOR" = "true" ]; then
        echo "✅ The \`latest\` tag now points to this version."
    else
        echo "ℹ️ The \`latest\` tag was not updated (newer major version exists)."
    fi
} >> "$OUTPUT_FILE"
