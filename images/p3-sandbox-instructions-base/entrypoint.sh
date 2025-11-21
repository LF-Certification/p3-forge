#!/bin/bash

set -e

if [ $# -eq 0 ]; then
	echo "Error: No target dir specified"
	echo "Usage: $0 <target_dir>"
	exit 1
fi

TARGET_DIR="$1"

echo "Ensuring directory: $TARGET_DIR"
mkdir -p "$TARGET_DIR"

if [ -f /instructions/task.en.md ]; then
	src=/instructions/task.en.md
elif [ -f /instructions/instructions.md ]; then
	src=/instructions/instructions.md
else
	echo "No instructions.md or task.en.md found in /instructions" >&2
	exit 1
fi

set -x
cp "$src" "$TARGET_DIR/"
set +x
echo "Copied $(basename "$src") to $TARGET_DIR/"

exit 0
