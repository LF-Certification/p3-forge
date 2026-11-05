#!/bin/bash
set -euo pipefail

# Detect which images have changed compared to a base reference
# Usage: ./scripts/detect-changed-images.sh [BASE_REF] [--json]

BASE_REF="origin/main"
OUTPUT_FORMAT="text"

# Parse arguments
for arg in "$@"; do
    case $arg in
        --json)
            OUTPUT_FORMAT="json"
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
        *)
            BASE_REF="$arg"
            ;;
    esac
done

# Select the base commit.
# A fast-forward push MUST compare against GITHUB_EVENT_BEFORE.
# Any other push (new branch, force push, unreachable sha) MUST compare
# against the merge base with BASE_REF. An unreachable sha fails the
# ancestor check, so a force push cannot fall through to a full rebuild.
if [ -n "${GITHUB_EVENT_BEFORE:-}" ] &&
    git merge-base --is-ancestor "${GITHUB_EVENT_BEFORE}" HEAD 2>/dev/null; then
    BASE_REF="${GITHUB_EVENT_BEFORE}"
else
    BASE_REF=$(git merge-base "${BASE_REF}" HEAD)
fi

# Get changed files in images/ directory. A failed diff MUST fail the
# script. Do not fall back to a full image list.
CHANGED_FILES=$(git diff --name-only "${BASE_REF}" HEAD -- images/)

if [ -z "$CHANGED_FILES" ]; then
    if [ "$OUTPUT_FORMAT" = "json" ]; then
        echo '{"has_changes": false, "changed_images": []}'
    else
        echo "No changes detected in images/"
    fi
    exit 0
fi

# Extract unique image directories (top-level only), excluding image roots
# deleted by the change under inspection.
CHANGED_IMAGES=$(
    echo "$CHANGED_FILES" |
        sed 's|^images/\([^/]*\)/.*|images/\1|; s|^images/\([^/]*\)$|images/\1|' |
        grep "^images/" |
        sort -u |
        while read -r image; do
            if [ -f "$image/Dockerfile" ]; then
                echo "$image"
            fi
        done
)

if [ -z "$CHANGED_IMAGES" ]; then
    if [ "$OUTPUT_FORMAT" = "json" ]; then
        echo '{"has_changes": false, "changed_images": []}'
    else
        echo "No buildable image changes detected in images/"
    fi
    exit 0
fi

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
