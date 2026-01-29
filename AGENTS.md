# AGENTS.md

This file provides guidance to code when working with code in this repository.

## ExecPlans

When writing complex features or significant refactors, use an ExecPlan (as described in .agent/PLANS.md) from design to implementation.

## Additional AGENTS.md Files

if there is AGENTS.md/CLAUDE.md in subdirectories, please also follow the guidelines described in those files.

## Project Overview

**mcpd** is an elastic control plane and runtime for Model Context Protocol (MCP) servers. It provides on-demand startup, scale-to-zero hibernation, unified routing, and intelligent tool filtering for managing multiple MCP servers locally.

**Key Capabilities**:
- Elastic runtime with automatic instance lifecycle management
- Unified gateway (`mcpdmcp`) providing single entry point for all MCP servers
- Smart SubAgent powered by CloudWeGo/Eino for context-aware tool filtering
- GUI support (`mcpdui`) built with Wails 3 for configuration and monitoring
- Profile-based configuration with tag-based visibility
- Native observability with Prometheus metrics and Grafana dashboards

## Architecture

Three-layer architecture for maximum decoupling:

1. **Core (`mcpd`)**: Central control plane managing instance lifecycles, scheduling, and aggregation
2. **Gateway (`mcpdmcp`)**: Protocol bridge acting as standard MCP server for AI clients
3. **App (`mcpdui`)**: Wails-driven GUI for configuration, monitoring, and core lifecycle hosting

**Internal Structure**:
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

## Development Commands

**Prerequisites**: Go 1.25+, protoc (for gRPC), golangci-lint (for linting), wire (installed via `make tools`)

### Core Development

```bash
# Build all packages
make build

# Run tests
make test
# Or directly with go
go test ./...

# Format code
make fmt

# Lint code
make lint-check
# Auto-fix linting issues
make lint-fix

# Generate Wire dependency injection code
make wire

# Generate gRPC protobuf code
make proto

# Install development tools (wire)
make tools
```

### Running the Application

```bash
# Start core control plane (reads config from current directory)
go run ./cmd/mcpd serve --config .

# Start gateway (connects to core via RPC)
go run ./cmd/mcpdmcp --server weather
# Or with tags
go run ./cmd/mcpdmcp --tag chat

# Validate configuration without running
go run ./cmd/mcpd validate --config .
```

### Docker Compose Development

```bash
# Start full development environment (core + observability)
make dev

# Start only observability stack (Prometheus + Grafana)
make obs

# Stop all services
make down

# Restart core service
make reload
```

**Observability URLs**:
- Metrics: http://localhost:9090/metrics
- Health: http://localhost:9090/healthz
- Grafana Dashboard: http://localhost:4000

### Wails GUI Development

```bash
# Generate TypeScript bindings for Go services
make wails-bindings

# Run Wails in development mode (hot reload)
make wails-dev

# Build Wails application
make wails-build

# Package Wails application for distribution
make wails-package
```

**Frontend Development**: See `frontend/CLAUDE.md` for React/TypeScript frontend-specific guidance.

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

## Testing

### Running Tests

```bash
# Run all tests
make test
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with race detector
go test -race ./...

# Run tests with coverage
go test -cover ./...

# Run tests for specific package
go test ./internal/infra/scheduler

# Run specific test function
go test -run TestSchedulerBasic ./internal/infra/scheduler
```

### Test Organization

- Test files use `_test.go` suffix
- Table-driven tests preferred for multiple scenarios
- Use `testify/assert` and `testify/require` for assertions
- Mock interfaces defined in test files when needed

### Common Test Pattern

```go
func TestFeature(t *testing.T) {
    tests := []struct {
        name string
        // test fields
    }{
        {name: "scenario1"},
        {name: "scenario2"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test logic
        })
    }
}
```

## Code Architecture Patterns

### Domain-Driven Design

The codebase follows Domain-Driven Design principles with clear separation:

**Domain Layer** (`internal/domain/`):
- Pure business logic and interfaces
- No dependencies on infrastructure
- Core types: `ServerSpec`, `Instance`, `Catalog`, `Profile`, `Transport`, `Scheduler`, `Router`, `Lifecycle`
- Domain errors: `RouteError`, `ProtocolError`

**Application Layer** (`internal/app/`):
- Orchestrates domain services
- Uses Google Wire for dependency injection
- Key components: `ControlPlane`, `Application`, catalog providers, reload manager
- Wires together scheduler, lifecycle, router, and telemetry

**Infrastructure Layer** (`internal/infra/`):
- Concrete implementations of domain interfaces
- External integrations (gRPC, Prometheus, file system)
- Organized by technical concern (catalog, scheduler, lifecycle, router, transport, etc.)

### Dependency Injection with Wire

The project uses Google Wire for compile-time dependency injection.

**Key Files**:
- `internal/app/wire.go`: Wire build tags and initialization function
- `internal/app/wire_gen.go`: Generated code (DO NOT EDIT)
- `internal/app/wire_sets.go`: Provider sets
- `internal/app/providers.go`: Provider functions

**Regenerating Wire Code**:
```bash
make wire
```

