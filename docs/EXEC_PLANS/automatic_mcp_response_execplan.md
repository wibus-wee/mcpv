# Simplify automatic_mcp Output and automatic_eval Validation

This ExecPlan is a living document governed by `.agent/PLANS.md`; every section below must be kept up to date as work progresses. It describes simplifying the `mcpv.automatic_mcp` response to return raw MCP tool definitions, switching schema deduplication to a session identifier instead of client-provided hashes, and adding argument validation for `mcpv.automatic_eval` that returns tool schemas on errors.

## Purpose / Big Picture

完成后，客户端调用 `mcpv.automatic_mcp` 会得到与 MCP Server `tools/list` 同形的工具定义 JSON，而不再包含 `schema_hash`、`spec_key`、`full_schema` 等自定义字段，同时通过 `sessionId` 在服务端做去重与记忆。对于 `mcpv.automatic_eval`，当工具参数不符合工具输入 schema 时，会返回带有错误信息和该工具 schema 的 error 结果，方便 AI 自行修正参数。可以通过调用 `automatic_mcp` 两次观察第二次返回的工具列表缩减，以及用错误参数调用 `automatic_eval` 看见 error payload 中包含工具 schema 来验证。

## Progress

- [x] (2025-12-30 05:41Z) Create ExecPlan for simplifying automatic_mcp payloads and adding automatic_eval validation.
- [x] (2025-12-30 05:41Z) Update domain/proto/tool schemas to use sessionId and raw tool JSON payloads, regenerate protobufs, and adjust gateway serialization.
- [x] (2025-12-30 05:41Z) Implement session-based deduplication in SubAgent and fallback paths, returning only raw tool definitions to clients.
- [x] (2025-12-30 05:41Z) Add automatic_eval argument validation and error payload containing tool schema, plus focused unit tests.
- [x] (2025-12-30 05:46Z) Run `GOCACHE=/Users/wibus/dev/mcpv/.go-cache go test ./...` and capture linker warnings for `internal/ui` on macOS version mismatch.

## Surprises & Discoveries

- Observation: `go test ./...` emits `ld: warning: object file ... was built for newer 'macOS' version (26.0) than being linked (11.0)` when building `internal/ui`.
  Evidence: Warnings captured in the test output for `mcpv/internal/ui`.

## Decision Log

- Decision: Replace `knownSchemaHashes` with a caller-provided `sessionId` and track deduplication on the server.
  Rationale: Client-side hash tracking adds extra payload and complexity; a stable session identifier aligns with the requested "AI memory" model.
  Date/Author: 2025-12-30 / Codex
- Decision: Return raw MCP tool JSON objects in `automatic_mcp` instead of custom `FilteredTool` structs.
  Rationale: The request explicitly wants direct mapping to MCP server tool definitions and avoids base64-encoded schema blobs.
  Date/Author: 2025-12-30 / Codex

## Outcomes & Retrospective

实现已完成，并运行 `go test ./...` 通过；存在 macOS 版本链接警告但不影响测试结果。若该告警与本地工具链配置有关，可后续单独处理。

## Context and Orientation

`internal/app/control_plane.go` contains the core orchestration for `AutomaticMCP` and `AutomaticEval`. `internal/infra/subagent/subagent.go` implements the LLM-based filtering and deduplication. `internal/infra/gateway/gateway.go` exposes `mcpv.automatic_mcp` and `mcpv.automatic_eval` to MCP clients, serializing responses into `mcp.CallToolResult`. Protobuf contracts live in `proto/mcpv/control/v1/control.proto`, with generated files in `pkg/api/control/v1/`. Tool definitions are stored as `json.RawMessage` in `internal/domain.ToolDefinition` and already mirror MCP `tools/list` payloads.

## Plan of Work

First, update the input/output contracts to remove `knownSchemaHashes` and custom schema metadata from `automatic_mcp`. Add a `sessionId` string parameter to `AutomaticMCP` requests, propagate it through domain types, gRPC, and gateway tool schemas, and define `automatic_mcp` responses as arrays of raw tool JSON objects. Update the gateway handler to construct the JSON response manually so that `tools` are emitted as objects, not base64-encoded strings.

