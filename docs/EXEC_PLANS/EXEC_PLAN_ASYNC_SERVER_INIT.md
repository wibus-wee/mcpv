# Async MCP Server Initialization and Status Surfaces

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds. Maintain this document in accordance with /.agent/PLANS.md.

## Purpose / Big Picture

Right now the mcpv Core blocks during startup while every downstream MCP server (defined in the catalog) launches and finishes its `initialize` handshake. A single slow `npx`-based server is enough to keep the whole Core in an "error" state, even though the Core infrastructure (control plane, RPC server, UI bridge) is already operational. After this change the Core will become responsive immediately, while each server is initialized asynchronously in the background. Every server will export its own initialization status (pending, starting, ready, degraded, failed) so operators can see which servers are usable, which are still warming up, and which failed. Core health is no longer tied to any specific MCP server.

## Progress

- [x] (2025-12-28 12:45Z) Authored initial ExecPlan draft.
- [ ] Implement async initialization manager and status tracking.
- [ ] Wire manager into ControlPlane/App lifecycle and expose status API.
- [ ] Update frontend to surface per-server status along with runtime indicators.
- [ ] Add tests and manual validation for the new behavior.

## Surprises & Discoveries

- Observation: Pending — populate once implementation uncovers notable behavior.
  Evidence: Pending.

## Decision Log

- Decision: Track initialization status per `specKey` with derived `serverName`, counting ready vs required instances and capturing last error strings to keep UX actionable.
  Rationale: `specKey` uniquely identifies a server spec across profiles and lets the scheduler map status back to pools; human-readable names are still needed for UI display, so both values are stored.
  Date/Author: 2025-12-28 / GitHub Copilot.

## Outcomes & Retrospective

Pending completion. Summarize impact, test coverage, and follow-ups once the plan is executed.

## Context and Orientation

The Core entry point lives in `internal/app/app.go`. It loads catalog profiles, builds a `specRegistry` map of fingerprinted server specs, instantiates the lifecycle manager (`internal/infra/lifecycle/manager.go`), and creates the scheduler (`internal/infra/scheduler/basic.go`). During scheduler construction we currently start pools synchronously, so the Core callback (`ServeConfig.OnReady`) fires only after every MCP server finishes its `initialize` RPC.

The Wails UI bridge in `internal/ui/service.go` exposes `GetCoreState()` and `GetRuntimeStatus()` to the frontend (`frontend/src/modules/config`). `GetRuntimeStatus()` already returns live instance stats from the scheduler, but it assumes pools exist, so when startup blocks it never reports anything useful.

We will introduce a `ServerInitializationManager` (new file under `internal/app/`) that asynchronously requests the scheduler to provision each server's minimum ready instances. This manager will track a state struct per `specKey` with fields for required `minReady`, `readyCount`, `failedCount`, `state`, and timestamps. The ControlPlane will expose these statuses through a new method (e.g., `GetServerInitStatus`) so the UI can render per-server progress bars independent of Core health.

Key files touched:
- `internal/app/app.go`: decouple Core startup from server initialization, construct and launch the new manager, expose it via ControlPlane.
- `internal/app/server_init_manager.go` (new): asynchronous orchestration and status bookkeeping.
- `internal/app/control_plane.go`: store a pointer to the manager and surface its status through a new domain-facing method.
- `internal/domain/controlplane.go` & `internal/domain/types.go`: define status structs/interfaces.
- `internal/ui/service.go`: add `GetServerInitStatus` and stop conflating server failures with `GetCoreState` errors.
- Frontend under `frontend/src/modules/config`: show per-server status in the runtime panel, using the new API alongside existing runtime indicators.

## Plan of Work

1. **Domain modeling.** Extend `internal/domain/types.go` with a `ServerInitState` enum (pending, starting, ready, degraded, failed) and a `ServerInitStatus` struct capturing `specKey`, `serverName`, `minReady`, `ready`, `failed`, optional `lastError`, and last update time. Update `internal/domain/controlplane.go` to add a `GetServerInitStatus(context.Context) ([]ServerInitStatus, error)` method so all layers know about the new surface.

2. **ServerInitializationManager implementation.** Create `internal/app/server_init_manager.go`. The manager stores the scheduler, spec registry, initial minReady targets, and a `map[string]*ServerInitStatus`. Provide methods `Start(ctx)`, `Statuses() []domain.ServerInitStatus`, and helpers to record successes/failures. `Start` should iterate `specRegistry`, read each spec's `MinReady`, and spin off goroutines that call `scheduler.SetDesiredMinReady` (if necessary) and `scheduler.Acquire` in a loop until counts are satisfied or context is canceled. Errors should not abort other servers; instead update status to `ServerInitDegraded` or `ServerInitFailed` depending on whether any instance succeeded.

