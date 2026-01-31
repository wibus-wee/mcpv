# Transactional Hot Reload With Lenient Default

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This plan must be maintained in accordance with `.agent/PLANS.md` from the repository root.

## Purpose / Big Picture

After this change, hot reload becomes transactional by default: either all runtime, scheduler, registry, and index state moves to the new catalog snapshot, or everything rolls back to the previous snapshot. The default reload mode remains lenient (no forced shutdown), but strict mode can still choose to exit after a failed transaction. This removes partial-update ambiguity while keeping operational safety.

## Progress

- [x] 2026-01-30 12:00 Drafted the transactional reload ExecPlan and aligned with lenient default.
- [x] 2026-01-30 12:20 Implement field-aware diff classification for spec and runtime config, including restart-required runtime fields.
- [x] 2026-01-30 12:50 Remove runtime-change rejection in DynamicCatalogProvider and route restart-required handling to reload transaction.
- [x] 2026-01-30 12:50 Introduce reload transaction planner + executor with rollback actions.
- [x] 2026-01-30 13:40 Implement dynamic runtime reconfiguration for router, indexes, registry monitor, and ping manager (SubAgent changes are restart-required).
- [x] 2026-01-30 14:40 Add tests for classification, transaction rollback, and lenient/strict behavior.
- [x] 2026-01-30 15:10 Update docs and example config to reflect transactional reload policy.

## Surprises & Discoveries

- Observation: Runtime config changes are rejected before diff emission in the dynamic catalog provider.
  Evidence: `internal/app/catalog/catalog_provider_dynamic.go` returns `ErrReloadRestartRequired` when runtime differs.
- Observation: Tool refresh interval changes do not take effect after reload because ticker is only created on Start.
  Evidence: `internal/infra/aggregator/index_core.go` uses interval only at Start.
- Observation: Client monitor interval is fixed at startup and is not reconfigured on runtime updates.
  Evidence: `internal/app/controlplane/registry.go` creates a ticker once.
- Observation: `go test ./...` fails without frontend assets because `embed.go` expects `frontend/dist`.
  Evidence: `embed.go:5:12: pattern frontend/dist: no matching files found`.

## Decision Log

- Decision: Keep `DefaultReloadMode` as lenient and make transactions the default behavior.
  Rationale: Matches the operational preference for availability while still guaranteeing consistency via rollback.
  Date/Author: 2026-01-30 / Codex
- Decision: Implement transactional reload via explicit plan + rollback actions instead of full shadow state cloning.
  Rationale: Scheduler and instance lifecycle cannot be cheaply cloned; compensating actions are practical and testable.
  Date/Author: 2026-01-30 / Codex
- Decision: Classify spec changes into tools-only, runtime-behavior-only, and restart-required; classify runtime config fields into dynamic vs restart-required.
  Rationale: Enables precise action planning and avoids unnecessary restarts.
  Date/Author: 2026-01-30 / Codex
- Decision: Treat SubAgent runtime changes as restart-required until a safe swap mechanism is implemented.
  Rationale: Avoids races and partial state while transactional reload is being stabilized.
  Date/Author: 2026-01-30 / Codex

## Outcomes & Retrospective

Transactional hot reload is now implemented with rollback, runtime diff classification, dynamic runtime reconfiguration for router/index/registry/ping, and updated metrics/logs. Tests cover classification, rollback, and strict/lenient behavior. SubAgent changes are treated as restart-required pending a safe swap mechanism. Manual acceptance runs remain to validate end-to-end behavior with real configs.

## Context and Orientation

Hot reload is driven by `internal/app/controlplane/reload.go`. Catalog updates are produced by `internal/app/catalog/catalog_provider_dynamic.go`. Spec diffing is in `internal/domain/catalog_diff.go`, and scheduler diff application is in `internal/infra/scheduler/catalog.go`. Runtime indexes (tools, resources, prompts) live in `internal/app/runtime/state.go` and `internal/infra/aggregator/*`. Router timeout is set at construction in `internal/infra/router/router.go`. Client monitor intervals are set in `internal/app/controlplane/registry.go`.

A “transactional reload” means the system must finish in one of two states: fully old snapshot or fully new snapshot. Mixed states across scheduler, registry, or indexes are not allowed.

