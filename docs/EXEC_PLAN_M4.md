# Harden M4 observability: logging fields, healthz, schema validation

This ExecPlan is a living document. The sections Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective must be kept up to date as work proceeds.

This document must be maintained in accordance with .agent/PLANS.md from the repository root.

## Purpose / Big Picture

After this change, the core control plane emits structured logs with a consistent set of fields, exposes an optional healthz HTTP endpoint that reports whether background loops are alive, and rejects invalid catalog files using a JSON Schema before the existing semantic checks run. A user can start mcpd, curl /healthz for a quick liveness snapshot, and get clear, schema-based config errors when running mcpd validate. The goal is to make observability and validation reliable without changing the core/gateway split or the MCP protocol surface.

## Progress

- [x] (2025-12-25 15:22Z) Create M4 ExecPlan for observability hardening.
- [x] (2025-12-25 15:37Z) Add JSON Schema validation for catalog config before semantic checks.
- [x] (2025-12-25 15:37Z) Add a health tracker and /healthz HTTP endpoint with env toggles.
- [x] (2025-12-25 15:37Z) Unify log fields and event names for lifecycle, scheduler, and router logs.
- [x] (2025-12-25 15:37Z) Add tests and doc updates; run fmt/test.

## Surprises & Discoveries

- Observation: Local TCP listen can be denied in the sandbox, causing telemetry HTTP server tests to fail.
  Evidence: listen tcp 127.0.0.1:0: bind: operation not permitted.

## Decision Log

- Decision: Use github.com/google/jsonschema-go with an embedded JSON Schema for catalog validation.
  Rationale: The module already exists in go.mod, it validates draft 2020-12, and it keeps schema validation deterministic without adding new dependencies.
  Date/Author: 2025-12-25 / Codex.

- Decision: Use env toggles MCPD_METRICS_ENABLED and MCPD_HEALTHZ_ENABLED, with listen address configured in catalog.
  Rationale: This enables healthz without changing the env-based toggles while allowing port configuration to avoid conflicts.
  Date/Author: 2025-12-25 / Codex.

- Decision: Track goroutine liveness via heartbeat timestamps from the idle manager, ping manager, and tool refresh loop.
  Rationale: It is the simplest way to detect background loop stalls without invasive scheduler state inspection.
  Date/Author: 2025-12-25 / Codex.

- Decision: Keep the JSON Schema focused on structure and types, while leaving range and protocolVersion checks in the existing semantic validation.
  Rationale: It adds strictness for unknown keys and type mismatches without changing existing error messaging patterns.
  Date/Author: 2025-12-25 / Codex.

## Outcomes & Retrospective

Completed M4 observability hardening: schema validation is in place, healthz is available behind env flags, log fields are consistent, and tests pass with sandbox-aware skips for TCP listen. Remaining work is the optional follow-up to add richer schema constraints if stricter validation is desired.

## Context and Orientation

The core entry point is cmd/mcpd/main.go, which wires internal/app/app.go. The app layer loads the catalog via internal/infra/catalog/loader.go, starts the scheduler (internal/infra/scheduler/basic.go), router (internal/infra/router/router.go), lifecycle manager (internal/infra/lifecycle/manager.go), tool index (internal/infra/aggregator/aggregator.go), and the gRPC control plane (internal/infra/rpc/server.go). Telemetry currently provides a Prometheus metrics server in internal/infra/telemetry/server.go and a log broadcaster in internal/infra/telemetry/log_broadcaster.go, but there is no healthz endpoint and log fields are inconsistent across subsystems.

In this plan, “healthz endpoint” means an HTTP endpoint at /healthz that returns HTTP 200 with a JSON body when the background loops (idle manager, ping manager, tool refresh) have emitted a recent heartbeat, and HTTP 503 with details when any loop is stale. “Goroutine liveness” refers to these background loops continuing to tick at their configured intervals.

## Plan of Work

First, add schema-based config validation to internal/infra/catalog/loader.go. Create a JSON Schema that matches the current catalog shape, embed it in the catalog package, and validate the expanded config content before Viper unmarshalling and the existing semantic checks. Keep the existing validation logic to enforce protocol version and cross-field constraints, but rely on the schema to reject unknown keys and incorrect types early. Update loader tests to cover schema failures without weakening existing assertions.

Next, add a health tracker in internal/infra/telemetry that can register a heartbeat for a named loop and report whether it is stale. Extend the telemetry HTTP server so it can serve /metrics and /healthz from the same mux, and expose a new StartHTTPServer entry point that takes options for which endpoints to enable. Wire this from internal/app/app.go using MCPD_METRICS_ENABLED and MCPD_HEALTHZ_ENABLED so either endpoint can be enabled without changing the catalog file. Add heartbeats in the scheduler idle and ping loops, and in the tool index refresh loop, and unregister them when the loops stop.

Then, unify structured logging fields and event names across lifecycle, scheduler, and router. Introduce a small set of field helpers (event, serverType, instanceID, state, duration_ms) in telemetry, and use them in the places where start/stop, ping failures, idle reaps, and route errors are logged. Ensure error logs use zap.Error so the error field remains consistent. The goal is not to add noisy logs, but to make the existing logs uniform and easier to filter.

