# ControlPlane Discovery 拆分为独立服务

这是一个可执行计划（ExecPlan），它是一个活文档。必须在执行过程中持续更新 `Progress`、`Surprises & Discoveries`、`Decision Log` 和 `Outcomes & Retrospective`。

仓库内存在 `.agent/PLANS.md`，本计划必须符合其要求并持续维护。

## Purpose / Big Picture

完成本计划后，ControlPlane 不再依赖单一的 `DiscoveryService`。工具、资源、提示将分别由 `ToolDiscoveryService`、`ResourceDiscoveryService`、`PromptDiscoveryService` 负责；共享的可见性解析与过滤逻辑被集中到一个支持组件中。这样可以消除单一职责违背的问题，降低耦合并提升可维护性，同时对外 API 行为保持不变。验证方式是运行 controlplane 包测试并观察全部通过，且工具/资源/提示的 list/watch/call 行为与此前一致。

## Progress

- [x] (2026-02-07 23:05Z) 阅读 `internal/app/controlplane/discovery.go` 与其调用链，确认职责拆分边界与依赖点。
- [x] (2026-02-07 23:25Z) 新增 Tool/Resource/Prompt discovery 服务与共享支持组件。
- [x] (2026-02-07 23:30Z) 调整 ControlPlane、Automation、Wire 与测试以使用新服务。
- [x] (2026-02-07 23:35Z) 运行 `go test ./internal/app/controlplane` 验证行为稳定。

## Surprises & Discoveries

暂无。

## Decision Log

- Decision: 采用方案 2，删除单一 `DiscoveryService`，由 ControlPlane 直接依赖三个独立 discovery 服务，并引入共享支持组件。
  Rationale: 最符合 SRP 与长期演进，避免 facade 遮掩真实耦合。
  Date/Author: 2026-02-07 / Codex

## Outcomes & Retrospective

- ControlPlane 已拆分为 Tool/Resource/Prompt 三个独立 discovery 服务，shared support 统一封装可见性与缓存逻辑。
- ControlPlane/Automation/Wire/测试已完成依赖替换，`go test ./internal/app/controlplane` 通过。

## Context and Orientation

当前 discovery 逻辑集中在 `internal/app/controlplane/discovery.go`，负责 tools/resources/prompts 的 list/watch/resolve 以及缓存与过滤。ControlPlane 位于 `internal/app/controlplane/controlplane.go`，通过 `DiscoveryService` 暴露 `domain.DiscoveryAPI` 接口；Automation 位于 `internal/app/controlplane/automation.go`，依赖 `DiscoveryService` 来获取工具列表与执行工具。依赖注入位于 `internal/app/wire_sets.go` 与生成文件 `internal/app/wire_gen.go`。

“Discovery” 指在控制面里为客户端提供工具、资源、提示的可见列表与访问入口。“可见性”由客户端标签/指定服务和 specKey 决定。

## Plan of Work

先抽出共享支持组件 `discoverySupport`，集中处理：客户端服务器解析、可见 specKeys 解析、可见性过滤帮助函数、SpecKey 集合构建与 MetadataCache 访问。然后分别创建三个服务：`ToolDiscoveryService`、`ResourceDiscoveryService`、`PromptDiscoveryService`，把原 `DiscoveryService` 中对应的方法迁移进各自服务，保留原有行为与错误返回。最后删除 `DiscoveryService` 类型，并在 `ControlPlane`、`AutomationService`、Wire 以及测试中替换为新服务依赖。必要时更新 `wire_gen.go`（优先通过 `make wire` 生成）。

## Concrete Steps

在仓库根目录执行以下步骤：

1) 新增共享支持与分页工具。

   - 文件：`internal/app/controlplane/discovery_support.go`
     - 定义 `type discoverySupport struct { state *State; registry *ClientRegistry }`
     - 提供方法：`resolveClientServer`、`resolveVisibleSpecKeys`、`visibleServers`、`isServerVisible`、`metadataCache`、`toSpecKeySet`。
   - 文件：`internal/app/controlplane/discovery_pagination.go`
     - 放置 `snapshotPageSize` 常量与 `paginateResources`、`paginatePrompts`、`indexAfterResourceCursor`、`indexAfterPromptCursor`。

