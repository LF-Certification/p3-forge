#!/bin/bash

# Script to set up bash completion for the project's Makefile

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPLETION_FILE="$SCRIPT_DIR/bash_completion"

echo "Setting up bash completion for Makefile targets..."

# Check if bash-completion is available
if ! command -v bash-completion >/dev/null 2>&1; then
    echo "ℹ️  bash-completion not found. Install it with:"
    echo "   macOS: brew install bash-completion"
    echo "   Ubuntu/Debian: sudo apt-get install bash-completion"
    echo "   CentOS/RHEL: sudo yum install bash-completion"
fi

# Source the completion file for the current session
if [ -f "$COMPLETION_FILE" ]; then
    echo "✅ Loading completion for current session..."
    source "$COMPLETION_FILE"
    echo "✅ Completion loaded!"
    echo ""
    echo "To make this permanent, add this line to your ~/.bashrc or ~/.bash_profile:"
    echo "source $COMPLETION_FILE"
    echo ""
    echo "Or run: echo 'source $COMPLETION_FILE' >> ~/.bashrc"
else
    echo "❌ Completion file not found at $COMPLETION_FILE"
    exit 1
fi

echo "Now try: make build-<TAB> or make tag-<TAB>"
echo ""
echo "💡 Note: If you're using devbox, completion is automatically loaded via devbox.json!"