## Plan of Work

Milestone 1: Diff classification and runtime field policy. Replace the current binary spec classification with a field-aware classifier. Introduce a runtime diff that marks fields as dynamic or restart-required. Update tests in `internal/domain` to cover all classifications.

Milestone 2: Catalog provider emits all diffs. Remove runtime-change rejection in `DynamicCatalogProvider`. Runtime restart requirements are enforced in the reload transaction, not in the provider.

Milestone 3: Transaction planner + executor. Create a reload plan that enumerates actions and their compensation steps. The executor applies actions in order, collects rollback actions as they succeed, and if any action fails, executes rollback in reverse order. The transaction is committed only after all actions succeed; only then is the control-plane state advanced and the applied revision updated.

Milestone 4: Runtime reconfiguration. Add runtime-apply hooks that update router timeout, tool refresh interval/concurrency, tool namespace strategy, client monitor intervals, and ping manager interval. SubAgent changes are treated as restart-required until a safe swap is implemented.

Milestone 5: Observability and docs. Add clear logs for reload plan, action list, and rollback outcomes. Update metrics for reload apply success/failure and rollback success/failure. Document transactional semantics and runtime field policies in a new doc, and update example config.

Milestone 6: Tests and acceptance. Add unit tests for classification and transaction rollback. Add integration-style tests for reload apply success and failure in both lenient and strict modes. Validate with a manual scenario showing no partial state after an induced failure.

## Concrete Steps

Run these from the repo root (`/Users/wibus/Desktop/hot-reload`):

    rg -n "CatalogDiff|ClassifySpecDiff|ReloadMode" internal/domain
    rg -n "ReloadManager|applyUpdate|handleApplyError" internal/app/controlplane
    rg -n "UpdateSpecs|Start\\(ctx\\)" internal/infra/aggregator
    rg -n "StartMonitor" internal/app/controlplane/registry.go
    rg -n "NewBasicRouter" internal/infra/router/router.go

After implementation:

    go test ./internal/domain/...
    go test ./internal/app/controlplane/...
    go test ./internal/infra/aggregator/...
    go test ./...

Expected results: all tests pass. The new rollback tests fail before the change and pass after.

## Validation and Acceptance

Manual acceptance scenario:

1. Start core with a config that has one server and tool exposure enabled.
2. Change only tool exposure or tags in the server spec.
3. Observe logs: transaction applies tools-only update; no restart; tools list updates.
4. Change idleSeconds/maxConcurrent/minReady for the server spec.
5. Observe logs: transaction applies scheduler-only update; no restart; minReady and idle behavior adjust.
6. Change cmd/transport/env/protocolVersion.
7. Observe logs: transaction performs restart-required actions; old instances drain and new instances start.
8. Inject a forced failure during reload (for example, simulate scheduler ApplyCatalogDiff error in tests).
9. Observe: rollback runs, old snapshot remains active, and no partial state is visible.
10. Repeat step 8 with strict mode: rollback runs, then the process exits intentionally.

Acceptance is met when steps 3–10 behave exactly as described and tests pass.

## Idempotence and Recovery

All steps are repeatable. If a transaction fails, rollback restores the previous snapshot. Under lenient mode, the core continues running with the previous snapshot. Under strict mode, rollback runs and then the core exits to preserve strong failure semantics.

## Artifacts and Notes

Expected log samples:

    reload plan created: added=1 removed=0 tools_only=1 runtime_updates=2 restart_required=0
    reload transaction committed: revision=... latency=...
    reload transaction failed: stage=... rollback=success

Expected metrics:

    mcpv_reload_apply_total{mode="lenient",result="failure"} 1
    mcpv_reload_rollback_total{result="success"} 1

## Interfaces and Dependencies

In `internal/domain/catalog_diff.go`, define:

    type SpecDiffClassification string
    const (
        SpecDiffNone SpecDiffClassification = "none"
        SpecDiffToolsOnly SpecDiffClassification = "tools_only"
        SpecDiffRuntimeBehavior SpecDiffClassification = "runtime_behavior"
        SpecDiffRestartRequired SpecDiffClassification = "restart_required"
    )

    type RuntimeDiff struct {
        DynamicFields []string
        RestartRequiredFields []string
    }

    func DiffRuntimeConfig(prev, next RuntimeConfig) RuntimeDiff
    func (d RuntimeDiff) IsEmpty() bool
    func (d RuntimeDiff) RequiresRestart() bool

