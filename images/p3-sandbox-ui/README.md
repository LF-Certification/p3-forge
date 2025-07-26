# P3 Sandbox UI

A modern web-based UI for lab environments that provides dynamic tool tabs with lazy-loaded iframes. This UI reads configuration from the `UI_CONFIG` environment variable to determine which tools to display and their URLs.

## Overview

The P3 Sandbox UI creates a responsive Bootstrap-based interface with:
- Dynamic tool tab management with lazy loading
- Configuration polling for real-time updates
- Sandbox expiration countdown timer with API integration
- Collapsible instructions panel
- Security-focused iframe sandboxing

## Configuration

The UI reads its configuration from the `UI_CONFIG` environment variable, which should contain a JSON object with the following structure. All fields are required:

```json
{
  "config": {
    "version": "dev-main-6b26377",
    "defaultTool": "terminal1",
    "expiresAt": "2025-07-26T01:00:00Z"
  },
  "tools": [
    {
      "name": "browser1",
      "url": "https://browser1-target-sandbox.sagad.d.lf-labs.org/"
    },
    {
      "name": "terminal1",
      "url": "https://target-sandbox.sagad.d.lf-labs.org/159fe0b1/tools/terminal1/"
    }
  ]
}
```

### Configuration Properties

- `config`: Configuration metadata
  - `version`: Build information of the UI
  - `defaultTool`: Specifies which tool the UI should display on first load
  - `expiresAt`: RFC3339-formatted timestamp when the sandbox expires
- `tools`: Array of tool configurations
  - `name`: Name as specified in the Sandbox spec
  - `url`: URL to load in the iframe

**Note**: The UI automatically converts tool names to human-readable titles (e.g., "terminal1" becomes "Terminal 1", "browser1" becomes "Browser 1").

If no `UI_CONFIG` environment variable is provided, a default configuration with a single terminal will be used.

## Building

```bash
# Build all components
make build

# Build Docker container
make container
```

## Testing Locally

### Quick UI_CONFIG Test (Recommended)
```bash
make test-ui-config
```
Builds the container and runs it with a simple terminal configuration at http://localhost:8080

### Interactive Test Script
```bash
./test-local.sh
```
Runs multiple test scenarios with different configurations and lets you verify each one interactively.

### Full Development Environment
```bash
make serve
```
Starts the complete environment with terminals and VS Code services at http://localhost:80

### Manual Docker Test
```bash
docker build -t ui-test .
docker run --rm -p 8080:80 \
  -e 'UI_CONFIG={"config": {"defaultTool": "vscode", "expiresAt": "2025-07-26T01:00:00Z"}, "tools": [{"name": "vscode", "url": "/vscode/"}]}' \
  ui-test
```

## Testing Verification

When testing, verify the following:

1. **Configuration Injection**: Check browser dev tools that `configStr` contains the expected JSON
2. **Tab Creation**: Verify that tabs are created for each tool in the configuration
3. **Default Tool**: Confirm the tool marked as `"default": true` is active initially
4. **Lazy Loading**: Verify iframes only load content when you click their tabs
5. **URL Routing**: Check that configured URLs (e.g., `/terminal/`, `/vscode/`) proxy correctly

## Development

The project uses a multi-stage Docker build with:
- **Build stage**: Node.js for compiling assets
- **Runtime stage**: Caddy for serving with environment variable injection

Key components:
- `lab-ui/src/app.js`: Main application logic
- `lab-ui/src/index.html`: HTML template with placeholders
- `scripts/ui-entrypoint.sh`: Runtime configuration injection
- `dev/`: Development environment with docker-compose

## Origins

Source files originally copied from [LF-Certification/lab-images/ui ref 2f60081](https://github.com/LF-Certification/lab-images/tree/2f600814089befa9e8d675bff0a5b2f2b90170e8).
