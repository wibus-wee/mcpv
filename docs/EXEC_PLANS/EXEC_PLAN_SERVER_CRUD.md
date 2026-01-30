# Server CRUD APIs and UI Wiring

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This plan must be maintained in accordance with `.agent/PLANS.md` in the repository root.

## Purpose / Big Picture

After this change, the mcpv UI can create and update server definitions directly from the Servers page. The backend exposes `CreateServer` and `UpdateServer` APIs that validate and normalize incoming server specs, write them to the single config file, and surface consistent errors. The frontend edit sheet calls these APIs and relies on backend validation, showing a toaster on failure. You can verify success by adding a server in the UI, reloading config, and seeing the server appear in the list.

## Progress

- [x] (2026-01-25 18:40Z) Added catalog editor support for CreateServer/UpdateServer including normalization, validation, and profile file updates.
- [x] (2026-01-25 18:43Z) Exposed CreateServer/UpdateServer in UI service types and regenerated Wails bindings.
- [x] (2026-01-25 18:47Z) Wired the frontend edit sheet to the new APIs and removed front-end validation.
- [x] (2026-01-25 18:55Z) Ran `go test ./internal/infra/catalog ./internal/ui` (passed with macOS linker warnings).

## Surprises & Discoveries

- Observation: `wails3 generate bindings -ts` emitted warnings about Go 1.25 packages when using a Go 1.24-built Wails binary, but still produced bindings.
  Evidence: warnings referencing `package requires newer Go version go1.25` during bindings generation.
- Observation: `go test` passed but printed macOS linker warnings about object files built for a newer macOS version.
  Evidence: `ld: warning: object file ... was built for newer 'macOS' version (26.0) than being linked (11.0)`.

## Decision Log

- Decision: Use full-replacement updates (scheme 1) with backend normalization defaults.
  Rationale: Simpler API surface and consistent config writing semantics.
  Date/Author: 2026-01-25, Codex.

## Outcomes & Retrospective

TBD. This section will be updated after implementation.

## Context and Orientation

The backend uses a single YAML config file to define `servers` and runtime settings. Configuration edits are handled by `internal/infra/catalog` and surfaced via `internal/ui` services for the Wails frontend. The existing server UI uses `ServerService.ListServers`, `GetServer`, `SetServerDisabled`, and `DeleteServer` but lacks create/update. The editor functions that write server config live in `internal/infra/catalog/profile_editor.go` and are wrapped by `internal/infra/catalog/editor.go`.

Key files:

- `internal/infra/catalog/editor.go`: high-level editor methods used by UI services.
- `internal/infra/catalog/profile_editor.go`: YAML parse/update helpers for servers in the config file.
- `internal/ui/types.go`: Wails-exposed request/response types.
- `internal/ui/server_service.go`: Wails service for server CRUD.
- `frontend/src/modules/servers/components/server-edit-sheet.tsx`: UI sheet for add/edit.
- `frontend/bindings/mcpv/internal/ui/*`: generated bindings used by the frontend.

## Plan of Work

First, add Create/Update capability in the catalog editor. Extend `internal/infra/catalog/profile_editor.go` with `CreateServer` and `UpdateServer` functions that load the YAML, parse existing `servers`, then append or replace a server spec. Add sentinel errors for “already exists” and “not found”. In `internal/infra/catalog/editor.go`, add `CreateServer` and `UpdateServer` methods that normalize and validate a `domain.ServerSpec`, then call the profile editor and write the updated file. Reuse existing normalization helpers (`normalizeTags`, `normalizeImportEnv`) and the shared `validateServerSpec` logic to ensure consistent validation.

Second, expose the new APIs to the UI layer. Add `CreateServerRequest` and `UpdateServerRequest` in `internal/ui/types.go`, and implement `CreateServer` / `UpdateServer` in `internal/ui/server_service.go` using a new mapping helper that converts `ServerSpecDetail` into `domain.ServerSpec`. Ensure errors map through `mapCatalogError`.

Third, regenerate Wails bindings (or update generated TS if the generator is unavailable). Add the new methods to `frontend/bindings/mcpv/internal/ui/serverservice.ts` and new request types in `frontend/bindings/mcpv/internal/ui/models.ts` so the frontend can call the APIs.

Fourth, wire the frontend edit sheet. In `frontend/src/modules/servers/components/server-edit-sheet.tsx`, remove local validation and call `ServerService.CreateServer` or `ServerService.UpdateServer` depending on edit mode. Build a full server spec for update by taking the existing server detail and overriding the edited fields. For create, fill required fields from the form and rely on backend defaults for non-exposed fields. On success, call `ReloadConfig` and trigger `onSaved`; on error, show a toaster and return.

Finally, add or extend tests in `internal/infra/catalog/profile_editor_test.go` to cover create/update behaviors and ensure errors are surfaced when a server already exists or is missing.

## Concrete Steps

From the repository root (`/Users/wibus/dev/mcpv`):

1) Edit `internal/infra/catalog/profile_editor.go` and `internal/infra/catalog/editor.go` to add create/update logic.
2) Edit `internal/ui/types.go`, `internal/ui/mapping.go`, and `internal/ui/server_service.go` to expose the new APIs.
3) Regenerate or update Wails bindings with:

    make wails-bindings

4) Edit `frontend/src/modules/servers/components/server-edit-sheet.tsx` to call the new APIs and remove local validation.
5) Run targeted tests:

    go test ./internal/infra/catalog
    go test ./internal/ui

## Validation and Acceptance

- Running `go test ./internal/infra/catalog` and `go test ./internal/ui` should pass.
- Starting the app and adding a server via the UI should write to the config file and show the server in the list after reload.
- Editing a server should persist the change and reflect in the UI without requiring a manual config edit.
- Invalid input should surface via toaster (frontend) and `ErrCodeInvalidRequest` (backend) without crashing.

## Idempotence and Recovery

Each update rewrites the same config file deterministically. If a step fails, revert only the modified files from that step and re-run the relevant tests. The CRUD operations are safe to rerun; creating an existing server or updating a missing server yields a controlled error.

## Artifacts and Notes

Example server creation request (conceptual):

    {"spec":{"name":"local-tools","transport":"stdio","cmd":["/path/mcp"],"idleSeconds":300,"maxConcurrent":5}}

## Interfaces and Dependencies

In `internal/ui/types.go`, define:

    type CreateServerRequest struct { Spec ServerSpecDetail `json:"spec"` }
    type UpdateServerRequest struct { Spec ServerSpecDetail `json:"spec"` }

In `internal/ui/server_service.go`, define:

    func (s *ServerService) CreateServer(ctx context.Context, req CreateServerRequest) error
    func (s *ServerService) UpdateServer(ctx context.Context, req UpdateServerRequest) error

In `internal/infra/catalog/editor.go`, define:

    func (e *Editor) CreateServer(ctx context.Context, spec domain.ServerSpec) error
    func (e *Editor) UpdateServer(ctx context.Context, spec domain.ServerSpec) error

Change Note: Initial ExecPlan created for server CRUD backend + UI wiring.
