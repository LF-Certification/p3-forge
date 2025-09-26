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
    echo "  MKDOCS_SITE_URL   Site URL (default: https://example.com/)"
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

# Expect instructions.md file
echo "Looking for instructions.md..."
if [ ! -f "$SOURCE_PATH/instructions.md" ]; then
    echo "Error: instructions.md not found in source path: $SOURCE_PATH"
    exit 1
fi

# Copy the pre-configured MkDocs structure
echo "Setting up MkDocs structure..."
cp -r /mkdocs/* "$TEMP_SITE/"

# Copy instructions.md to docs/index.md
cp "$SOURCE_PATH/instructions.md" "$TEMP_SITE/docs/index.md"

cd "$TEMP_SITE"

# Build the site
echo "Building static site with MkDocs..."
MKDOCS_OUTPUT="/tmp/mkdocs-output-$$"
mkdir -p "$MKDOCS_OUTPUT"

# Build the site with MkDocs
mkdocs build --config-file mkdocs.yaml --site-dir "$MKDOCS_OUTPUT" --clean

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
