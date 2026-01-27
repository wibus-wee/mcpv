# Implement Full MCP Sampling, Elicitation, and Tasks Support in mcpd Core

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This plan must be maintained in accordance with `.agent/PLANS.md` at the repository root.

## Purpose / Big Picture

After this change, mcpd will honor the MCP protocol requirements for sampling and elicitation when acting as a client to downstream MCP servers. Servers can request model sampling from mcpd via `sampling/createMessage`, and can request structured user input via `elicitation/create`. These behaviors are observable through JSON-RPC traffic routed to downstream MCP servers.

Scope update (2026-01-27): Task support is deferred. The task-related APIs are left as TODO placeholders returning unimplemented errors until upstream MCP protocol support is available in go-sdk.

## Progress

- [x] (2026-01-27 00:00Z) Located repository ExecPlan requirements and core routing/transport files.
- [x] (2026-01-27 00:00Z) Identified MCP client/server gaps for sampling, elicitation, tasks, and URL elicitation errors.
- [x] (2026-01-27 00:00Z) Define protocol data models and interfaces for sampling and elicitation in `internal/domain`.
- [x] (2026-01-27 00:00Z) Implement client-side sampling and elicitation handlers, and wire them into transport and lifecycle initialization.
- [ ] (Deferred) Implement task manager, task-augmented tool calls, and tasks RPC endpoints.
- [x] (2026-01-27 00:00Z) Add URL_ELICITATION_REQUIRED error propagation and tests.
- [ ] Validate with unit tests and end-to-end flow (user reported tests passing; not re-run here).

## Surprises & Discoveries

- Observation: The current MCP client path always advertises sampling capabilities, but the transport connection never handles `sampling/createMessage`.
  Evidence: `internal/infra/lifecycle/manager.go` sets `ClientCapabilities.Sampling`, and `internal/infra/transport/connection.go` always responds “method not supported” to server-initiated requests.
- Observation: The go-sdk version used (`v1.1.0`) does not include tasks in its MCP server or protocol structs.
  Evidence: `mcp/protocol.go` has no `task` field for tool calls, and `mcp/server.go` method table lacks tasks methods.
- Observation: Downstream JSON-RPC errors from tool calls are converted into `CallToolResult` errors, masking protocol-level errors like `URL_ELICITATION_REQUIRED`.
  Evidence: `internal/infra/aggregator/aggregator.go:decodeToolResult` wraps `resp.Error` into a tool result.

## Decision Log

- Decision: Implement sampling/elicitation handlers at the transport client connection layer instead of replacing the existing router with go-sdk Client.
  Rationale: mcpd currently uses raw JSON-RPC routing, and replacing it would require large refactors of routing, scheduler, and aggregator logic. A handler interface preserves current architecture while enabling new protocol methods.
  Date/Author: 2026-01-27 / Codex
- Decision: Implement tasks in mcpd core via a dedicated TaskManager and new control plane APIs, and route task-related JSON-RPC methods to downstream servers using existing router facilities.
  Rationale: Tasks are not supported in the current go-sdk MCP server used by the gateway, so the most stable path is to add task support to core routing and control-plane APIs first, while keeping the gateway unchanged.
  Date/Author: 2026-01-27 / Codex
- Decision: Propagate URL elicitation errors as protocol-level errors instead of converting them into tool results.
  Rationale: The MCP spec defines `URL_ELICITATION_REQUIRED` as a JSON-RPC error (-32042) with structured data that clients must receive and act on.
  Date/Author: 2026-01-27 / Codex
- Decision: Defer tasks implementation to TODO placeholders.
  Rationale: go-sdk v1.2.0 does not expose MCP tasks methods or capabilities, so wiring full task support into mcpd would be misleading. Keep unimplemented stubs until upstream support lands.
  Date/Author: 2026-01-27 / Codex

## Outcomes & Retrospective

Not started.

## Context and Orientation

mcpd is a daemon that starts and routes requests to downstream MCP servers. Core request routing occurs in `internal/infra/router/router.go` (methods are validated via `internal/domain/methods.go`) and transport connections are built in `internal/infra/transport`. The MCP client initialization and capability negotiation is performed in `internal/infra/lifecycle/manager.go`. Tool calls are aggregated and routed by `internal/infra/aggregator/aggregator.go` and exposed through the control plane in `internal/app/control_plane.go` and `internal/infra/rpc`.

Key files for this change:

