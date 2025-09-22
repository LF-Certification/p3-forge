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

# Symlink or copy the HTML files to nginx default location
# Using symlink to avoid copying large directories
if [ -d "/usr/share/nginx/html" ]; then
    rm -rf /usr/share/nginx/html
fi
ln -s "$HTML_DIR" /usr/share/nginx/html

echo "Starting nginx with HTML directory: $HTML_DIR"
exec nginx -g 'daemon off;'
