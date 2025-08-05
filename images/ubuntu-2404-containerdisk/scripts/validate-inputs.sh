#!/bin/bash
set -euo pipefail

# Validate workflow inputs
echo "=== Validating Workflow Inputs ==="

# Validate registry format
if [[ ! "${REGISTRY}" =~ ^[a-zA-Z0-9._-]+(\.[a-zA-Z0-9._-]+)*$ ]]; then
  echo "ERROR: Invalid registry format: ${REGISTRY}"
  exit 1
fi

# Validate image tag format (no spaces, valid characters)
if [[ ! "${IMAGE_TAG}" =~ ^[a-zA-Z0-9._-]+$ ]]; then
  echo "ERROR: Invalid image tag format: ${IMAGE_TAG}"
  echo "Image tags must contain only alphanumeric characters, dots, underscores, and hyphens"
  exit 1
fi

# Check tag length (Docker has 128 char limit)
if [ ${#IMAGE_TAG} -gt 128 ]; then
  echo "ERROR: Image tag too long (${#IMAGE_TAG} chars, max 128): ${IMAGE_TAG}"
  exit 1
fi

echo "✅ All workflow inputs validated successfully"
echo "Registry: ${REGISTRY}"
echo "Image: ${IMAGE_NAME}:${IMAGE_TAG}"