2) 拆出工具服务。

   - 文件：`internal/app/controlplane/discovery_tools.go`
     - 定义 `ToolDiscoveryService` 与构造函数 `NewToolDiscoveryService(state *State, registry *ClientRegistry) *ToolDiscoveryService`。
     - 迁移 `ListTools`、`ListToolCatalog`、`WatchTools`、`CallTool`、`CallToolAll`、`GetToolSnapshotForClient`、`filterToolSnapshot`、`cachedToolSnapshotForServer` 以及 tool catalog 构建函数。

3) 拆出资源服务。

   - 文件：`internal/app/controlplane/discovery_resources.go`
     - 定义 `ResourceDiscoveryService` 与构造函数。
     - 迁移 `ListResources`、`ListResourcesAll`、`WatchResources`、`ReadResource`、`ReadResourceAll`、`filterResourceSnapshot` 与 `closedResourceSnapshotChannel`。

4) 拆出提示服务。

   - 文件：`internal/app/controlplane/discovery_prompts.go`
     - 定义 `PromptDiscoveryService` 与构造函数。
     - 迁移 `ListPrompts`、`ListPromptsAll`、`WatchPrompts`、`GetPrompt`、`GetPromptAll`、`filterPromptSnapshot` 与 `closedPromptSnapshotChannel`。

5) 清理旧实现并调整依赖。

   - 删除 `internal/app/controlplane/discovery.go` 中 `DiscoveryService` 类型与相关函数，保留迁移后的共享函数在新文件中。
   - `internal/app/controlplane/controlplane.go`：替换字段与构造函数参数，方法委派到三个新服务。
   - `internal/app/controlplane/automation.go`：依赖 `ToolDiscoveryService`，使用其 `ListTools`/`CallTool`。
   - `internal/app/wire_sets.go`：替换 provider 为新服务构造函数。
   - `internal/app/wire_gen.go`：通过 `make wire` 或手动同步更新。
   - `internal/app/controlplane/controlplane_test.go`：使用新服务构造 ControlPlane。

6) 运行 gofmt 与测试。

## Validation and Acceptance

运行 controlplane 包测试并确保通过：

    cd /Users/wibus/dev/mcpd
    go test ./internal/app/controlplane

期望结果：测试全部通过。工具/资源/提示 list/watch/call 行为与重构前保持一致。若需要更强验证，再运行 `go test ./...`。

## Idempotence and Recovery

重构是纯代码改动，可重复执行 gofmt 与测试而不产生副作用。若出现编译或行为差异，优先检查 ControlPlane/Automation 的依赖是否完整替换，以及 `wire_gen.go` 是否与 `wire_sets.go` 同步。

## Artifacts and Notes

关键新增文件预期包括：

    internal/app/controlplane/discovery_support.go
    internal/app/controlplane/discovery_pagination.go
    internal/app/controlplane/discovery_tools.go
    internal/app/controlplane/discovery_resources.go
    internal/app/controlplane/discovery_prompts.go

## Interfaces and Dependencies

- 新服务构造函数：

    NewToolDiscoveryService(state *State, registry *ClientRegistry) *ToolDiscoveryService
    NewResourceDiscoveryService(state *State, registry *ClientRegistry) *ResourceDiscoveryService
    NewPromptDiscoveryService(state *State, registry *ClientRegistry) *PromptDiscoveryService

- ControlPlane 构造函数签名更新为接受三个服务。
- AutomationService 依赖 `ToolDiscoveryService`，不再依赖 DiscoveryService。
- `domain.DiscoveryAPI` 保持不变，由 ControlPlane 继续实现。

Plan change note: 2026-02-07 / Codex - 创建本 ExecPlan，准备按方案 2 实施。
Plan change note: 2026-02-07 / Codex - 完成拆分与依赖更新，并记录测试结果。
