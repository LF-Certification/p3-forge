#!/bin/sh

if [ -z "$UI_CONFIG" ]; then
    echo "ERROR: UI_CONFIG env var not set"
    exit 1
fi

echo "Injecting UI configuration into HTML template..."

# Escape quotes for sed
escaped_config=$(echo "$UI_CONFIG" | sed 's/"/\\\\"/g')

find /usr/share/nginx/html/ -name "*.html" -type f | while read -r file; do
    echo "Processing: $file"
    sed "s|UI_CONFIG_PLACEHOLDER|$escaped_config|g" "$file" > "$file.tmp"
    mv "$file.tmp" "$file"
done

echo "Configuration injection complete. Starting server..."
exec nginx -g "daemon off;"
