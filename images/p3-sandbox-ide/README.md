# P3 Sandbox IDE

A web-based IDE container built on code-server that automatically syncs with remote hosts via rsync for seamless file editing.

## Features

- **Automatic rsync synchronization**: Connects to remote hosts and syncs their filesystem bidirectionally
- **SSH connectivity testing**: Robust connection handling with retries
- **Web-based IDE**: Full VS Code experience in the browser
- **Configurable workspace**: Support for custom remote directories
- **Clean error handling**: Graceful fallback and proper cleanup
- **Security hardening**: Runs as non-root user (UID 1000) with dropped capabilities

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `TARGET_HOST` | Yes | - | Hostname or IP of the target host to connect to |
| `TARGET_USER` | Yes | - | Username for SSH connection to target host |
| `WORKSPACE_DIR` | No | `/home/$TARGET_USER` | Remote directory to sync as workspace |
| `SYNC_INTERVAL` | No | `30` | Sync interval in seconds for background daemon |
| `PASSWORD` | No | `password` | Password for code-server web interface |

## Volume Mounts

The container expects SSH credentials to be mounted at:
- `/home/user/.ssh/id_rsa` - SSH private key
- `/home/user/.ssh/config` - SSH configuration (optional)

## How It Works

1. **SSH Setup**: Configures SSH with provided keys and config
2. **Connection Testing**: Tests SSH connectivity with retries (up to 30 attempts)
3. **Initial Sync**: Downloads remote files to `/home/user/workspace`
4. **Background Daemon**: Starts bidirectional sync daemon for continuous synchronization
5. **Code-Server**: Starts VS Code server with the synced workspace
6. **Cleanup**: Stops sync daemon and performs final sync on container shutdown

## Integration with Sandbox Operator

When used with the P3 Sandbox Operator, the IDE tool automatically:
- Receives SSH keys from the sandbox's SSH secret
- Gets target host/user configuration from the tool definition
- Provides a web interface accessible through the sandbox ingress

## Example Usage

```yaml
tools:
  - name: my-ide
    ide:
      version: v0.2.0
      targetHost: my-server
      targetUser: developer
      workspaceDir: /opt/project  # optional
```

This creates an IDE that connects to `my-server` as user `developer` and opens the `/opt/project` directory.

## Security Context

The IDE container is designed to run securely with the following security context:

```yaml
securityContext:
  runAsUser: 1000
  runAsGroup: 1000
  runAsNonRoot: true
  capabilities:
    drop: ["ALL"]
  readOnlyRootFilesystem: false  # rsync needs write access for sync
  allowPrivilegeEscalation: false

podSecurityContext:
  runAsUser: 1000
  runAsGroup: 1000
  runAsNonRoot: true
  fsGroup: 1000  # Ensures mounted volumes have correct permissions
```

This configuration ensures:
- Container runs as UID/GID 1000 (non-root)
- All Linux capabilities are dropped for security
- No privilege escalation is allowed
- Volume permissions are managed via fsGroup

## Troubleshooting

If the IDE fails to connect:

1. **Check SSH connectivity**: Ensure target host is reachable and SSH service is running
2. **Verify SSH keys**: Confirm SSH keys are properly mounted and have correct permissions
3. **Check user permissions**: Ensure target user exists and has appropriate file permissions
4. **Review logs**: Container logs provide detailed connection and sync information
5. **Check sync daemon**: Review `/tmp/sync.log` for background sync status

The IDE will start even if initial sync fails, allowing access to local tools and debugging. The sync daemon will continue attempting to establish connectivity in the background.
