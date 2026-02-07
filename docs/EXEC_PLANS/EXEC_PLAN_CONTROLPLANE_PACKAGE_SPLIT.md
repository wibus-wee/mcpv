# Controlplane Package Split

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document follows `.agent/PLANS.md` from the repository root and must be maintained according to it.

## Purpose / Big Picture

将 `internal/app/controlplane` 拆分为更小的子包（registry、discovery、observability、automation），保留 `controlplane` 作为对外 facade。这样可以减少单文件体积、降低职责耦合，便于后续演进与测试隔离。该重构不改变对外行为，验证方式是控制面相关测试继续通过，以及 `internal/app` 级别编译与测试通过。

## Progress

- [x] (2026-02-07 14:10) 创建子包目录并迁移 `controlplane` 内服务文件，修正 package 名称与 import。
- [x] (2026-02-07 14:20) 为子包添加最小化 State 接口，扩展 `internal/app/controlplane/state.go` 的 getter，增加 `internal/app/controlplane/services.go` 作为兼容 wrapper。
- [x] (2026-02-07 14:35) 迁移并补齐测试：registry 行为测试、discovery 分页测试；修复控制面测试对内部字段的直接访问。
- [x] (2026-02-07 14:45) 运行 `gofmt`，并通过 `go test ./internal/app/controlplane/...` 与 `go test ./internal/app/...`。
- [ ] 如果 `internal/app/wire_gen.go` 因依赖图变更而失效，运行 `make wire` 并确认编译通过（由使用者执行）。

## Surprises & Discoveries

- 发现 `internal/app/controlplane/reload_test.go` 直接访问 `registry.specCounts`，拆包后不可见。
  Evidence: `go test ./internal/app/controlplane/...` 报错 `registry.specCounts undefined`。
- 发现 registry 测试中使用不存在的 `domain.NoopInitManager`。
  Evidence: 编译提示未定义类型，改为返回 `*bootstrap.ServerInitializationManager` 的空实现。

## Decision Log

- Decision: 将 `controlplane` 拆为 `registry`、`discovery`、`observability`、`automation` 四个子包，并保留 `controlplane.ControlPlane` facade。
  Rationale: 子包边界对应职责域，减少跨职责修改的触碰面；Facade 保持外部调用路径稳定。
  Date/Author: 2026-02-07 / Codex
- Decision: 在 `internal/app/controlplane/services.go` 中提供 type alias 与构造函数 wrapper。
  Rationale: 最小化对 Wire 注入与上层调用的侵入式修改。
  Date/Author: 2026-02-07 / Codex
- Decision: 不新增公开的 `SpecCounts` 读取接口，而是通过 `ResolveVisibleSpecKeys` 验证行为。
  Rationale: 保持 registry 内部状态封装，避免增加非必要 API 面。
  Date/Author: 2026-02-07 / Codex

## Outcomes & Retrospective

拆包后控制面职责更清晰，子包内部依赖更单一，测试仍通过。当前没有发现行为差异。若后续 Wire 注入出现构建失败，可通过运行 `make wire` 修复依赖生成文件。

## Context and Orientation

当前仓库中控制面逻辑集中在 `internal/app/controlplane`，包含客户端注册、工具/资源/提示词发现、日志与运行时状态观测、自动化工具筛选等职责。此重构将其拆分为子包：

- `internal/app/controlplane/registry`: 客户端注册、可见性解析、Caller probe、激活/释放逻辑。
- `internal/app/controlplane/discovery`: tool/resource/prompt 的 list/watch、分页与缓存读取。
- `internal/app/controlplane/observability`: 日志流、运行时状态、初始化状态。
- `internal/app/controlplane/automation`: 自动 MCP 逻辑与工具筛选。

`internal/app/controlplane` 仍保留 `ControlPlane` facade 与 `State`，并通过 `services.go` 暴露兼容构造函数，避免上层调用迁移。

## Plan of Work

首先将原 `controlplane` 中四类职责文件移动到对应子包，并修正每个文件的 package 名称与 import 引用。为每个子包补齐一个最小的 `State` 接口，避免循环依赖。其次更新 `internal/app/controlplane/state.go`，提供子包所需的 getter，并在 `services.go` 中提供 type alias 与构造函数 wrapper，让 Wire 与既有调用点继续使用 `controlplane.NewXxxService`。然后修复测试：移除对 registry 内部字段的直接访问，改为使用公开方法验证行为；补齐 discovery 分页逻辑的单测。最后运行 gofmt 与 go test 验证。

## Concrete Steps

在仓库根目录执行以下命令：

1) 格式化新增或迁移后的文件：

   gofmt -w internal/app/controlplane/registry/*.go
   gofmt -w internal/app/controlplane/discovery/*.go
   gofmt -w internal/app/controlplane/observability/*.go
   gofmt -w internal/app/controlplane/automation/*.go
   gofmt -w internal/app/controlplane/*.go

2) 运行控制面测试：

   go test ./internal/app/controlplane/...

   预期输出包含：

     ok   mcpv/internal/app/controlplane
     ok   mcpv/internal/app/controlplane/registry
     ok   mcpv/internal/app/controlplane/discovery

3) 验证 `internal/app` 级别构建：

   go test ./internal/app/...

4) 如果出现 Wire 生成相关错误，再执行：

   make wire

## Validation and Acceptance

- 运行 `go test ./internal/app/controlplane/...` 与 `go test ./internal/app/...`，应全部通过。
- 关键验收点：控制面功能对外 API 不变；registry/discovery/observability/automation 子包各自可独立编译；分页测试验证 resource/prompt 游标逻辑无回归。

## Idempotence and Recovery

- `gofmt` 与 `go test` 可重复执行，无副作用。
- 若需要回滚拆包，可使用 `git status` 确认变更范围，再用 `git restore` 回退对应文件。重跑测试确认恢复到原有行为。

## Artifacts and Notes

- 新增或迁移后的关键路径：

  internal/app/controlplane/registry/
  internal/app/controlplane/discovery/
  internal/app/controlplane/observability/
  internal/app/controlplane/automation/
  internal/app/controlplane/services.go

- 近期测试输出示例：

  ok   mcpv/internal/app/controlplane
  ok   mcpv/internal/app/controlplane/registry
  ok   mcpv/internal/app/controlplane/discovery
  ok   mcpv/internal/app/bootstrap

## Interfaces and Dependencies

- `internal/app/controlplane/state.go` 需提供：

  - `Scheduler() domain.Scheduler`
  - `InitManager() *bootstrap.ServerInitializationManager`
  - `BootstrapManager() *bootstrap.Manager`
  - `Logger() *zap.Logger`
  - `Context() context.Context`

- 子包 State 接口：

  - `internal/app/controlplane/registry/state.go`：用于 client registry 与 visibility。
  - `internal/app/controlplane/discovery/state.go`：用于 tool/resource/prompt discovery。
  - `internal/app/controlplane/observability/state.go`：用于日志与运行时/初始化状态。
  - `internal/app/controlplane/automation/state.go`：用于自动 MCP 流程。

- 兼容 wrapper：

  `internal/app/controlplane/services.go` 提供 `NewClientRegistry`、`NewToolDiscoveryService`、`NewResourceDiscoveryService`、`NewPromptDiscoveryService`、`NewObservabilityService`、`NewAutomationService` 的构造函数和 type alias，供 Wire 与上层继续使用 `controlplane` 包。

---

Change Log: Created initial ExecPlan for controlplane package split after completing the refactor so the plan reflects the current state and the remaining optional Wire regeneration step.
