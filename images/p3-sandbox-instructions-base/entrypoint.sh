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

# Check for directory mode (index.md present) first
if [ -f /instructions/index.md ]; then
	echo "Directory mode: found index.md"
	set -x
	cp -r /instructions/. "$TARGET_DIR/"
	set +x
	echo "Copied instructions directory to $TARGET_DIR/"
elif [ -f /instructions/task.en.md ]; then
	src=/instructions/task.en.md
	set -x
	cp "$src" "$TARGET_DIR/"
	set +x
	echo "Copied $(basename "$src") to $TARGET_DIR/"
elif [ -f /instructions/instructions.md ]; then
	src=/instructions/instructions.md
	set -x
	cp "$src" "$TARGET_DIR/"
	set +x
	echo "Copied $(basename "$src") to $TARGET_DIR/"
else
	echo "No index.md, task.en.md, or instructions.md found in /instructions" >&2
	exit 1
fi

exit 0
