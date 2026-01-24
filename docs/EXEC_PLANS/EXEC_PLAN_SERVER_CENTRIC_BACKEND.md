# Server-Centric Config Backend Migration

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This plan must be maintained in accordance with `.agent/PLANS.md` in the repository root.

## Purpose / Big Picture

After this change, mcpd loads a single configuration file (via `--config <file>`), treats MCP servers as the primary data unit, and filters tool/resource/prompt visibility by client tags instead of profiles/caller mappings. A client registers with tags (for example, `--tag vscode --tag chat`) and sees servers when either the server has no tags, the client has no tags, or the tag intersection is non-empty. SubAgent no longer depends on profiles; it is globally configured and only enabled for clients whose tags match the configured allowed tags. This should be observable by starting mcpd with a single config file, registering two clients with different tags, and confirming each client only lists tools for the matching servers. `mcpd validate --config <file>` must validate the new single-file schema.

## Progress

- [x] (2026-01-24 09:02Z) Drafted the server-centric backend ExecPlan with scoped milestones and acceptance criteria.
- [x] (2026-01-24 09:02Z) Refined naming, API shapes, and tag semantics for server-centric backend scope.
- [x] (2026-01-24 09:55Z) Updated catalog runtime client-check field names in tests for the new config schema.
- [x] (2026-01-24 09:58Z) Migrated catalog diff tests to server-centric catalog state and runtime comparisons.
- [x] (2026-01-24 10:01Z) Updated telemetry tests for client-scoped route metrics.
- [x] (2026-01-24 10:05Z) Updated RPC control service and tests to map caller fields onto client-facing control plane APIs.
- [x] (2026-01-24 10:22Z) Updated app control-plane tests and runtime observability helpers to match client-tag flow.
- [x] (2026-01-24 10:28Z) Began UI refactor: client naming, tag exposure, and active-client events.
- [x] (2026-01-24 11:12Z) Switched catalog providers and reload manager to single-file config loading and tag-aware diffs.
- [x] (2026-01-24 11:12Z) Refactored control plane state, registry, discovery, and automation to client-tag filtering.
- [x] (2026-01-24 11:12Z) Updated RPC/gateway APIs to use client naming and tags; regenerated bindings.
- [x] (2026-01-24 11:18Z) Replaced profile-focused UI services with server-centric APIs and file-based config editor updates.
- [x] (2026-01-24 11:18Z) Added server-centric sample config and updated root sample to match single-file schema.

## Surprises & Discoveries

- Observation: None yet. This section will be updated as implementation uncovers unexpected behavior.

## Decision Log

- Decision: Use a single config file via `--config <file>` and eliminate `profiles/*.yaml` and `callers.yaml`.
  Rationale: Matches the server-centric mental model and removes configuration indirection.
  Date/Author: 2026-01-24 09:02Z, Codex.
- Decision: Tag visibility uses union semantics; servers with empty `tags` are visible to all clients.
  Rationale: Keeps the default experience simple while still allowing scoped groups.
  Date/Author: 2026-01-24 09:02Z, Codex.
- Decision: Client tags use the same rule as server tags: trim, lowercase, de-duplicate, stable sort.
  Rationale: Ensures deterministic matching and consistent UI display.
  Date/Author: 2026-01-24 09:02Z, Codex.
- Decision: Tags are normalized (trim, lowercase, de-duplicate, stable sort) for both server and client inputs.
  Rationale: Ensures deterministic filtering and stable UI display.
  Date/Author: 2026-01-24 09:02Z, Codex.
- Decision: Tags are excluded from spec fingerprints, so tag-only changes must be tracked explicitly in catalog diffs.
  Rationale: Prevents silent visibility changes and ensures watcher refreshes are triggered.
  Date/Author: 2026-01-24 09:02Z, Codex.
- Decision: SubAgent is globally configured with `enabledTags` and never changes visibility; it only filters tools already visible to the client.
  Rationale: Avoids double-filtering and aligns with the “tag decides visible servers” rule.
  Date/Author: 2026-01-24 09:02Z, Codex.
- Decision: Rename “callers” to “clients” throughout backend APIs and UI-facing types, including RPC and gateway.
  Rationale: Aligns terminology with the user-facing mental model.
  Date/Author: 2026-01-24 09:02Z, Codex.
- Decision: `RegisterClient` returns normalized tags and a visible server count for logging/diagnostics, not spec keys.
  Rationale: Keeps the response small while still useful to clients and logs.
  Date/Author: 2026-01-24 09:02Z, Codex.

## Outcomes & Retrospective

TBD. This section will summarize outcomes once implementation reaches a stable checkpoint.

## Context and Orientation

