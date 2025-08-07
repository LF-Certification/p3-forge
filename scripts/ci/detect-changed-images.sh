#!/bin/bash
set -euo pipefail

# Detect which images have changed compared to a base reference
# Usage: ./scripts/detect-changed-images.sh [BASE_REF] [--json]

BASE_REF="${1:-origin/main}"
OUTPUT_FORMAT="text"

# Parse arguments
for arg in "$@"; do
    case $arg in
        --json)
            OUTPUT_FORMAT="json"
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [BASE_REF] [--json]"
            echo ""
            echo "Detect which images have changed compared to BASE_REF"
            echo ""
            echo "Arguments:"
            echo "  BASE_REF    Git reference to compare against (default: origin/main)"
            echo "  --json      Output results in JSON format"
            echo ""
            echo "Environment variables:"
            echo "  GITHUB_EVENT_BEFORE  Use this as BASE_REF if set (CI mode)"
            exit 0
            ;;
    esac
done

if [ -n "${GITHUB_EVENT_BEFORE:-}" ] && [ "${GITHUB_EVENT_BEFORE}" != "0000000000000000000000000000000000000000" ]; then
    # If set, then let's use that because that points to a commit sha before the current push event.
    BASE_REF="${GITHUB_EVENT_BEFORE}"
elif [ -n "${GITHUB_EVENT_BEFORE:-}" ] && [ "${GITHUB_EVENT_BEFORE}" == "0000000000000000000000000000000000000000" ]; then
    # The env var will be "000.000" in a PR or branch so we'll use a different approach:
    # Attempt to find the merge base (common ancestor) between the current BASE_REF (origin/main) and HEAD
    BASE_REF=$(git merge-base "${BASE_REF}" HEAD 2>/dev/null || echo "${BASE_REF}")
fi

# Get changed files in images/ directory
CHANGED_FILES=$(git diff --name-only "${BASE_REF}...HEAD" -- images/ 2>/dev/null || git ls-files images/)

if [ -z "$CHANGED_FILES" ]; then
    if [ "$OUTPUT_FORMAT" = "json" ]; then
        echo '{"has_changes": false, "changed_images": []}'
    else
        echo "No changes detected in images/"
    fi
    exit 0
fi

# Extract unique image directories (top-level only)
CHANGED_IMAGES=$(echo "$CHANGED_FILES" | sed 's|^images/\([^/]*\)/.*|images/\1|; s|^images/\([^/]*\)$|images/\1|' | grep "^images/" | sort -u)

if [ "$OUTPUT_FORMAT" = "json" ]; then
    # Output JSON format for CI consumption
    CHANGED_IMAGES_JSON=$(echo "$CHANGED_IMAGES" | jq -R -s -c 'split("\n")[:-1]')
    echo "{\"has_changes\": true, \"changed_images\": $CHANGED_IMAGES_JSON}"
else
    # Human-readable output
    echo "Changed files:"
    echo "$CHANGED_FILES"
    echo ""
    echo "Changed image directories:"
    echo "$CHANGED_IMAGES"
fi
