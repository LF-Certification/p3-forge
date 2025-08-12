# P3 Sandbox IDE

A simplified web-based IDE container built on code-server that works with local filesystems. When used with the P3 Sandbox Operator, the workspace can be pre-mounted by an SSHFS init container for seamless remote file access.

## Features

- **Simplified local filesystem access**: Works directly with mounted or local workspaces
- **Web-based IDE**: Full VS Code experience in the browser
- **SSHFS integration**: Automatically detects and uses SSHFS-mounted remote workspaces
- **Fallback support**: Gracefully handles local-only workspaces when remote mounting fails
- **Security hardening**: Runs as non-root user (UID 1000) with dropped capabilities

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `LOCAL_WORKDIR` | No | `$HOME/workspace` | Local directory path for workspace |

## Volume Mounts

The container expects:
- Workspace volume mounted at `/home/coder/workspace` (or `$LOCAL_WORKDIR`)

## How It Works

1. **Workspace Detection**: Checks for workspace directory availability
2. **SSHFS Detection**: Detects if workspace is SSHFS-mounted via status file
3. **Code-Server Startup**: Starts VS Code server with the available workspace
4. **Clean Operation**: No background processes or sync daemons needed

## Integration with Sandbox Operator

When used with the P3 Sandbox Operator, the IDE tool:
- Uses an SSHFS init container to pre-mount remote workspaces
- Automatically detects mounted remote filesystems
- Falls back to local workspace when remote mounting fails
- Provides a web interface accessible through the sandbox ingress

## Example Usage

```yaml
tools:
  - name: my-ide
    ide:
      version: v1.1.0
      targetHost: my-server
      targetUser: developer
      workspaceDir: /opt/project  # optional
```

This creates an IDE with an init container that mounts `my-server:/opt/project` via SSHFS, making it available to the main IDE container.

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

If the IDE fails to work correctly:

1. **Check workspace mount**: Verify the workspace volume is properly mounted
2. **Review container logs**: Look for workspace detection and SSHFS status messages
3. **Verify SSHFS init container**: If using remote workspaces, check init container logs
4. **Check file permissions**: Ensure workspace has proper permissions for UID 1000
5. **Local fallback**: The IDE will work with local workspace even if remote mounting fails

The simplified architecture eliminates most connection and sync issues by handling remote mounting at the init container level.