Second, implement server-side deduplication keyed by `sessionId`. Both the SubAgent path and the fallback path should check a per-session cache keyed by the provided `sessionId` (or fallback to caller id if empty). Only tools that are new or have changed hashes for that session should be returned, and the cache should be updated on each call. Maintain the existing LLM filtering behavior so only relevant tools are considered.

Third, add argument validation in `AutomaticEval`. Lookup the tool definition for the requested tool name, parse its `inputSchema`, validate the provided arguments, and if validation fails return a `mcp.CallToolResult` error whose text content is a JSON object containing `error` and `toolSchema`. Only schema validation failures should be converted into this structured error response; other failures should continue to propagate as errors.

Finally, add unit tests for the argument validation/error payload, update any existing tests affected by the protobuf or domain changes, regenerate protobufs via `make proto`, and run `go test ./...` (or `make test`) to verify.

## Concrete Steps

1. Edit `internal/domain/subagent.go` and `internal/domain/controlplane.go` to replace `knownSchemaHashes` with `sessionId`, and to update `AutomaticMCP` results to carry raw tool JSON arrays.
2. Update `proto/mcpv/control/v1/control.proto` for the new request/response fields; run `make proto` in the repo root to regenerate `pkg/api/control/v1/control.pb.go` and `pkg/api/control/v1/control_grpc.pb.go`.
3. Update `internal/infra/gateway/gateway.go` to parse the new request fields and build a JSON response that embeds tool schemas as raw objects (using `json.RawMessage`).
4. Update `internal/infra/subagent/subagent.go` (and any cache helpers) to use session-based deduplication and return only tool JSON objects for tools that need resending.
5. Update `internal/app/control_plane.go` to adjust `AutomaticMCP` fallback behavior and implement argument validation for `AutomaticEval`, returning structured error payloads with tool schemas.
6. Add tests in `internal/app` to validate schema errors and error payloads; update `internal/infra/rpc/control_service_test.go` fake control plane signatures to match the new domain types.
7. Run `go test ./...` (or `make test`) from the repo root and record the results.

## Validation and Acceptance

Run `make proto` and then `go test ./...` in the repository root. Expect all tests to pass. Validate manually by calling `mcpv.automatic_mcp` twice with the same `sessionId` and verifying that the second response contains fewer tool objects when no tool schemas changed. Then call `mcpv.automatic_eval` with a valid tool name but invalid arguments; expect a JSON error payload where `toolSchema` matches the tool definition returned by `automatic_mcp`.

## Idempotence and Recovery

Edits are additive and can be repeated. If protobuf regeneration fails, revert changes to `proto/mcpv/control/v1/control.proto` and the generated files, then rerun `make proto`. If `automatic_eval` validation causes unexpected regressions, revert the validation helper and keep the structured error response generator isolated to allow quick rollback.

## Artifacts and Notes

Expected `make proto` run (repo root):

  $ make proto

Expected failing error payload example (from `automatic_eval`):

  {"error":"invalid tool arguments: ...","toolSchema":{"name":"example.tool","inputSchema":{"type":"object","required":["foo"]}}}

## Interfaces and Dependencies

The `AutomaticMCP` request type must include `SessionID string` alongside `Query` and `ForceRefresh`. The `AutomaticMCP` result type must include `ETag string`, `Tools []json.RawMessage`, `TotalAvailable int`, and `Filtered int`. `AutomaticEval` must validate arguments against the tool definition's `inputSchema` using `github.com/google/jsonschema-go/jsonschema` (already used in `internal/infra/catalog/schema.go`). The gateway handler should serialize `Tools` as raw JSON objects via `json.RawMessage` so the MCP client receives normal tool definitions without base64 encoding.

Note: Created this ExecPlan to capture the requested changes to `automatic_mcp` output shape, session-based deduplication, and `automatic_eval` argument validation.
