---
name: project-overview
description: Provides comprehensive overview of mcpd project architecture, capabilities, and key technical details. Use when understanding the project structure, architecture layers, or core concepts.
user-invocable: false
---

# mcpd Project Overview

**mcpd** is an elastic control plane and runtime for Model Context Protocol (MCP) servers. It provides on-demand startup, scale-to-zero hibernation, unified routing, and intelligent tool filtering for managing multiple MCP servers locally.

## Key Capabilities

- Elastic runtime with automatic instance lifecycle management
- Unified gateway (`mcpvmcp`) providing single entry point for all MCP servers
- Smart SubAgent powered by CloudWeGo/Eino for context-aware tool filtering
- GUI support (`mcpvui`) built with Wails 3 for configuration and monitoring
- Profile-based configuration with tag-based visibility
- Native observability with Prometheus metrics and Grafana dashboards

## Architecture

Three-layer architecture for maximum decoupling:

1. **Core (`mcpv`)**: Central control plane managing instance lifecycles, scheduling, and aggregation
2. **Gateway (`mcpvmcp`)**: Protocol bridge acting as standard MCP server for AI clients
3. **App (`mcpvui`)**: Wails-driven GUI for configuration, monitoring, and core lifecycle hosting

### Internal Structure

- `internal/domain/`: Domain models and interfaces (ServerSpec, Instance, Transport, Scheduler, Router, etc.)
- `internal/app/`: Application glue layer with explicit subpackages:
  - `controlplane/`: Control plane facade, registry/discovery/observability/automation, reload application
  - `bootstrap/`: Bootstrap and server initialization management
  - `catalog/`: Catalog providers and state loading
  - `runtime/`: Runtime indexes and aggregation state
- `internal/infra/`: Infrastructure adapters organized by concern:
  - `catalog/`: Configuration loading and validation
  - `scheduler/`: Instance lifecycle and capacity constraints coordination
  - `lifecycle/`: Instance startup, handshake, and shutdown management
  - `router/`: Request routing to managed MCP server instances
  - `transport/`: stdio and streamable HTTP transport implementations
  - `aggregator/`: Tool/resource/prompt aggregation and indexing
  - `gateway/`: MCP gateway client manager and registry
  - `subagent/`: Eino-based intelligent tool filtering
  - `telemetry/`: Prometheus metrics and health checks
  - `rpc/`: gRPC server and client for core-gateway communication
- `internal/ui/`: Wails bridge layer exposing core functionality to GUI

## Configuration

Configuration is YAML-based with a single file per profile. See `docs/catalog.example.yaml` or `runtime.yaml` for reference.

**Key Configuration Sections**:
- `servers[]`: MCP server definitions with transport, command, tags, lifecycle settings
- `subAgent`: Eino-based tool filtering configuration (model, provider, API key, enabled tags)
- `observability`: Prometheus metrics endpoint configuration
- `rpc`: gRPC server configuration for core-gateway communication
- Runtime settings: timeouts, concurrency limits, retry policies, bootstrap behavior

**Server Configuration Example**:
```yaml
servers:
  - name: my-server
    transport: stdio
    cmd: [node, ./path/to/server.js]
    tags: [chat]
    idleSeconds: 60
    maxConcurrent: 1
    strategy: stateless
    activationMode: on-demand
    protocolVersion: "2025-11-25"
```

**Profile System**:
- Multiple profiles supported with caller-to-profile mapping
- Default profile used as fallback
- Tag-based visibility: clients see only servers matching their tags

## Transport Types and Activation Modes

### Transport Types

**stdio** (most common):
- Launches subprocess and communicates via stdin/stdout
- Suitable for Node.js, Python, and other scripting languages
- Process lifecycle managed by mcpv

**streamable_http**:
- Connects to HTTP endpoint with Server-Sent Events (SSE) streaming
- Suitable for long-running HTTP services
- No process management needed

**composite**:
- Advanced use case combining multiple transports
- Rarely needed

### Activation Modes

**on-demand** (default):
- Instances start when first request arrives
- Ideal for resource efficiency
- Cold start latency on first request

**always-on**:
- Keeps `minReady` instances running at all times
- Use for latency-sensitive servers
- Higher resource usage

**disabled**:
- Server defined but not activated
- Useful for testing or staged rollouts

### Server Strategies

**stateless**:
- Requests can be routed to any available instance
- Instances can be freely started/stopped
- Most common and efficient

**stateful**:
- Router maintains sticky sessions
- Requests from same client go to same instance
- Session expires after client inactivity timeout

### Gateway Behavior

The `mcpvmcp` gateway:
- Registers with core on startup via gRPC
- Maintains persistent connection to core
- Translates MCP JSON-RPC to core's internal protocol
- Handles tool/resource/prompt aggregation
- Supports both single-server and tag-based modes

### Flags

- `--config`: Specify config directory (default: current directory)
- `--rpc`: Override RPC address for gateway
- `--tag`: Filter servers by tag in gateway mode
- `--server`: Single-server mode for gateway

## Key Technical Details

### Hot Reload

The core watches the config directory for changes and automatically reloads:
- Server spec additions/removals
- Configuration updates
- Profile changes
- Active instances are gracefully drained before applying changes

### Concurrency Control

- `maxConcurrent`: Maximum parallel requests per instance
- `minReady`: Minimum instances to keep warm
- Scheduler spawns new instances when capacity is reached
- Instances are reused across requests (connection pooling)

### Tool Namespace Strategy

To avoid tool name collisions across servers:
- **prefix**: Tools named as `server__toolname` (default)
- **suffix**: Tools named as `toolname__server`

### Bootstrap Modes

- **metadata**: Only fetch server metadata (tools/resources/prompts) on startup
- **full**: Perform full initialization including handshake
- **none**: Skip bootstrap entirely

### Scale-to-Zero Runtime

Instances are launched on-demand and shut down after idle timeout:
- Scheduler manages instance pools with concurrency limits
- Router maintains session affinity for stateful servers
- Lifecycle manager handles startup, handshake, and graceful shutdown
- Probe system monitors instance health

### Tool Aggregation

The aggregator maintains a unified index across all active instances:
- Automatic refresh on `list_changed` notifications
- Concurrent refresh workers with rate limiting
- Namespace strategy (prefix or suffix) to avoid tool name collisions
- Supports tools, resources, and prompts

## Version Information

- **Go**: 1.25+
- **Wails**: 3.0.0-alpha.53
- **MCP SDK**: github.com/modelcontextprotocol/go-sdk v1.2.0
- **Eino**: github.com/cloudwego/eino v0.7.15
- **Prometheus**: github.com/prometheus/client_golang v1.23.2

## Additional Resources

- **MCP Specification**: https://modelcontextprotocol.io/
- **Wails Documentation**: https://wails.io/
- **Wire Documentation**: https://github.com/google/wire
- **CloudWeGo Eino**: https://github.com/cloudwego/eino
- **Frontend Guide**: See `frontend/CLAUDE.md` for React/TypeScript frontend development
