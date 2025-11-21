#!/bin/sh

# entrypoint.sh - Sample hook runner for Sandbox Operator
# This script simulates hook execution based on the hook name passed as first argument

set -e

if [ $# -eq 0 ]; then
	echo "Error: No hook name provided"
	echo "Usage: $0 <hook-name> [additional-args...]"
	echo "Available hooks: setup, score"
	exit 1
fi

HOOK_NAME="$1"

# Validate hook name
case "$HOOK_NAME" in
setup)
	echo "Running setup hook..."
	/var/lib/p3-platform/hooks/setup.sh
	echo "Setup completed successfully"
	;;
score)
	echo "Running score hook..."
	/var/lib/p3-platform/hooks/score.sh
	echo "Scoring completed successfully"
	;;
*)
	echo "Error: Invalid hook name '$HOOK_NAME'"
	echo "Available hooks: setup, score"
	exit 1
	;;
esac

echo "Hook '$HOOK_NAME' finished"
exit 0
