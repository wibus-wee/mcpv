# Subpackage Refactor for Bootstrap, Plugin, and Reload

This ExecPlan is a living document. The sections Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective must be kept up to date as work proceeds.

This plan follows the requirements in /Users/wibus/dev/mcpd/.agent/PLANS.md and must be maintained in accordance with that file.

## Purpose / Big Picture

Reduce the oversized control plane files by moving distinct responsibilities into subpackages with clear boundaries. The change should preserve runtime behavior while making the codebase easier to reason about, test, and extend. Success is visible when the refactored packages compile, the existing tests pass, and the large files are split into smaller focused units without changing observable behavior.

## Progress

- [x] (2026-02-08 14:31Z) Drafted the ExecPlan skeleton and selected the subpackage-first refactor strategy.
- [x] (2026-02-08 15:18Z) Created subpackage scaffolding for bootstrap, plugin, and reload responsibilities.
- [x] (2026-02-08 15:18Z) Moved bootstrap metadata and server-init logic into new subpackages and updated call sites.
- [x] (2026-02-08 15:18Z) Moved plugin lifecycle, socket, process, and handshake logic into subpackages and updated call sites/tests.
- [x] (2026-02-08 15:18Z) Split reload manager helpers and moved transaction/observer types into a reload subpackage.
- [x] (2026-02-08 15:18Z) Updated wiring and ran package-level tests (bootstrap, controlplane, plugin).

## Surprises & Discoveries

- Moving reload observer into a subpackage made the test-only core logger override inaccessible, so an explicit Observer.SetCoreLogger helper was added to keep tests readable.

## Decision Log

- Decision: Use subpackage boundaries for bootstrap, plugin, and reload, and allow API breaks where it simplifies boundaries and reduces coupling.
  Rationale: The user explicitly chose a subpackage-level refactor and allows destructive changes. Stronger boundaries reduce long-term maintenance cost even if import paths change.
  Date/Author: 2026-02-08 / Codex
- Decision: Add Observer.SetCoreLogger to preserve reload tests after moving observer into a subpackage.
  Rationale: The previous tests replaced the core logger directly on the observer; a small setter keeps the test intent without exposing internal fields.
  Date/Author: 2026-02-08 / Codex

## Outcomes & Retrospective

- Completed subpackage refactor for bootstrap, plugin, and reload; behavior preserved and package-level tests pass. Remaining work is optional cleanup or higher-level end-to-end verification.

## Context and Orientation

The refactor targets four oversized files, now split into subpackages:

- /Users/wibus/dev/mcpd/internal/app/bootstrap/metadata/manager.go (bootstrap metadata fetch, concurrency, progress tracking).
- /Users/wibus/dev/mcpd/internal/app/bootstrap/serverinit/manager.go (initialization orchestration, retry logic, status tracking).
- /Users/wibus/dev/mcpd/internal/infra/plugin/manager/manager.go (plugin lifecycle, gRPC connection, socket management, metadata handshake).
- /Users/wibus/dev/mcpd/internal/app/controlplane/reload.go plus /Users/wibus/dev/mcpd/internal/app/controlplane/reload/* (catalog reload, transaction, observer, rollback).

There are existing tests in:

- /Users/wibus/dev/mcpd/internal/app/bootstrap/metadata/manager_test.go
- /Users/wibus/dev/mcpd/internal/app/bootstrap/serverinit/manager_test.go
- /Users/wibus/dev/mcpd/internal/app/controlplane/reload_test.go
- /Users/wibus/dev/mcpd/internal/app/controlplane/reload/transaction_test.go
- /Users/wibus/dev/mcpd/internal/infra/plugin/manager/manager_harness_test.go
- /Users/wibus/dev/mcpd/internal/infra/plugin/manager/e2e_test.go

There is also wiring that references these types:

- /Users/wibus/dev/mcpd/internal/app/providers.go
- /Users/wibus/dev/mcpd/internal/app/wire_sets.go
- /Users/wibus/dev/mcpd/internal/app/wire_gen.go

Definitions used below:

- Subpackage: a Go package located in a subdirectory of a parent package directory, used to enforce smaller, dependency-bounded units.
- Orchestrator: a component that coordinates multiple subcomponents and owns the execution flow but does not embed low-level details.

## Plan of Work

First, create subpackages for bootstrap metadata, activation policy, and server initialization, then move the existing logic into those packages. Keep /Users/wibus/dev/mcpd/internal/app/bootstrap/server_startup_orchestrator.go in the parent package, but update the orchestrator to depend on the new subpackages.

Second, refactor the plugin manager by moving socket preparation, process lifecycle, gRPC handshake, and instance cleanup into subpackages under /Users/wibus/dev/mcpd/internal/infra/plugin/. The manager itself moves into /Users/wibus/dev/mcpd/internal/infra/plugin/manager and stays as the orchestration layer.

Third, refactor reload by moving transaction and observer logic into a reload subpackage, and splitting the reload manager into smaller files so no single file exceeds 500 lines. The public ReloadManager API stays in /Users/wibus/dev/mcpd/internal/app/controlplane.

Finally, update all call sites, tests, and wiring. Run targeted package tests to prove behavior is unchanged.

## Concrete Steps

1) Create new directories for subpackages and move code with minimal behavior changes.

   Commands (run from /Users/wibus/dev/mcpd):

     rg -n "bootstrap.NewManager|ServerInitializationManager|plugin.NewManager|ReloadManager" internal
     rg -n "bootstrap/" internal/app

2) Bootstrap subpackages.

   Commands:

     mkdir -p /Users/wibus/dev/mcpd/internal/app/bootstrap/metadata
     mkdir -p /Users/wibus/dev/mcpd/internal/app/bootstrap/serverinit

3) Plugin subpackages.

   Commands:

     mkdir -p /Users/wibus/dev/mcpd/internal/infra/plugin/process
     mkdir -p /Users/wibus/dev/mcpd/internal/infra/plugin/socket
     mkdir -p /Users/wibus/dev/mcpd/internal/infra/plugin/handshake
     mkdir -p /Users/wibus/dev/mcpd/internal/infra/plugin/instance

4) Reload subpackage and file splits.

   Commands:

     mkdir -p /Users/wibus/dev/mcpd/internal/app/controlplane/reload

5) Update imports and wiring, then run tests.

   Commands:

     go test ./internal/app/bootstrap/...
     go test ./internal/app/controlplane/...
     go test ./internal/infra/plugin/...

   If Wire generation is needed and toolchain is available:

     make wire

## Validation and Acceptance

The refactor is accepted when:

- The code compiles with the new subpackage structure.
- Existing tests in bootstrap, controlplane reload, and plugin packages pass without behavior changes.
- No single file in the target areas exceeds ~500 lines after the split.

## Idempotence and Recovery

All steps are safe to re-run; moving files is reversible by moving them back. If Wire regeneration is unavailable, update /Users/wibus/dev/mcpd/internal/app/wire_gen.go manually and record the reason in the Surprises & Discoveries section.

## Artifacts and Notes

Expected high-level layout (example):

    /Users/wibus/dev/mcpd/internal/app/bootstrap/metadata/manager.go
    /Users/wibus/dev/mcpd/internal/app/bootstrap/metadata/targets.go
    /Users/wibus/dev/mcpd/internal/app/bootstrap/serverinit/manager.go
    /Users/wibus/dev/mcpd/internal/app/bootstrap/serverinit/retry_policy.go
    /Users/wibus/dev/mcpd/internal/infra/plugin/manager/manager.go
    /Users/wibus/dev/mcpd/internal/infra/plugin/process/process.go
    /Users/wibus/dev/mcpd/internal/infra/plugin/socket/allocator.go
    /Users/wibus/dev/mcpd/internal/infra/plugin/handshake/client.go
    /Users/wibus/dev/mcpd/internal/infra/plugin/instance/instance.go
    /Users/wibus/dev/mcpd/internal/app/controlplane/reload/observer.go
    /Users/wibus/dev/mcpd/internal/app/controlplane/reload/transaction.go

## Interfaces and Dependencies

Bootstrap metadata package in /Users/wibus/dev/mcpd/internal/app/bootstrap/metadata:

    type Manager struct { /* orchestrates metadata bootstrap */ }

    type ManagerOptions struct {
        Scheduler   domain.Scheduler
        Lifecycle   domain.Lifecycle
        Specs       map[string]domain.ServerSpec
        SpecKeys    map[string]string
        Runtime     domain.RuntimeConfig
        Cache       *domain.MetadataCache
        Logger      *zap.Logger
        Concurrency int
        Timeout     time.Duration
        Mode        domain.BootstrapMode
    }

    func NewManager(opts ManagerOptions) *Manager
    func (m *Manager) Bootstrap(ctx context.Context)
    func (m *Manager) WaitForCompletion(ctx context.Context) error
    func (m *Manager) GetProgress() domain.BootstrapProgress
    func (m *Manager) IsCompleted() bool
    func (m *Manager) GetCache() *domain.MetadataCache

