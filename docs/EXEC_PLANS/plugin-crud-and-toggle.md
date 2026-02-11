# Plugin CRUD and Toggle Support

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

PLANS guidance lives at `/.agents/PLANS.md` in this repository. This ExecPlan must be maintained in accordance with that file.

## Purpose / Big Picture

完成后，用户可以在 GUI 的 Plugins 页面直接新增、编辑、删除插件，并通过开关启用或禁用插件，不再需要手动改 YAML。启用与禁用会通过配置文件生效，Reload 成功后插件会真实地参与或退出治理流水线。验证方式是：在 UI 中创建一个插件并保存，触发 reload 后插件出现在列表中；禁用该插件后不再运行且状态显示为 stopped。

## Progress

- [x] (2026-02-11 00:00Z) ExecPlan drafted and saved to `docs/EXEC_PLANS/plugin-crud-and-toggle.md`.
- [x] (2026-02-11 00:40Z) Added disabled support to plugin catalog schema, normalizer, and domain types.
- [x] (2026-02-11 00:45Z) Implemented catalog editor operations for plugins (create, update, delete, set disabled).
- [x] (2026-02-11 00:55Z) Implemented PluginService CRUD and toggle, and updated list mapping to reflect disabled state.
- [x] (2026-02-11 01:00Z) Updated plugin runtime pipeline and manager to ignore disabled plugins.
- [x] (2026-02-11 01:20Z) Updated frontend plugin UI to call CRUD/toggle and reload config.
- [x] (2026-02-11 02:15Z) Kept local optimistic toggle state until backend list matches desired value to avoid immediate reversion.
- [ ] (2026-02-11 01:30Z) Regenerated Wails bindings; lint-fix failed; tests failed due to sandboxed network binding (details in Surprises & Discoveries).

## Surprises & Discoveries

- Observation: `make wails-bindings` completed with cache-related warnings about missing Go build cache and a `dock_darwin.go` C import warning, but bindings were generated.
  Evidence: Wails reported “Processed: 750 Packages, 13 Services, 59 Methods, 110 Models” with warnings.
- Observation: `make lint-fix` failed with “no go files to analyze”.
  Evidence: `golangci-lint run --config .golangci.yml --fix` exited with error 5.
- Observation: `make test` failed because `httptest` could not bind a local port in `internal/infra/lifecycle`.
  Evidence: `listen tcp6 [::1]:0: bind: operation not permitted` during `TestManager_StartInstance_StreamableHTTP`.

## Decision Log

- Decision: Use a `plugins[].disabled` boolean in the catalog file as the source of truth for plugin enable/disable.
  Rationale: The server module already uses file-backed disabled state and reload, so this keeps behavior consistent and avoids a separate state store.
  Date/Author: 2026-02-11 / Codex.

## Outcomes & Retrospective

Pending implementation. This section will summarize user-visible behavior and any remaining gaps after milestones complete.

## Context and Orientation

This repository exposes a Wails-based GUI for managing MCP servers and governance plugins. The plugin system reads `plugins` from the catalog YAML file, normalizes them into `domain.PluginSpec`, and the governance pipeline executes enabled plugins in a fixed category order. The UI calls Go services in `internal/ui/services`, and those are surfaced to the frontend through generated Wails bindings in `frontend/src/bindings`.

Key paths and their roles:

- `internal/domain/plugin.go` defines `PluginSpec` and plugin categories/flows. This is the core type used by the pipeline and plugin manager.
- `internal/infra/catalog/normalizer/raw_types.go` and `internal/infra/catalog/normalizer/plugin.go` parse YAML plugin entries into `domain.PluginSpec`.
- `internal/infra/catalog/validator/schema.json` defines the schema for the catalog file, including `plugins` entries.
- `internal/infra/catalog/editor/editor.go` and `internal/infra/catalog/editor/profile_editor.go` implement file edits for server CRUD. We will add plugin CRUD here.
- `internal/ui/services/plugin_service.go` currently lists plugins and has a stub Toggle method; CRUD is not implemented.
- `internal/ui/types/types.go` defines data contracts used by Wails bindings; new request types will be added here.
- `internal/infra/plugin/manager/manager.go` starts and stops plugin processes based on specs.
- `internal/infra/pipeline/engine.go` decides which plugins to run for a request/response.
- Frontend plugin UI lives under `frontend/src/modules/plugin/` and uses hooks in `frontend/src/modules/plugin/hooks.ts`.
- Reload is done by `ConfigService.ReloadConfig` and a small helper in `frontend/src/modules/servers/lib/reload-config.ts` that normalizes errors for UI.

