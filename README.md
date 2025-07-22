# sandbox-images

Container image build system with automated semantic versioning for sandbox environments.

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
   source scripts/bash_completion

   # Now you can use tab completion:
   make build [TAB]      # Shows available image paths
   make tag [TAB]        # Shows available image paths
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

### Building and Releasing

```bash
# Build an image locally
make build images/alpine

# Or use convenient dynamic targets
make build-alpine

# Preview next version (based on conventional commits)
make preview-next-tag images/alpine

# Or use dynamic target
make preview-alpine

# Create a semantic version tag
make tag images/alpine

# Or use dynamic target
make tag-alpine
```

### Development

```bash
# List all available dynamic targets
make list-dynamic-targets

# Check what version would be released next
make preview-next-tag images/ui

# Or use dynamic target
make preview-ui
```

## Image Structure

Images are organized in a flat structure under the `images/` directory:

```
images/
├── alpine/         # Alpine Linux base image
├── ubuntu/         # Ubuntu base image
├── ui/             # UI application
├── api/            # API service
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

### Image Tags

Released images include multiple tags for flexibility:
- `:latest` - Most recent release
- `:v1.2.3` - Exact semantic version
- `:v1.2` - Minor version family
- `:v1` - Major version family

See [`.github/workflows/README.md`](.github/workflows/README.md) for complete CI/CD documentation.

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

# 4. Build and test
make build images/debian

# 5. Create tag
make tag images/debian
```

### Example: Updating an Existing Image

```bash
# 1. Make changes
vim images/alpine/Dockerfile

# 2. Commit with conventional commit message
git add images/alpine/Dockerfile
git commit -m "fix(alpine): update security patches"

# 3. Preview version
make preview-next-tag images/alpine

# 4. Create tag
make tag images/alpine
```
