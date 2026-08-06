# P3 Sandbox IDE

A web-based IDE container built on code-server for sandbox environments.

## Features

- **Web-based IDE**: VS Code experience in the browser
- **Workspace mounting**: Works with mounted workspace directories
- **No authentication**: Configured for secure sandbox environments
- **Minimal configuration**: Simplified setup with disabled telemetry and updates

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `LOCAL_WORKDIR` | Yes | Path to workspace directory |
| `TARGET_HOST` | Yes | SSH host for the target VM |
| `TARGET_USER` | Yes | SSH user for the target VM |
| `REMOTE_WORKDIR` | Yes | Initial directory for terminals on the target VM |

## Configuration

The container includes:
- **Port**: Exposes 8080 for web access
- **Authentication**: Disabled (auth: none)
- **Telemetry**: Disabled
- **Update checks**: Disabled
- **Workspace trust**: Disabled

## How It Works

1. **Validation**: Checks that `LOCAL_WORKDIR` environment variable is set
2. **Workspace Detection**: Waits for workspace directory to exist
3. **Directory Listing**: Shows workspace contents for debugging
4. **Code-Server Startup**: Starts VS Code server with the workspace
5. **Integrated Terminals**: Opens the target VM user's login shell

## Volume Mounts

Mount your workspace directory to the path specified by `LOCAL_WORKDIR`.

Mount a dedicated directory containing only the sandbox SSH key and client config read-only at `/home/coder/.ssh`. Never mount your personal `~/.ssh`; the IDE would expose every key in it.

## Example Docker Run

```bash
docker run -p 8080:8080 \
  -e LOCAL_WORKDIR=/workspace \
  -e TARGET_HOST=vm1 \
  -e TARGET_USER=candidate \
  -e REMOTE_WORKDIR=/home/candidate \
  -v /path/to/your/code:/workspace \
  -v /path/to/sandbox-ssh:/home/coder/.ssh:ro \
  p3-sandbox-ide
```

## Troubleshooting

If the IDE fails to start:

1. **Check LOCAL_WORKDIR**: Ensure the environment variable is set
2. **Verify workspace mount**: Ensure the workspace directory is properly mounted
3. **Review container logs**: Check for workspace detection messages
4. **Directory permissions**: Ensure the workspace directory is accessible
