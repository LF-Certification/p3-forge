#!/bin/bash

# entrypoint.sh - Lifecycle hook runner for Sandbox Operator
# The hook name is passed as the first argument.

set -euo pipefail

timestamp_stream() {
	jq --raw-input --compact-output --unbuffered \
		'{timestamp: (now * 1000 | floor), message: .}'
}

timestamp_line() {
	printf '%s\n' "$1" | timestamp_stream
}

run_timestamped() {
	# Keep streams separate so log collectors retain stdout/stderr status.
	local stdout_fd stdout_pid stderr_fd stderr_pid
	local hook_status stdout_status stderr_status

	exec {stdout_fd}> >(timestamp_stream)
	stdout_pid=$!
	exec {stderr_fd}> >(timestamp_stream >&2)
	stderr_pid=$!

	# Collect every status without allowing a logging failure to mask the hook.
	set +e
	"$@" >&"$stdout_fd" 2>&"$stderr_fd"
	hook_status=$?
	exec {stdout_fd}>&-
	exec {stderr_fd}>&-
	wait "$stdout_pid"
	stdout_status=$?
	wait "$stderr_pid"
	stderr_status=$?
	set -e

	if ((hook_status != 0)); then
		return "$hook_status"
	fi
	if ((stdout_status != 0)); then
		return "$stdout_status"
	fi
	return "$stderr_status"
}

if [ $# -eq 0 ]; then
	timestamp_line "Error: No hook name provided" >&2
	timestamp_line "Usage: $0 <hook-name> [additional-args...]" >&2
	timestamp_line "Available hooks: setup, score" >&2
	exit 1
fi

HOOK_NAME="$1"

# Validate hook name
case "$HOOK_NAME" in
setup)
	timestamp_line "Running setup hook..."
	run_timestamped /var/lib/p3-platform/hooks/setup.sh
	timestamp_line "Setup completed successfully"
	;;
score)
	timestamp_line "Running score hook..."
	# The operator parses the score report directly from this output stream.
	/var/lib/p3-platform/hooks/score.sh
	timestamp_line "Scoring completed successfully"
	;;
*)
	timestamp_line "Error: Invalid hook name '$HOOK_NAME'" >&2
	timestamp_line "Available hooks: setup, score" >&2
	exit 1
	;;
esac

timestamp_line "Hook '$HOOK_NAME' finished"
exit 0
