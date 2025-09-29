#!/bin/bash
set -e

echo "=== Testing p3-sandbox-instructions-generator ==="
echo ""

# Clean previous test output
rm -rf test-output
mkdir -p test-output

# Build the Docker image
echo "Building Docker image..."
docker build -t p3-sandbox-instructions-generator:test .

# Run the container with test files
echo ""
echo "Running container with test markdown files..."
docker run --rm \
  -v "$(pwd)/test:/mnt/instructions/src:ro" \
  -v "$(pwd)/test-output:/tmp/instructions" \
  p3-sandbox-instructions-generator:test \
  /mnt/instructions/src /tmp/instructions

# Check if output was generated
echo ""
echo "Checking generated output..."
if [ -f "test-output/index.html" ]; then
    echo "✓ index.html generated successfully"
    echo ""
    echo "Generated files:"
    ls -la test-output/ | head -10
    echo ""
    echo "HTML content preview (first 20 lines):"
    head -20 test-output/index.html
    echo ""
    echo "=== Test PASSED ==="
else
    echo "✗ index.html not found - test FAILED"
    echo "Contents of test-output:"
    ls -la test-output/
    exit 1
fi
