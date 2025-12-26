# Refactor lock contention in tool aggregation and scheduler

This ExecPlan is a living document. The sections Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective must be kept up to date as work proceeds.

This document must be maintained in accordance with .agent/PLANS.md from the repository root.

## Purpose / Big Picture

本次改造的目标是降低工具聚合与调度器的锁竞争，让工具查询与调用在高并发下更稳定，避免长时间持锁阻塞读路径。完成后，`tools/list` 的快照读取与 `tools/call` 的解析将不再依赖全局锁，工具刷新与广播不会在持锁时做大规模内存分配与遍历，调度器在多 serverType 并发时不会被单一全局锁拖慢。验证方式是运行现有测试并观察核心读路径不再持有大锁，同时行为与测试结果保持一致。

## Progress

- [x] (2025-12-26 02:40Z) 梳理 ToolIndex 快照与订阅流程，确认锁边界与风险点。
- [x] (2025-12-26 02:40Z) 将 ToolIndex 的快照与目标索引迁移到原子状态容器，重建快照与广播改为锁外执行。
- [x] (2025-12-26 02:40Z) 将 BasicScheduler 切换为按 serverType 的局部状态锁，调整空闲回收与探测逻辑。
- [ ] (2025-12-26 02:40Z) 更新 scheduler 单测断言与必要的结构访问，执行格式化与测试验证（completed: 更新断言与格式化；remaining: 执行测试验证）。

## Surprises & Discoveries

暂无。该部分会在实现过程中记录任何意外行为或需要回滚的设计判断。

## Decision Log

Decision: 使用 `atomic.Value` 保存 `ToolSnapshot` 与目标索引的不可变组合，并将读路径改为无锁读取。
Rationale: 读路径频率高且对延迟敏感，使用原子快照可避免与刷新/广播争锁，同时保持快照一致性。
Date/Author: 2025-12-26 / Codex.

Decision: 订阅通道在 ctx 结束时仅移除订阅而不主动关闭。
Rationale: 广播改为锁外发送后，关闭通道会与发送产生竞态并触发 panic；订阅者已持有 ctx，可自行退出读取循环。
Date/Author: 2025-12-26 / Codex.

Decision: BasicScheduler 采用按 serverType 的局部状态锁。
Rationale: 调度器的实例列表与 sticky 绑定可以按 serverType 隔离，避免不同服务之间的互斥等待。
Date/Author: 2025-12-26 / Codex.

## Outcomes & Retrospective

尚未完成。完成后记录达成效果、遗留项与经验。

## Context and Orientation

当前工具聚合由 `internal/infra/aggregator/aggregator.go` 中的 `ToolIndex` 管理。它负责从下游 server 拉取工具列表、聚合为 `domain.ToolSnapshot`，并通过订阅通道广播变更。当前实现使用单个 `sync.RWMutex` 保护快照、目标索引、订阅者和刷新状态，导致 `Snapshot`、`Resolve`、`rebuildSnapshot` 与 `broadcastLocked` 在高并发场景下互相阻塞。调度器由 `internal/infra/scheduler/basic.go` 中的 `BasicScheduler` 实现，维护实例列表与 sticky 绑定，同样使用单一 `sync.Mutex` 保护所有 serverType 的状态，跨 serverType 的并发请求会互相等待。

本次重构的核心对象是两个模块：`ToolIndex` 与 `BasicScheduler`。`ToolSnapshot` 是工具视图快照，包含 `ETag` 与工具列表；订阅者通过 `WatchTools` 接收快照更新；调度器用于为路由选择实例，并负责 idle 回收与 ping 探测。

## Plan of Work

先在 `ToolIndex` 中引入一个不可变的状态载体，用 `atomic.Value` 存放 `ToolSnapshot` 与工具名到目标的映射，将 `Snapshot` 与 `Resolve` 改为无锁读取。重建快照时先复制 `serverCache` 再在锁外完成排序、哈希与合并，确认 `ETag` 变化后再一次性替换状态。广播时仅在锁内复制订阅者列表，实际发送在锁外完成，避免持锁遍历。订阅释放时只移除订阅，不关闭通道，避免锁外发送导致的关闭竞态。

随后重构 `BasicScheduler` 的内部结构，引入按 `serverType` 划分的局部状态，确保实例列表与 sticky 绑定只在单 serverType 的互斥锁内更新。`Acquire`、`Release`、`reapIdle`、`probeInstances` 与 `StopAll` 都将改为先获取对应 `serverState`，再在局部锁内完成必要操作。保留对慢操作（启动、停止、ping）的锁外执行，避免阻塞其他 serverType 的调度请求。

最后更新 `internal/infra/scheduler/basic_test.go` 中对内部结构的断言，使其适配新的状态容器。完成后运行格式化与测试命令，确认行为一致。

## Concrete Steps

在仓库根目录执行代码修改，更新 `internal/infra/aggregator/aggregator.go` 与 `internal/infra/scheduler/basic.go`，并同步调整 `internal/infra/scheduler/basic_test.go`。完成代码修改后运行格式化命令：

    gofmt -w internal/infra/aggregator/aggregator.go internal/infra/scheduler/basic.go internal/infra/scheduler/basic_test.go

测试阶段优先运行与本次改动直接相关的用例：

    go test ./internal/infra/aggregator ./internal/infra/scheduler

如环境限制导致 Go build cache 无法写入，设置 `GOCACHE` 到工作区后重试。

## Validation and Acceptance

验收标准是工具聚合与调度器行为保持一致，且 `go test ./internal/infra/aggregator ./internal/infra/scheduler` 通过。`ControlPlane.WatchTools` 的订阅流保持可用，`ToolIndex.Snapshot` 与 `ToolIndex.Resolve` 不再依赖全局锁即可返回结果。调度器在多个 serverType 并发请求下不应共享同一把互斥锁。

## Idempotence and Recovery

修改为纯代码变更，可重复执行，不涉及持久化格式或配置结构变化。若测试失败，可对照本 ExecPlan 的步骤逐一回退到锁内操作，并保留 `ToolIndex` 的原有读写流程作为临时回滚路径。

## Artifacts and Notes

本次变更应仅修改 Go 源码与测试文件，不新增外部依赖。必要时在此记录关键 diff 片段与测试输出。

## Interfaces and Dependencies

在 `internal/infra/aggregator/aggregator.go` 中定义 `toolIndexState`，包含 `snapshot domain.ToolSnapshot` 与 `targets map[string]domain.ToolTarget`。`ToolIndex` 需要新增 `atomic.Value` 字段保存该状态，同时保留现有 `Router`、`RuntimeConfig` 与 `HealthTracker` 的依赖。`BasicScheduler` 需要新增 `serverState` 结构体，包含 `instances []*trackedInstance` 与 `sticky map[string]*trackedInstance`，并使用局部互斥锁保护。

Plan Update Note: Initial creation of the lock contention refactor ExecPlan (2025-12-26 02:33Z).
Plan Update Note: Updated progress and decision log after implementing ToolIndex and BasicScheduler refactors (2025-12-26 02:40Z).
