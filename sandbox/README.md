# Sandbox Base Images

This directory contains base images for P3 sandbox VMs, built using the sandbox CLI.

## Directory Structure

```
sandbox/
├── matrix.yaml           # Image combinations built by CI
└── vm/
    ├── distro/           # Base distribution images
    │   ├── centos-bootc/
    │   ├── debian/
    │   └── ubuntu/
    └── kubernetes/       # Kubernetes-ready images
        ├── k3s/          # Pre-built single-node k3s cluster
        ├── k8sn/         # Uninitialized Kubernetes node
        └── tekton/       # K3s with Tekton Pipelines and tkn
```

## Image Categories

### Distro Images (`sandbox/vm/distro/`)

Base distribution images with common customizations for lab/exam environments. These are the foundation for all other images.

| Image          | Description                                       |
|----------------|---------------------------------------------------|
| `debian`       | Debian stable with base customizations            |
| `ubuntu`       | Ubuntu LTS with base customizations               |
| `centos-bootc` | CentOS Stream bootc host with sandbox integration |

### Kubernetes Images (`sandbox/vm/kubernetes/`)

Images with Kubernetes components pre-installed.

| Image    | Description                                                                         |
|----------|-------------------------------------------------------------------------------------|
| `k3s`    | Pre-built single-node k3s cluster                                                   |
| `tekton` | K3s with Tekton Pipelines and the `tkn` CLI pre-installed                           |
| `k8s`    | Pre-built single-node opinionated Kubernetes cluster (use `k8sn` for customization) |
| `k8sn`   | Uninitialized Kubernetes node for multi-VM clusters (control plane or worker)       |
| `k8scl`  | Self-contained multi-node Kubernetes cluster                                        |

## Build Matrix (`sandbox/matrix.yaml`)

> **WIP:** CI does not consume this matrix yet.

`matrix.yaml` defines which distro and Kubernetes image combinations CI builds. Keys under `vm` match directories under `sandbox/vm/distro/`, and each list entry matches a directory under `sandbox/vm/kubernetes/`. An empty list means that CI builds only the distro image.

Every distro image and listed Kubernetes combination is built for both `amd64` and `arm64`.

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

Tekton images add the Tekton version to their Kubernetes version tags:

| Tag | Example | Condition |
|-----|---------|-----------|
| `:tekton-k8s` | `:1.9.0-k8s1.36.1` | Only for default distro |
| `:tekton-k8s-distro_version` | `:1.9.0-k8s1.36.1-trixie` | Always |
| `:tekton-k8s-distro_version-timestamp` | `:1.9.0-k8s1.36.1-trixie-20260807T1301` | Always (immutable) |

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

Or for Tekton workloads:

```yaml
spec:
  virtualmachines:
    - name: node
      baseImage: tekton:1.9.0-k8s1.36.1
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

1. Changed distros are built first.
2. Kubernetes images are rebuilt only for the distro and image combinations listed in `sandbox/matrix.yaml`.

The p3-forge workflow in `.github/workflows/dispatch-sandbox-vm-build.yml` dispatches the build to the `p3forge-build.yml` workflow in sandbox-building-service.