The current backend loads a profile store directory (`runtime.yaml`, `profiles/*.yaml`, `callers.yaml`) through `internal/infra/catalog/profile_store.go` and constructs per-profile indexes in `internal/app/control_plane_state.go`. Client visibility is resolved by caller-to-profile mapping in `internal/app/control_plane_registry.go`. Tools, resources, and prompts are listed from per-profile indexes in `internal/app/control_plane_discovery.go`. RPC and gateway APIs carry a `caller` string in `proto/mcpd/control/v1/control.proto` and `internal/infra/rpc/control_service.go`.

In the new model, a “client” is a connected MCP consumer (previously “caller”) that registers a name, pid, and `tags` list. A “tag” is a lowercase string used to match a client to servers. A “server catalog” is a single config file containing runtime settings and a list of server specs with optional tags. There are no profiles or caller mappings. Server visibility uses union matching: a server with no tags is visible to all clients, a client with no tags sees all servers, otherwise tags must intersect.

Key files to modify include `internal/infra/catalog/loader.go`, `internal/infra/catalog/schema.json`, `internal/app/catalog_provider_dynamic.go`, `internal/app/control_plane_state.go`, `internal/app/control_plane_registry.go`, `internal/app/control_plane_discovery.go`, `internal/app/control_plane_automation.go`, `internal/domain/controlplane.go`, `internal/domain/catalog_state.go`, `internal/domain/catalog_diff.go`, `internal/domain/types.go`, `internal/infra/rpc/control_service.go`, `internal/infra/gateway/gateway.go`, `internal/ui/*.go`, and `proto/mcpd/control/v1/control.proto`.

## Milestones

Milestone 1: Single-file config schema and tag normalization. Update `internal/domain/types.go`, `internal/domain/subagent.go`, `internal/domain/spec_fingerprint.go`, `internal/infra/catalog/schema.json`, and `internal/infra/catalog/loader.go` to add server tags, normalize them, and add `subAgent.enabledTags`. Update `internal/ui/config_service.go` to report file-based config mode. Run `go test ./internal/infra/catalog` and expect an `ok` line such as `ok mcpd/internal/infra/catalog`.

Milestone 2: Single-file catalog provider and diffing. Update `internal/app/catalog_provider_dynamic.go`, `internal/app/catalog_provider_static.go`, `internal/domain/catalog_state.go`, `internal/domain/catalog_diff.go`, and `internal/app/reload_manager.go` to load from a file path and detect tag-only changes. Update `cmd/mcpd/main.go` to describe `--config` as a file path. Run `go test ./internal/app` and expect `ok mcpd/internal/app`.

Milestone 3: Control plane refactor for clients and tags. Update `internal/domain/controlplane.go`, `internal/app/control_plane_state.go`, `internal/app/control_plane_registry.go`, `internal/app/control_plane_discovery.go`, and `internal/app/control_plane_observability.go` to replace profiles with a global runtime and to filter by client tags. Add a tag-change notification path so tool/resource/prompt watchers re-emit snapshots when only tags change. Run `go test ./internal/app` and expect `ok mcpd/internal/app`.

Milestone 4: RPC and gateway rename to clients, add tags. Update `proto/mcpd/control/v1/control.proto`, regenerate with `make proto`, and update `internal/infra/rpc/control_service.go`, `internal/infra/gateway/gateway.go`, and `internal/infra/gateway/log_bridge.go` plus tests. Run `go test ./internal/infra/rpc` and expect `ok mcpd/internal/infra/rpc`.

Milestone 5: UI services, events, and docs. Replace `internal/ui/profile_service.go` with server/client services, update `internal/ui/types.go`, `internal/ui/events.go`, and docs/config examples under `docs/`. Run `go test ./internal/ui`, then run `go run ./cmd/mcpd validate --config ./docs/catalog.server-centric.yaml` and expect the command to exit cleanly with no validation errors.

## Plan of Work

First, convert the configuration model to a single catalog file. Update `internal/domain/types.go` to add `Tags []string` on `ServerSpec` and adjust `internal/domain/spec_fingerprint.go` so tags do not affect spec fingerprints. Extend `internal/infra/catalog/schema.json` and `internal/infra/catalog/loader.go` to parse, normalize, and validate `servers[].tags` (trim, lowercase, dedupe, stable sort). Introduce a new `subAgent.enabledTags` field in the runtime config to drive SubAgent activation by client tags; remove per-profile SubAgent config from `domain.Catalog` and schema. Keep runtime settings at the top level (no new `runtime` object) to minimize config churn while still supporting a single file.

