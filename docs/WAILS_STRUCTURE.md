# Wails App 工程结构建议

## 目标

- 保持单一产品形态，Wails 仅作为可视化入口
- 复用现有核心编排，不引入跨模块依赖
- 结构清晰、默认约定优先，降低配置成本

## 原则

- Wails 入口与 CLI 入口并列，职责清晰
- 核心逻辑仍集中在 `internal/app` 与 `internal/domain`
- Wails 适配层收敛在 `internal/ui`，只做桥接与组合
- 前端目录放在仓库根，遵循 Wails 默认结构
- URL scheme 在 Wails 入口注册并由事件回调处理，避免在核心层处理外部唤起

## 推荐目录结构

```
cmd/
  mcpd/                     # CLI entry
  mcpd-wails/               # Wails entry
internal/
  app/                      # App orchestration
  domain/                   # Core domain models
  infra/                    # Adapters and implementations
  ui/                       # Wails bridge layer
pkg/
  api/                      # Optional public shared types
frontend/                   # Wails frontend (web UI)
build/                      # Wails build config and assets
configs/                    # Optional runtime configs (non-secret)
docs/
  STRUCTURE.md              # Current structure
  WAILS_STRUCTURE.md        # Wails structure guide
Makefile
README.md
go.mod
```

## 目录职责说明

- `cmd/mcpd`：CLI 入口，负责参数解析、日志与配置加载。
- `cmd/mcpd-wails`：Wails 入口，负责应用启动、生命周期管理与 UI 绑定注册。
- `internal/app`：核心编排层，供 CLI/Wails 复用，不直接依赖 UI。
- `internal/ui`：Wails 适配层，负责把 `internal/app` 封装成可绑定服务与事件流。
- `frontend`：前端 UI 与构建配置，建议与 Wails 官方默认保持一致。
- `build`：Wails 打包配置、图标与平台相关资源。
- `pkg/api`：仅用于对外复用的公共类型，内部使用不强求通过 `pkg`。

## Wails 适配层建议

- `internal/ui` 只做边界适配，避免业务逻辑下沉。
- 前端通过绑定服务调用核心能力，事件流由 `internal/ui` 统一发布。
- 如果需要新增面向 UI 的数据模型，优先放在 `internal/ui`，不要污染 `internal/domain`。

## 入口职责建议

- `cmd/mcpd-wails` 只做启动与依赖注入，核心流程仍在 `internal/app`。
- 不在入口层堆积业务逻辑，避免未来 CLI 与 Wails 行为分叉。

## URL Scheme 约定

- 在 Wails 应用层通过 `application.Options.Protocols` 注册 scheme
- 通过 `application.Events.ApplicationOpenedWithURL` 接收唤起参数
- 入口层只做解析与转发，业务处理仍交给 `internal/ui`
