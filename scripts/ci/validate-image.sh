#!/bin/bash
set -e

IMAGE_PATH="${1:-}"
if [ -z "$IMAGE_PATH" ]; then
    echo "Usage: $0 <image-path>"
    echo "Example: $0 images/alpine"
    exit 1
fi

if [ ! -d "$IMAGE_PATH" ]; then
    echo "Error: Image directory '$IMAGE_PATH' does not exist"
    exit 1
fi

if [ ! -f "$IMAGE_PATH/Dockerfile" ]; then
    echo "Error: Dockerfile not found in '$IMAGE_PATH'"
    exit 1
fi

echo "✅ Image directory and Dockerfile validated"