Second, replace the profile-store catalog providers with a single-file catalog provider. Modify `internal/app/catalog_provider_dynamic.go` and `internal/app/catalog_provider_static.go` to load `domain.Catalog` from a file path, watch the file’s parent directory for atomic writes, and reject runtime changes that require restart. Replace `domain.CatalogState` to store a single catalog plus a computed spec registry and runtime, then update `domain.CatalogDiff` to drop profile/client fields and focus on spec-key changes and runtime changes. Add explicit detection for tag-only changes (for example, a `TagsChanged` boolean or a list of servers whose tags changed). Update `internal/app/reload_manager.go` to use the new diff shape. Update CLI help text in `cmd/mcpd/main.go` so `--config` explicitly expects a file path.

Alongside the single-file switch, update config UI metadata. `internal/ui/config_service.go` should return `ConfigModeResponse.Mode = "file"` (instead of `"directory"`) and `OpenConfigInEditor` should open the file path. Any editor/inspector helpers in `internal/infra/catalog` should be updated to handle a single file instead of a directory.

Third, refactor control plane state, registry, and discovery around clients and tags. Rename “caller” to “client” in `internal/domain/controlplane.go` and all control-plane interfaces. Store client state with normalized tags in `internal/app/control_plane_registry.go`, update register/unregister logic, and compute active spec counts based on client-tag matching. Replace per-profile runtimes with a single global runtime holding `ToolIndex`, `ResourceIndex`, and `PromptIndex` built from all servers. Update `internal/app/control_plane_discovery.go` so `ListTools`, `WatchTools`, `CallTool`, and related resource/prompt methods filter by allowed spec keys derived from the client’s tags. Compute ETags on the filtered snapshots to keep client-specific cache behavior deterministic. Keep `ListToolCatalog` as an unfiltered, admin-style view for UI inventory; remove “AllProfiles” variants that only existed for the profile model. Ensure tag-only changes trigger a visibility refresh for watchers (for example, by bumping a visibility revision and re-sending filtered snapshots).

Observability updates: update `StreamLogs`, `WatchRuntimeStatus`, and `WatchServerInitStatus` to filter by the client’s visible spec set; add an explicit “all servers” variant for admin use (for example, `StreamLogsAllServers` and `WatchRuntimeStatusAllServers`) and retire “AllProfiles” naming.

Fourth, update SubAgent enablement to follow client tags. Replace `ProfileSubAgentConfig` usage in `internal/domain/subagent.go` and `internal/app/control_plane_automation.go` with a `SubAgentConfig.EnabledTags []string` check. If `enabledTags` is empty, SubAgent is enabled for all clients; otherwise it is enabled only when the client’s tag set intersects `enabledTags`. SubAgent should operate on the tool snapshot already filtered by tag visibility.

Fifth, rename RPC/gateway APIs from caller to client, and pass tags during registration. Update `proto/mcpd/control/v1/control.proto` to use `RegisterClient`, `UnregisterClient`, and `client` fields instead of `caller`, adding `repeated string tags` to `RegisterClientRequest`. Add `visible_server_count` and `tags` to `RegisterClientResponse` for logging. Regenerate protobuf bindings and update `internal/infra/rpc/control_service.go`, `internal/infra/gateway/gateway.go`, and related tests to use the new names and tag propagation. Reserve the old method and field numbers in the proto file to make the break explicit.

Finally, update backend UI services to expose server-centric data to the frontend. Replace `internal/ui/profile_service.go` with `ServerService` and `ClientService` that return server lists/details and active clients. Update `internal/ui/types.go` to define `ServerSummary`, `ServerDetail`, and `ActiveClient` (with tags), and update `internal/ui/events.go` to emit `clients:active` (instead of `callers:active`). Replace or update the config editor in `internal/infra/catalog` to support single-file edits for runtime updates, server imports, disable toggles, and deletions. Update backend docs and sample config under `docs/` and repository root to reflect the single-file schema and tag behavior.

## Concrete Steps

From the repository root (`/Users/wibus/dev/mcpd`), implement the config-model and control-plane changes first, then RPC/gateway updates, then UI services. Use `rg -n` to find remaining “caller” and “profile” references and rename them as you go. After each subsystem change, run focused tests to avoid large breakage.

Suggested commands during implementation:

    rg -n "caller|profile|profiles|callers" internal proto
    make proto
    go test ./internal/infra/catalog
    go test ./internal/app
    go test ./internal/infra/rpc
    go test ./internal/ui

If protobuf changes are made, regenerate bindings and ensure `go test ./...` still passes.

## Validation and Acceptance

Run `go test ./...` and expect all tests to pass. Start the server with a single config file:

    go run ./cmd/mcpd serve --config ./docs/catalog.server-centric.yaml

Then register two clients (for example via `grpcurl`) with different tags:

    grpcurl -plaintext -d '{"client":"vscode","pid":1234,"tags":["vscode"]}' 127.0.0.1:7090 mcpd.control.v1.ControlPlaneService.RegisterClient
    grpcurl -plaintext -d '{"client":"chat","pid":1235,"tags":["chat"]}' 127.0.0.1:7090 mcpd.control.v1.ControlPlaneService.RegisterClient