Extend `CatalogDiff` with:

    RuntimeDiff RuntimeDiff
    RuntimeBehaviorSpecKeys []string

In `internal/infra/router/router.go`, add:

    func (r *BasicRouter) SetTimeout(timeout time.Duration)

In `internal/app/runtime/state.go`, add:

    func (r *State) ApplyRuntimeConfig(ctx context.Context, prev, next domain.RuntimeConfig) error

In `internal/infra/aggregator`, add:

    func (a *ToolIndex) ApplyRuntimeConfig(cfg domain.RuntimeConfig)
    func (a *ResourceIndex) ApplyRuntimeConfig(cfg domain.RuntimeConfig)
    func (a *PromptIndex) ApplyRuntimeConfig(cfg domain.RuntimeConfig)

In `internal/app/controlplane/registry.go`, add:

    func (r *ClientRegistry) UpdateRuntimeConfig(ctx context.Context, prev, next domain.RuntimeConfig)

In `internal/app/controlplane/reload.go`, add:

    type ReloadPlan struct { ... }
    type ReloadAction struct { ... }
    type RollbackAction struct { ... }
    func buildReloadPlan(prev, next domain.CatalogState, diff domain.CatalogDiff) ReloadPlan
    func (m *ReloadManager) applyTransaction(ctx context.Context, plan ReloadPlan) error

Transaction semantics:

- Only after all actions succeed, advance the control-plane catalog state and applied revision.
- If any action fails, execute rollback actions in reverse order and restore old catalog state.

Restart-required runtime fields:

- RPC listen address
- Observability listen address
- Any field required by the runtime to open new listeners
- SubAgent configuration

Dynamic runtime fields (safe to apply):

- RouteTimeoutSeconds
- ToolRefreshSeconds
- ToolRefreshConcurrency
- ClientCheckSeconds
- ClientInactiveSeconds
- ToolNamespaceStrategy
- ExposeTools

Spec runtime-behavior fields (safe without restart):

- IdleSeconds
- MaxConcurrent
- MinReady
- DrainTimeoutSeconds
- ActivationMode
- SessionTTLSeconds

Spec restart-required fields:

- Transport
- Cmd
- Env
- Cwd
- ProtocolVersion
- HTTP config
- Strategy changes (treated as restart-required for safety)

## Notes About Rollback Actions

Rollback for additions should stop newly started specs and reset minReady. Rollback for removals should restore previous specs and reapply previous minReady if necessary. Rollback for restart-required changes should restore the old spec key, then start old minReady and drain any newly started instances from the failed attempt.

The rollback plan must be built at the same time as the forward plan so it is deterministic and does not depend on runtime inspection.

Plan update 2026-01-30 12:20: Completed Milestone 1 by adding spec/runtime diff classification and runtime diff tests, so progress reflects this completion.
Plan update 2026-01-30 12:30: Removed runtime-change rejection in DynamicCatalogProvider and marked Milestone 2 as partially complete.
Plan update 2026-01-30 12:50: Completed Milestone 2 and Milestone 3 by enforcing runtime restart requirements in reload and implementing a transactional reload executor with rollback steps.
Plan update 2026-01-30 13:10: Marked SubAgent runtime changes as restart-required and updated the Decision Log and runtime field lists.
Plan update 2026-01-30 13:40: Applied dynamic runtime reconfiguration for router, indexes, registry monitor, and ping manager, and updated progress accordingly.
Plan update 2026-01-30 14:10: Added rollback-focused reload manager tests and marked Milestone 5 as partially complete.
Plan update 2026-01-30 14:40: Added strict/lenient behavior tests and marked Milestone 5 complete.
Plan update 2026-01-30 15:10: Added hot reload documentation and updated the example catalog config with reload mode.
Plan update 2026-01-30 15:20: Updated Milestone 4 description to note SubAgent changes are restart-required for now.
Plan update 2026-01-30 15:30: Added Outcomes & Retrospective summary now that all milestones are complete.
Plan update 2026-01-30 15:40: Added full test run discovery about missing frontend assets to Surprises & Discoveries.
