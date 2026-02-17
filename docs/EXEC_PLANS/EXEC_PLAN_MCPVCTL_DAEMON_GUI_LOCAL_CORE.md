# Plan mcpvctl Daemon Management and GUI Local Core Integration

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This plan is governed by `/.agents/PLANS.md` from the repository root. Maintain this document in accordance with that file.

## Purpose / Big Picture

After this change, the `mcpvctl` CLI can install, start, stop, and check a local `mcpv` core as a system service managed by the OS (systemd on Linux, launchd on macOS). Here, “daemon” means “a system-managed service”, not a manually detached process. The GUI (Wails app) can reuse the same service manager to start a local core when the UI cannot find a running core, but only after explicit user consent. The CLI interface remains consistent with Unix conventions: minimal output by default, machine-readable `--json`, and reliable exit codes for automation. Users can prove it works by running `mcpvctl daemon install`, `mcpvctl daemon start`, `mcpvctl daemon status`, and then opening the GUI to see it connect to the locally started core.

## Progress

- [x] (2026-02-17 03:30Z) Map existing `mcpvctl` and `mcpv` entrypoints, identify existing process/path utilities, and decide systemd/launchd service file locations.
- [x] (2026-02-17 03:55Z) Add a reusable service manager package with install/uninstall/start/stop/status/ensure that targets systemd and launchd.
- [x] (2026-02-17 04:10Z) Implement `mcpvctl daemon` subcommands with Unix-style output and exit codes, including install/uninstall.
- [x] (2026-02-17 04:20Z) Integrate service manager into Wails UI with explicit consent gating and status reporting.
- [x] (2026-02-17 04:25Z) Update documentation and add tests; run `make lint-fix` and `make test`.

## Surprises & Discoveries

- Observation: `launchctl stop` returns “No such process” when the job is not running, even if it is installed; the daemon manager treats this as a non-fatal stop.
  Evidence: handled in `internal/infra/daemon/launchd_darwin.go`.

## Decision Log

- Decision: Treat `mcpvctl` as the manager for the `mcpv` core (not `mcpvmcp`).
  Rationale: `mcpvctl` is a control-plane client for the core; `mcpvmcp` is a gateway and should not be the managed daemon.
  Date/Author: 2026-02-17 / Codex

- Decision: Implement a shared daemon manager package used by both CLI and GUI.
  Rationale: Avoids duplicate process-management logic and keeps GUI behavior consistent with CLI behavior.
  Date/Author: 2026-02-17 / Codex

- Decision: Keep CLI output Unix-like with stable exit codes and optional `--json`.
  Rationale: Enables scripting and automation while remaining human-readable by default.
  Date/Author: 2026-02-17 / Codex

- Decision: Interpret “daemon” as OS-managed services (systemd/launchd), not a detached background process.
  Rationale: Matches the clarified requirement and provides standard lifecycle controls and reliability.
  Date/Author: 2026-02-17 / Codex

## Outcomes & Retrospective

Delivered a systemd/launchd-backed daemon manager, a `mcpvctl daemon` command group, and Wails service hooks to start the core with explicit consent. Added service definition generation, status handling, and tests for both Linux and macOS paths. Lint and tests pass.

## Context and Orientation

The `mcpv` core is the control-plane server and currently runs from `cmd/mcpv`. The CLI `mcpvctl` is a gRPC client that talks to the core over the control-plane RPC server in `internal/infra/rpc`. The gateway binary `mcpvmcp` is separate; it is not the core and should not be treated as the managed daemon. Wails UI logic lives in `internal/ui` and exposes services to the frontend in `internal/ui/services`. The UI already has OS path helpers in `internal/ui/system_paths.go` (for locating `mcpvmcp`).

In this plan, “daemon” means a system-managed service. On Linux, that means a systemd user service (`systemctl --user`), stored under `~/.config/systemd/user/`. On macOS, that means a launchd user agent stored under `~/Library/LaunchAgents/` and managed with `launchctl`. The GUI will check service status and, with user consent, start the core if it is not running.

## Plan of Work

Begin by mapping the existing CLI, core, and UI code so the service manager can be introduced without breaking current behavior. Review `cmd/mcpv/main.go` to confirm how the core is launched (`mcpv serve --config <path>`), and review `cmd/mcpvctl` to see where a new `daemon` command group should live. Identify OS path helpers in `internal/ui/system_paths.go` to reuse for locating `mcpv`.

