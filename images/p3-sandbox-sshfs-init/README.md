# P3 SSHFS Init Container

A privileged init container that mounts remote filesystems via SSHFS for use by the P3 Sandbox IDE.

## Overview

This container runs as a privileged init container that performs SSHFS mounting and then continuously monitors the mount to ensure it remains active. It automatically remounts if the connection is lost and handles graceful cleanup on termination signals.

The mounted filesystem is accessible to the non-privileged main container via a shared volume.

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
| `MOUNT_POINT` | Yes | - | Local mount point path |
| `SSH_KEY_PATH` | No | `/home/coder/.ssh/id_rsa` | SSH private key path |
| `SSH_CONFIG_PATH` | No | `/home/coder/.ssh/config` | SSH config file path |
| `MAX_RETRIES` | No | `10` | Maximum retry attempts for connections |
| `RETRY_DELAY` | No | `5` | Delay between retry attempts (seconds) |
| `SSHFS_UID` | No | `1000` | UID for SSHFS mount ownership |
| `SSHFS_GID` | No | `1000` | GID for SSHFS mount ownership |

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
- **Mount Verification**: Verifies successful mount and displays directory contents

### SSHFS Options
- **Keep-Alive**: ServerAlive settings to maintain connections (ServerAliveInterval=15, ServerAliveCountMax=3)
- **Permission Mapping**: Maps to configurable UID/GID (default 1000:1000)
- **Allow Other**: Enables access from main container
- **Debug Mode**: Includes sshfs_debug option for troubleshooting

### Security
- **SSH Key Permissions**: Checks and warns about SSH key permissions (expects 600)
- **Host Key Management**: Disables strict host key checking for automated use
- **Known Hosts**: Uses /dev/null for UserKnownHostsFile to avoid conflicts

## How It Works

1. **Validation**: Validates required environment variables and SSH key existence
2. **SSH Testing**: Tests SSH connectivity with retry logic and proper error reporting
3. **Remote Directory**: Verifies or creates remote workspace directory
4. **SSHFS Mount**: Mounts remote filesystem with appropriate options including UID/GID mapping
5. **Verification**: Verifies successful mount and displays mount details
6. **Continuous Monitoring**: Runs infinite loop checking mount status every 5 seconds
7. **Auto-Recovery**: Attempts remount if mount is lost (after checking SSH connectivity)
8. **Signal Handling**: Graceful cleanup with forced unmount on SIGTERM/SIGINT

## Status Indicators

The container provides comprehensive logging throughout the mounting process, including:

- SSH connectivity test results
- Remote directory verification/creation
- SSHFS mount command details
- Mount verification with file listing
- Continuous monitoring status
- Automatic remount attempts

## Integration

This init container is designed to be used with containerized IDE environments where remote filesystem access is needed via SSHFS.

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

The container can be run manually for testing:

```bash
docker run --privileged \
  -e TARGET_HOST=example.com \
  -e TARGET_USER=developer \
  -e REMOTE_WORKDIR=/home/developer/workspace \
  -e MOUNT_POINT=/workspace \
  -v ssh-keys:/home/coder/.ssh:ro \
  -v workspace:/workspace \
  ghcr.io/lf-certification/p3-sandbox-sshfs-init:latest
```

This provides a secure, reliable way to mount remote filesystems for IDE access while maintaining strong security boundaries between privileged mounting operations and user workspaces.
