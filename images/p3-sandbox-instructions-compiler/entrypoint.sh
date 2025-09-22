#!/bin/sh
set -e

# Function to display usage
usage() {
    echo "Usage: $0 <source_path> <target_path>"
    echo ""
    echo "Arguments:"
    echo "  source_path  Path to the directory containing markdown files"
    echo "  target_path  Path where the compiled static site will be generated"
    echo ""
    echo "Environment Variables (optional):"
    echo "  HUGO_THEME        Hugo theme to use (default: uses embedded minimal theme)"
    echo "  HUGO_BASE_URL     Base URL for the site (default: /)"
    echo "  HUGO_TITLE        Site title (default: Documentation)"
    echo "  HUGO_PARAMS       Additional Hugo parameters as JSON string"
    exit 1
}

# Check arguments
if [ $# -ne 2 ]; then
    echo "Error: Exactly 2 arguments required"
    usage
fi

SOURCE_PATH="$1"
TARGET_PATH="$2"

# Validate source path exists
if [ ! -d "$SOURCE_PATH" ]; then
    echo "Error: Source path does not exist or is not a directory: $SOURCE_PATH"
    exit 1
fi

# Create target path if it doesn't exist
mkdir -p "$TARGET_PATH"

echo "Starting Hugo static site compilation..."
echo "  Source path: $SOURCE_PATH"
echo "  Target path: $TARGET_PATH"

# Create a temporary Hugo site
TEMP_SITE="/tmp/hugo-site-$$"
mkdir -p "$TEMP_SITE"

# Initialize Hugo site structure
cd "$TEMP_SITE"

# Create Hugo configuration
cat > config.toml <<EOF
baseURL = "${HUGO_BASE_URL:-/}"
title = "${HUGO_TITLE:-Documentation}"
languageCode = "en-us"
theme = ""

[markup]
  defaultMarkdownHandler = "goldmark"
  [markup.goldmark]
    [markup.goldmark.renderer]
      unsafe = true
  [markup.highlight]
    style = "monokai"
    lineNos = false
    tabWidth = 4

[params]
  description = "Compiled documentation from markdown files"
EOF

# Add custom parameters if provided
if [ -n "$HUGO_PARAMS" ]; then
    echo "$HUGO_PARAMS" >> config.toml
fi

# Create basic directory structure
mkdir -p content
mkdir -p layouts/_default
mkdir -p static
mkdir -p themes

# Create a minimal theme if no theme is specified
if [ -z "$HUGO_THEME" ]; then
    echo "Using embedded minimal theme..."

    # Create base template
    cat > layouts/_default/baseof.html <<'EOF'
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{ .Title }} | {{ .Site.Title }}</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
        }
        header {
            border-bottom: 2px solid #e7e7e7;
            margin-bottom: 30px;
            padding-bottom: 10px;
        }
        nav ul {
            list-style: none;
            padding: 0;
        }
        nav li {
            display: inline;
            margin-right: 20px;
        }
        nav a {
            color: #0366d6;
            text-decoration: none;
        }
        nav a:hover {
            text-decoration: underline;
        }
        h1, h2, h3, h4, h5, h6 {
            margin-top: 24px;
            margin-bottom: 16px;
        }
        pre {
            background: #f6f8fa;
            border-radius: 3px;
            padding: 16px;
            overflow: auto;
        }
        code {
            background: #f6f8fa;
            padding: 2px 4px;
            border-radius: 3px;
            font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
        }
        pre code {
            background: none;
            padding: 0;
        }
        blockquote {
            border-left: 4px solid #dfe2e5;
            margin: 0;
            padding-left: 16px;
            color: #6a737d;
        }
        table {
            border-collapse: collapse;
            width: 100%;
            margin: 16px 0;
        }
        table th, table td {
            border: 1px solid #dfe2e5;
            padding: 6px 13px;
        }
        table th {
            background: #f6f8fa;
        }
    </style>
</head>
<body>
    <header>
        <h1>{{ .Site.Title }}</h1>
        <nav>
            <ul>
                <li><a href="/">Home</a></li>
            </ul>
        </nav>
    </header>
    <main>
        {{ block "main" . }}{{ end }}
    </main>
    <footer style="margin-top: 50px; padding-top: 20px; border-top: 1px solid #e7e7e7; color: #666; font-size: 0.9em;">
        <p>Generated with Hugo</p>
    </footer>
