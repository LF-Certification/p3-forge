#!/bin/sh

# ui-entrypoint.sh - UI container entrypoint for Sandbox Operator
# This script substitutes the UI_CONFIG environment variable into the HTML template

if [ -z "$UI_CONFIG" ]; then
    echo "Error: UI_CONFIG environment variable is not set"
    exit 1
fi

echo "$UI_CONFIG"
escaped_config=$(echo "$UI_CONFIG" | sed 's/"/\\\\"/g')
sed "s|UI_CONFIG_PLACEHOLDER|$escaped_config|g" /usr/share/nginx/html/index.html > /tmp/index.html
mv /tmp/index.html /usr/share/nginx/html/index.html

exec nginx -g "daemon off;"
