# Git-Tag Versioning Strategy

This repository uses git tags to manage container image versions, following semantic versioning (semver) with a multi-tag publishing strategy.

## Directory Structure

Images are organized in a flat directory structure:
```
images/
├── alpine/
│   └── Dockerfile
└── ui/
    ├── Dockerfile
    └── scripts/
```

Git tags are namespaced by image path, with each tag representing the complete, tested configuration of a specific image at that point in time.

## Tag Strategy

When a new version is built, the CI/CD pipeline pushes it to multiple tags based on the following rules:

### New Major/Minor Release (e.g., v1.0.0, v2.0.0)
- `latest` - Always points to the newest version
- `vX` - Points to the latest patch of the major version
- `vX.Y` - Points to the latest patch of the minor version
- `vX.Y.Z` - Immutable, specific version tag

**Example**: Building `ui:v1.0.0` pushes to:
- `ghcr.io/lf-certification/ui:latest`
- `ghcr.io/lf-certification/ui:v1`
- `ghcr.io/lf-certification/ui:v1.0`
- `ghcr.io/lf-certification/ui:v1.0.0`

### New Major Version Supersedes Previous
**Example**: Building `ui:v2.0.0` pushes to:
- `ghcr.io/lf-certification/ui:latest` (updated)
- `ghcr.io/lf-certification/ui:v2` (new)
- `ghcr.io/lf-certification/ui:v2.0` (new)
- `ghcr.io/lf-certification/ui:v2.0.0` (new)

### Patch Releases for Older Versions
**Example**: Building `ui:v1.0.1` (after v2.0.0 exists) pushes to:
- `ghcr.io/lf-certification/ui:v1` (updated)
- `ghcr.io/lf-certification/ui:v1.0` (updated)
- `ghcr.io/lf-certification/ui:v1.0.1` (new)

**Note**: `latest` remains unchanged (still points to v2.0.0), and `v1.0.0` remains frozen.

## Immutable Tags

Tags with the pattern `vX.Y.Z` are considered frozen and should never be overwritten once published.

## Automated Version Management

This repository uses **Conventional Commits** and **git-cliff** for automated semantic versioning with namespaced tags.

### Namespacing: Git Tags vs Docker Images

**Git tags** are namespaced by image name to avoid collisions between image directories:
- `alpine-v1.0.0` - Git tag for alpine image
- `ui-v1.2.3` - Git tag for UI image

**Docker images** are published to the registry:
- `ghcr.io/lf-certification/alpine:v1.0.0` - Published container
- `ghcr.io/lf-certification/ui:v1.2.3` - Published container

### Example Workflow
1. Make changes to `images/ui/`
2. Commit: `feat: add new iframe security headers`
3. Preview: `make release-dry-run images/ui` (shows next version will be v1.3.0)
4. Release: `make release images/ui`
5. Result:
   - Creates git tag: `ui-v1.3.0`
   - CI builds: `ghcr.io/lf-certification/ui:v1.3.0`
   - Also tagged: `ui:v1.3`, `ui:v1`, `ui:latest`

### Make Commands
```bash
# Path-based commands (works with any image)
make release images/alpine              # Creates alpine-vX.Y.Z release
make release images/ui                  # Creates ui-vX.Y.Z release
make release-dry-run images/alpine      # Preview next version

# Dynamic targets (auto-generated for each image)
make release-alpine                     # Creates alpine-vX.Y.Z release
make release-dry-run-alpine             # Preview alpine next version
make list-dynamic-targets               # Show all available dynamic targets
```

## Dev Builds

Dev builds are automatically triggered when changes are pushed to any branch that affects files in `images/`. These builds use a special versioning scheme for pre-release testing:

### Dev Build Versioning
- **Format**: `dev-<branch>-<short-sha>`
- **Example**: `ghcr.io/lf-certification/ui:dev-feature-auth-abc1234`

### Automatic Triggering
Dev builds are created when:
1. Changes are pushed to any branch (including `main`)
2. At least one file in `images/` directory is modified
3. Only affected images are built and published

### Dev Build Tags
Each dev build is tagged with:
- `dev-<branch>-<short-sha>` - Immutable dev build identifier
- `dev-<branch>-latest` - Latest dev build for the branch
- `dev-latest` - Latest dev build from main branch only

**Example**: Push to `feature/new-auth` affecting `images/ui/` creates:
- `ghcr.io/lf-certification/ui:dev-feature-new-auth-abc1234`
- `ghcr.io/lf-certification/ui:dev-feature-new-auth-latest`

### Dev Build Retention Policy

Dev builds are automatically cleaned up to prevent registry bloat:
- **Retention Period**: 14 days from creation
- **Scope**: All dev build tags (`dev-*`) are subject to cleanup
- **Schedule**: Daily at 00:00 UTC
- **Exception**: `dev-latest` tag is preserved as long as main branch has recent activity

## Consumer Usage

- Use `latest` for released versions (non-dev)
- Use `dev-<branch>-latest` for testing specific branch changes
- Use `dev-<branch>-<sha>` for reproducible dev builds
- Use `vX` for production when you want automatic patch updates
- Use `vX.Y` when you want to pin to a specific minor version
- Use `vX.Y.Z` for maximum stability and reproducibility
