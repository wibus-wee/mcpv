# 降低聚合刷新冷启动与调度器状态竞争

本 ExecPlan 为可演进文档，必须在实施过程中持续更新 `Progress`、`Surprises & Discoveries`、`Decision Log`、`Outcomes & Retrospective`。本文件需遵循仓库根目录 `.agent/PLANS.md` 的要求。

## Purpose / Big Picture

目标是让工具/资源/提示词的聚合刷新不再触发冷启动，并限制并发刷新带来的启动毛刺；同时强化调度器在 StartInstance 与 StopSpec 并发时的状态原子性，避免“停止后复活”的实例写回。完成后，`tools/list`、`resources/list`、`prompts/list` 的刷新只会复用已就绪实例，刷新并发可控；调度器能在 StopSpec 期间安全丢弃 in-flight 启动实例。验证方式是运行现有单测并新增覆盖路径，确认刷新过程不会触发新实例启动，且 StopSpec 并发时实例不会被错误地加入池。

## Progress

- [x] (2025-12-30 10:20Z) 创建 ExecPlan 文档，明确刷新策略与调度器原子性目标。
- [x] (2025-12-30 11:10Z) 实现路由层的“禁止冷启动”选项，并在聚合刷新中启用。
- [x] (2025-12-30 11:10Z) 为工具/资源/提示词刷新引入并发上限，更新配置与示例。
- [x] (2025-12-30 11:10Z) 调整调度器 StartInstance 流程与 StopSpec 竞争处理，补齐测试。
- [x] (2025-12-30 11:10Z) 运行 gofmt 与相关测试，记录验证结果。

## Surprises & Discoveries

暂无。实现过程中记录任何意外行为或回滚原因。

## Decision Log

- Decision: 聚合刷新不再触发冷启动，仅复用已就绪实例。
  Rationale: 轮询刷新在 server 数量大且启动慢时会放大启动毛刺，将刷新与启动解耦可显著降低峰值成本。
  Date/Author: 2025-12-30 / Codex.

- Decision: 新增 `toolRefreshConcurrency` 作为刷新并发上限，并提供默认值。
  Rationale: 控制并发刷新可避免瞬时并发启动，同时允许用户根据规模调优。
  Date/Author: 2025-12-30 / Codex.

- Decision: 调度器引入 generation 机制以保证 StopSpec 与 StartInstance 的原子性。
  Rationale: 防止 StopSpec 期间的 in-flight 启动实例被写回池，确保状态转换可验证。
  Date/Author: 2025-12-30 / Codex.

## Outcomes & Retrospective

尚未完成。实现后补充成果、遗留项与经验。

## Context and Orientation

聚合刷新目前由 `internal/infra/aggregator/aggregator.go`、`internal/infra/aggregator/resource_index.go`、`internal/infra/aggregator/prompt_index.go` 分别负责，它们在 `refresh` 中对所有 serverType 并发发起 `tools/list`、`resources/list`、`prompts/list`。路由由 `internal/infra/router/router.go` 实现，始终调用 `scheduler.Acquire`，会在无就绪实例时触发 `StartInstance`。调度器实现位于 `internal/infra/scheduler/basic.go`，当前通过 `startCh` 串行化启动，但没有对 StopSpec 并发启动的写回做原子性校验。

本计划将新增路由选项与调度器能力：为路由增加 `RouteWithOptions`，允许 list 刷新禁止冷启动；为调度器增加 `AcquireReady` 并在 StartInstance 写回时校验 pool generation。配置由 `internal/domain/types.go`、`internal/domain/constants.go` 与 `internal/infra/catalog/loader.go` 管理，示例配置在 `docs/catalog.example.yaml`。

## Plan of Work

先在 domain 层定义 `RouteOptions` 与 `toolRefreshConcurrency`，扩展 `RuntimeConfig` 与 catalog loader 的读取、默认值与校验，并同步 `internal/ui` 相关类型与示例配置。随后在 router 层引入 `RouteWithOptions`，通过 `AllowStart` 控制是否允许触发冷启动；在聚合刷新中改用 `RouteWithOptions` 并在 list 刷新时禁止冷启动，遇到“无就绪实例”时跳过更新而不是报错。为聚合刷新引入 worker pool，并发度由 `toolRefreshConcurrency` 控制。最后在 scheduler 层增加 `AcquireReady` 与 pool generation，确保 StopSpec 期间的启动实例不写回池，并补充单测覆盖该场景。

## Concrete Steps

在仓库根目录依次修改以下文件：

    internal/domain/types.go
    internal/domain/constants.go
    internal/infra/catalog/loader.go
    internal/ui/types.go
    internal/ui/service.go
    internal/infra/router/router.go
    internal/infra/aggregator/aggregator.go
    internal/infra/aggregator/resource_index.go
    internal/infra/aggregator/prompt_index.go
    internal/infra/scheduler/basic.go
    docs/catalog.example.yaml
    internal/infra/aggregator/*_test.go
    internal/infra/router/router_test.go
    internal/infra/scheduler/basic_test.go
    internal/infra/catalog/loader_test.go

格式化命令：

    gofmt -w internal/domain/types.go internal/domain/constants.go internal/infra/catalog/loader.go internal/ui/types.go internal/ui/service.go internal/infra/router/router.go internal/infra/aggregator/aggregator.go internal/infra/aggregator/resource_index.go internal/infra/aggregator/prompt_index.go internal/infra/scheduler/basic.go

测试命令（按需拆分执行）：

    go test ./internal/infra/aggregator ./internal/infra/router ./internal/infra/scheduler ./internal/infra/catalog

## Validation and Acceptance

验收标准：

1. `tools/list`/`resources/list`/`prompts/list` 的刷新在无就绪实例时不会触发冷启动，并且不会出现大量并发启动。
2. StopSpec 与 StartInstance 并发时，StartInstance 返回的实例不会被写回池。
3. 相关单测通过，且新增用例能在旧实现中失败、在新实现中通过。

## Idempotence and Recovery

改动为纯代码与配置字段扩展，可重复执行。不涉及持久化格式迁移。若回滚，可移除 `RouteWithOptions`、`AcquireReady` 与 generation 校验，并恢复原有刷新并发逻辑。

## Artifacts and Notes

实现过程中记录关键 diff 与测试输出片段（如新增的 StopSpec 并发测试）。

## Interfaces and Dependencies

新增或调整的接口与类型必须在以下位置存在：

- `internal/domain/types.go`:

  - `type RouteOptions struct { AllowStart bool }`
  - `type RuntimeConfig struct { ToolRefreshConcurrency int }`
  - `var ErrNoReadyInstance = errors.New("no ready instance")`

- `internal/domain/constants.go`:

  - `const DefaultToolRefreshConcurrency = 4`

- `internal/domain/types.go` 的 `Scheduler` 接口新增：

  - `AcquireReady(ctx context.Context, specKey, routingKey string) (*Instance, error)`

- `internal/domain/types.go` 的 `Router` 接口新增：

  - `RouteWithOptions(ctx context.Context, serverType, specKey, routingKey string, payload json.RawMessage, opts RouteOptions) (json.RawMessage, error)`

Plan Update Note: Initial creation of aggregation refresh and scheduler atomicity ExecPlan (2025-12-30 10:20Z).
Plan Update Note: Marked refresh policy, scheduler changes, and test validation as complete after implementing RouteWithOptions, refresh worker limits, and generation checks (2025-12-30 11:10Z).