Finally, update docs and tests. Add tests for schema validation failures, healthz responses (healthy and stale), and adjust telemetry server tests to use the new HTTP server entry point. Update docs/catalog.example.yaml to document the healthz env flag alongside the existing metrics flag. Run gofmt, go vet, and tests (with GOCACHE pointing into the workspace if needed).

## Concrete Steps

All commands run from /Users/wibus/dev/mcpd.

1) Review current catalog loader and telemetry server code to anchor the edits.
    rg -n "Load\(|routeTimeoutSeconds|StartMetricsServer" internal/infra

2) Implement schema validation, add the embedded schema, and update loader tests.
    go test ./internal/infra/catalog

3) Implement the health tracker and HTTP server, wire app, scheduler, and tool index, then update telemetry tests.
    go test ./internal/infra/telemetry ./internal/infra/scheduler ./internal/infra/aggregator

4) Apply logging field unification and adjust any affected tests.
    go test ./internal/infra/lifecycle ./internal/infra/router

5) Run formatting, vet, and the full test suite.
    make fmt
    make vet
    make test

If the Go build cache is restricted in this environment, set GOCACHE to a writable directory inside the repo, for example:
    GOCACHE=/Users/wibus/dev/mcpd/.cache/go-build make test

## Validation and Acceptance

- Config validation: running `mcpd validate --config <bad file>` fails with a schema error for unknown keys or wrong types, and still reports semantic errors like protocolVersion mismatch.
- Healthz endpoint: with MCPD_HEALTHZ_ENABLED=true, starting `mcpd serve --config docs/catalog.example.yaml` exposes http://localhost:9090/healthz (or the address set in `observability.listenAddress`) returning HTTP 200 with a JSON body that lists the registered checks. If a loop is intentionally stalled in a test, /healthz returns HTTP 503 with that check marked stale.
- Logging: lifecycle start/stop, ping failure, idle reap, and route errors include the fields event, serverType, instanceID (when available), state (when available), duration_ms (when available), and error (when applicable).
- Tests: `go test ./...` (or `make test`) passes, and new tests covering schema validation and healthz behavior are present.

## Idempotence and Recovery

Schema validation is a pure read and can be re-run without side effects. The health tracker uses in-memory timestamps and does not persist state, so restarting the process resets healthz state. The HTTP server binds to `observability.listenAddress` only when enabled; if the port is in use, adjust the config, disable the env flags, or stop the conflicting process and retry. All changes are additive and safe to reapply.

## Artifacts and Notes

Example healthz response (healthy):
    {"status":"ok","checks":[{"name":"scheduler.idle","healthy":true,"lastBeat":"2025-12-25T15:37:00Z","staleAfterMs":3000}]}

Example schema error (unknown key):
    schema validation failed: validating catalog: additionalProperties: object has unexpected property "unknownKey"

## Interfaces and Dependencies

New or updated interfaces and files should exist after this change:

- internal/infra/catalog/schema.json: JSON Schema describing the catalog structure. It must disallow unknown top-level keys and unknown server spec keys.
- internal/infra/catalog/schema.go: embed schema.json and expose a validateCatalogSchema(raw string) error helper.
- internal/infra/telemetry/health.go:
    type HealthTracker struct { ... }
    func NewHealthTracker() *HealthTracker
    func (t *HealthTracker) Register(name string, staleAfter time.Duration) *Heartbeat
    type Heartbeat struct { ... }
    func (h *Heartbeat) Beat()
    func (h *Heartbeat) Stop()
    func (t *HealthTracker) Report() HealthReport
    type HealthReport struct { Status string; Checks []HealthCheckStatus }
    type HealthCheckStatus struct { Name string; Healthy bool; LastBeat time.Time; StaleAfterMs int64 }

- internal/infra/telemetry/server.go:
    type HTTPServerOptions struct {
        Addr          string
        EnableMetrics bool
        EnableHealthz bool
        Health        *HealthTracker
    }
    func StartHTTPServer(ctx context.Context, opts HTTPServerOptions, logger *zap.Logger) error

- internal/infra/telemetry/log_fields.go:
    const FieldEvent = "event"
    const FieldServerType = "serverType"
    const FieldInstanceID = "instanceID"
    const FieldState = "state"
    const FieldDurationMs = "duration_ms"
    const (
        EventStartAttempt     = "start_attempt"
        EventStartSuccess     = "start_success"
        EventStartFailure     = "start_failure"
        EventInitializeFailure = "initialize_failure"
        EventPingFailure      = "ping_failure"
        EventRouteError       = "route_error"
        EventIdleReap         = "idle_reap"
        EventStopSuccess      = "stop_success"
        EventStopFailure      = "stop_failure"
    )

- internal/infra/scheduler/basic.go: add optional health tracker fields and beat from idle/ping loops.
- internal/infra/aggregator/aggregator.go: add optional health tracker and beat from the refresh loop.
- internal/app/app.go: create HealthTracker, start HTTP server when env flags are enabled, and pass the tracker into scheduler/tool index.

Plan Update Note: Initial creation of the M4 ExecPlan.
Plan Update Note: Marked implementation complete, recorded sandbox listen constraint, added decisions, and captured example outputs after running tests.
