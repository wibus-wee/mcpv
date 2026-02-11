# macOS Auto Update Install & Restart

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This plan must be maintained in accordance with `.agents/PLANS.md` from the repository root.

## Purpose / Big Picture

After this change, a user can click “Download” in the Updates UI, wait for the download to finish, and then the app will automatically install the new build and restart itself with minimal interruption. This covers unsigned, non-notarized macOS builds for internal use. The user experience is: confirm download manually, then observe install progress, and the app restarts into the new version without additional prompts. You can see it working by triggering a download, watching install progress advance, and seeing the app relaunch.

## Progress

- [x] (2026-02-11 00:00Z) Drafted ExecPlan and captured requirements.
- [x] (2026-02-11 00:45Z) Implemented UpdateInstaller backend (DMG/ZIP extract, app validation, safe replace, restart helper).
- [x] (2026-02-11 00:50Z) Added SystemService install APIs and progress polling types.
- [x] (2026-02-11 01:05Z) Added frontend install progress UI and auto-start install after download completes.
- [x] (2026-02-11 01:10Z) Regenerated Wails bindings and verified updated service models.
- [x] (2026-02-11 01:25Z) Added DMG install helper script and wired release packaging to include it.
- [x] (2026-02-11 01:30Z) Added DMG artifacts to GUI CI upload workflow.

## Surprises & Discoveries

- Observation: `make test` still fails in sandboxed environment due to `httptest` binding to a local port.
  Evidence: `TestManager_StartInstance_StreamableHTTP` fails with `bind: operation not permitted`.

## Decision Log

- Decision: Download requires explicit user click, while install and restart run automatically after download completion.
  Rationale: Aligns with “download must be user-confirmed, others automatic” requirement while minimizing user disruption.
  Date/Author: 2026-02-11 / Codex
- Decision: Ship an install helper script inside DMG to guide first-time unsigned installs.
  Rationale: Provides a clearer onboarding path for Gatekeeper without changing runtime update behavior.
  Date/Author: 2026-02-11 / Codex
- Decision: Upload DMG artifacts in GUI CI alongside ZIPs.
  Rationale: Keeps internal CI artifacts aligned with release packaging for easy testing.
  Date/Author: 2026-02-11 / Codex

## Outcomes & Retrospective

Install and restart automation is implemented; manual download still required. Full test suite remains blocked in the sandbox due to network binding restrictions.

## Context and Orientation

The current update flow has a Go UpdateChecker and UpdateDownloader. UpdateChecker emits update availability and UpdateDownloader downloads a release asset to a temporary file. The UI lives in `frontend/src/routes/settings/advanced.tsx` and already shows update checking and download progress. Wails services live under `internal/ui/services/`, with `SystemService` exposing update APIs and bindings generated under `frontend/bindings/`. The new work adds a backend installer for DMG/ZIP and a frontend install progress UI.

## Plan of Work

Add a Go `UpdateInstaller` that can read a downloaded DMG/ZIP, locate a valid `.app` bundle, remove quarantine attributes defensively, replace the current app safely (with backup and rollback), and trigger a restart via a helper script. Expose two SystemService methods (`StartUpdateInstall` and `GetUpdateInstallProgress`) plus types in `internal/ui/types/types.go` so the frontend can poll for progress. Update the Advanced settings UI to auto-start install after a successful download, poll install progress every second, and display status/percent until the app restarts.

## Concrete Steps

Run the following from the repository root (`/Users/wibus/dev/mcpd`):

1) Edit Go types and services, and add the installer:

   - Create `internal/ui/update_installer.go` with the installer implementation.
   - Update `internal/ui/types/types.go` with `UpdateInstallRequest` and `UpdateInstallProgress`.
   - Update `internal/ui/services/types_alias.go` and `internal/ui/services/system_service.go` to expose the new APIs.

2) Update the frontend:

   - Modify `frontend/src/routes/settings/advanced.tsx` to auto-start install after download completes and to poll install progress.

3) Regenerate bindings:

   - Run `GOCACHE=/tmp/mcpv-gocache make wails-bindings`.

Example expected output snippet for bindings generation:

   INFO  Processed: ... Services, ... Methods, ... Models
   INFO  Output directory: /Users/wibus/dev/mcpd/frontend/bindings

## Validation and Acceptance

Manual acceptance:

- Trigger a manual update check in the UI, click Download, wait for download completion.
- Observe the install status change from preparing/extracting to replacing and restarting.
- The app closes and reopens automatically into the new version.

Tests:

- Run `GOCACHE=/tmp/mcpv-gocache make test` and expect all tests to pass except any existing sandbox-restricted tests. Note the known sandbox limitation if it occurs.

## Idempotence and Recovery

The steps are safe to rerun. If installation fails after backup, the installer will attempt to restore the original app. If a previous backup remains, the next run creates a new timestamped backup. Restart and cleanup helpers are stored under `/tmp` and self-delete.

## Artifacts and Notes

Relevant files to inspect after changes:

- `internal/ui/update_installer.go`
- `internal/ui/services/system_service.go`
- `internal/ui/types/types.go`
- `frontend/src/routes/settings/advanced.tsx`

## Interfaces and Dependencies

The following types and methods must exist after implementation:

- In `internal/ui/types/types.go`:
  - `UpdateInstallRequest` with `FilePath string`.
  - `UpdateInstallProgress` with fields like `Status string`, `Percent float64`, `Message string`, `FilePath string`, `AppPath string`, `BackupPath string`.

- In `internal/ui/services/system_service.go`:
  - `StartUpdateInstall(ctx context.Context, req UpdateInstallRequest) (UpdateInstallProgress, error)`
  - `GetUpdateInstallProgress() (UpdateInstallProgress, error)`

- In `frontend/src/routes/settings/advanced.tsx`:
  - Auto-start install after download completion.
  - Poll install progress via `SystemService.GetUpdateInstallProgress()` every 1s while running.

Plan update note: Added DMG artifact uploads to GUI CI workflow and documented the decision.
