# Catalog subpackage refactor with store split

This ExecPlan is a living document. The sections Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective must be kept up to date as work proceeds.

This plan follows the requirements in /Users/wibus/dev/mcpd/.agent/PLANS.md and must be maintained in accordance with that file.

## Purpose / Big Picture

Split the oversized catalog infrastructure package into focused subpackages so loading, validation, normalization, editing, and profile-store operations are clearly separated. After this change, developers can reason about configuration loading and editing in isolation, and the codebase reflects the actual data flow (decode -> validate -> normalize -> edit). The change is successful when all catalog-related code compiles, the existing tests pass under the new packages, and the system can still load and validate configuration files.

## Progress

- [x] (2026-02-08 16:10Z) Created ExecPlan for catalog subpackage refactor with store split.
- [x] (2026-02-08 16:18Z) Moved catalog files into loader, normalizer, validator, editor, and store subpackages.
- [x] (2026-02-08 16:18Z) Updated package declarations, imports, and cross-package call sites.
- [x] (2026-02-08 16:18Z) Updated and relocated tests to match new package layout.
- [x] (2026-02-08 16:21Z) Verified compilation via `go test ./internal/infra/catalog/...`.

## Surprises & Discoveries

- Observation: Removing ResolveRuntimePath dropped the filepath import, causing a build failure in runtime_editor.go during the first catalog test run.
  Evidence: `go test ./internal/infra/catalog/...` failed with `undefined: filepath` until the import was restored.

## Decision Log

- Decision: Add a dedicated store subpackage and move profile store loading and caller mapping edits into it.
  Rationale: Profile store responsibilities are distinct from file-level catalog editing and should be isolated.
  Date/Author: 2026-02-08 / Codex

## Outcomes & Retrospective

Pending implementation.

## Context and Orientation

The current catalog infrastructure lives in /Users/wibus/dev/mcpd/internal/infra/catalog and bundles multiple responsibilities in one package. It handles loading YAML config files, decoding with defaults, schema validation, semantic validation, normalization, profile store loading, and config editing. The goal is to split this into five subpackages under /Users/wibus/dev/mcpd/internal/infra/catalog:

- loader: reading files, expanding environment variables, decoding with defaults, and orchestrating normalize + validate.
- validator: schema and semantic validation for catalog data.
- normalizer: canonicalizing raw config into domain types.
- editor: editing profile config and runtime config files.
- store: profile store loading, path resolution, and caller mapping edits.

Call sites currently import mcpv/internal/infra/catalog directly. These will be updated to the new subpackages. Tests in the catalog package will also be relocated to the corresponding subpackage directories.

## Plan of Work

First, create the new subpackage directories and move each file into the package that matches its responsibility. Update package declarations and imports to reflect the new package names, and adjust cross-package function calls where needed (for example, loader will call validator and normalizer).

Second, update external call sites in internal/app and internal/ui to import the new subpackages. Replace references to catalog.Loader, catalog.Editor, and related types with the new package paths.

Third, relocate and update tests to the new packages, adjusting test imports and package names so they compile and exercise the same behavior as before.

Finally, run targeted tests for the new packages and record any unexpected failures in Surprises & Discoveries.

## Concrete Steps

1) Create the subpackage directories and move files into place.
   Working directory: /Users/wibus/dev/mcpd
   Commands:
     mkdir -p internal/infra/catalog/{loader,normalizer,validator,editor,store}
     mv internal/infra/catalog/loader.go internal/infra/catalog/loader/loader.go
     mv internal/infra/catalog/decoder.go internal/infra/catalog/loader/decoder.go
     mv internal/infra/catalog/defaults.go internal/infra/catalog/loader/defaults.go
     mv internal/infra/catalog/env_expand.go internal/infra/catalog/loader/env_expand.go
     mv internal/infra/catalog/loader_test.go internal/infra/catalog/loader/loader_test.go
     mv internal/infra/catalog/raw_types.go internal/infra/catalog/normalizer/raw_types.go
     mv internal/infra/catalog/normalizer_server.go internal/infra/catalog/normalizer/server.go
     mv internal/infra/catalog/normalizer_runtime.go internal/infra/catalog/normalizer/runtime.go
     mv internal/infra/catalog/normalizer_plugin.go internal/infra/catalog/normalizer/plugin.go
     mv internal/infra/catalog/validator_server.go internal/infra/catalog/validator/server.go
     mv internal/infra/catalog/schema.go internal/infra/catalog/validator/schema.go
     mv internal/infra/catalog/schema.json internal/infra/catalog/validator/schema.json
     mv internal/infra/catalog/editor.go internal/infra/catalog/editor/editor.go
     mv internal/infra/catalog/profile_editor.go internal/infra/catalog/editor/profile_editor.go
     mv internal/infra/catalog/profile_editor_test.go internal/infra/catalog/editor/profile_editor_test.go
     mv internal/infra/catalog/runtime_editor.go internal/infra/catalog/editor/runtime_editor.go
     mv internal/infra/catalog/runtime_editor_test.go internal/infra/catalog/editor/runtime_editor_test.go
     mv internal/infra/catalog/profile_store.go internal/infra/catalog/store/profile_store.go
     mv internal/infra/catalog/profile_store_test.go internal/infra/catalog/store/profile_store_test.go
     mv internal/infra/catalog/store_editor.go internal/infra/catalog/store/store_editor.go
     mv internal/infra/catalog/store_editor_test.go internal/infra/catalog/store/store_editor_test.go