- `internal/infra/lifecycle/manager.go`: builds `initialize` request and advertises client capabilities.
- `internal/infra/transport/connection.go`: reads JSON-RPC messages and handles server-initiated requests.
- `internal/domain/methods.go`: validates which methods are allowed by server capabilities.
- `internal/infra/aggregator/aggregator.go`: builds tool calls, decodes responses, and maps errors.
- `proto/mcpd/control/v1/control.proto`: control plane gRPC API.

Terms used in this plan:

- “Downstream MCP server”: the MCP server that mcpd connects to and routes requests to.
- “Sampling”: server-to-client request `sampling/createMessage` for LLM generation.
- “Elicitation”: server-to-client request `elicitation/create` for structured user input.
- “Task-augmented tool call”: a `tools/call` request with a `task` field in params, which returns a task handle instead of immediate tool output.

## Plan of Work

First, define protocol-facing types and handler interfaces in `internal/domain` so that sampling, elicitation, and tasks have explicit, testable contracts. Then wire sampling and elicitation into the transport layer so that when a downstream MCP server sends server-initiated requests, mcpd can respond correctly. Update lifecycle initialization to advertise only the capabilities that are actually supported.

Next, defer task support until the upstream MCP protocol supports it in go-sdk. For now, expose TODO placeholders that return unimplemented errors.

Finally, adjust error handling so protocol-level errors like `URL_ELICITATION_REQUIRED` propagate without being converted into tool results. Add comprehensive tests for sampling, elicitation, tasks, and error propagation.

## Concrete Steps

1. Define protocol models and handler interfaces.

   Update or add files:

   - `internal/domain/protocol.go` (new) or `internal/domain/types.go` (extend):
     - `SamplingRequest`, `SamplingResult`, mirroring MCP `CreateMessage` parameters/result.
     - `ElicitationRequest`, `ElicitationResult` with `action` = `accept|decline|cancel` and optional `content`.
     - `Task`, `TaskStatus`, `TaskResult`, `TaskError` (JSON-RPC error shape).
     - `TaskCreateOptions` with `ttl` and optional fields.
   - `internal/domain/handlers.go` (new):
     - `SamplingHandler` interface with `CreateMessage(ctx, params) (result, error)`.
     - `ElicitationHandler` interface with `Elicit(ctx, params) (result, error)`.
     - `TaskStore` interface for `List`, `Get`, `Result`, `Cancel`, `Create`.
   - `internal/domain/errors.go` (new):
     - `ProtocolError` struct (code, message, data) to preserve JSON-RPC errors.
     - Constant `ErrCodeURLElicitationRequired = -32042`.

2. Wire sampling and elicitation into lifecycle initialization.

   Edit `internal/infra/lifecycle/manager.go`:

   - Inject a new `ClientCapabilityProvider` into the lifecycle manager constructor. This provider describes whether sampling and elicitation handlers are configured.
   - Only set `ClientCapabilities.Sampling` when a sampling handler is available.
   - Only set `ClientCapabilities.Elicitation` when an elicitation handler is available.

   Update `internal/app/providers.go` and `internal/app/wire_gen.go` to construct and pass this provider based on runtime configuration (see step 4).

3. Implement server-initiated request handling in transport connection.

   Edit `internal/infra/transport/connection.go`:

   - Replace the placeholder `handleServerCall` to route known methods:
     - `sampling/createMessage` to `SamplingHandler`.
     - `elicitation/create` to `ElicitationHandler`.
   - For each request, validate parameters and return JSON-RPC responses with either `result` or `error`.
   - If no handler is configured, return a JSON-RPC error indicating “method not supported”.

4. Implement concrete sampling and elicitation handlers.

   Add new packages:

   - `internal/infra/sampling`: an implementation that uses the configured SubAgent model to generate responses. It converts MCP sampling messages into the model’s prompt format.
   - `internal/infra/elicitation`: a minimal implementation that can be swapped. Default behavior should be “decline” or “cancel” to avoid blocking a daemon.

   Wire these handlers in `internal/app/application.go` by reading runtime configuration and attaching them to the lifecycle manager / transport.

5. (Deferred) Implement TaskManager and control plane task APIs.

   Add a new package `internal/infra/tasks`:

   - Manage tasks in-memory with mutex protection.
   - Store `taskId`, `status`, timestamps, TTL, pollInterval, and a `result` or `error`.
   - On `Create`, spawn a goroutine that calls a provided function and updates task status.
   - Support cancellation by storing a `context.CancelFunc`.

   Update control plane and RPC:

   - Extend `proto/mcpd/control/v1/control.proto` with new RPC methods:
     - `CreateTaskToolCall` (or `CallToolTask`) to request task-augmented tool calls.
     - `TasksGet`, `TasksList`, `TasksResult`, `TasksCancel`.
   - Regenerate `pkg/api/control/v1` using `protoc`.
   - Implement RPC handlers in `internal/infra/rpc/control_service.go`.
   - Implement control plane methods in `internal/app/control_plane.go` and service in `internal/app/control_plane_discovery.go` or a new `taskService`.

