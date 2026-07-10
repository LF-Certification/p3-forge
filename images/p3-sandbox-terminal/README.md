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
5. **Clipboard Support**: Provides a gesture-gated OSC 52 path for plain tmux mouse selections, supports native Shift+drag (Linux/Windows) or Option+drag (macOS), and leaves right-click and middle-click to the browser
6. **Persistent Sessions**: Uses tmux session named "remote" for connection persistence
7. **Browser Shortcut Guard**: Uses a custom ttyd index to cancel browser-chrome shortcuts (`Ctrl`/`Cmd`+`S`, `F`, `P`, `U`, `F1`, and backspace outside editable controls) in capture phase while leaving xterm's normal key handling to forward them to the pty. ttyd's nested-frame leave alert is disabled; top-window close protection belongs to the embedding PCI.

## Clipboard behavior

Plain mouse drag selects through tmux. The Chromium-oriented path opens a 250 ms, one-use window for tmux's OSC 52 response after a completed drag. Unsolicited, replayed, or late OSC 52 writes cannot write the clipboard; a malformed response also cannot write and consumes the window. This sharply limits remote clipboard writes but cannot eliminate a malicious remote process racing the legitimate response during that brief window.

Shift+drag on Linux and Windows, or Option+drag on macOS, bypasses tmux for an xterm-native selection and copies directly when the drag ends. The bridge uses Pointer Events and pointer capture for primary mouse and pen input where supported, with mouse events as a compatibility fallback; touch does not authorize clipboard writes. Pointer capture normally preserves release handling outside the terminal element, but browsers cannot guarantee delivery after the pointer crosses into another document or iframe, so release across an iframe boundary may require an explicit keyboard or browser copy. Use `Ctrl+Shift+C` to copy an active native selection. On macOS, use `Cmd+C`; the terminal deliberately leaves `Cmd+Shift+C` untouched because browsers reserve it for developer tools. A copy shortcut with no selection continues to the terminal.

Right-click and middle-click are never intercepted. This preserves the browser context menu and leaves middle-button behavior to the browser and operating system: Linux environments may provide native primary-selection paste, while platforms without that convention may not paste on middle-click. Use keyboard paste or the browser context menu as the portable fallback. `Ctrl+V` and `Cmd+V` retain their normal xterm behavior. The bridge ignores all OSC 52 clipboard-read queries and tmux right-click menus remain disabled so the browser menu can open.

Chromium embedders must delegate `clipboard-write` for bridge-managed copies. The sandbox UI delegates `clipboard-read; clipboard-write` only to terminal tools so browser and xterm paste paths can use clipboard access where supported; the bridge itself does not read the clipboard or synthesize paste. Firefox and Safari use different clipboard permission and transient-activation rules, so modified native selection plus explicit keyboard or browser copy/paste is the supported fallback there. Clipboard denial never prevents normal terminal use.

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
