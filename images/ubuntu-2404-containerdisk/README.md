# Custom Ubuntu 24.04 ContainerDisk with Overlay Initramfs

This directory contains the automation to build a custom Ubuntu 24.04 containerdisk image that includes an overlay initramfs script for 2-layer disk architecture.

## Architecture Overview

The custom initramfs script (`overlay-initramfs-script.sh`) sets up an overlay filesystem combining:
- `/dev/vda` - ContainerDisk (read-only base image)
- `/dev/vdb` - PVC (read-write overlay storage)
- `/dev/vdc` - CloudInit (configuration)

## Prerequisites

### 1. KubeVirt VM Requirements
Your build VM must have:
- Nested virtualization enabled
- Sufficient resources (4+ vCPU, 8GB+ RAM)
- Large PVC for build workspace (20GB+ recommended)

### 2. VM Specification
```yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: containerdisk-builder
  namespace: your-namespace
spec:
  running: true
  template:
    metadata:
      labels:
        kubevirt.io/vm: containerdisk-builder
    spec:
      domain:
        cpu:
          cores: 4
          features:
            - name: vmx  # Intel nested virtualization
            - name: svm  # AMD nested virtualization
        memory:
          guest: 8Gi
        devices:
          disks:
            - name: containerdisk
              disk:
                bus: virtio
            - name: cloudinitdisk
              disk:
                bus: virtio
            - name: workdisk
              disk:
                bus: virtio
            - name: docker-credentials
              disk:
                bus: virtio
          filesystems:
            - name: docker-credentials
              virtiofs: {}
      volumes:
        - name: containerdisk
          containerDisk:
            image: quay.io/containerdisks/ubuntu:24.04
        - name: cloudinitdisk
          cloudInitNoCloud:
            userData: |
              #cloud-config
              users:
                - name: ubuntu
                  sudo: ALL=(ALL) NOPASSWD:ALL
                  ssh_authorized_keys:
                    - ssh-rsa YOUR_PUBLIC_KEY_HERE
              packages:
                - qemu-utils
                - kpartx
                - podman
              runcmd:
                - modprobe nbd max_part=8
                - modprobe dm-mod
        - name: workdisk
          persistentVolumeClaim:
            claimName: build-workspace-pvc
        - name: docker-credentials
          secret:
            secretName: docker-registry-secret
```

### 3. Create Required Resources

#### Build Workspace PVC
```bash
kubectl apply -f - << 'EOF'
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: build-workspace-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 50Gi
EOF
```

#### Docker Registry Secret
```bash
# Method 1: Docker registry secret (recommended)
kubectl create secret docker-registry docker-registry-secret \
  --docker-server=quay.io \
  --docker-username=YOUR_USERNAME \
  --docker-password=YOUR_PASSWORD \
  --docker-email=YOUR_EMAIL

# Method 2: Generic secret with username/password files
kubectl create secret generic docker-registry-secret \
  --from-literal=username=YOUR_USERNAME \
  --from-literal=password=YOUR_PASSWORD
```

## Build Process

### 1. SSH into the Build VM
```bash
# Get VM IP
kubectl get vmi containerdisk-builder -o jsonpath='{.status.interfaces[0].ipAddress}'

# SSH into VM
ssh ubuntu@VM_IP
```

### 2. Clone Repository and Navigate
```bash
git clone https://github.com/lf-certification/sandbox-images.git
cd sandbox-images/images/ubuntu-2404-containerdisk
```

### 3. Verify Files
```bash
# Ensure these files exist:
ls -la
# Should show:
# - overlay-initramfs-script.sh
# - build.sh
# - Makefile
# - README.md
```

### 4. Check Requirements
```bash
make check-requirements
```

### 5. Build the ContainerDisk

#### Local Build Only
```bash
make build
```

#### Build and Push to Registry
```bash
# Using environment variables
make build-and-push REGISTRY=quay.io/your-org IMAGE_TAG=v1.0.0

# Or export variables
export REGISTRY=quay.io/your-org
export IMAGE_TAG=v1.0.0
make build-and-push
```

### 6. Test the Built Image (Optional)
```bash
# Requires virtctl and kubectl configured
make test REGISTRY=quay.io/your-org IMAGE_TAG=v1.0.0
```

## Configuration Options

### Environment Variables
- `REGISTRY` - Container registry to push to (e.g., `quay.io/your-org`)
- `IMAGE_NAME` - Custom image name (default: `custom-ubuntu`)
- `IMAGE_TAG` - Image tag (default: `24.04`)
- `DOCKER_CREDS_PATH` - Path to Docker credentials (default: `/mnt/docker-creds`)

### Makefile Targets
- `make help` - Show available targets and usage
- `make build` - Build the containerdisk locally
- `make push REGISTRY=...` - Push existing image to registry
- `make build-and-push REGISTRY=...` - Build and push in one step
- `make clean` - Clean up build artifacts
- `make test REGISTRY=...` - Create test VM with the built image
- `make check-requirements` - Verify all tools are available

## Troubleshooting

### Common Issues

#### 1. NBD Module Not Loading
```bash
sudo modprobe nbd max_part=8
# If this fails, check if NBD is compiled into kernel:
grep -i nbd /boot/config-$(uname -r)
```

#### 2. Permission Denied on /dev/nbd0
```bash
# Check if running with sufficient privileges
sudo whoami
# Ensure user is in docker group for podman
sudo usermod -aG docker $USER
```

#### 3. Registry Push Authentication Failed
```bash
# Check if credentials are mounted correctly
ls -la /mnt/docker-creds/
# Manually test login
podman login quay.io
```

#### 4. Insufficient Disk Space
```bash
# Check available space
df -h
# Clean up if needed
make clean
podman system prune -a
```

### Manual Credential Setup
If automatic credential detection fails:
```bash
# For docker-registry secret format
mkdir -p ~/.docker
cp /mnt/docker-creds/.dockerconfigjson ~/.docker/config.json

# For username/password format
podman login quay.io --username $(cat /mnt/docker-creds/username) --password $(cat /mnt/docker-creds/password)
```

## Using the Built ContainerDisk

Once built and pushed, use your custom containerdisk in KubeVirt VMs:

```bash
# Create VM with custom containerdisk
virtctl create vm my-custom-vm \
  --instancetype=u1.medium \
  --preference=ubuntu \
  --volume-import=type:registry,url:docker://quay.io/your-org/custom-ubuntu:v1.0.0,size:10Gi \
  --volume-pvc=pvc-name:my-overlay-pvc,size:20Gi

# The VM will automatically use the overlay filesystem:
# - Base system on read-only containerdisk
# - Persistent changes on the PVC overlay
```

## File Structure
```
images/ubuntu-2404-containerdisk/
├── README.md                     # This file
├── Makefile                      # Build automation
├── build.sh                      # Main build script
└── overlay-initramfs-script.sh   # Custom initramfs script
```

## Build Output
After successful build:
- Local image: `custom-ubuntu:24.04`
- Registry image: `${REGISTRY}/custom-ubuntu:${IMAGE_TAG}`
- Build artifacts in: `build-workspace/` (auto-cleaned)

## Security Notes
- The build process requires sudo access for mounting filesystems
- Docker credentials are automatically detected from mounted K8s secrets
- Build artifacts are cleaned up automatically on script exit
- All temporary containers and mounts are removed on completion or failure
