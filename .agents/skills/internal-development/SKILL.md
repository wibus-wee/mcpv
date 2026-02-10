---
name: internal-development
description: Guidelines for Go backend development including architecture patterns, code style, testing, and common development tasks. Use when working on internal/, cmd/, or Go code.
user-invocable: false
---

# Internal Development Guidelines

Guidelines for developing the Go backend of mcpd, including core control plane, gateway, and infrastructure components.

## Prerequisites

- Go 1.25+
- protoc (for gRPC)
- golangci-lint (for linting)
- wire (installed via `make tools`)

## Development Commands

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

## Important Guidelines

- Code, identifiers, comments, and CLI commands use **English**
- Prioritize **readability and maintainability** over premature optimization
- Use `camelCase` for variables/functions, `PascalCase` for types/structs
- **Avoid defensive programming**; validate only at trust boundaries
- Keep abstractions minimal; use interfaces only to isolate change points
- Evaluate risk before modifying public APIs or protocols

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

1. Define protobuf message and service in `proto/mcpv/control/v1/control.proto`
2. Run `make proto` to generate Go code
3. Implement handler in `internal/app/control_plane_api.go`
4. Add client method in `internal/infra/gateway/gateway.go`

### Modifying Configuration Schema

1. Update domain types in `internal/domain/types.go`
2. Update validation in `internal/app/validate.go`
3. Update example in `docs/catalog.example.yaml`
4. Run `go run ./cmd/mcpv validate` to test

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