**Wire Pattern Example**:
```go
// Provider function
func NewScheduler(cfg Config) Scheduler {
    return &basicScheduler{cfg: cfg}
}

// Wire set
var SchedulerSet = wire.NewSet(
    NewScheduler,
    wire.Bind(new(domain.Scheduler), new(*basicScheduler)),
)
```

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

## Code Style and Conventions

### Naming Conventions

- **Types**: PascalCase for exported, camelCase for unexported
- **Functions**: camelCase for unexported, PascalCase for exported
- **Interfaces**: Named by capability (e.g., `Scheduler`, `Router`, `Transport`)
- **Implementations**: Often prefixed with strategy (e.g., `basicScheduler`, `metricRouter`)
- **Constants**: PascalCase with descriptive names

### Error Handling

- Return errors, don't panic (except for truly unrecoverable situations)
- Wrap errors with context: `fmt.Errorf("context: %w", err)`
- Use domain-specific error types when needed (`RouteError`, `ProtocolError`)
- Avoid defensive programming; validate only at trust boundaries

### Logging

- Use structured logging with `zap.Logger`
- Log levels: Debug, Info, Warn, Error
- Include context fields: `zap.String("server", name)`, `zap.Int("instance", id)`
- Avoid logging sensitive information (credentials, tokens)

### Concurrency

- Use context for cancellation and timeouts
- Protect shared state with mutexes (`sync.RWMutex` for read-heavy workloads)
- Use channels for coordination between goroutines
- Always handle goroutine lifecycle (start, stop, cleanup)

## Transport Types and Activation Modes

### Transport Types

**stdio** (most common):
- Launches subprocess and communicates via stdin/stdout
- Suitable for Node.js, Python, and other scripting languages
- Process lifecycle managed by mcpd

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

The `mcpdmcp` gateway:
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

## Important Guidelines from docs/AI_RULES.md

- Code, identifiers, comments, and CLI commands use **English**
- Prioritize **readability and maintainability** over premature optimization
- Use `camelCase` for variables/functions, `PascalCase` for types/structs
- **Avoid defensive programming**; validate only at trust boundaries
- Keep abstractions minimal; use interfaces only to isolate change points
- Evaluate risk before modifying public APIs or protocols

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

## Additional Resources

- **MCP Specification**: https://modelcontextprotocol.io/
- **Wails Documentation**: https://wails.io/
- **Wire Documentation**: https://github.com/google/wire
- **CloudWeGo Eino**: https://github.com/cloudwego/eino
- **Frontend Guide**: See `frontend/CLAUDE.md` for React/TypeScript frontend development

## Version Information

- **Go**: 1.25+
- **Wails**: 3.0.0-alpha.53
- **MCP SDK**: github.com/modelcontextprotocol/go-sdk v1.2.0
- **Eino**: github.com/cloudwego/eino v0.7.15
- **Prometheus**: github.com/prometheus/client_golang v1.23.2

## Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests in a specific package
go test ./internal/infra/scheduler

# Run a specific test
go test -run TestSchedulerBasic ./internal/infra/scheduler
```

### Test Organization

- Test files use `*_test.go` naming convention
- Tests are co-located with implementation files
- Use table-driven tests for multiple scenarios
- Mock external dependencies using interfaces

## Common Development Tasks

### Modifying Core Logic

1. Update domain interfaces in `internal/domain/` if needed
2. Implement changes in `internal/infra/` or `internal/app/`
3. Update wire providers if adding new dependencies
4. Regenerate wire code: `make wire`
5. Run tests: `make test`
6. Lint: `make lint-check`

### Adding Wails UI Features

1. Implement Go service methods in `internal/ui/`
2. Regenerate TypeScript bindings: `make wails-bindings`
3. Use bindings in frontend: `import { ServiceMethod } from '@/bindings/...'`
4. See `frontend/CLAUDE.md` for frontend-specific guidance

### Adding a New Transport Type

1. Define transport in `internal/domain/transport.go`
2. Implement `domain.Transport` interface in `internal/infra/transport/`
3. Register transport in lifecycle manager
4. Add configuration schema to `internal/domain/types.go`

### Adding a New RPC Method

1. Define protobuf message and service in `proto/mcpd/control/v1/control.proto`
2. Run `make proto` to generate Go code
3. Implement handler in `internal/app/control_plane_api.go`
4. Add client method in `internal/infra/gateway/gateway.go`

### Modifying Configuration Schema

1. Update domain types in `internal/domain/types.go`
2. Update validation in `internal/app/validate.go`
3. Update example in `docs/catalog.example.yaml`
4. Run `go run ./cmd/mcpd validate` to test

### Adding Metrics

1. Define metric in `internal/domain/metrics.go`
2. Register metric in `internal/infra/telemetry/prometheus.go`
3. Instrument code with metric updates
4. Metrics automatically exposed at `/metrics` endpoint

## Troubleshooting

### Wire Generation Fails

```bash
# Ensure wire is installed
make tools

# Check for circular dependencies in provider functions
# Review internal/app/wire_sets.go for issues
```

### Tests Failing

```bash
# Run tests with verbose output
go test -v ./...

# Run specific test with race detector
go test -race -run TestName ./path/to/package
```
