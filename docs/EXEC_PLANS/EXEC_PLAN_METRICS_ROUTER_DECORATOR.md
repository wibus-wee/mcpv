# Metrics Router Decorator and Pool Capacity Observability

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

Repository root contains `.agent/PLANS.md`; this document must be maintained in accordance with that file.

## Purpose / Big Picture

After this change, routing metrics are recorded by a decorator rather than embedded in business logic, and route metrics include caller/profile dimensions plus a clearer failure reason label. The scheduler also emits a pool capacity ratio metric (mcpd_pool_capacity_ratio) so local operators can see when a server is running hot. Success is visible in Prometheus metrics output and in passing tests.

## Progress

- [x] (2025-12-30 09:45Z) Add routing context and route error staging, plus the metrics decorator and context injection.
- [x] (2025-12-30 09:45Z) Remove BasicRouter metrics wiring, return staged route errors, and classify connection closed errors.
- [x] (2025-12-30 09:45Z) Extend Metrics interface, Prometheus implementation, and scheduler pool capacity metric updates.
- [x] (2025-12-30 09:46Z) Run full tests and capture outcomes and artifacts.

## Surprises & Discoveries

- Observation: macOS linker warnings about object files built for newer versions appear during tests.
  Evidence: go test output includes ld warnings when linking mcpd/internal/ui.test.

## Decision Log

- Decision: Keep BasicRouter pure and move metrics collection into MetricRouter, with route context injected by discovery.
  Rationale: Preserves clean routing logic while enabling higher-cardinality metrics without inflating the router implementation.
  Date/Author: 2025-12-30 / Codex
- Decision: Model route error staging in domain and classify metric reasons from staged errors plus sentinel connection-closed error.
  Rationale: Provides stable reason labels without relying on string parsing or log-specific behavior.
  Date/Author: 2025-12-30 / Codex
- Decision: Emit pool capacity ratio in scheduler using busy count and max concurrent capacity.
  Rationale: Exposes a direct saturation signal for local deployments.
  Date/Author: 2025-12-30 / Codex

## Outcomes & Retrospective

- MetricRouter now handles route metrics with caller/profile/reason labels and staged route errors.
- Scheduler emits pool capacity ratio and Prometheus metrics include the new gauge.
- Full test suite passes with macOS linker warnings only.

## Context and Orientation

Routing logic lives in `internal/infra/router/router.go`, and today BasicRouter calls Metrics directly. The Metrics interface is in `internal/domain/metrics.go`. Prometheus implementation and tests are in `internal/infra/telemetry/prometheus.go` and `internal/infra/telemetry/prometheus_test.go`. Scheduler state lives in `internal/infra/scheduler/basic.go` and already reports active instance counts. Call paths flow from `internal/app/control_plane_discovery.go` to aggregator indices and then to the router.

This change introduces route context (caller/profile) and route error staging so metrics can label reasons, plus a pool capacity ratio metric.

## Plan of Work

Start by defining route context and route error staging in domain, then extend the Metrics interface to accept a richer route metric and a pool capacity ratio. Implement a MetricRouter decorator in the router package, remove metrics hooks from BasicRouter, and have discovery inject caller/profile into the route context. Add a connection-closed sentinel error in domain and return it from the transport layer. Finally, update the scheduler to emit pool capacity ratio and update Prometheus metrics and tests.

## Concrete Steps

Work from `/Users/wibus/dev/mcpd`.

1) Define route context, route error staging, and route metric structure.

   - Add `internal/domain/route_context.go` with `RouteContext` and `WithRouteContext`/`RouteContextFrom`.
   - Add `internal/domain/route_error.go` with `RouteStage`, `RouteError`, and helpers.
   - Update `internal/domain/metrics.go` to include `RouteMetric`, change `ObserveRoute` signature, and add `SetPoolCapacityRatio`.

2) Implement MetricRouter and route error staging.

   - Add `internal/infra/router/metric_router.go` that wraps a router and emits metrics using context metadata.
   - Update `internal/infra/router/router.go` to remove metric wiring and return staged route errors.
   - Update `internal/app/control_plane_discovery.go` to inject route context in CallTool/ReadResource/GetPrompt.
   - Update `internal/infra/transport/connection.go` to use `domain.ErrConnectionClosed` for closed connections.

3) Extend metrics and scheduler pool capacity ratio.

   - Update `internal/infra/telemetry/prometheus.go` to add mcpd_pool_capacity_ratio and expanded route labels.
   - Update `internal/infra/scheduler/basic.go` to compute busy/total capacity ratio and call `SetPoolCapacityRatio`.
   - Update `internal/infra/telemetry/prometheus_test.go` for the new labels and gauge.

4) Validate and backfill.

   - Run `GOCACHE=/Users/wibus/dev/mcpd/.gocache go test ./...`.
   - Update Progress, Decision Log, Outcomes, and Artifacts with the results.

## Validation and Acceptance

Run `GOCACHE=/Users/wibus/dev/mcpd/.gocache go test ./...` and expect a passing test suite. Prometheus metrics should include `mcpd_route_duration_seconds` with caller/profile/status/reason labels and `mcpd_pool_capacity_ratio`. Route failures should map to reasons like timeout_cold_start, timeout_execution, and conn_closed.

## Idempotence and Recovery

Edits are safe to repeat. If compilation fails, align Metrics interface changes across implementations and call sites. If Prometheus tests fail due to label changes, update the test fixtures to match the new label set.

## Artifacts and Notes

- New decorator file: `internal/infra/router/metric_router.go`.
- New domain context/error files: `internal/domain/route_context.go`, `internal/domain/route_error.go`.

## Interfaces and Dependencies

- `internal/domain`:

  - `type RouteContext struct { Caller string; Profile string }`
  - `func WithRouteContext(ctx context.Context, meta RouteContext) context.Context`
  - `func RouteContextFrom(ctx context.Context) (RouteContext, bool)`
  - `type RouteStage string` with values `decode`, `validate`, `acquire`, `call`
  - `type RouteError struct { Stage RouteStage; Err error }`
  - `type RouteMetric struct { ServerType string; Caller string; Profile string; Status string; Reason string; Duration time.Duration }`
  - `type Metrics interface { ObserveRoute(RouteMetric); SetPoolCapacityRatio(specKey string, ratio float64); ... }`

- `internal/infra/router`:

  - `type MetricRouter struct { inner domain.Router; metrics domain.Metrics }`
  - `func NewMetricRouter(inner domain.Router, metrics domain.Metrics) *MetricRouter`

Plan Revision Note: Updated progress, decisions, outcomes, and surprises after implementing the decorator, metrics changes, and tests, to reflect the completed work and observed linker warnings.
