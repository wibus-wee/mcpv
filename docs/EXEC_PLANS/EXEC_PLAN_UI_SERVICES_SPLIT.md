# Split Wails UI Services

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

Follow `.agent/PLANS.md` from the repository root while maintaining this plan.

## Purpose / Big Picture

将 `internal/ui/service.go` 拆分为多个职责单一的 Wails service，并在 `app.go` 中注册多个 service。前端 bindings 与调用点更新为新的 service 名称。完成后，UI 服务层的改动将更可维护，单文件规模显著降低，并且前后端 API 边界更清晰。验证方式：生成/使用新的 bindings，前端调用正常；运行测试或启动应用能正常启动并可调用核心 API。

## Progress

- [x] (2026-01-14 05:57Z) 创建 ExecPlan 文档并补充上下文与计划。
- [x] (2026-01-14 06:30Z) 拆分 `internal/ui/service.go` 为多个 service 文件并共享依赖。
- [x] (2026-01-14 06:30Z) 更新 `app.go` 注册多个 service 并调整初始化流程。
- [x] (2026-01-14 06:30Z) 更新前端 bindings 与调用点引用新的 service。
- [x] (2026-01-14 06:30Z) 更新相关文档说明与校验步骤。
- [x] (2026-01-14 06:30Z) 验证与收尾：执行 `go test ./internal/ui`。
- [x] (2026-01-14 06:32Z) 生成 Wails bindings：执行 `make wails-bindings`。

## Surprises & Discoveries

- Observation: `go test ./internal/ui` 链接阶段出现 macOS 版本警告，但测试通过。
  Evidence: `ld: warning: object file ... was built for newer 'macOS' version (26.0) than being linked (11.0)`
- Observation: `make wails-bindings` 使用 wails3 (Go 1.24) 产生 go1.25 版本警告，但生成成功。
  Evidence: `package requires newer Go version go1.25 (application built with go1.24)`

## Decision Log

- Decision: 采用多 service 拆分（Core/Discovery/Config/Profile/Runtime/Log/SubAgent/System/Debug）并共享一个依赖容器。
  Rationale: 减少单文件复杂度，保持清晰的职责边界，同时复用 `Manager`、logger 与 Wails app 实例。
  Date/Author: 2026-01-14 / Codex

## Outcomes & Retrospective

已完成多 service 拆分、前端调用更新与文档同步，`internal/ui` 单测通过，已生成最新 bindings。仍建议运行 `pnpm -C frontend typecheck` 验证前端类型。

## Context and Orientation

当前 UI 入口服务为 `internal/ui/service.go`，其中包含了 Wails 前端所需的所有导出方法，文件规模约 965 行，耦合度高。前端 bindings 位于 `frontend/bindings/mcpd/internal/ui/`，并通过 `@bindings/mcpd/internal/ui` 在前端使用。Wails app 入口在仓库根目录的 `app.go`，当前仅注册单个服务。目标是利用 Wails 支持多 service 的能力，将 UI 服务拆分为多个职责明确的服务类型，同时保持 `Manager` 生命周期与事件派发逻辑不变。

## Plan of Work

首先在 `internal/ui` 下引入共享依赖容器（例如 `ServiceDeps`），负责持有 `Manager`、logger 与 Wails app 引用，并提供 `getControlPlane`、`catalogEditor` 等通用帮助函数。随后将原 `service.go` 中的方法按职责拆分到多个新 service 文件（System/Core/Discovery/Log/Config/Profile/Runtime/SubAgent/Debug）。每个 service 仅持有依赖容器与自身需要的状态（例如日志流相关字段）。

接着在 `internal/ui/service.go` 中实现 service registry，用于集中创建各 service 实例并返回 `[]application.Service`，供 `app.go` 注册。`app.go` 调整为使用 registry 创建服务并注入 `Manager` 与 Wails app。

随后更新前端 bindings：新增对应服务的 binding 文件，并更新 `frontend/bindings/mcpd/internal/ui/index.ts` 导出新的 service。前端代码中将 `WailsService` 的调用点替换为对应 service（例如 `CoreService`, `ConfigService`, `ProfileService`, `RuntimeService` 等）。

最后更新文档（`docs/WAILS_BINDINGS.md`, `docs/CONFIG_VISUALIZATION_DESIGN.md`, `cmd/mcpd-wails/README.md`）以反映多 service 注册与新的 API 入口，并记录验证步骤。

## Concrete Steps

在仓库根目录执行以下步骤：

1) 新增 ExecPlan 文件并填充内容。
2) 读取 `internal/ui/service.go`，按职责拆分为多个文件，新增 `internal/ui/service_deps.go` 并将 registry 保持在 `internal/ui/service.go`。
3) 更新 `app.go` 使用 registry 注册多个 service。
4) 更新 `frontend/bindings/mcpd/internal/ui/` 目录下的 bindings 文件与 `index.ts`。
5) 更新前端 `frontend/src` 内所有 `WailsService` 调用点。
6) 更新文档与说明。

示例命令（不在此处实际执行）：

    cd /Users/wibus/conductor/workspaces/mcpd/tianjin
    make wails-bindings
    make test
    pnpm -C frontend typecheck

## Validation and Acceptance

- 生成新的 bindings 后，前端可通过 `CoreService`, `ConfigService`, `ProfileService` 等模块成功调用对应方法。
- `make test` 通过，且 `internal/ui` 的单测可编译运行。
- 运行 Wails 应用后，基础功能（核心启动、配置读取、工具列表、日志流）不报错。

## Idempotence and Recovery

拆分操作可重复执行；若某个步骤失败，可回滚对应文件修改并重新应用。绑定文件与前端调用更新需要成对推进，避免出现旧服务名与新服务名混用导致的编译错误。

## Artifacts and Notes

关键文件与目录：
- `internal/ui/*.go`
- `app.go`
- `frontend/bindings/mcpd/internal/ui/`
- `frontend/src/**`
- `docs/WAILS_BINDINGS.md`

## Interfaces and Dependencies

需要新增以下服务类型（均在 `internal/ui` 包内）：

- `type SystemService struct { ... }`
- `type CoreService struct { ... }`
- `type DiscoveryService struct { ... }`
- `type LogService struct { ... }`
- `type ConfigService struct { ... }`
- `type ProfileService struct { ... }`
- `type RuntimeService struct { ... }`
- `type SubAgentService struct { ... }`
- `type DebugService struct { ... }`

并新增一个依赖容器：

- `type ServiceDeps struct { ... }`
- `func NewServiceRegistry(coreApp *app.App, logger *zap.Logger) *ServiceRegistry`
- `func (r *ServiceRegistry) Services() []application.Service`

每个 service 暴露的方法应保持与原 `WailsService` 中对应方法一致（签名与语义不变），仅更换 service 名称以减少耦合。

Plan Change Note (2026-01-14 06:30Z): 更新 Progress/Outcomes 以反映已完成的拆分与验证，记录 `go test` 的 macOS 链接警告，并同步将 registry 落点调整为 `internal/ui/service.go` 以匹配实际实现。
Plan Change Note (2026-01-14 06:32Z): 记录执行 `make wails-bindings` 并补充对应警告说明。