Expect a response containing `tags` and `visible_server_count`, for example:

    {"tags":["vscode"],"visible_server_count":2}

Then verify tool visibility:

    grpcurl -plaintext -d '{"client":"vscode"}' 127.0.0.1:7090 mcpd.control.v1.ControlPlaneService.ListTools
    grpcurl -plaintext -d '{"client":"chat"}' 127.0.0.1:7090 mcpd.control.v1.ControlPlaneService.ListTools

Confirm that the tool lists differ according to server tags, and that servers with empty tags appear in both lists. Confirm SubAgent enablement only when client tags intersect `subAgent.enabledTags` by invoking `IsSubAgentEnabled` and observing `false` for non-matching tags. Confirm `mcpd validate --config <file>` validates the new single-file schema.

## Idempotence and Recovery

All edits are additive and deterministic. If a step fails, revert only the specific file edits in that step and re-run the corresponding tests. Keep a backup copy of the old profile-store configuration directory if it is still needed for comparison, but do not attempt to migrate it; this change is intentionally breaking.

## Artifacts and Notes

Example single-file config (indentation shown for clarity; do not use code fences inside the ExecPlan):

    routeTimeoutSeconds: 30
    rpc:
      listenAddress: "127.0.0.1:7090"
    subAgent:
      enabledTags: ["vscode"]
      provider: "openai"
      model: "gpt-4.1-mini"
    servers:
      - name: "weather"
        cmd: ["/path/to/mcp", "weather"]
        tags: ["vscode"]
      - name: "news"
        cmd: ["/path/to/mcp", "news"]
        tags: ["chat"]
      - name: "shared"
        cmd: ["/path/to/mcp", "shared"]

## Interfaces and Dependencies

Update the following Go types and interfaces to exist after this plan:

    In internal/domain/types.go:
        type ServerSpec struct {
            ...
            Tags []string `json:"tags,omitempty"`
        }

    In internal/domain/controlplane.go:
        type ActiveClient struct {
            Client        string
            PID           int
            Tags          []string
            LastHeartbeat time.Time
        }

        type ActiveClientSnapshot struct {
            Clients     []ActiveClient
            GeneratedAt time.Time
        }

        type ClientRegistration struct {
            Client             string
            Tags               []string
            VisibleServerCount int
        }

        type ControlPlaneRegistry interface {
            RegisterClient(ctx context.Context, client string, pid int, tags []string) (ClientRegistration, error)
            UnregisterClient(ctx context.Context, client string) error
            ListActiveClients(ctx context.Context) ([]ActiveClient, error)
            WatchActiveClients(ctx context.Context) (<-chan ActiveClientSnapshot, error)
        }

        type ControlPlaneStore interface {
            GetCatalog() Catalog
        }

        type ControlPlaneDiscovery interface {
            ListTools(ctx context.Context, client string) (ToolSnapshot, error)
            WatchTools(ctx context.Context, client string) (<-chan ToolSnapshot, error)
            CallTool(ctx context.Context, client, name string, args json.RawMessage, routingKey string) (json.RawMessage, error)
            ...
        }

        type ControlPlaneObservability interface {
            StreamLogs(ctx context.Context, client string, minLevel LogLevel) (<-chan LogEntry, error)
            StreamLogsAllServers(ctx context.Context, minLevel LogLevel) (<-chan LogEntry, error)
            WatchRuntimeStatus(ctx context.Context, client string) (<-chan RuntimeStatusSnapshot, error)
            WatchRuntimeStatusAllServers(ctx context.Context) (<-chan RuntimeStatusSnapshot, error)
            WatchServerInitStatus(ctx context.Context, client string) (<-chan ServerInitStatusSnapshot, error)
            WatchServerInitStatusAllServers(ctx context.Context) (<-chan ServerInitStatusSnapshot, error)
        }

    In proto/mcpd/control/v1/control.proto:
        message RegisterClientRequest { string client = 1; int64 pid = 2; repeated string tags = 3; }
        message RegisterClientResponse { repeated string tags = 1; int32 visible_server_count = 2; }

No new external libraries are required; reuse existing packages such as `fsnotify`, `zap`, and `hashutil`.

## Breaking Change Strategy

This change intentionally breaks existing clients. The proto should reserve removed method and field names/numbers (for example, `RegisterCaller`, `caller`, and any `AllProfiles` method names) so reuse is explicit and safe. Update README and docs to call out the breaking change, and bump the user-facing version string if you ship binaries. If a compatibility window is required later, add shim RPC handlers that translate old method names to the new client/tag flow.

Change Note: Refined naming, API shapes, tag normalization rules, and breaking change guidance to align with the confirmed client/tag model and the single-file config requirement.
