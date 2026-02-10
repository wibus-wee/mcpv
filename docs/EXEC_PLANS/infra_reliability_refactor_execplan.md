# Harden Infra Concurrency, JSON-RPC Semantics, and Metrics

This ExecPlan is a living document governed by `.agent/PLANS.md`; every section below must be kept up to date as work progresses. It describes a deliberate, best-practice refactor across infra components to eliminate deadlocks, enforce JSON-RPC correctness, close subscription lifecycles, and improve observability while preserving compatibility.

## Purpose / Big Picture

完成后，mcpv 的基础设施层将具备更稳定的并发行为、符合 JSON-RPC 标准的错误响应、明确且可关闭的订阅通道生命周期，以及更完整的 Prometheus 指标。这会直接降低死锁/空指针风险，避免客户端收到不规范错误，并让运维侧能区分成功/失败与耗时。可以通过运行单测、检查新增指标名、以及手动触发订阅/工具调用路径观察行为来验证。

## Progress

- [x] (2026-01-27 06:00Z) Create ExecPlan for infra reliability refactor and best-practice hardening.
- [x] (2026-01-27 06:10Z) Implement concurrency fixes, subscription lifecycle changes, schema validation reuse, and JSON-RPC error compliance.
- [x] (2026-01-27 06:10Z) Update observability metrics and deterministic env formatting, plus documentation comments.
- [x] (2026-01-27 06:10Z) Run gofmt on touched Go files.
- [x] (2026-01-27 07:49Z) Run `make test` and record outcomes.

## Surprises & Discoveries

- Observation: `make test` failed due to Go build cache permission denial in `/Users/wibus/Library/Caches/go-build/...` under sandbox.
  Evidence: `open /Users/wibus/Library/Caches/go-build/...: operation not permitted` during `go test ./...`.
- Observation: `NormalizeTransport` was missing in `internal/domain`, causing compile failure.
  Evidence: `internal/domain/spec_fingerprint.go:14:15: undefined: NormalizeTransport` resolved by adding `internal/domain/transport_kind.go`.
- Observation: `make test` succeeds with macOS linker warnings in `internal/ui` about object files built for newer macOS.
  Evidence: `ld: warning: object file ... was built for newer 'macOS' version (26.0) than being linked (11.0)` during `go test ./...`.

## Decision Log

- Decision: Close subscription channels on context cancellation using RW locks to prevent send-after-close races.
  Rationale: Provides explicit lifecycle semantics and avoids channel leaks while remaining safe under concurrent broadcast.
  Date/Author: 2026-01-27 / Codex
- Decision: Construct JSON-RPC error responses by decoding a wire-format response with the standard -32601 code.
  Rationale: The SDK’s wire error type is internal; decoding preserves correct error codes without depending on internal packages.
  Date/Author: 2026-01-27 / Codex
- Decision: Add new Prometheus metrics for instance start/stop outcomes and durations while keeping existing counters.
  Rationale: Improves observability without breaking existing dashboards or alerts.
  Date/Author: 2026-01-27 / Codex
- Decision: Treat unsupported sampling/elicitation calls as JSON-RPC method-not-found responses.
  Rationale: Aligns with JSON-RPC semantics for unsupported methods and ensures correct error codes on the wire.
  Date/Author: 2026-01-27 / Codex

## Outcomes & Retrospective

Implementation is complete; formatting is applied. Tests pass, with macOS linker warnings for `internal/ui`.

## Context and Orientation

The infra layer lives in `internal/infra`. Concurrency and subscription broadcasts are in `internal/infra/aggregator/index_core.go`, `internal/infra/aggregator/runtime_status_index.go`, and `internal/infra/aggregator/server_init_index.go`. JSON-RPC transport handling is in `internal/infra/transport/connection.go`. MCP transports are implemented in `internal/infra/transport/mcp_transport.go` and `internal/infra/transport/streamable_http.go`. Tool schema validation is duplicated between `internal/infra/aggregator/aggregator.go` and `internal/infra/gateway/tool_registry.go`. Prometheus metrics are in `internal/infra/telemetry/prometheus.go`. The catalog loader is in `internal/infra/catalog/loader.go`, and the gateway request handlers are in `internal/infra/gateway/gateway.go`.

## Plan of Work

