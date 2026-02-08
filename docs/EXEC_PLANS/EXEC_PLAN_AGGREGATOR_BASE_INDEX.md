# Aggregator Base Index De-duplication

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document follows `.agent/PLANS.md` from the repository root and must be maintained according to it.

## Purpose / Big Picture

Eliminate large-scale duplication across tool/prompt/resource indexes by extracting a shared base index with hooks. After this change, all lifecycle/refresh/subscribe/bootstrap/list-change logic is centralized. Each concrete index only implements the type-specific cache/build/resolve/call logic. This reduces maintenance cost and ensures consistency. Behavior remains unchanged and all aggregator tests still pass.

## Progress

- [x] (2026-02-08 10:42) Introduce BaseIndex (generic over Snapshot/Target/Cache) with hook interface for list-change/bootstrap/refresh/lifecycle logic.
- [x] (2026-02-08 10:42) Refactor ToolIndex to embed BaseIndex and keep tool-specific fetch/build/resolve/call logic only.
- [x] (2026-02-08 10:42) Refactor PromptIndex and ResourceIndex to embed BaseIndex.
- [x] (2026-02-08 10:43) Update tests if needed; run `go test ./internal/infra/aggregator/...`.

## Surprises & Discoveries

- Observation: None yet.
  Evidence: N/A.

## Decision Log

- Decision: Use a compositional BaseIndex with hooks rather than inheritance or copy/paste.
  Rationale: Keeps behavior centralized and makes adding new index types low effort.
  Date/Author: 2026-02-07 / Codex

## Outcomes & Retrospective

- Outcome: Tool/Prompt/Resource indexes now delegate lifecycle/list-change/bootstrap logic to BaseIndex with type-specific hooks.
- Outcome: Duplicate fields and methods removed; indexes focus on cache/build/resolve/call logic.
- Outcome: BaseIndex is embedded to promote common methods, removing wrapper boilerplate.
- Outcome: `go test ./internal/infra/aggregator/...` passes.

## Context and Orientation

Current index files in `internal/infra/aggregator/index/` share ~90% of the same structure and lifecycle logic. The duplicated areas include: base context management, list-change handling, bootstrap refresh, refresh gating, snapshot subscribe and update, and specs/runtime updates. The differences are mostly in: cache fetch/build logic, routing targets, and per-type API calls (call tool / read resource / get prompt).

## Plan of Work

### Phase 1: BaseIndex

Create `internal/infra/aggregator/index/base_index.go` with:

- A generic `BaseIndex[Snapshot, Target, Cache]` struct storing:
  - router/specs/specKeys/cfg/metadataCache/logger/health/gate/listChanges
  - base context / bootstrap waiters / server snapshots
  - `index *core.GenericIndex[...]`
- A `BaseHooks[Snapshot, Target, Cache]` interface (or struct of funcs) containing:
  - `BuildSnapshot(cache map[string]Cache) (Snapshot, map[string]Target)`
  - `FetchServerCache(ctx, serverType, spec) (Cache, error)`
  - `CacheETag(cache Cache) string`
  - `SnapshotETag(snapshot Snapshot) string`
  - `CopySnapshot(snapshot Snapshot) Snapshot`
  - `EmptySnapshot() Snapshot`
  - `ShouldStart(cfg domain.RuntimeConfig) bool`
  - `OnRefreshError(serverType, err) core.RefreshErrorDecision`
  - `ListChangeKind() domain.ListChangeKind`
  - `ShouldListChange(cfg domain.RuntimeConfig) bool` (tool uses ExposeTools, others always true)

Implement shared methods in BaseIndex:

- `Start/Stop/Refresh/Snapshot/Subscribe/Resolve`
- `UpdateSpecs/ApplyRuntimeConfig`
- `SetBootstrapWaiter/startBootstrapRefresh`
- `startListChangeListener`
- `setBaseContext/baseContext/clearBaseContext`
- `SnapshotForServer` should be provided by a hook for per-type snapshot storage (or keep in Base if identical across types)

### Phase 2: Refactor ToolIndex

Modify `tool_index.go`:

- Keep only tool-specific cache/build/resolve/call logic.
- Embed `BaseIndex[domain.ToolSnapshot, domain.ToolTarget, toolCache]`.
- Replace lifecycle methods with thin wrappers or delegate to BaseIndex.
- Ensure `CachedSnapshot` remains tool-specific (metadata cache only applies to tools).
- Keep public API unchanged.

### Phase 3: Refactor PromptIndex/ResourceIndex

Apply same refactor with their cache types and build logic.

### Phase 4: Validation

- Run `go test ./internal/infra/aggregator/...`.
- Ensure no API changes and behavior stable.

## Concrete Steps

- Create `internal/infra/aggregator/index/base_index.go` with generic BaseIndex + hooks.
- Refactor tool/prompt/resource index files accordingly.
- Run:

  gofmt -w internal/infra/aggregator/index/*.go
  go test ./internal/infra/aggregator/...

## Validation and Acceptance

- All aggregator tests pass.
- File size and duplication significantly reduced.
- No API breakage for callers of `aggregator.ToolIndex/ResourceIndex/PromptIndex`.

## Idempotence and Recovery

- Changes can be reverted by restoring old index files from git.
- BaseIndex introduction is additive and can be disabled if needed.

## Artifacts and Notes

- New file: `internal/infra/aggregator/index/base_index.go`.
- Modified files: `tool_index.go`, `prompt_index.go`, `resource_index.go`.

## Interfaces and Dependencies

- BaseIndex depends on `internal/infra/aggregator/core` for GenericIndex/RefreshGate/BootstrapWaiter.
- Concrete indexes keep their current public API signatures.

---

Change Log: Initial creation of ExecPlan for aggregator base index de-duplication.
