# RPC 治理逻辑抽离与控制服务拆分重构

这是一个可执行计划（ExecPlan），它是一个活文档。必须在执行过程中持续更新 `Progress`、`Surprises & Discoveries`、`Decision Log` 和 `Outcomes & Retrospective`。

仓库内存在 `.agent/PLANS.md`，本计划必须符合其要求并持续维护。

## Purpose / Big Picture

完成本计划后，RPC 控制服务在处理 tools/resources/prompts/runtime 等列表与订阅请求时，会通过统一的治理“守卫”执行 request decision、response decision 与 proto mutation，重复逻辑被消除。用户不会看到行为变化，但代码结构更清晰，后续新增治理能力时不再需要在每个 RPC 方法里重复粘贴逻辑。验证方式是运行现有 RPC 单测并观察全部通过，同时检查列表/订阅响应仍可被治理插件拦截或改写。

## Progress

- [x] (2026-02-07 22:10Z) 阅读现有 `internal/infra/rpc/control_service.go` 与 `internal/infra/governance/executor.go`，确认重复治理模式与可抽取范围。
- [x] (2026-02-07 22:25Z) 新增治理守卫与辅助函数，并将控制服务拆分为按领域的多个 Go 文件。
- [x] (2026-02-07 22:40Z) 将 tools/resources/prompts/runtime/automatic_mcp 的治理流程迁移到守卫，保持错误消息与行为一致。
- [x] (2026-02-07 22:45Z) 对新文件执行 gofmt。
- [x] (2026-02-07 22:55Z) 运行 `go test ./internal/infra/rpc` 验证重构未破坏行为。

## Surprises & Discoveries

暂无。

## Decision Log

- Decision: 使用 `governanceGuard` 作为 RPC 层的装饰器，提供 `applyRequest` 与 `applyProtoResponse`。
  Rationale: 复用最常见的治理流程，同时保留 `Executor.Execute` 在调用类 RPC 中的专用能力。
  Date/Author: 2026-02-07 / Codex
- Decision: 将 `control_service.go` 拆分为按领域的多文件结构。
  Rationale: 降低单文件复杂度，方便定位与单点修改。
  Date/Author: 2026-02-07 / Codex
- Decision: 保持所有错误消息前缀与 gRPC code 不变。
  Rationale: 避免影响上层调用与测试断言。
  Date/Author: 2026-02-07 / Codex

## Outcomes & Retrospective

- RPC 治理逻辑集中到 `governanceGuard`，list/watch/automatic_mcp 等路径不再重复 request/response 处理。
- 控制服务按领域拆分为多文件，便于维护与扩展。
- `go test ./internal/infra/rpc` 已通过，确认重构未破坏现有行为。

## Context and Orientation

RPC 控制服务位于 `internal/infra/rpc/`，其核心入口是 `ControlService`，负责处理 gRPC 请求并调用 `ControlPlaneAPI`。治理插件的执行引擎是 `internal/infra/governance/executor.go` 中的 `Executor`，它负责在 request/response 两个方向执行插件链并返回决策。列表与订阅类 RPC（如 `ListTools`/`WatchTools`）在原实现中重复执行“请求决策 -> 响应决策 -> proto mutation”的流程，本计划将其抽离到一个统一的“守卫”类型并复用。

“治理”指插件对请求或响应进行允许/拒绝/变更的决策；“proto mutation”指治理插件返回新的 JSON 片段后，系统将其反序列化并覆盖到 proto 消息中。

## Plan of Work

先引入 `governanceGuard`，其职责是：在 request 阶段调用 `Executor.Request` 并可选执行请求参数的变更；在 response 阶段编码 proto 消息、执行 `Executor.Response` 并应用 mutation。随后将 `control_service.go` 拆分为按领域的多个文件（tools/resources/prompts/tasks/runtime 等），并逐个把 List/Watch/AutomaticMCP 的治理流程迁移到守卫中。最后整理 import 并执行 gofmt，确保代码风格一致。

## Concrete Steps

在仓库根目录执行以下步骤：

1) 新增治理守卫与辅助函数。

   - 文件：`internal/infra/rpc/control_service_governance.go`
   - 定义 `governanceGuard`、`applyRequest`、`applyProtoResponse`、`applyProtoMutation` 与 `mustMarshalJSON`。

2) 拆分 `ControlService` 实现为多个文件。

   - `internal/infra/rpc/control_service.go`：结构体与基础 RPC。
   - `internal/infra/rpc/control_service_tools.go`：tools 与 automation 相关 RPC。
   - `internal/infra/rpc/control_service_resources.go`：resources 相关 RPC。
   - `internal/infra/rpc/control_service_prompts.go`：prompts 相关 RPC。
   - `internal/infra/rpc/control_service_tasks.go`：tasks 相关 RPC。
   - `internal/infra/rpc/control_service_runtime.go`：logs/runtime 相关 RPC。
   - `internal/infra/rpc/control_service_errors.go`：错误映射与治理拒绝逻辑。

3) 替换 list/watch/automatic_mcp 的治理逻辑为 `governanceGuard` 调用。

4) 执行 gofmt。

   命令示例：

       cd /Users/wibus/dev/mcpd
       gofmt -w internal/infra/rpc/control_service*.go

## Validation and Acceptance

运行 RPC 包测试并确保通过：

    cd /Users/wibus/dev/mcpd
    go test ./internal/infra/rpc

期望结果：测试全部通过，且列表/订阅类 RPC 的行为未变。若需要更强验证，可再跑 `go test ./...` 观察全仓通过。

## Idempotence and Recovery

所有修改为代码重构与新增文件，重复执行 gofmt 与测试不会产生副作用。如果发现行为变化，优先对比 `governanceGuard` 的错误消息前缀与旧实现是否一致，并检查是否遗漏了某个 list/watch 路径的治理调用。

## Artifacts and Notes

关键文件：

    internal/infra/rpc/control_service_governance.go
    internal/infra/rpc/control_service_tools.go
    internal/infra/rpc/control_service_resources.go
    internal/infra/rpc/control_service_prompts.go
    internal/infra/rpc/control_service_runtime.go

## Interfaces and Dependencies

必须保留以下接口与类型：

- `ControlService` 仍实现 `controlv1.ControlPlaneServiceServer`。
- `governanceGuard` 提供：

    applyRequest(ctx context.Context, req domain.GovernanceRequest, op string, mutate func([]byte) error) error
    applyProtoResponse(ctx context.Context, req domain.GovernanceRequest, op string, target proto.Message) error

- `Executor` 的使用保持不变：调用类 RPC 继续通过 `Executor.Execute` 处理双向治理。

Plan change note: 2026-02-07 / Codex - 创建本 ExecPlan 并在实施后更新 Progress 与 Outcomes，以记录拆分与治理守卫重构的完成情况。
Plan change note: 2026-02-07 / Codex - 补记测试执行结果并更新 Progress/Outcomes，确保计划与执行一致。