Create a new internal package that owns OS service lifecycle, for example `internal/infra/daemon`. This package should expose a small `Manager` with methods such as `Install`, `Uninstall`, `Start`, `Stop`, `Status`, and `EnsureRunning`. The manager should accept explicit inputs (config path, rpc address, log path, and binary path) and generate service definitions:

On Linux (systemd user services):
  - Unit file at `~/.config/systemd/user/mcpv.service`.
  - `ExecStart` should run `mcpv serve --config <path>`.
  - Use `systemctl --user daemon-reload`, `systemctl --user enable --now mcpv.service`, and `systemctl --user disable --now mcpv.service`.

On macOS (launchd user agents):
  - Plist at `~/Library/LaunchAgents/com.mcpv.core.plist`.
  - Use `launchctl bootstrap gui/$UID <plist>`, `launchctl bootout gui/$UID <plist>`, and `launchctl kickstart -k gui/$UID/com.mcpv.core`.

The manager should not keep its own PID state file. Instead, status is derived from `systemctl --user is-active` on Linux and `launchctl print` on macOS. If the service is not installed, status should clearly indicate “not installed.” `EnsureRunning` should install and start the service only when explicitly allowed by the caller.

Implement a `daemon` command group in `mcpvctl` with Unix-style subcommands: `daemon install`, `daemon uninstall`, `daemon start`, `daemon stop`, `daemon status`, and `daemon restart`. Each command should print minimal human output by default and support `--json` for structured output. Define exit codes: `0` for success, `3` for “not running” on `status`, `4` for “not installed”, and `1` for errors. Provide flags for `--config`, `--rpc`, `--log-file`, and `--mcpv-binary`. Default values should mirror the existing CLI defaults (`runtime.yaml` for config, `unix:///tmp/mcpv.sock` for RPC). `daemon install` should generate the correct service definition for the OS and write it to disk. `daemon start` should start the service via systemd/launchd and optionally verify the RPC endpoint by calling `GetInfo`.

Integrate the service manager into the GUI by adding a new Wails service, for example `internal/ui/services/daemon_service.go`, that exposes `Status`, `Install`, `Uninstall`, `Start`, `Stop`, and `EnsureRunning` methods. `EnsureRunning` must require explicit consent from the UI, passed in the request; if consent is not granted, it must return a structured “permission denied” error. The UI can call this service when it cannot connect to a core. Add a UI-level path resolution helper for `mcpv` and `mcpvctl` in `internal/ui/system_paths.go` (analogous to `ResolveMcpvmcpPath`) so the GUI can locate bundled binaries. Avoid any automatic start without consent.

Document the new behavior in `README.md` and add examples to show how to install, start, and stop the daemon, including how to override the config file and RPC address. Add tests for the service manager’s install/status transitions and idempotent start/stop. Provide at least one integration test in `internal/infra/daemon` that validates `Install`/`Uninstall` command generation without actually running `mcpv`.

## Concrete Steps

All commands run from `/Users/wibus/dev/mcpd`.

1) Review core and CLI entrypoints to confirm flags and defaults:
   - `sed -n '1,160p' cmd/mcpv/main.go`
   - `rg -n "new.*Cmd" cmd/mcpvctl -S`

2) Add the service manager package:
   - Create `internal/infra/daemon/manager.go` with the `Manager` type and methods `Install`, `Uninstall`, `Start`, `Stop`, `Status`, `EnsureRunning`.
   - Create `internal/infra/daemon/systemd.go` and `internal/infra/daemon/launchd.go` for OS-specific command generation and execution.
   - Add a unit test file `internal/infra/daemon/manager_test.go` to cover install/status transitions and error handling.

3) Add CLI commands:
   - Create `cmd/mcpvctl/daemon.go` with `newDaemonCmd`, `newDaemonInstallCmd`, `newDaemonUninstallCmd`, `newDaemonStartCmd`, `newDaemonStopCmd`, `newDaemonStatusCmd`, and `newDaemonRestartCmd`.
   - Reuse existing flag helpers where possible; add new ones for `--config`, `--log-file`, and `--mcpv-binary`.

