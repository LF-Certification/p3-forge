#!/bin/bash

# test-local.sh - Script to test the UI_CONFIG environment variable locally

set -e

echo "Building and testing p3-sandbox-ui with UI_CONFIG environment variable..."

# Change to the p3-sandbox-ui directory
cd "$(dirname "$0")"

# Build the container
echo "Building container..."
docker build -t p3-sandbox-ui-test .

# Test with different configurations
echo -e "\n=== Test 1: Default VS Code configuration ==="
UI_CONFIG='{"config": {"version": "test-local-1", "defaultTool": "vscode", "expiresAt": "2025-07-26T01:00:00Z"}, "tools": [{"name": "terminal", "url": "/terminal/"}, {"name": "terminal2", "url": "/terminal2/"}, {"name": "vscode", "url": "/vscode/"}]}'

docker run --rm -d --name ui-test \
  -p 8080:80 \
  -e "UI_CONFIG=$UI_CONFIG" \
  p3-sandbox-ui-test

echo "Container started. Testing at http://localhost:8080"
echo "Waiting for container to be ready..."
sleep 3

# Check if the UI_CONFIG was injected properly
echo "Checking if configuration was injected..."
curl -s http://localhost:8080 | grep -q "Terminal" && echo "✓ Configuration injection successful" || echo "✗ Configuration injection failed"

echo -e "\nTest complete. Container is running at http://localhost:8080"
echo "Press any key to stop the container and run the next test..."
read -n 1

# Stop the test container
docker stop ui-test

echo -e "\n=== Test 2: Terminal-only configuration ==="
UI_CONFIG='{"config": {"version": "test-local-2", "defaultTool": "terminal", "expiresAt": "2025-07-26T01:00:00Z"}, "tools": [{"name": "terminal", "url": "/terminal/"}]}'

docker run --rm -d --name ui-test2 \
  -p 8080:80 \
  -e "UI_CONFIG=$UI_CONFIG" \
  p3-sandbox-ui-test

echo "Container started with terminal-only config at http://localhost:8080"
sleep 3

# Check if the UI_CONFIG was injected properly
curl -s http://localhost:8080 | grep -q "Terminal" && echo "✓ Terminal-only configuration successful" || echo "✗ Terminal-only configuration failed"

echo -e "\nTest complete. Container is running at http://localhost:8080"
echo "Press any key to stop the container..."
read -n 1

# Stop the test container
docker stop ui-test2

echo -e "\n=== Running full dev environment ==="
echo "Starting full dev environment with docker-compose..."
cd dev
docker-compose up -d

echo -e "\nFull environment is running at http://localhost:80"
echo "Services:"
echo "  - Main UI: http://localhost:80"
echo "  - Terminal 1: http://localhost:7681"
echo "  - Terminal 2: http://localhost:7682"
echo "  - VS Code: http://localhost:8080"
echo ""
echo "Press any key to stop all services..."
read -n 1

# Stop dev environment
docker-compose down

echo "All tests complete!"
