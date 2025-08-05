#!/bin/bash
set -euo pipefail

echo "=== Pushing containerdisk image ==="
cd "${WORK_DIR}"

IMAGE_FULL_NAME="${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"

# Check if image exists locally
if ! podman images --format "{{.Repository}}:{{.Tag}}" | grep -q "^${IMAGE_FULL_NAME}$"; then
  echo "ERROR: Image $IMAGE_FULL_NAME not found locally"
  podman images
  exit 1
fi

echo "Pushing image: $IMAGE_FULL_NAME"

# Push with retry logic
PUSH_ATTEMPTS=3
for attempt in $(seq 1 $PUSH_ATTEMPTS); do
  echo "Push attempt $attempt/$PUSH_ATTEMPTS"

  if podman push "$IMAGE_FULL_NAME"; then
    echo "✅ Push successful on attempt $attempt"
    PUSH_SUCCESS=true
    break
  else
    echo "❌ Push failed on attempt $attempt"
    if [ $attempt -lt $PUSH_ATTEMPTS ]; then
      echo "Retrying in 10 seconds..."
      sleep 10
    fi
  fi
done

if [ "${PUSH_SUCCESS:-false}" != "true" ]; then
  echo "ERROR: Failed to push after $PUSH_ATTEMPTS attempts"
  exit 1
fi

# Also push latest tag if on main branch
if [ "${GITHUB_REF:-}" = "refs/heads/main" ]; then
  LATEST_TAG="${REGISTRY}/${IMAGE_NAME}:latest"
  echo "Pushing latest tag: $LATEST_TAG"

  for attempt in $(seq 1 $PUSH_ATTEMPTS); do
    echo "Latest tag push attempt $attempt/$PUSH_ATTEMPTS"

    if podman push "$LATEST_TAG"; then
      echo "✅ Latest tag push successful on attempt $attempt"
      break
    else
      echo "❌ Latest tag push failed on attempt $attempt"
      if [ $attempt -lt $PUSH_ATTEMPTS ]; then
        echo "Retrying in 10 seconds..."
        sleep 10
      fi
    fi
  done
fi

echo ""
echo "=== Push Summary ==="
echo "Successfully pushed: $IMAGE_FULL_NAME"
if [ "${GITHUB_REF:-}" = "refs/heads/main" ]; then
  echo "Successfully pushed: ${REGISTRY}/${IMAGE_NAME}:latest"
fi

# Set output for downstream steps
echo "image-url=$IMAGE_FULL_NAME" >> "${GITHUB_OUTPUT:-/dev/null}"