</body>
</html>
EOF

    # Create single page template
    cat > layouts/_default/single.html <<'EOF'
{{ define "main" }}
<article>
    <h1>{{ .Title }}</h1>
    {{ .Content }}
</article>
{{ end }}
EOF

    # Create list template
    cat > layouts/_default/list.html <<'EOF'
{{ define "main" }}
<div>
    <h1>{{ .Title }}</h1>
    {{ .Content }}
    <ul>
        {{ range .Pages }}
        <li><a href="{{ .Permalink }}">{{ .Title }}</a></li>
        {{ end }}
    </ul>
</div>
{{ end }}
EOF

    # Create index template
    cat > layouts/index.html <<'EOF'
{{ define "main" }}
<div>
    <h1>{{ .Site.Title }}</h1>
    {{ if .Site.Params.description }}
    <p>{{ .Site.Params.description }}</p>
    {{ end }}
    <h2>Pages</h2>
    <ul>
        {{ range .Site.RegularPages }}
        <li><a href="{{ .Permalink }}">{{ .Title }}</a></li>
        {{ end }}
    </ul>
</div>
{{ end }}
EOF

else
    # Use specified theme
    echo "Using theme: $HUGO_THEME"
    # Theme should be mounted as a volume or downloaded here
    if [ ! -d "themes/$HUGO_THEME" ]; then
        echo "Warning: Theme $HUGO_THEME not found in themes directory"
    fi
fi

# Copy markdown files to content directory
echo "Copying markdown files from source..."
find "$SOURCE_PATH" -name "*.md" -o -name "*.markdown" | while read -r file; do
    # Get relative path
    rel_path=$(echo "$file" | sed "s|^$SOURCE_PATH/||")
    # Create directory structure
    dest_dir=$(dirname "content/$rel_path")
    mkdir -p "$dest_dir"
    # Copy file
    cp "$file" "content/$rel_path"
    echo "  Copied: $rel_path"
done

# Copy any static assets (images, etc.)
if [ -d "$SOURCE_PATH/static" ]; then
    echo "Copying static assets..."
    cp -r "$SOURCE_PATH/static/"* static/ 2>/dev/null || true
fi

# Also copy common image formats from anywhere in source
echo "Copying image files..."
find "$SOURCE_PATH" -type f \( -name "*.png" -o -name "*.jpg" -o -name "*.jpeg" -o -name "*.gif" -o -name "*.svg" \) | while read -r file; do
    rel_path=$(echo "$file" | sed "s|^$SOURCE_PATH/||")
    dest_dir=$(dirname "static/$rel_path")
    mkdir -p "$dest_dir"
    cp "$file" "static/$rel_path"
    echo "  Copied image: $rel_path"
done

# Build the site to a subdirectory to avoid permission issues with mount point
echo "Building static site with Hugo..."
HUGO_OUTPUT="/tmp/hugo-output-$$"
mkdir -p "$HUGO_OUTPUT"
hugo --destination "$HUGO_OUTPUT" --cleanDestinationDir

# Check if build was successful
if [ $? -eq 0 ]; then
    echo "Successfully compiled static site"

    # Copy the built files to the target directory
    echo "Copying built files to target directory..."
    cp -r "$HUGO_OUTPUT"/* "$TARGET_PATH"/ 2>/dev/null || cp -r "$HUGO_OUTPUT"/. "$TARGET_PATH"/ 2>/dev/null || true

    # List generated files
    echo ""
    echo "Generated files:"
    find "$TARGET_PATH" -type f | head -20

    # Count files
    total_files=$(find "$TARGET_PATH" -type f | wc -l)
    echo ""
    echo "Total files generated: $total_files"

    # Cleanup Hugo output directory
    rm -rf "$HUGO_OUTPUT"
else
    echo "Error: Hugo build failed"
    rm -rf "$HUGO_OUTPUT"
    exit 1
fi

# Cleanup
rm -rf "$TEMP_SITE"

echo "Compilation complete!"