6. (Deferred) Support task-augmented tool calls.

   Update `internal/infra/aggregator/aggregator.go`:

   - Add a new code path that accepts a `task` option and returns `CreateTaskResult` with `taskId`, initial status, and `pollInterval`.
   - Preserve JSON-RPC errors in task results rather than converting them to `CallToolResult`.

7. Propagate URL elicitation errors.

   Update `internal/infra/aggregator/aggregator.go` and any call sites:

   - When a JSON-RPC error is returned with code `-32042`, do not convert it into a tool result.
   - Return a `ProtocolError` object that can be surfaced to callers or stored in tasks.

8. Tests and validation.

   Add unit tests:

   - `internal/infra/transport/connection_test.go` for `sampling/createMessage` and `elicitation/create` request handling.
   - `internal/infra/tasks/manager_test.go` for create/get/result/cancel/ttl behavior.
   - `internal/infra/aggregator/aggregator_test.go` for task-augmented tool calls and URL elicitation error propagation.

## Validation and Acceptance

Run tests and verify observable behaviors:

- Unit tests:

    (workdir: /Users/wibus/dev/mcpd)
    make test

  Expected: all existing tests pass, and new task/sampling/elicitation tests pass.

- Manual validation for sampling and elicitation:

  Run a mocked downstream MCP server that sends `sampling/createMessage` and `elicitation/create`. Confirm mcpd replies with valid JSON-RPC responses.

- Task-augmented tool call flow:

  Use control plane gRPC to invoke a tool call with `task` option.
  Expected:
    - Immediate response returns a task handle with status `working`.
    - `tasks/get` returns status updates and `pollInterval`.
    - `tasks/result` blocks until completion and returns final tool result or error.
    - `tasks/cancel` cancels in-flight tasks and updates status to `cancelled`.

- URL elicitation error propagation:

  Simulate downstream server returning JSON-RPC error code `-32042`. Confirm mcpd surfaces the error as a protocol error rather than embedding it in a tool result.

## Idempotence and Recovery

All steps are additive and can be re-run. TaskManager uses in-memory state and is safe across restarts but does not persist tasks. If a task test fails, it can be rerun with `go test` and no cleanup is required. If proto regeneration is needed, rerun the generation command and commit the updated files.

## Artifacts and Notes

Expected JSON-RPC request for task-augmented tool call:

    {
      "jsonrpc": "2.0",
      "id": 1,
      "method": "tools/call",
      "params": {
        "name": "get_weather",
        "arguments": {"city": "New York"},
        "task": {"ttl": 60000}
      }
    }

Expected CreateTaskResult response:

    {
      "jsonrpc": "2.0",
      "id": 1,
      "result": {
        "task": {
          "taskId": "786512e2-9e0d-44bd-8f29-789f320fe840",
          "status": "working",
          "statusMessage": "The operation is now in progress.",
          "createdAt": "2025-11-25T10:30:00Z",
          "lastUpdatedAt": "2025-11-25T10:40:00Z",
          "ttl": 60000,
          "pollInterval": 5000
        }
      }
    }

URL elicitation error shape:

    {
      "jsonrpc": "2.0",
      "id": 2,
      "error": {
        "code": -32042,
        "message": "This request requires more information.",
        "data": {
          "elicitations": [{
            "mode": "url",
            "elicitationId": "550e8400-e29b-41d4-a716-446655440000",
            "url": "https://example.com/connect?elicitationId=...",
            "message": "Authorization is required."
          }]
        }
      }
    }

## Interfaces and Dependencies

Define the following in `internal/domain`:

- `type SamplingHandler interface { CreateMessage(ctx context.Context, params *SamplingRequest) (*SamplingResult, error) }`
- `type ElicitationHandler interface { Elicit(ctx context.Context, params *ElicitationRequest) (*ElicitationResult, error) }`
- `type TaskManager interface { Create(ctx, ttl, fn) (Task, error); Get(ctx, taskId); List(ctx, cursor, limit); Result(ctx, taskId); Cancel(ctx, taskId) }`

Implement in:

- `internal/infra/transport/connection.go`: dispatch server calls using these handlers.
- `internal/infra/tasks/manager.go`: in-memory task management.
- `internal/infra/aggregator/aggregator.go`: support task-augmented tool calls and protocol error propagation.
