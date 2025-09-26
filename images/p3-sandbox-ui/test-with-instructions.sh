#!/bin/bash

set -e
echo "Building and testing p3-sandbox-ui with sidebar layout..."

cd "$(dirname "$0")"

echo "Building container..."
docker build -t p3-sandbox-ui-test .

echo -e "\n=== Testing sidebar layout with instructions ==="
UI_CONFIG='{"config":{"version":"v1","defaultTool":"vm1-terminal","expiresAt":"2026-09-26T09:00:28Z","showTimer":true},"tools":[{"kind":"terminal","name":"vm1-terminal","url":"https://example.com/terminal"},{"kind":"instructions","name":"myinstructions","url":"https://example.com/instructions"}]}'

echo "Starting container with test configuration..."
echo "Instructions should appear in left sidebar, terminal in main area"
echo "Access at http://localhost:8080"
echo "Press Ctrl+C to stop"

docker run --rm --name ui-test-sidebar \
  -p 8080:80 \
  -e "UI_CONFIG=$UI_CONFIG" \
  p3-sandbox-ui-test
