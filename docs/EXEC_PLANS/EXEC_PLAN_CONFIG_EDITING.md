# Config Editing: Profiles, Callers, Servers (No Hot Reload)

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.agent/PLANS.md` in the repository root.

## Purpose / Big Picture

用户在配置页面完成核心编辑操作：新建/删除 profile、修改 caller 到 profile 映射、删除 server、开关 server。所有改动写入 profile store 文件，UI 提示“需要重启 Core 生效”。用户不再需要手动编辑 YAML 才能维护配置。

## Progress

- [x] (2025-12-29T14:25Z) 为 server 增加 `disabled` 字段并在运行时跳过禁用项，同时 UI 可见该状态。
- [x] (2025-12-29T14:25Z) 增加配置编辑后端接口：create/delete profile、set/remove caller mapping、toggle/delete server。
- [x] (2025-12-29T14:25Z) 前端实现 profile 创建/删除入口、server 开关与删除、callers 映射编辑。
- [x] (2025-12-29T14:26Z) 增加 catalog 编辑逻辑测试与关键 UI 文案/说明更新（docs 已更新，已运行 go test ./internal/infra/catalog）。

## Surprises & Discoveries

- Observation: None yet.
  Evidence: N/A.

## Decision Log

- Decision: 使用 `disabled: true` 作为 server 开关字段。
  Rationale: 保留 server 配置但不参与运行时装载，便于快速恢复。
  Date/Author: 2025-12-29 / Codex.

- Decision: 所有编辑均写入 profile store 文件并提示手动重启 Core 生效，不做热更新。
  Rationale: 当前 Core 只在启动时加载配置，避免引入运行时重编排复杂度。
  Date/Author: 2025-12-29 / Codex.

- Decision: 运行时只加载启用的 server，UI 仍展示禁用项并允许恢复。
  Rationale: UI 需要可见禁用项，但运行时需保持干净的 spec 集合。
  Date/Author: 2025-12-29 / Codex.

## Outcomes & Retrospective

- Pending. This section will be updated after implementation milestones complete.

## Context and Orientation

配置编辑主要涉及 `internal/infra/catalog`（读写 profile store 文件）、`internal/ui/service.go`（Wails 服务接口）、`frontend/src/modules/config`（配置页面 UI）。profile store 结构为目录：`profiles/*.yaml` + `callers.yaml`。当前 loader 会将 `profiles/*.yaml` 解析为 `domain.Profile`，并在 Core 启动时构建运行时索引；UI 通过 `WailsService.ListProfiles` 与 `GetProfile` 展示配置。

“Server 开关”指保留配置但不参与运行时加载；“caller 映射”指 `callers.yaml` 中 caller -> profile 映射关系。

## Plan of Work

先在 domain 与 catalog 层加入 `disabled` 字段与编辑能力，再在 WailsService 暴露接口，最后补齐 UI 行为。运行时装载时只处理 enabled servers，但 UI 使用 profile store 里的原始列表展示 disabled 状态。

具体实现上，在 `internal/app/buildProfileSummary` 阶段对 server 进行过滤，以免禁用项进入 scheduler 与 tool index；而 `profile store` 仍保留全部 servers 供 UI 使用。编辑操作通过新的 catalog helper 读写 YAML 文档，不依赖 Core 运行时。

## Concrete Steps

1) Domain & Loader

- 在 `internal/domain/types.go` 的 `ServerSpec` 增加 `Disabled bool` 字段（json tag 为 `disabled,omitempty`）。
- 更新 `internal/infra/catalog/schema.json`，在 serverSpec 中新增 `disabled` boolean。
- 在 `internal/app/app.go` 的 `buildProfileSummary` 里过滤 `Disabled` servers，仅将启用项放入 runtime profiles/spec registry；保留原 profile store 供 UI 展示。

2) Catalog 编辑能力

- 在 `internal/infra/catalog` 新增编辑函数：
  - `SetServerDisabled(path, serverName string, disabled bool) (ProfileUpdate, error)`
  - `DeleteServer(path, serverName string) (ProfileUpdate, error)`
  - `CreateProfile(storePath, name string) (string, error)`
  - `DeleteProfile(storePath, name string) error`
  - `SetCallerMapping(storePath, caller, profile string, profiles map[string]domain.Profile) (ProfileUpdate, error)`
  - `RemoveCallerMapping(storePath, caller string) (ProfileUpdate, error)`
- 编辑函数读写 YAML 时保留 runtime 配置与其它 servers，不丢字段（但会重新格式化 YAML）。

3) WailsService API

- 在 `internal/ui/types.go` 增加请求类型：
  - `UpdateServerStateRequest` (`Profile`, `Server`, `Disabled`)
  - `DeleteServerRequest` (`Profile`, `Server`)
  - `CreateProfileRequest` (`Name`)
  - `DeleteProfileRequest` (`Name`)
  - `UpdateCallerMappingRequest` (`Caller`, `Profile`)
- 在 `internal/ui/service.go` 增加对应方法并进行最小校验：
  - `SetServerDisabled(ctx, req)`
  - `DeleteServer(ctx, req)`
  - `CreateProfile(ctx, req)`
  - `DeleteProfile(ctx, req)`
  - `SetCallerMapping(ctx, req)`
  - `RemoveCallerMapping(ctx, caller)`
- 所有写操作在 `configMode.IsWritable` 为 true 时才执行。

4) Frontend UI

- `frontend/src/modules/config/components/profile-detail-panel.tsx`：
  - 在 server 行添加 Switch（Enabled/Disabled）与 Delete 按钮。
  - 切换与删除调用对应 WailsService 方法，成功后刷新 profile 与 profiles 列表。
- `frontend/src/modules/config/components/profiles-list.tsx`：
  - 顶部增加 “New profile” 入口（Dialog + Input）。
  - 删除 profile 放在 profile 详情面板的 Header 区域（默认 profile 禁止删除）。
- `frontend/src/modules/config/components/callers-list.tsx`：
  - 允许编辑 profile 选择（Select），并提供删除映射按钮。
  - 增加添加 mapping 的表单（caller 输入 + profile 选择）。
- 所有保存成功后提示“Restart Core to apply changes”。

5) Tests & Docs

- 在 `internal/infra/catalog` 新增测试覆盖：
  - disabled toggle 写回
  - delete server
  - create/delete profile
  - caller mapping update/remove
- 更新 `docs/CONFIG_VISUALIZATION_DESIGN.md` 说明编辑能力与重启限制。

## Validation and Acceptance

- 运行 `go test ./internal/infra/catalog`，新增测试全部通过。
- 手动验证：创建 profile、添加/修改/删除 caller 映射、删除 server、关闭/开启 server；确认对应 YAML 发生变化且 UI 提示需重启。重启 Core 后运行时仅包含 enabled servers。

## Idempotence and Recovery

- 创建/删除 profile 与 caller 更新均可重复执行，不会破坏其它 profile。
- 若写入失败，文件保持原状，UI 显示错误并可重试。
- 默认 profile 不可删除，避免系统进入无默认配置状态。

## Artifacts and Notes

- Example disabled server YAML snippet:

    servers:
      - name: weather
        cmd: ["node", "./weather-demo-mcp/build/index.js"]
        idleSeconds: 60
        maxConcurrent: 1
        strategy: stateless
        sessionTTLSeconds: 0
        minReady: 0
        protocolVersion: "2025-11-25"
        disabled: true

## Interfaces and Dependencies

- Go: `domain.ServerSpec` includes `Disabled bool`.
- Go: Catalog editing helpers in `internal/infra/catalog` handle read/modify/write without losing unrelated fields.
- Go: WailsService exports mutation methods listed above.
- TS: UI uses generated bindings in `@bindings/mcpv/internal/ui/wailsservice` for all mutations.

---

Plan Update Note (2025-12-29T14:26Z): 更新 Progress 以标记文档与测试已完成，并记录 go test ./internal/infra/catalog 通过。