2) Update package declarations to match new package names and fix cross-package dependencies (loader -> validator/normalizer; editor -> validator/normalizer; store -> loader).

3) Update call sites that import mcpv/internal/infra/catalog to import the appropriate subpackage instead.

4) Update tests to use the new package names and paths.

5) Run tests.
   Commands:
     go test ./internal/infra/catalog/...
     go test ./internal/app/...
     go test ./internal/ui/...

## Validation and Acceptance

Run `go test ./internal/infra/catalog/...` and expect all catalog-related tests to pass. Then run `go test ./internal/app/...` and `go test ./internal/ui/...` to ensure call sites compile with the new import paths. The change is accepted when these tests pass and the config loading and editing APIs build successfully.

## Idempotence and Recovery

The file moves and edits are idempotent when rerun; if a move fails because a file already exists in the target, delete the stale target or move it back before retrying. If compile errors appear, fix imports and package names iteratively and re-run tests.

## Artifacts and Notes

No artifacts yet.

## Interfaces and Dependencies

Loader package (internal/infra/catalog/loader) must expose:

    type Loader struct { /* config loader */ }
    func NewLoader(logger *zap.Logger) *Loader
    func (l *Loader) Load(ctx context.Context, path string) (domain.Catalog, error)
    func (l *Loader) LoadRuntimeConfig(ctx context.Context, path string) (domain.RuntimeConfig, error)

Normalizer package (internal/infra/catalog/normalizer) must expose:

    func NormalizeRuntimeConfig(cfg RawRuntimeConfig) (domain.RuntimeConfig, []string)
    func NormalizeServerSpec(raw RawServerSpec) (domain.ServerSpec, bool)
    func NormalizePluginSpecs(raw []RawPluginSpec) ([]domain.PluginSpec, []string)
    func NormalizeTags(tags []string) []string
    func NormalizeEnvMap(env map[string]string) map[string]string

Validator package (internal/infra/catalog/validator) must expose:

    func ValidateCatalogSchema(raw string) error
    func ValidateServerSpec(spec domain.ServerSpec, index int) []string

Editor package (internal/infra/catalog/editor) must expose:

    type Editor struct { /* config editor */ }
    type EditorError struct { Kind EditorErrorKind; Message string; Err error }
    type ImportRequest struct { Servers []domain.ServerSpec }
    type RuntimeConfigUpdate struct { /* fields */ }
    type SubAgentConfigUpdate struct { /* fields */ }

    func NewEditor(path string, logger *zap.Logger) *Editor
    func (e *Editor) Inspect(ctx context.Context) (ConfigInfo, error)
    func (e *Editor) ImportServers(ctx context.Context, req ImportRequest) error
    func (e *Editor) UpdateRuntimeConfig(ctx context.Context, update RuntimeConfigUpdate) error
    func (e *Editor) UpdateSubAgentConfig(ctx context.Context, update SubAgentConfigUpdate) error
    func (e *Editor) CreateServer(ctx context.Context, spec domain.ServerSpec) error
    func (e *Editor) UpdateServer(ctx context.Context, spec domain.ServerSpec) error
    func (e *Editor) SetServerDisabled(ctx context.Context, serverName string, disabled bool) error
    func (e *Editor) DeleteServer(ctx context.Context, serverName string) error

Store package (internal/infra/catalog/store) must expose:

    type ProfileStoreLoader struct { /* profile store loader */ }
    type ProfileStoreOptions struct { AllowCreate bool }

    func NewProfileStoreLoader(logger *zap.Logger) *ProfileStoreLoader
    func (l *ProfileStoreLoader) Load(ctx context.Context, path string, opts ProfileStoreOptions) (domain.ProfileStore, error)
    func ResolveProfilePath(storePath string, profileName string) (string, error)
    func ResolveRuntimePath(storePath string, allowCreate bool) (string, error)
    func CreateProfile(storePath string, name string) (string, error)
    func DeleteProfile(storePath string, name string) error
    func SetCallerMapping(storePath string, caller string, profile string, profiles map[string]domain.Profile) (StoreUpdate, error)
    func RemoveCallerMapping(storePath string, caller string, profiles map[string]domain.Profile) (StoreUpdate, error)
    type StoreUpdate struct { Path string; Data []byte }

Plan update (2026-02-08): Marked file moves, import updates, and test relocations as completed after applying the refactor. Remaining work is to run and record tests.
Plan update (2026-02-08): Logged the catalog package test run completion after `go test ./internal/infra/catalog/...` succeeded and recorded the missing import fix discovered during testing.
