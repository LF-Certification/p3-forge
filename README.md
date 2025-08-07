# P3 Sandbox Images

Source files for images used with the P3 Sandbox Operator.

## Quick Start

### Prerequisites

- Git
- Docker
- [devbox](https://www.jetify.com/devbox) (for development tools)
- [direnv](https://direnv.net/) (for automatic environment setup)

### Setup

1. **Clone and enter the repository:**
   ```bash
   git clone https://github.com/lf-certification/sandbox-images.git
   cd sandbox-images
   ```

2. **Set up development environment:**
   ```bash
   # Load development tools
   direnv allow

   # Install pre-commit hooks and verify setup
   make setup
   ```

3. **Enable enhanced bash completion (optional):**
   ```bash
   # Source the completion script for tab completion on make targets and image paths
   source scripts/bash-completion

   # Now you can use tab completion:
   make build [TAB]      # Shows available image paths
   make release [TAB]    # Shows available image paths
   ```

4. **View available commands:**
   ```bash
   make help
   ```

## Common Workflows

### Creating a New Image

```bash
# Initialize a new image directory with structure and initial tag
make init-image images/name v0.1.0

# Example: Create a new ubuntu image directory
make init-image images/ubuntu v0.1.0
```

Note: the initial version is optional. If you do not provide one, v0.1.0 will be used.

### Creating a Release

```bash
# Preview next version (based on conventional commits)
make release-dry-run images/alpine

# Or use dynamic target
make release-dry-run-alpine

# Create a semantic version release
make release images/alpine

# Or use dynamic target
make release-alpine

# IMPORTANT: Push the newly created tag to trigger the release workflow
git push origin --follow-tags
```

### Development

```bash
# List all available dynamic targets
make list-dynamic-targets

# Check what version would be released next
make release-dry-run images/ui

# Or use dynamic target
make release-dry-run-ui

# View git commit history for an image
make commit-graph images/alpine

# Or use dynamic target
make commit-graph-alpine

# Run all pre-commit hooks on all files
make runall-pre-commit
```

## Image Structure

Images are organized in a flat structure under the `images/` directory:

```
images/
├── alpine/         # Alpine Linux base image
├── ubuntu/         # Ubuntu base image
├── ui/             # UI application
├── node/           # Node.js runtime
├── nginx/          # Nginx web server
└── redis/          # Redis database
```

### Image Examples and Purposes

- **Base images** (alpine, ubuntu, debian) - Minimal foundation images with common system packages, users, and security hardening
- **Application images** (ui, api, frontend) - Complete applications ready for deployment
- **Tool images** (debugger, profiler) - Development and debugging utilities
- **Runtime images** (node, python, java) - Programming language environments with specific versions
- **Service images** (nginx, redis, postgres) - Infrastructure components
- **Custom images** - Organization-specific or specialized images

Each image directory contains:
- `Dockerfile` - Container build instructions
- `scripts/` - Supporting scripts and assets (optional)

### Image Directory Standards

To ensure compatibility with the build pipeline, all image directories must follow these standards:

1. **Dockerfile as Build Entrypoint**
   - The `Dockerfile` will always be the build entrypoint for the pipeline
   - If the image requires complex build logic, use multi-stage Docker builds
   - The pipeline expects to run `docker build .` from the image directory root

2. **Self-Contained and Portable**
   - Don't make assumptions about the directory name or location
   - The image directory should be self-contained and build successfully regardless of:
     - Directory name (e.g., `p3-sandbox-ui`, `ui`, `web-app`)
     - Directory location (e.g., `images/ui/`, `containers/ui/`, `/tmp/ui/`)
   - Use relative paths and discover the build context dynamically

3. **No Repository Structure Assumptions**
   - Don't make assumptions about the repository structure
   - Repository organization may change without prior warning
   - Avoid hardcoded paths that reference parent directories or specific repo layouts
   - Use techniques like walking up the directory tree to find build markers (e.g., `Dockerfile`)

**Example of Compliant Build Script:**
```bash
# ✅ Good: Find build root dynamically
current_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
build_root="$current_dir"
while [[ "$build_root" != "/" ]]; do
    if [[ -f "$build_root/Dockerfile" ]]; then
        break
    fi
    build_root="$(dirname "$build_root")"
done

# ❌ Bad: Hardcoded assumptions
path=$(git rev-parse --show-toplevel)/ui/lab-ui  # Assumes git repo and specific structure
```

These standards ensure that images remain portable and compatible with the automated build pipeline, regardless of how the repository structure evolves.

## Versioning

This repository uses **git-tag based versioning** with:
- Automated semantic versioning via [git-cliff](https://git-cliff.org/)
- Conventional commits for version bumping
- Namespaced git tags (e.g., `alpine-v1.0.0`)
- Multi-tag Docker publishing (`:latest`, `:v1`, `:v1.0`, `:v1.0.0`)

See [VERSIONING.md](VERSIONING.md) for complete details.

## Commit Convention

This repository enforces [Conventional Commits](https://www.conventionalcommits.org/) via pre-commit hooks for automatic version bumping:

```bash
# Patch release (v1.0.0 → v1.0.1)
git commit -m "fix(alpine): resolve security vulnerability"

# Minor release (v1.0.0 → v1.1.0)
git commit -m "feat(alpine): add new development tools"

# Major release (v1.0.0 → v2.0.0)
git commit -m "feat(alpine)!: upgrade to Alpine 3.20

BREAKING CHANGE: requires new base image configuration"
```

## Examples

### Example: Adding a New Base Image

```bash
# 1. Initialize the image directory
make init-image images/debian v0.1.0

# 2. Customize the Dockerfile
vim images/debian/Dockerfile

# 3. Commit changes
git add images/debian/
git commit -m "feat(debian): add initial debian base image"

# 4. Preview the release
make release-dry-run images/debian

# 5. Create release
make release images/debian
```

### Example: Updating an Existing Image

```bash
# 1. Make changes
vim images/alpine/Dockerfile

# 2. Commit with conventional commit message
git add images/alpine/Dockerfile
git commit -m "fix(alpine): update security patches"

# 3. Preview version
make release-dry-run images/alpine

# 4. Create release
make release images/alpine
```
