#!/usr/bin/env bash
set -ex

# Find the image root by walking up until we find a Dockerfile
current_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
image_root="$current_dir"

while [[ "$image_root" != "/" ]]; do
    if [[ -f "$image_root/Dockerfile" ]]; then
        break
    fi
    image_root="$(dirname "$image_root")"
done

if [[ ! -f "$image_root/Dockerfile" ]]; then
    echo "Error: Could not find Dockerfile in parent directories"
    exit 1
fi

path="$image_root/lab-ui"

mkdir -p "$path"/{dist,build}

# Render markdown instructions
INSTRUCTIONS_MD="$image_root/instructions.en.md"
INSTRUCTIONS_HTML="$path/build/instructions.html"

# Check if instructions.en.md exists
if [ -f "$INSTRUCTIONS_MD" ]; then
  # Make sure render-markdown.sh is executable
  chmod +x "$path/scripts/render-markdown.sh"

  # Render markdown to HTML
  "$path/scripts/render-markdown.sh" "$INSTRUCTIONS_MD" "$INSTRUCTIONS_HTML"
else
  # If instructions file doesn't exist, create an empty HTML file
  echo "<p>No instructions available.</p>" > "$INSTRUCTIONS_HTML"
fi

favicon=$(base64 -w0 "$path/src/favicon.webp")

cp "$path/src/index.html" "$path/dist/index.html"
sed -i "s|/\* Favicon will be injected here \*/|${favicon}|" "$path/dist/index.html"
sed -i \
  "/* Custom Bootstrap theme will be injected here */r $path/../bootstrap-theme/dist/custom.css" \
  "$path/dist/index.html"
sed -i \
  "/* GitHub Markdown CSS will be injected here */r $path/../github-markdown-css/dist/github-markdown.css" \
  "$path/dist/index.html"
sed -i "/* App code will be injected here */r $path/src/app.js" "$path/dist/index.html"

# Use a more reliable approach for injecting instructions
INSTRUCTIONS_CONTENT=$(cat "$INSTRUCTIONS_HTML")
# Use awk to replace the placeholder div with instructions content
awk -v content="$INSTRUCTIONS_CONTENT" '
/<div class="p-3 overflow-auto markdown-body">/ {
  print $0;
  print content;
  getline;
  next;
}
{print}' "$path/dist/index.html" > "$path/dist/index.html.tmp"
mv "$path/dist/index.html.tmp" "$path/dist/index.html"

# Do the same for minified version
cp "$path/src/index.html" "$path/build/index.min.html"
sed -i "s|/\* Favicon will be injected here \*/|${favicon}|" "$path/build/index.min.html"
sed -i \
  "/* Custom Bootstrap theme will be injected here */r $path/../bootstrap-theme/dist/custom.min.css" \
  "$path/build/index.min.html"
sed -i \
  "/* GitHub Markdown CSS will be injected here */r $path/../github-markdown-css/dist/github-markdown.min.css" \
  "$path/build/index.min.html"
npx -y uglify-js "$path/src/app.js" -o "$path/build/app.min.js"
sed -i "/* App code will be injected here */r $path/build/app.min.js" "$path/build/index.min.html"

# Use the same awk approach for minified version
awk -v content="$INSTRUCTIONS_CONTENT" '
/<div class="p-3 overflow-auto markdown-body">/ {
  print $0;
  print content;
  getline;
  next;
}
{print}' "$path/build/index.min.html" > "$path/build/index.min.html.tmp"
mv "$path/build/index.min.html.tmp" "$path/build/index.min.html"

npx -y minify "$path/build/index.min.html" > "$path/dist/index.min.html"
