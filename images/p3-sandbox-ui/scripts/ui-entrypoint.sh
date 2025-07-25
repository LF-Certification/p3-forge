#!/bin/sh

# ui-entrypoint.sh - UI container entrypoint for P3 Sandbox UI
# This script substitutes the UI_CONFIG environment variable into the HTML template

set -e

if [ -z "$UI_CONFIG" ]; then
    echo "Warning: UI_CONFIG environment variable is not set, using default configuration"
    UI_CONFIG='{"tools": [{"id": "terminal", "title": "Terminal", "url": "/terminal/", "default": true}]}'
fi

echo "Injecting UI configuration into HTML template..."

# Escape the JSON for safe sed replacement
# Handle quotes, backslashes, and other special characters
escaped_config=$(echo "$UI_CONFIG" | sed 's/\\/\\\\/g; s/"/\\"/g; s/\$/\\$/g; s/`/\\`/g')

# Replace the placeholder in all HTML files in the dist directory
find /usr/share/caddy/dist -name "*.html" -type f | while read -r file; do
    echo "Processing: $file"
    # Use perl for more reliable substitution
    perl -i -pe "s/UI_CONFIG_PLACEHOLDER/$escaped_config/g" "$file"
done

echo "Configuration injection complete. Starting Caddy..."

# Start Caddy with the provided configuration
exec caddy run --config /etc/caddy/Caddyfile --adapter caddyfile