Bootstrap server init package in /Users/wibus/dev/mcpd/internal/app/bootstrap/serverinit:

    type Manager struct { /* coordinates init and retry */ }

    func NewManager(scheduler domain.Scheduler, state *domain.CatalogState, logger *zap.Logger) *Manager
    func (m *Manager) ApplyCatalogState(state *domain.CatalogState)
    func (m *Manager) Start(ctx context.Context)
    func (m *Manager) Stop()
    func (m *Manager) SetMinReady(specKey string, minReady int, cause domain.StartCause) error
    func (m *Manager) RetrySpec(specKey string) error
    func (m *Manager) Statuses(ctx context.Context) []domain.ServerInitStatus

Bootstrap orchestrator in /Users/wibus/dev/mcpd/internal/app/bootstrap/server_startup_orchestrator.go must be updated to depend on metadata.Manager and serverinit.Manager.

Plugin manager package in /Users/wibus/dev/mcpd/internal/infra/plugin/manager:

    type Manager struct { /* orchestrates plugin instances */ }
    type ManagerOptions struct {
        RootDir string
        Logger  *zap.Logger
        Metrics domain.Metrics
    }

    func NewManager(opts ManagerOptions) (*Manager, error)
    func (m *Manager) RootDir() string
    func (m *Manager) GetStatus(specs []domain.PluginSpec) []Status
    func (m *Manager) IsRunning(name string) bool
    func (m *Manager) Snapshot() []domain.PluginSpec
    func (m *Manager) Apply(ctx context.Context, specs []domain.PluginSpec) error
    func (m *Manager) Stop(ctx context.Context)
    func (m *Manager) Handle(ctx context.Context, spec domain.PluginSpec, req domain.GovernanceRequest) (domain.GovernanceDecision, error)

Plugin subpackages:

- /Users/wibus/dev/mcpd/internal/infra/plugin/socket: path allocation and cleanup for Unix sockets.
- /Users/wibus/dev/mcpd/internal/infra/plugin/process: process start/stop helpers and platform-specific process handling.
- /Users/wibus/dev/mcpd/internal/infra/plugin/handshake: gRPC dial + metadata/config/ready handshake logic.
- /Users/wibus/dev/mcpd/internal/infra/plugin/instance: Instance struct with Stop/Cleanup helpers.

Reload subpackage in /Users/wibus/dev/mcpd/internal/app/controlplane/reload:

    type Step struct {
        Name     string
        Apply    func(context.Context) error
        Rollback func(context.Context) error
    }

    type Transaction struct { /* applies steps with rollback */ }
    func NewTransaction(observer *Observer, logger *zap.Logger) *Transaction
    func (t *Transaction) Apply(ctx context.Context, steps []Step, mode domain.ReloadMode) error

    type Observer struct { /* metrics and logs */ }
    func NewObserver(metrics domain.Metrics, coreLogger, logger *zap.Logger) *Observer

    type ApplyError struct { Stage string; Err error }

ReloadManager in /Users/wibus/dev/mcpd/internal/app/controlplane must use the reload subpackage and be split into smaller files (manager, apply, helpers).