3. **App wiring.** In `internal/app/app.go`, after building `summary.specRegistry` and before calling `cfg.OnReady`, instantiate the manager with the scheduler and registry. Launch it via `go manager.Start(appCtx)` so it runs in the background. The Core should call `cfg.OnReady` immediately after RPC/server wiring succeeds, regardless of downstream server state. Store the manager pointer inside `profileSummary` or directly on the ControlPlane so it can be queried later.

4. **ControlPlane integration.** Update `internal/app/control_plane.go` to accept a `ServerInitializationManager` (or interface) in its constructor and store it on the struct. Implement the domain-facing `GetServerInitStatus` method by delegating to the manager. For safety, return an empty slice if the manager is nil (e.g., tests).

5. **UI service API.** In `internal/ui/service.go`, add `GetServerInitStatus()` that calls the new control-plane method, converts the domain structs to UI structs (either reuse existing runtime types or add a dedicated UI struct), and expose it through the Wails bindings (`frontend/bindings/...`). Ensure `GetCoreState()` now only reflects true Core errors (e.g., control plane unavailable) so the initialization of downstream servers no longer reports as a Core failure.

6. **Frontend UX.** Create a hook (e.g., `useServerInitStatus` in `frontend/src/modules/config/hooks.ts`) that polls `GetServerInitStatus`. Update `server-runtime-status.tsx` or a sibling component to render each server's init state: show labels like "Starting", "Ready (1/1)", "Failed (last error ...)". If a server is failed, surface that even if runtime status is empty. Ensure the detail panel combines init status with live runtime stats for clarity.

7. **Tests.** Add unit tests for the manager (new `_test.go` file) to cover success, partial failure, and cancellation. Update existing control plane tests to assert that `GetServerInitStatus` handles nil managers gracefully. Extend frontend component tests if applicable (or rely on manual validation described later).

## Concrete Steps

1. From the repo root run `make test` to confirm the current baseline is green.
2. Implement steps 1-5 above (backend changes). After Go code edits, run `make test` again; expect all packages to pass.
3. Rebuild Wails bindings (`cd frontend && pnpm run bindings` if required by current workflow) and run `pnpm run lint` to keep frontend tidy.
4. Launch the dev environment with `make dev` (ensures Core + frontend dev server). Observe logs to verify Core reports ready immediately while server init messages continue asynchronously.
5. With the app running, open the UI (default http://localhost:5173 or the configured port) and watch the new per-server status indicators progress from Pending → Ready/Failed.

## Validation and Acceptance

- Run `make test` at repo root; expect success with new backend tests included.
- Start the system via `make dev`. Within a few seconds the log should report `core ready` even if downstream servers are still starting. No `GetCoreState` errors should appear because an MCP server is slow.
- In the UI config page, each server should show an initialization badge such as `Starting`, `Ready (1/1)`, or `Failed`. Slow `npx` servers should visibly transition states without blocking other servers or core functionality.
- Force a failure (e.g., temporarily break a server command) and verify only that server shows `Failed`, while others remain usable and Core stays `Ready`.

## Idempotence and Recovery

The manager must tolerate restarts: it reads the current registry each time and only spawns goroutines for configured servers, so repeated launches are safe. If the Core process restarts, servers will be re-evaluated from scratch. Within the manager, guard against duplicate goroutines by creating them once during `Start`. If the plan's steps fail halfway, re-run `make test` and `make dev`; there are no destructive migrations.

## Artifacts and Notes

Add representative log snippets in this section once the implementation produces them, for example:

    2025-12-28T13:10:02.345Z INFO server-init manager initialized servers=3
    2025-12-28T13:10:05.678Z INFO server-init ready server=weather ready=1/1
    2025-12-28T13:10:12.001Z WARN server-init failed server=context7 attempt=1 error="initialize: recv initialize: context deadline exceeded"

These samples help future maintainers verify expectations.

## Interfaces and Dependencies

- `domain.Scheduler` is already available and exposes `Acquire`, `Release`, and pool inspection. The manager will call `Acquire` with an empty routing key to trigger instance creation; released instances immediately go back to the pool but still count toward readiness by virtue of the pool maintaining them.
- ControlPlane consumers (RPC service, UI service) will depend on the new `GetServerInitStatus` method for per-server telemetry.
- Frontend additions rely on the existing SWR data-fetching pattern in `frontend/src/modules/config/hooks.ts` and component composition in `frontend/src/modules/config/components/`.

Document updates to this plan (including implementation milestones and test evidence) at the bottom of this file as work proceeds.
