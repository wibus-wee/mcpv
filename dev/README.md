# Docker Compose Development Environment

This directory contains configuration for local development with Docker Compose.

## Architecture

- **dev**: MCP Inspector with Go environment - Inspector launches and manages mcpd-gateway
- **core**: mcpd control plane (gRPC + metrics)
- **prometheus**: Metrics collection and monitoring (http://localhost:9500)

## Quick Start

```bash
# Build and start all services
make dev

# View logs
docker compose logs -f dev

# Stop all services
make down

# Rebuild after Dockerfile changes
docker compose up --build
```

## Services

### MCP Inspector (dev service)
- **UI**: http://localhost:6274
- **WebSocket**: ws://localhost:6277
- **Purpose**: Debug MCP protocol, launch and manage mcpd-gateway

### mcpd-core (core service)
- **Metrics**: http://localhost:9090/metrics
- **Purpose**: Control plane gRPC + orchestration

### mcpd-gateway (launched by Inspector)
- **Purpose**: MCP server bridge for tools/list and tools/call

### Prometheus
- **UI**: http://localhost:9500
- **Purpose**: Scrape and visualize mcpd metrics

## Using MCP Inspector

1. Start services: `make dev`
2. Open Inspector UI: http://localhost:6274
3. Configure gateway in Inspector:
   - **Command**: `go run /app/cmd/mcpd-gateway --rpc core:9091`
4. Click "Connect" to start mcpd-gateway
5. Inspector will show all MCP protocol messages

## Viewing Metrics

1. Ensure mcpd-core is running (started by Docker Compose)
2. Open Prometheus: http://localhost:9500
3. Query mcpd metrics:
   - `mcpd_route_duration_seconds`
   - `mcpd_instance_starts_total`
   - `mcpd_instance_stops_total`
   - `mcpd_active_instances`

## Development Workflow

1. Make code changes in your local directory
2. Changes are automatically reflected via volume mount
3. Restart mcpd-gateway in Inspector UI (disconnect/reconnect)
4. View protocol messages in Inspector
5. View metrics in Prometheus

## Configuration Files

- `dev/catalog.dev.yaml`: mcpd-core configuration (includes RPC listen address)
- `dev/prometheus.yaml`: Prometheus scrape configuration
- `dev/Dockerfile.dev`: Development container with Go + Node.js + Inspector

## Ports

- `6274`: MCP Inspector UI
- `6277`: MCP Inspector WebSocket
- `9090`: mcpd-core Prometheus metrics
- `9500`: Prometheus UI

## Troubleshooting

**Inspector can't start mcpd-gateway:**
- Check that the command path is correct: `/app/cmd/mcpd-gateway`
- Ensure the core service is running: `docker compose ps core`
- Verify the RPC address: `--rpc core:9091`
- Check logs: `docker compose logs dev`

**Prometheus shows no data:**
- Ensure mcpd-core is running: `docker compose logs core`
- Verify metrics endpoint: `curl http://localhost:9090/metrics`
- Check Prometheus targets: http://localhost:9500/targets

**Port conflicts:**
- Stop other services using ports 6274, 6277, 9090, or 9500
- Or modify port mappings in `docker-compose.yml`
