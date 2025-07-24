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

# In CI mode, use the before commit if available
if [ -n "${GITHUB_EVENT_BEFORE:-}" ] && [ "${GITHUB_EVENT_BEFORE}" != "0000000000000000000000000000000000000000" ]; then
    BASE_REF="${GITHUB_EVENT_BEFORE}"
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

# Extract unique image directories
CHANGED_IMAGES=$(echo "$CHANGED_FILES" | xargs -I {} dirname {} | grep "^images/" | sort -u)

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
