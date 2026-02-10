# UI Settings Store with bbolt

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

PLANS.md is available at `.agent/PLANS.md` from the repository root. This plan must be maintained in accordance with that file.

## Purpose / Big Picture

本计划的目标是在 Wails 后端新增一个可演进的 UI 配置存储层。完成后，GUI 可以通过 Wails bindings 读取、更新、重置 UI 配置，并支持全局与 workspace 级别的覆盖规则。用户能够在 UI 中保存表格排序、面板折叠、主题偏好等状态，并在重启后恢复。

## Progress

- [x] (2026-02-10 00:00Z) 添加 bbolt 依赖到 go.mod/go.sum。
- [x] (2026-02-10 00:20Z) 创建 UI settings 存储包（bbolt 数据结构、迁移框架、路径解析）。
- [x] (2026-02-10 00:24Z) 在 Wails UI 服务层新增 UISettingsService，并挂载到 ServiceRegistry。
- [x] (2026-02-10 00:26Z) 扩展 internal/ui/types 与服务别名，提供 UI settings 的请求与响应类型。
- [x] (2026-02-10 00:28Z) 让 Manager 在应用生命周期内管理 UI settings store（初始化与关闭）。
- [x] (2026-02-10 00:32Z) 添加最小单测覆盖读写、合并与重置行为。

## Surprises & Discoveries

- 暂无。

## Decision Log

- Decision: 使用 bbolt 作为 UI settings 存储引擎，并以 JSON blob 作为 section payload。
  Rationale: bbolt 轻量且支持事务语义，JSON payload 方便调试与迁移。
  Date/Author: 2026-02-10 / Codex

## Outcomes & Retrospective

- 暂无。

## Context and Orientation

本仓库的 Wails 相关逻辑位于 `internal/ui/`。`internal/ui/services` 暴露 Wails 服务，`internal/ui/types` 定义前端可用的序列化类型。`internal/ui/manager.go` 管理核心生命周期，可用于挂载 UI settings store 的生命周期。当前没有 UI 配置存储实现，需要新增 `internal/ui/uiconfig` 包来封装 bbolt。

## Plan of Work

首先在 `internal/ui/uiconfig/` 新增存储实现，包含：bbolt 打开/关闭、schema 初始化、section 读写、全局/工作区合并、迁移框架与路径解析。其次在 `internal/ui/manager.go` 中加入 UI settings store 的惰性初始化与 Shutdown 关闭。接着在 `internal/ui/services` 新增 `UISettingsService`，暴露 Get/Update/Reset/GetEffective 与 WorkspaceId 计算，并加入 `ServiceRegistry`。然后在 `internal/ui/types/types.go` 新增相关 request/response 类型，并在 `internal/ui/services/types_alias.go` 中追加别名。最后补充 `internal/ui/uiconfig` 的单测，覆盖读写、合并、重置。

## Concrete Steps

在仓库根目录 `/Users/wibus/dev/mcpd` 执行以下步骤：

1) 添加依赖：
   go get go.etcd.io/bbolt@latest

2) 新增 `internal/ui/uiconfig` 文件：
   - `store.go`：bbolt 存储实现
   - `path.go`：路径解析与 workspaceId 计算
   - `migrate.go`：schema 版本与迁移框架

3) 修改 `internal/ui/manager.go`：
   - 增加 UI settings store 字段
   - 增加惰性 getter
   - 在 Shutdown 时关闭 store

4) 新增 `internal/ui/services/ui_settings_service.go` 并注册到 `ServiceRegistry`。

5) 修改 `internal/ui/types/types.go` 与 `internal/ui/services/types_alias.go` 增加 UI settings 类型。

6) 新增测试 `internal/ui/uiconfig/store_test.go`。

## Validation and Acceptance

- 运行 `go test ./internal/ui/uiconfig`，应通过所有测试。
- 通过调用 Wails 服务（或单元测试）验证：
  - `GetUISettings` 返回默认空 sections。
  - `UpdateUISettings` 写入 section 后可读回。
  - `GetEffectiveUISettings` 在 workspace 覆盖时优先 workspace section。
  - `ResetUISettings` 可清空对应 scope。

## Idempotence and Recovery

- 迁移与 schema 初始化在 DB 打开时执行，可重复运行。
- 如果 UI settings DB 文件损坏，可删除 `~/.config/mcpv/ui-settings.db` 重新生成；重启后会以默认空配置恢复。

## Artifacts and Notes

- 预期新增目录：`internal/ui/uiconfig/`
- 预期新增服务：`internal/ui/services/ui_settings_service.go`

## Interfaces and Dependencies

依赖库：`go.etcd.io/bbolt`。

在 `internal/ui/uiconfig` 中提供以下关键接口：

- `type Store struct { ... }`
- `func OpenStore(path string) (*Store, error)`
- `func (s *Store) Get(scope Scope, workspaceID string) (SettingsSnapshot, error)`
- `func (s *Store) GetEffective(workspaceID string) (SettingsSnapshot, error)`
- `func (s *Store) Update(scope Scope, workspaceID string, updates map[string]json.RawMessage, removes []string) (SettingsSnapshot, error)`
- `func (s *Store) Reset(scope Scope, workspaceID string) (SettingsSnapshot, error)`
- `func (s *Store) Close() error`
- `func WorkspaceIDForPath(path string) string`

在 `internal/ui/services/ui_settings_service.go` 中提供以下方法：

- `GetUISettings(ctx context.Context, req UISettingsScopeRequest) (UISettingsSnapshot, error)`
- `GetEffectiveUISettings(ctx context.Context, req UISettingsEffectiveRequest) (UISettingsSnapshot, error)`
- `UpdateUISettings(ctx context.Context, req UpdateUISettingsRequest) (UISettingsSnapshot, error)`
- `ResetUISettings(ctx context.Context, req ResetUISettingsRequest) (UISettingsSnapshot, error)`
- `GetWorkspaceID(ctx context.Context) (UISettingsWorkspaceIDResponse, error)`

## Plan Update Notes

2026-02-10: 完成存储包、服务、类型、生命周期与测试实现，并同步更新进度。
