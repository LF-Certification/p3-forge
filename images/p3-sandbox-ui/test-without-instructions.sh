#!/bin/bash

set -e
echo "Building and testing p3-sandbox-ui with UI_CONFIG environment variable..."

cd "$(dirname "$0")"

echo "Building container..."
docker build -t p3-sandbox-ui-test .

echo -e "\n=== Test 1: Default VS Code configuration ==="
UI_CONFIG='{"config": {"version": "test-local-1", "defaultTool": "tool3", "expiresAt": "2026-07-26T01:00:00Z"}, "tools": [{"name": "tool1", "url": "https://relaxdiego.com/resume"}, {"name": "tool2", "url": "https://relaxdiego.com/ca-cert"}, {"name": "tool3", "url": "https://relaxdiego.com/2012/02/all-about-the-ecu.html"}]}'

docker run --rm --name ui-test \
  -p 8080:80 \
  -e "UI_CONFIG=$UI_CONFIG" \
  p3-sandbox-ui-test