First, remove the potential deadlock in `GenericIndex.Refresh` by using a buffered job queue and simplifying producer flow. Next, refactor subscription management in the aggregator indices to explicitly close channels on context cancel while holding a read/write lock during broadcast to prevent send-after-close races. Centralize `isObjectSchema` in `internal/infra/mcpcodec` and update callers and tests. Fix the loader to return `nil` on success, and harden gateway handlers with nil-safe access to request params.

Then, make JSON-RPC errors compliant by returning a standard “method not found” error object from the transport when receiving unknown calls, and add debug logging for dropped responses. Update the Streamable HTTP transport to clone requests before mutating headers. Make environment variable formatting deterministic. Finally, expand Prometheus metrics to use duration and error parameters while preserving existing counters, and add missing doc comments for exported types and methods. Format Go code and run tests.

## Concrete Steps

1. Edit `internal/infra/aggregator/index_core.go` to buffer job channels, add safe subscription closing, and adjust broadcast locking.
2. Edit `internal/infra/aggregator/runtime_status_index.go` and `internal/infra/aggregator/server_init_index.go` to close subscriber channels on context cancel and to broadcast under a read lock.
3. Add `internal/infra/mcpcodec/schema.go` with `IsObjectSchema`, update `internal/infra/aggregator/aggregator.go`, `internal/infra/gateway/tool_registry.go`, and `internal/infra/aggregator/aggregator_test.go` to use it, and remove duplicate helpers.
4. Fix `internal/infra/catalog/loader.go` to return `nil` on successful load.
5. Update `internal/infra/gateway/gateway.go` to guard `req.Params` access in tool handlers.
6. Update `internal/infra/transport/connection.go` with JSON-RPC method-not-found response construction and add debug logs for dropped responses.
7. Update `internal/infra/transport/streamable_http.go` to clone requests before adding headers.
8. Update `internal/infra/transport/command_launcher.go` to sort environment variables deterministically.
9. Update `internal/infra/telemetry/prometheus.go` and `internal/infra/telemetry/prometheus_test.go` to add and validate new metrics using duration and error params.
10. Add missing Go doc comments in `internal/infra/rpc/server.go`, `internal/infra/transport/mcp_transport.go`, and `internal/infra/transport/streamable_http.go`.
11. Run formatting and tests from repo root:

   $ gofmt -w internal/infra/aggregator/index_core.go internal/infra/aggregator/runtime_status_index.go internal/infra/aggregator/server_init_index.go internal/infra/aggregator/aggregator.go internal/infra/aggregator/aggregator_test.go internal/infra/gateway/tool_registry.go internal/infra/mcpcodec/schema.go internal/infra/catalog/loader.go internal/infra/gateway/gateway.go internal/infra/transport/connection.go internal/infra/transport/streamable_http.go internal/infra/transport/command_launcher.go internal/infra/telemetry/prometheus.go internal/infra/telemetry/prometheus_test.go internal/infra/rpc/server.go internal/infra/transport/mcp_transport.go

   $ make test

## Validation and Acceptance

Run `make test` in the repository root and expect all tests to pass. Verify that subscription consumers terminate cleanly on context cancellation without panics. Verify that JSON-RPC responses to unsupported methods include error code -32601. Confirm new metrics names are present via `prometheus.Registry.Gather()` in tests, and that `formatEnv` output is stable across runs.

## Idempotence and Recovery

All edits are safe to re-apply. If any change causes regressions, revert the specific file edits and rerun `make test` to confirm recovery. The new metrics are additive, so dashboards using existing metrics remain intact.

## Artifacts and Notes

Expected JSON-RPC wire error shape for unsupported methods:

  {"jsonrpc":"2.0","id":"<id>","error":{"code":-32601,"message":"method not found"}}

Expected new metric names to appear during tests:

  mcpv_instance_start_duration_seconds
  mcpv_instance_start_result_total
  mcpv_instance_stop_result_total

## Interfaces and Dependencies

In `internal/infra/mcpcodec/schema.go`, define:

  func IsObjectSchema(schema any) bool

It must accept `map[string]any`, `json.RawMessage`, raw JSON bytes, or arbitrary structs and return true if the schema type includes "object" (case-insensitive).

In `internal/infra/transport/connection.go`, define a helper that builds a `*jsonrpc.Response` for method-not-found using standard JSON-RPC error code -32601.

---

Change note: Recorded successful test run and macOS linker warnings.
