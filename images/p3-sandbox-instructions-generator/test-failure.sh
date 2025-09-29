#!/bin/bash
set -e

echo "=== Testing p3-sandbox-instructions-generator failure case ==="
echo ""

# Clean previous test output
rm -rf test-output test-empty
mkdir -p test-output test-empty

# Build the Docker image
echo "Building Docker image..."
docker build -t p3-sandbox-instructions-generator:test .

# Run the container with empty directory (should fail)
echo ""
echo "Running container with empty directory (expecting failure)..."
if docker run --rm \
  -v "$(pwd)/test-empty:/mnt/instructions/src:ro" \
  -v "$(pwd)/test-output:/tmp/instructions" \
  p3-sandbox-instructions-generator:test \
  /mnt/instructions/src /tmp/instructions 2>&1 | grep -q "Error: instructions.md not found"; then
    echo "✓ Correctly failed with expected error message"
    echo "=== Test PASSED ==="
else
    echo "✗ Did not fail as expected"
    echo "=== Test FAILED ==="
    exit 1
fi
