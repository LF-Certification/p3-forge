#!/bin/sh

# ui-entrypoint.sh - UI container entrypoint for P3 Sandbox UI
# This script substitutes the UI_CONFIG environment variable into the HTML template

if [ -z "$UI_CONFIG" ]; then
    echo "Warning: UI_CONFIG environment variable is not set, using default configuration"
    UI_CONFIG='{"tools": [{"id": "terminal", "title": "Terminal", "url": "/terminal/", "default": true}]}'
fi

echo "Injecting UI configuration into HTML template..."

# Escape quotes for sed (same approach as old-ui)
escaped_config=$(echo "$UI_CONFIG" | sed 's/"/\\\\"/g')

# Replace the placeholder in all HTML files in the dist directory
find /usr/share/caddy/dist -name "*.html" -type f | while read -r file; do
    echo "Processing: $file"
    sed "s|UI_CONFIG_PLACEHOLDER|$escaped_config|g" "$file" > "$file.tmp"
    mv "$file.tmp" "$file"
done

echo "Configuration injection complete. Starting Caddy..."

# Start Caddy with the provided configuration
exec caddy run --config /etc/caddy/Caddyfile --adapter caddyfile
