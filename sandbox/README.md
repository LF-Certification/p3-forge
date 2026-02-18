# Sandbox Base Images

This directory contains base images for P3 sandbox VMs, built using the sandbox CLI.

## Directory Structure

```
sandbox/
└── vm/
    ├── distro/           # Base distribution images
    │   ├── debian/
    │   └── ubuntu/
    └── kubernetes/       # Kubernetes-ready images
        ├── k3s/          # Pre-built single-node k3s cluster
        └── k8sn/         # Uninitialized Kubernetes node
```

## Image Categories

### Distro Images (`sandbox/vm/distro/`)

Base distribution images with common customizations for lab/exam environments. These are the foundation for all other images.

| Image | Description |
|-------|-------------|
| `ubuntu` | Ubuntu LTS with base customizations |
| `debian` | Debian stable with base customizations |

### Kubernetes Images (`sandbox/vm/kubernetes/`)

Images with Kubernetes components pre-installed.

| Image   | Description                                                                         |
|---------|--------------------------------------------------------------------------------------|
| `k3s`   | Pre-built single-node k3s cluster                                                   |
| `k8s`   | Pre-built single-node opinionated Kubernetes cluster (use `k8sn` for customization) |
| `k8sn`  | Uninitialized Kubernetes node for multi-VM clusters (control plane or worker)       |
| `k8scl` | Self-contained multi-node Kubernetes cluster                                        |

## Tagging Scheme

### Distro Images

| Tag | Example | Description |
|-----|---------|-------------|
| `:distro_version` | `:noble`, `:trixie` | Version codename |
| `:latest` | `:latest` | Latest version |
| `:distro_version-timestamp` | `:noble-20260217T1301` | Immutable build tag |

### Kubernetes Images

For each distro variant:

| Tag | Example | Condition |
|-----|---------|-----------|
| `:distro_version` | `:noble` | Always |
| `:k8s-distro_version` | `:1.35-noble` | Always |
| `:k8s-distro_version-timestamp` | `:1.35-noble-20260217T1301` | Always (immutable) |
| `:k8s` | `:1.35` | Only for default distro |
| `:latest` | `:latest` | Only for default distro |

## Usage

Reference images in your `sandbox.yaml`:

```yaml
spec:
  virtualmachines:
    - name: node
      baseImage: ubuntu:noble
      user: tux
```

Or for Kubernetes workloads:

```yaml
spec:
  virtualmachines:
    - name: cp
      baseImage: k8sn:1.35-noble
      user: tux
```

## Building Locally

```bash
# Build a distro image
sandbox build sandbox/vm/distro/ubuntu

# Build a kubernetes image with specific distro
SANDBOX_SETTING_DISTRO=ubuntu sandbox build sandbox/vm/kubernetes/k8sn
```

## CI/CD

Images are automatically built and published when changes are pushed to `sandbox/**` on the main branch:

1. Changed distros are built first
2. Kubernetes images are rebuilt for all distros (if any distro changed, or if the kubernetes config changed)

See `.github/workflows/sandbox-build.yml` for details.