Definitions used in this plan:

- Catalog file: the YAML configuration file referenced by the manager; it contains `servers`, `plugins`, and runtime settings. The exact path is surfaced by `ConfigService.GetConfigMode` and `ConfigService.GetConfigPath` in the UI.
- Governance pipeline: the runtime sequence that calls plugins per category in `internal/infra/pipeline/engine.go`.
- Disabled plugin: a plugin entry where `disabled: true` is set in the catalog. Disabled plugins must not be started by the manager and must not be executed by the pipeline.

## Plan of Work

The implementation will follow the same file-backed CRUD pattern used by servers. First, introduce a `disabled` boolean into the plugin catalog schema and normalize it into `domain.PluginSpec.Disabled`. This ensures the catalog file can represent enabled and disabled plugins, and the runtime sees that state.

Next, extend the catalog editor to edit plugins in the YAML document. This mirrors the server editor behavior: load the YAML, mutate the `plugins` list, and write the file back. Create, update, delete, and set disabled operations will be added, with validation that names are non-empty and unique.

Then implement PluginService CRUD methods that call the editor and map errors through `mapCatalogError`, plus a Toggle method that flips `disabled` and logs the intent. Update the plugin list mapping so that `Enabled` is derived from `!spec.Disabled`, and disabled plugins show status `stopped` without an error message.

After that, update the plugin runtime pipeline and manager to ignore disabled plugins. The manager should skip starting disabled plugins, and the pipeline should never consider them when building its category lists. This guarantees that toggling in the UI has real runtime effect.

Finally, update the frontend plugin module to call the new CRUD/toggle endpoints and to reload config after each change. Use the existing reload helper from the servers module so error messaging is consistent. Regenerate Wails bindings, then run lint fix and tests as required by repo policy.

Milestones will be used to stage the work: backend schema and editor updates first, service and runtime behavior next, then frontend changes and validation.

## Concrete Steps

All commands should be run from the repository root: `/Users/wibus/dev/mcpd`.

1) Update Go domain and catalog normalization:

   - Edit `internal/domain/plugin.go` to add `Disabled bool` to `PluginSpec`.
   - Edit `internal/infra/catalog/normalizer/raw_types.go` to add `Disabled bool` to `RawPluginSpec`.
   - Edit `internal/infra/catalog/normalizer/plugin.go` to map `raw.Disabled` into `PluginSpec.Disabled`.
   - Edit `internal/infra/catalog/validator/schema.json` to allow `disabled` in `pluginSpec`.

2) Add plugin editor helpers:

   - Extend `internal/infra/catalog/editor/profile_editor.go` with a `pluginSpecYAML` struct and functions:
     - `CreatePlugin(path string, plugin domain.PluginSpec) (ProfileUpdate, error)`
     - `UpdatePlugin(path string, plugin domain.PluginSpec) (ProfileUpdate, error)`
     - `SetPluginDisabled(path string, pluginName string, disabled bool) (ProfileUpdate, error)`
     - `DeletePlugin(path string, pluginName string) (ProfileUpdate, error)`
   - Extend `internal/infra/catalog/editor/editor.go` with methods that call these functions and write the updated file, mirroring server CRUD.

3) Update UI service contracts and implementation:

   - Add request types in `internal/ui/types/types.go`:
     - `CreatePluginRequest { Spec PluginSpecDetail }`
     - `UpdatePluginRequest { Spec PluginSpecDetail }`
     - `DeletePluginRequest { Name string }`
     - Optionally define `PluginSpecDetail` if needed; otherwise reuse `PluginListEntry` for edit payloads.
   - Add type aliases in `internal/ui/services/types_alias.go`.
   - Implement `CreatePlugin`, `UpdatePlugin`, `DeletePlugin`, and `TogglePlugin` in `internal/ui/services/plugin_service.go`.
   - Ensure `GetPluginList` sets `Enabled` as `!spec.Disabled` and uses status `stopped` with empty error when disabled.

4) Enforce disabled state at runtime:

   - In `internal/infra/plugin/manager/manager.go`, skip starting specs where `spec.Disabled` is true.
   - In `internal/infra/pipeline/engine.go`, filter out disabled specs when building `byCategory` in `Update`.

