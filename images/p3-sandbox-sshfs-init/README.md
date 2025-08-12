# P3 SSHFS Init Container

A privileged init container that mounts remote filesystems via SSHFS for use by the P3 Sandbox IDE.

## Overview

This init container is designed to run as a privileged container that performs SSHFS mounting before the main IDE container starts. The mounted filesystem is then accessible to the non-privileged main container via a shared volume.

## Architecture

1. **Init Container** (privileged): Mounts remote filesystem via SSHFS
2. **Shared Volume**: EmptyDir volume shared between init and main containers
3. **Main Container** (non-privileged): Accesses mounted filesystem through shared volume

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `TARGET_HOST` | Yes | - | Remote host to connect to |
| `TARGET_USER` | Yes | - | Username for SSH connection |
| `REMOTE_WORKDIR` | Yes | - | Remote directory to mount |
| `MOUNT_POINT` | No | `/workspace` | Local mount point path |
| `SSH_KEY_PATH` | No | `/home/coder/.ssh/id_rsa` | SSH private key path |
| `SSH_CONFIG_PATH` | No | `/home/coder/.ssh/config` | SSH config file path |
| `MAX_RETRIES` | No | `10` | Maximum retry attempts for connections |
| `RETRY_DELAY` | No | `5` | Delay between retry attempts (seconds) |

## Volume Mounts

The init container expects the following volume mounts:

- **Workspace Volume**: Mounted at `/workspace` (or `$MOUNT_POINT`)
- **SSH Files Volume**: Mounted at `/home/coder/.ssh` (contains SSH keys and config)

## Features

### Connection Handling
- **SSH Connectivity Testing**: Verifies SSH connection before attempting mount
- **Retry Logic**: Configurable retry attempts with delays
- **Connection Validation**: Tests authentication and host reachability

### Filesystem Management
- **Remote Directory Creation**: Creates remote workspace if it doesn't exist
- **Mount Verification**: Verifies successful mount and filesystem access
- **Proper Permissions**: Sets appropriate file permissions (UID 1000, GID 1000)

### SSHFS Options
- **Reconnection**: Automatic reconnection on network interruption
- **Keep-Alive**: ServerAlive settings to maintain connections
- **Permission Mapping**: Maps to UID/GID 1000 for main container access
- **Allow Other**: Enables access from main container

### Security
- **Proper SSH Key Permissions**: Ensures SSH key has 600 permissions
- **Host Key Management**: Disables strict host key checking for automated use
- **Known Hosts**: Uses temporary known_hosts to avoid conflicts

## How It Works

1. **Validation**: Validates required environment variables and SSH key existence
2. **SSH Testing**: Tests SSH connectivity with retry logic
3. **Remote Directory**: Verifies or creates remote workspace directory
4. **SSHFS Mount**: Mounts remote filesystem with appropriate options
5. **Verification**: Verifies successful mount and creates status marker
6. **Completion**: Ensures mount stability before init container exits

## Status Indicators

The init container creates a `.sshfs-status` file in the mounted directory to indicate successful mounting. The main container can check for this file to determine if SSHFS mounting was successful.

## Integration with IDE Tool

This init container is automatically configured by the P3 Sandbox Operator when deploying IDE tools. The operator:

1. Creates the privileged init container with proper security context
2. Configures environment variables from tool definition
3. Sets up volume mounts for workspace and SSH credentials
4. Ensures main container runs non-privileged

## Error Handling

The init container provides comprehensive error handling:

- **SSH Connection Failures**: Detailed error messages with troubleshooting steps
- **Mount Failures**: Retry logic with exponential backoff
- **Permission Issues**: Automatic SSH key permission correction
- **Remote Directory Issues**: Automatic directory creation

## Security Context

The init container requires a privileged security context:

```yaml
securityContext:
  privileged: true
  allowPrivilegeEscalation: true
  capabilities:
    add: ["SYS_ADMIN"]
```

This is necessary for:
- FUSE filesystem mounting
- Device access for SSHFS
- Mount system calls

The main container remains non-privileged for security.

## Troubleshooting

Common issues and solutions:

1. **SSH Connection Fails**
   - Verify target host is reachable
   - Check SSH service is running
   - Validate SSH key is authorized
   - Confirm target user exists

2. **Mount Fails**
   - Check FUSE is available in kernel
   - Verify container has SYS_ADMIN capability
   - Ensure mount point exists and is empty

3. **Permission Issues**
   - Confirm SSH key has 600 permissions
   - Check UID/GID mapping (1000:1000)
   - Verify remote directory permissions

4. **Network Issues**
   - Check network connectivity to target host
   - Verify firewall rules allow SSH traffic
   - Confirm DNS resolution

## Example Usage

The init container is typically used automatically by the P3 Sandbox Operator, but can be tested manually:

```bash
docker run --privileged \
  -e TARGET_HOST=example.com \
  -e TARGET_USER=developer \
  -e REMOTE_WORKDIR=/home/developer/workspace \
  -v ssh-keys:/home/coder/.ssh:ro \
  -v workspace:/workspace \
  ghcr.io/lf-certification/p3-sandbox-sshfs-init:latest
```

This provides a secure, reliable way to mount remote filesystems for IDE access while maintaining strong security boundaries between privileged mounting operations and user workspaces.
