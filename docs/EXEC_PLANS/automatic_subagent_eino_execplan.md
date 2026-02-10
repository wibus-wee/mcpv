# Automatic MCP SubAgent with cloudwego/eino

This ExecPlan is a living document governed by `.agent/PLANS.md`; every section below must be kept up to date as work progresses. It describes building an automatic MCP SubAgent inside `mcpv` that orchestrates tool discovery, deduplication, and execution via the cloudwego/eino LLM framework so that clients see only `mcpv.automatic_mcp`/`mcpv.automatic_eval`.

## Purpose / Big Picture

After this change, the MCP client will only need to configure a single MCP server entry (the `mcpv` process) and call `mcpv.automatic_mcp` to discover applicable tools and `mcpv.automatic_eval` to execute them. `mcpv` core will host a stateless SubAgent backed by cloudwego/eino that selects the right tool metadata per caller profile, deduplicates previously sent fields, and forwards evaluation requests, eliminating the need for clients to carry the full tool schema in every prompt while still letting them trigger existing MCP tools. A novice can see the behavior by running `cargo test ...` (or `make test`) once the integrations and session tracking are implemented.

## Progress

- [x] (2025-12-29 13:40Z) Create ExecPlan describing the automatic MCP SubAgent workflow, deduplication/session concerns, and cloudwego/eino integration.
- [ ] (2025-12-29 13:45Z) Identify where to hook the SubAgent and session cache into `internal/infra/mcp` and `internal/app` once implementation begins.
- [ ] (2025-12-29 13:50Z) Implement SubAgent adapter, session deduplication, new tools, and tests per the plan. (pending execution)

## Surprises & Discoveries

- None yet; planning phase only.

## Decision Log

- Decision: Adopt cloudwego/eino as the LLM framework for the SubAgent.
  Rationale: The user explicitly requested this framework and it satisfies the need for a controllable, local LLM orchestration layer as described earlier in the conversation.
  Date/Author: 2025-12-29 / Codex

## Outcomes & Retrospective

Planning only; no observable outcomes yet. The next milestone will produce new code paths in `internal/infra` and `internal/app` to host the automatic tools and to surface session-aware metadata to clients.

## Context and Orientation

`cmd/mcpv` wires CLI flags into `internal/app`, which orchestrates catalog, scheduler, router, lifecycle, and transport. Tool registration and execution logic live under `internal/infra/*`. The new automatic tool management will primarily touch `internal/infra/mcp`, `internal/app`'s registry glue, and the tool schema serializers that feed MCP `tools/list` responses. cloudwego/eino will be introduced as a dependency where the SubAgent needs prompt orchestration; it will operate purely inside `mcpv` core and not be exposed as a separate MCP server that clients configure.

## Plan of Work

We will sequentially (1) analyze the MCP session flow to understand where tool metadata is assembled so that deduplication can occur, (2) design and implement a lightweight cloudwego/eino SubAgent interface that accepts caller profile + tool registry snapshot and returns filtered metadata, (3) augment `mcpv` core to keep session maps keyed by caller IDs, track what metadata has already been emitted, detect when compression wiped the context, and decide when to resend full descriptions vs. names-only, (4) add `mcpv.automatic_eval` as the proxy tool that accepts `toolName + params`, validates permissions, and forwards to the real MCP tool implementation, and (5) provide hooks/tests to show that the `automatic_mcp` response shrinks over time and that the new eval path works end to end.

## Concrete Steps

1. Update `go.mod` to include the cloudwego/eino dependency and vendor artifacts if necessary for the build; run `go mod tidy`.
2. Implement the SubAgent abstraction (e.g., `internal/infra/mcp/subagent_eino.go`) that translates a caller profile and catalog entry list into a filtered set of metadata entries.
3. Extend `internal/app`'s catalog/scheduler wiring so that `mcpv.automatic_mcp` requests run through the SubAgent, then through a session deduplicator located alongside the existing context compressor.
4. Add a new MCP tool definition (`mcpv.automatic_eval`) in `internal/infra/mcp/tool_registry.go` that accepts caller identity, `toolName`, and serialized params, and invokes the underlying tool registry as a proxy.
5. Create integration tests (likely in `internal/infra/mcp` or `internal/domain`) that simulate a client calling `automatic_mcp` twice (with compression between) and assert that only changed metadata is emitted, while `automatic_eval` dispatches to the correct underlying tool and receives the same output as a direct call.
6. Document the new tooling workflow in `docs/` (using the profile store layout: `runtime.yaml`, `callers.yaml`, and `profiles/*.yaml`) so other contributors know how to configure `mcpv` for the proxy mode.

## Validation and Acceptance

Run `make test` (which runs `go test ./...`) and expect all existing cases to pass. Add a focused test `TestAutomaticmcpveduplication` that fails before the deduplication/session logic (for example by checking that two successive `automatic_mcp` responses have identical metadata hashes) and passes after. Validate manually by starting `mcpv` (`mcpv serve --config <profile-store-dir>`), pointing a simple MCP client at `mcpv.automatic_mcp`, observing that the first response includes tool summaries and the second (without any catalog change) only returns names/hashes, and then calling `mcpv.automatic_eval` with a stored `toolName` to confirm the real tool executes with the same result as calling it directly.

## Idempotence and Recovery

All steps can be rerun safely. If adding the new tool definitions fails, revert the added files to keep the existing registry intact; the SubAgent injection is additive and can be disabled by sticking to the legacy `tools/list` response. Running `go test ./...` after each change resets generated artifacts and ensures no stray state remains.

## Artifacts and Notes

Examples of expected command output:

1. `go test ./internal/infra/mcp -run TestAutomaticmcpveduplication`
   ```
   ok   github.com/mcpv/internal/infra/mcp 0.512s
   ```

2. Manual `mcpv.automatic_eval` invocation translation to the actual MCP tool shown in the log output under `logs/mcpv.log` (record exact log line once implemented).

## Interfaces and Dependencies

The SubAgent will expose a function like `func SelectToolsForCaller(ctx context.Context, caller CallerProfile, tools []ToolMetadata) ([]FilteredTool, error)` in `internal/infra/mcp/subagent.go`. `ToolMetadata` contains the MCP tool name, short description, namespace, and schema hash. `FilteredTool` includes the same fields plus a `VersionHash` and `ShouldResendFullSchema` boolean. The session deduplicator will live in `internal/infra/mcp/session_cache.go` and maintain a map from `callerID` to the last `VersionHash` values seen. `cloudwego/eino` will be added as a dependency and used to run the LLM prompt that decides tool relevance; it must not be exposed to clients but only invoked from `mcpv` core. The new `automatic_eval` tool simply forwards `toolName` and `params` (as stored in `mcpv./internal/infra/mcp/tool_proxy.go`) to the real tool once permissions are validated.

Note: Initial plan drafted to capture the requested addition of cloudwego/eino and automatic MCP tooling; no code changes are applied yet.

Note: Added more progress checkpoints to reflect planning granularity and to keep the living document in sync with the projected implementation steps.
