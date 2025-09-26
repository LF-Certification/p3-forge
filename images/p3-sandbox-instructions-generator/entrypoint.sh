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
    echo "  MKDOCS_THEME      MkDocs theme to use (default: material)"
    echo "  MKDOCS_SITE_NAME  Site name (default: Documentation)"
    echo "  MKDOCS_SITE_URL   Site URL (default: /)"
    echo "  MKDOCS_REPO_URL   Repository URL (optional)"
    echo "  MKDOCS_REPO_NAME  Repository name (optional)"
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

echo "Starting MkDocs static site compilation..."
echo "  Source path: $SOURCE_PATH"
echo "  Target path: $TARGET_PATH"

# Create a temporary MkDocs project
TEMP_SITE="/tmp/mkdocs-site-$$"
mkdir -p "$TEMP_SITE"

# Copy source files as-is (assuming proper MkDocs structure with docs/ directory)
echo "Copying source files..."
cp -r "$SOURCE_PATH"/* "$TEMP_SITE/" 2>/dev/null || cp -r "$SOURCE_PATH"/. "$TEMP_SITE/" 2>/dev/null || true

cd "$TEMP_SITE"

# Create MkDocs configuration only if it doesn't exist
if [ ! -f "mkdocs.yml" ]; then
    echo "No mkdocs.yml found. Creating minimal configuration for single-page rendering..."

    # Create mkdocs.yml with minimal theme
    cat > mkdocs.yml <<EOF
site_name: ${MKDOCS_SITE_NAME:-Documentation}
site_url: ${MKDOCS_SITE_URL:-/}
theme:
  name: ${MKDOCS_THEME:-material}
  custom_dir: overrides
  palette:
    - scheme: default
      primary: indigo
      accent: indigo
  font:
    text: Roboto
    code: Roboto Mono
  features:
    - content.code.copy

plugins:
  - minify:
      minify_html: true
      minify_js: true
      minify_css: true
      htmlmin_opts:
        remove_comments: true

markdown_extensions:
  - pymdownx.highlight:
      anchor_linenums: true
      line_spans: __span
      pygments_lang_class: true
  - pymdownx.inlinehilite
  - pymdownx.snippets
  - pymdownx.superfences
  - pymdownx.tabbed:
      alternate_style: true
  - pymdownx.details
  - pymdownx.mark
  - pymdownx.tilde
  - pymdownx.smartsymbols
  - admonition
  - tables
  - attr_list
  - md_in_html
  - toc:
      permalink: true

extra:
  generator: false

extra_css:
  - css/custom.css
EOF

    # Create custom CSS to hide navigation elements for single-page display
    mkdir -p overrides/css
    cat > overrides/css/custom.css <<'EOF'
/* Hide navigation sidebar */
.md-sidebar {
    display: none !important;
}

/* Hide header/navbar */
.md-header {
    display: none !important;
}

/* Hide navigation tabs */
.md-tabs {
    display: none !important;
}

/* Adjust main content to use full width */
.md-content {
    max-width: 100% !important;
}

.md-content__inner {
    margin: 0 !important;
    padding: 2rem !important;
}

/* Hide footer navigation */
.md-footer__inner:not(.md-grid) {
    display: none !important;
}

/* Adjust main layout without sidebar */
.md-main__inner {
    display: block !important;
    max-width: 100% !important;
}

.md-content__inner:before {
    display: none !important;
}
EOF

    # Add repository information if provided
    if [ -n "$MKDOCS_REPO_URL" ]; then
        echo "repo_url: $MKDOCS_REPO_URL" >> mkdocs.yml
    fi
    if [ -n "$MKDOCS_REPO_NAME" ]; then
        echo "repo_name: $MKDOCS_REPO_NAME" >> mkdocs.yml
    fi
else
    echo "Using existing mkdocs.yml configuration..."
fi

# Verify docs directory exists
if [ ! -d "docs" ]; then
    echo "Error: No 'docs' directory found. MkDocs requires documentation in a 'docs' directory."
    exit 1
fi

# Build the site
echo "Building static site with MkDocs..."
MKDOCS_OUTPUT="/tmp/mkdocs-output-$$"
mkdir -p "$MKDOCS_OUTPUT"

# Build the site with MkDocs
mkdocs build --site-dir "$MKDOCS_OUTPUT" --clean

# Check if build was successful
if [ $? -eq 0 ]; then
    echo "Successfully compiled static site"

    # Copy the built files to the target directory
    echo "Copying built files to target directory..."
    cp -r "$MKDOCS_OUTPUT"/* "$TARGET_PATH"/ 2>/dev/null || cp -r "$MKDOCS_OUTPUT"/. "$TARGET_PATH"/ 2>/dev/null || true

    # List generated files
    echo ""
    echo "Generated files:"
    find "$TARGET_PATH" -type f | head -20

    # Count files
    total_files=$(find "$TARGET_PATH" -type f | wc -l)
    echo ""
    echo "Total files generated: $total_files"

    # Cleanup MkDocs output directory
    rm -rf "$MKDOCS_OUTPUT"
else
    echo "Error: MkDocs build failed"
    rm -rf "$MKDOCS_OUTPUT"
    exit 1
fi

# Cleanup
rm -rf "$TEMP_SITE"

echo "Compilation complete!"
