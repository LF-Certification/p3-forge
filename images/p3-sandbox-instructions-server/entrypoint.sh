#!/bin/sh
set -e

# Check if HTML directory path is provided
if [ -z "$1" ]; then
    echo "Error: HTML directory path not provided"
    echo "Usage: docker run <image> /path/to/html/files"
    exit 1
fi

HTML_DIR="$1"

# Check if the directory exists
if [ ! -d "$HTML_DIR" ]; then
    echo "Error: Directory $HTML_DIR does not exist"
    exit 1
fi

# Copy HTML files to nginx directory
# Note: /usr/share/nginx/html should already exist with proper permissions
cp -r "$HTML_DIR"/. /usr/share/nginx/html

echo "Starting nginx with HTML directory: $HTML_DIR"
echo "Nginx will listen on port 8080"
exec nginx -g 'daemon off;'