5) Frontend plugin module updates:

   - Update `frontend/src/modules/plugin/hooks.ts` to include CRUD methods calling the new bindings.
   - Update `frontend/src/modules/plugin/components/plugin-edit-sheet.tsx` to call Create/Update, then call reload and mutate the list.
   - Add delete UI (likely in the edit sheet footer) that calls Delete and reloads.
   - Update `frontend/src/modules/plugin/components/plugin-list-table.tsx` toggle to call reload after Toggle.
   - Update `frontend/src/modules/plugin/README.md` and `frontend/src/modules/plugin/components/README.md` to reflect CRUD being implemented.

6) Regenerate bindings and run required project checks:

   - Run `make wails-bindings` to regenerate TypeScript bindings.
   - Run `make lint-fix` and `make test` as required by repo guidance.

Expected command snippets (examples, adjust if output differs):

   make wails-bindings
     ...
     Generating bindings...

   make lint-fix
     ...
     lint-fix complete

   make test
     ...
     ok   ./internal/...

## Validation and Acceptance

Behavioral acceptance requires both backend and UI checks:

- Backend validation: run `go test ./internal/infra/pipeline` and add a test that ensures a disabled plugin is not invoked (the test should fail before the change and pass after). Also run `go test ./internal/infra/catalog` if new editor tests are added.
- UI validation: run `make wails-dev`, open the GUI, go to the Plugins page, and perform the following:
  1) Add a plugin with a unique name and valid command. Save, then reload. The plugin should appear in the list.
  2) Toggle the plugin to disabled. After reload, the plugin should show `Enabled` off and status `stopped` without an error message.
  3) Delete the plugin. After reload, it should be removed from the list.

If reload fails due to invalid config, the UI should show the normalized error message from `reload-config.ts` and no in-memory state should claim success.

## Idempotence and Recovery

All editor operations rewrite the catalog file. They are safe to retry because they are deterministic and validate input. If an operation partially fails, rerun it after fixing the input or restoring the file.

Before testing with real configurations, make a backup of the catalog file (copy it to a temporary filename) so it can be restored. If a change introduces invalid YAML, restore from backup and retry.

## Artifacts and Notes

No artifacts yet. As work proceeds, capture short diffs or logs here, for example the change to a plugin entry showing `disabled: true` and a reload log indicating plugins updated.

## Interfaces and Dependencies

At the end of implementation, these interfaces and types must exist:

- `internal/domain/plugin.go` must include `Disabled bool` in `PluginSpec`.

- `internal/infra/catalog/editor/profile_editor.go` must define the plugin CRUD helpers with these signatures:

    func CreatePlugin(path string, plugin domain.PluginSpec) (ProfileUpdate, error)
    func UpdatePlugin(path string, plugin domain.PluginSpec) (ProfileUpdate, error)
    func SetPluginDisabled(path string, pluginName string, disabled bool) (ProfileUpdate, error)
    func DeletePlugin(path string, pluginName string) (ProfileUpdate, error)

- `internal/infra/catalog/editor/editor.go` must expose editor methods:

    func (e *Editor) CreatePlugin(ctx context.Context, spec domain.PluginSpec) error
    func (e *Editor) UpdatePlugin(ctx context.Context, spec domain.PluginSpec) error
    func (e *Editor) SetPluginDisabled(ctx context.Context, name string, disabled bool) error
    func (e *Editor) DeletePlugin(ctx context.Context, name string) error

- `internal/ui/services/plugin_service.go` must expose the Wails service methods:

    func (s *PluginService) CreatePlugin(ctx context.Context, req CreatePluginRequest) error
    func (s *PluginService) UpdatePlugin(ctx context.Context, req UpdatePluginRequest) error
    func (s *PluginService) DeletePlugin(ctx context.Context, req DeletePluginRequest) error
    func (s *PluginService) TogglePlugin(ctx context.Context, req TogglePluginRequest) error

- `internal/ui/types/types.go` must define request types used by bindings, and the frontend must import them through `@bindings/mcpv/internal/ui/types`.

No external libraries are required. Reuse the existing YAML editor and reload mechanisms in this repository.

Plan updated on 2026-02-11 to reflect completed implementation steps, binding generation, test/lint failures encountered during validation, the SWR mutation-based optimistic toggle adjustment, and holding local optimistic UI state until backend data matches.
