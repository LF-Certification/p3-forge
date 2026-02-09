# P3 Sandbox Terminal

A web-based terminal container built on ttyd that automatically connects to remote hosts via SSH with persistent sessions.

## Features

- **Auto-reconnecting SSH sessions**: Maintains persistent connection through tmux
- **JSON-based configuration**: Simple configuration via environment variables
- **Web-based terminal**: Full terminal experience in the browser
- **Session persistence**: Uses tmux to maintain sessions across reconnections
- **Security hardening**: Runs as non-root user (UID 1000) with proper SSH permissions

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `TERMINAL_CONFIG` | Yes | - | JSON configuration containing connection details |

### TERMINAL_CONFIG Format

```json
{
  "targetHost": "hostname-or-ip",
  "targetUser": "username",
  "retryInterval": 5
}
```

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `targetHost` | Yes | - | Hostname or IP of the target host to connect to |
| `targetUser` | Yes | - | Username for SSH connection to target host |
| `retryInterval` | No | 5 | Seconds to wait before reconnection attempts |

## Volume Mounts

The container expects SSH credentials to be mounted at:
- `/home/user/.ssh/id_rsa` - SSH private key
- `/home/user/.ssh/config` - SSH configuration (optional)

## How It Works

1. **Configuration Parsing**: Parses JSON configuration from environment variable
2. **SSH Setup**: Configures SSH with provided keys and proper permissions
3. **Terminal Session**: Starts ttyd with tmux session that auto-reconnects on failure
4. **Scrollback Support**: Enables tmux mouse mode for scroll-wheel history browsing
5. **Clipboard Support**: Enables xterm.js force-selection so Shift+drag (Linux/Windows) or Option+drag (macOS) bypasses tmux mouse capture for native clipboard copy
6. **Persistent Sessions**: Uses tmux session named "remote" for connection persistence

## Integration with Sandbox Operator

When used with the P3 Sandbox Operator, the terminal tool automatically:
- Receives SSH keys from the sandbox's SSH secret
- Gets target host/user configuration from the tool definition
- Provides a web interface accessible through the sandbox ingress

## Example Usage

```yaml
tools:
  - name: my-terminal
    terminal:
      version: v1.2.0
      targetHost: my-server
      targetUser: developer
      retryInterval: 10  # optional
```

This creates a terminal that connects to `my-server` as user `developer` with 10-second retry intervals.

## Security Context

The terminal container is designed to run securely with the following security context:

```yaml
securityContext:
  runAsUser: 1000
  runAsGroup: 1000
  runAsNonRoot: true
  capabilities:
    drop: ["ALL"]
  readOnlyRootFilesystem: true
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
- Read-only root filesystem for maximum security
- Volume permissions are managed via fsGroup

## Troubleshooting

If the terminal fails to connect:

1. **Check SSH connectivity**: Ensure target host is reachable and SSH service is running
2. **Verify SSH keys**: Confirm SSH keys are properly mounted and have correct permissions
3. **Check configuration**: Validate TERMINAL_CONFIG JSON format and values
4. **Review logs**: Container logs provide connection status and error information

The terminal will continuously retry connections based on the configured retry interval.
