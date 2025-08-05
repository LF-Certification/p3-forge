#!/bin/bash
set -euo pipefail

echo "Setting up authentication for registry: ${REGISTRY}"

# Determine registry type and authentication method
case "${REGISTRY}" in
  "ghcr.io")
    echo "Using GitHub token for GHCR authentication"
    echo "${GITHUB_TOKEN}" | podman login ${REGISTRY} -u ${GITHUB_ACTOR} --password-stdin
    ;;
  "quay.io")
    if [ -n "${QUAY_USERNAME:-}" ] && [ -n "${QUAY_PASSWORD:-}" ]; then
      echo "Using Quay.io credentials"
      echo "${QUAY_PASSWORD}" | podman login ${REGISTRY} -u "${QUAY_USERNAME}" --password-stdin
    else
      echo "WARNING: Quay.io credentials not found. Push may fail."
    fi
    ;;
  "docker.io")
    if [ -n "${DOCKERHUB_USERNAME:-}" ] && [ -n "${DOCKERHUB_TOKEN:-}" ]; then
      echo "Using Docker Hub credentials"
      echo "${DOCKERHUB_TOKEN}" | podman login ${REGISTRY} -u "${DOCKERHUB_USERNAME}" --password-stdin
    else
      echo "WARNING: Docker Hub credentials not found. Push may fail."
    fi
    ;;
  *)
    echo "Unknown registry. Attempting GitHub token authentication"
    echo "${GITHUB_TOKEN}" | podman login ${REGISTRY} -u ${GITHUB_ACTOR} --password-stdin || {
      echo "WARNING: Authentication failed. Push may not work."
    }
    ;;
esac