4) Add GUI integration:
   - Create `internal/ui/services/daemon_service.go` and wire it into `internal/ui/services/service.go` so the frontend can call it.
   - Add path resolution helpers in `internal/ui/system_paths.go` for `mcpv` and `mcpvctl`.
   - Define request/response types in `internal/ui/types/types.go` (e.g., `DaemonStatus`, `DaemonInstallRequest`, `DaemonEnsureRequest`).

5) Update documentation:
   - Add a `mcpvctl daemon` section to `README.md` with example commands and expected output.

6) Validate:
   - Run `make lint-fix`.
   - Run `make test`.

## Validation and Acceptance

Manual behavior checks:

1) Install and start the daemon:
   - Run: `./bin/core/mcpvctl daemon install --config ./runtime.yaml --rpc unix:///tmp/mcpv.sock`
   - Expect: stdout includes a “installed” message, exit code 0.
   - Run: `./bin/core/mcpvctl daemon start`
   - Expect: stdout includes a “started” message, exit code 0.

2) Check status:
   - Run: `./bin/core/mcpvctl daemon status --rpc unix:///tmp/mcpv.sock`
   - Expect: stdout shows “running” and the service name, exit code 0.

3) Stop the daemon:
   - Run: `./bin/core/mcpvctl daemon stop`
   - Expect: stdout shows “stopped”, exit code 0.

4) Uninstall:
   - Run: `./bin/core/mcpvctl daemon uninstall`
   - Expect: stdout shows “uninstalled”, exit code 0.

4) UI consent gating:
   - When the GUI cannot reach the core, the UI must prompt the user.
   - If the user declines, the GUI must not start the daemon and should display a clear error.
   - If the user accepts, the GUI calls `EnsureRunning` and then connects successfully.

Testing:

Run `make test` and expect all tests to pass. The new daemon manager tests should fail before the change and pass after.

## Idempotence and Recovery

Starting the daemon is idempotent: if the service is already active, the start command should return success without additional side effects. Stopping the daemon should be safe to repeat; a second stop should report “not running” with exit code 3. Installing and uninstalling are safe to repeat (install overwrites the unit/plist; uninstall removes it if present). If a command fails mid-way, rerun it after removing the service file and re-installing.

## Artifacts and Notes

Include short evidence snippets in this section once implemented, such as:

  $ mcpvctl daemon install --config ./runtime.yaml
  installed service=systemd user unit=~/.config/systemd/user/mcpv.service

  $ mcpvctl daemon status
  running service=mcpv.service

## Interfaces and Dependencies

New package in `internal/infra/daemon`:

    type Status struct {
        Installed bool
        Running bool
        ServiceName string
        ConfigPath string
        RPCAddress string
        LogPath string
    }

    type Manager struct {
        BinaryPath string
        ConfigPath string
        RPCAddress string
        LogPath string
    }

    func (m *Manager) Install(ctx context.Context) (Status, error)
    func (m *Manager) Uninstall(ctx context.Context) (Status, error)
    func (m *Manager) Start(ctx context.Context) (Status, error)
    func (m *Manager) Stop(ctx context.Context) (Status, error)
    func (m *Manager) Status(ctx context.Context) (Status, error)
    func (m *Manager) EnsureRunning(ctx context.Context, allowStart bool) (Status, error)

`cmd/mcpvctl` must add:

    mcpvctl daemon install [--config] [--rpc] [--log-file] [--mcpv-binary]
    mcpvctl daemon uninstall
    mcpvctl daemon start
    mcpvctl daemon stop
    mcpvctl daemon status
    mcpvctl daemon restart

Wails UI additions:

    type DaemonStatus struct { Installed bool; Running bool; RPC string; LogPath string }
    type DaemonEnsureRequest struct { AllowStart bool; ConfigPath string; RPCAddress string; LogPath string }
    func (s *DaemonService) Status(ctx context.Context) (DaemonStatus, error)
    func (s *DaemonService) EnsureRunning(ctx context.Context, req DaemonEnsureRequest) (DaemonStatus, error)

Plan Change Notes: Updated the definition of “daemon” to systemd/launchd-managed services and removed PID/state-file language from validation/testing to align with service-based behavior. Progress updated after implementation and validation.
