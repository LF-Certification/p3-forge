#!/usr/bin/env bash
set -e

# Input parameters
MD_FILE="$1"
OUTPUT_HTML_FILE="$2"

if [ ! -f "$MD_FILE" ]; then
  echo "Error: Markdown file $MD_FILE does not exist."
  exit 1
fi

# Read markdown content
MARKDOWN_CONTENT=$(cat "$MD_FILE")

# Create JSON payload for GitHub API
JSON_PAYLOAD=$(jq -n --arg text "$MARKDOWN_CONTENT" '{"text": $text, "mode": "gfm"}')

# Send to GitHub API and save the rendered HTML
echo "Rendering markdown via GitHub API..."
curl -s -X POST \
  -H "Accept: application/vnd.github+json" \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  https://api.github.com/markdown \
  -d "$JSON_PAYLOAD" > "$OUTPUT_HTML_FILE"

# Check if the output file was created and has content
if [ ! -s "$OUTPUT_HTML_FILE" ]; then
  echo "Error: Failed to render markdown or output file is empty."
  exit 1
fi

echo "Successfully rendered markdown to $OUTPUT_HTML_FILE"
