# A Simple MCP Weather Server written in TypeScript

A Model Context Protocol (MCP) server that provides weather information using the National Weather Service API. Supports both STDIO and HTTP transports.

See the [Quickstart](https://modelcontextprotocol.io/quickstart) tutorial for more information.

## Features

- **Three weather tools:**
  - `get-alerts`: Get weather alerts for a US state (2-letter code)
  - `get-forecast`: Get weather forecast for coordinates (latitude/longitude)
  - `echo`: Simple echo tool for testing

- **Dual transport support:**
  - STDIO transport (default) - for local process communication
  - HTTP transport - for remote server deployment

## Installation

```bash
pnpm install
pnpm run build
```

## Usage

### STDIO Mode (Default)

Run the server in STDIO mode for local process communication:

```bash
node build/index.js
```

Or explicitly set the transport mode:

```bash
TRANSPORT_MODE=stdio node build/index.js
```

### HTTP Mode

Run the server in HTTP mode to accept remote connections:

```bash
TRANSPORT_MODE=http node build/index.js
```

By default, the HTTP server listens on port 3000. You can customize the port:

```bash
PORT=8080 TRANSPORT_MODE=http node build/index.js
```

The server will be available at `http://localhost:3000/mcp` (or your custom port).

## Environment Variables

- `TRANSPORT_MODE`: Transport mode (`stdio` or `http`, default: `stdio`)
- `PORT`: HTTP server port (default: `3000`, only used in HTTP mode)

## Integration with mcpd

### STDIO Mode

Configure mcpd to launch the weather server as a subprocess:

```yaml
servers:
  - name: weather
    cmd:
      - /bin/sh
      - -c
      - node ./weather-demo-mcp/build/index.js
    strategy: stateless
    activationMode: "always-on"
    minReady: 1
    maxConcurrent: 1
    idleSeconds: 60
    protocolVersion: "2025-11-25"
```

### HTTP Mode

First, start the weather server in HTTP mode:

```bash
cd weather-demo-mcp
TRANSPORT_MODE=http node build/index.js
```

Then configure mcpd to connect to the HTTP endpoint:

```yaml
servers:
  - name: weather-http
    transport: streamable_http
    protocolVersion: "2025-06-18"
    http:
      endpoint: "http://localhost:3000/mcp"
      maxRetries: 3
    strategy: stateless
    activationMode: "always-on"
    minReady: 0
    maxConcurrent: 10
    idleSeconds: 60
```

## Testing

### Test STDIO Mode

```bash
# Build the project
pnpm run build

# Run in STDIO mode
node build/index.js
```

### Test HTTP Mode

```bash
# Build the project
pnpm run build

# Start HTTP server
TRANSPORT_MODE=http node build/index.js

# In another terminal, test with curl
curl -X POST http://localhost:3000/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}'
```

## API Examples

### Get Weather Alerts

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "get-alerts",
    "arguments": {
      "state": "CA"
    }
  }
}
```

### Get Weather Forecast

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "get-forecast",
    "arguments": {
      "latitude": 37.7749,
      "longitude": -122.4194
    }
  }
}
```

### Echo Test

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "echo",
    "arguments": {
      "message": "Hello, MCP!"
    }
  }
}
```

## Notes

- The National Weather Service API only supports US locations
- HTTP mode uses stateless transport (no session management)
- Default protocol version for HTTP is `2025-06-18`
- Default protocol version for STDIO is `2025-11-25`

