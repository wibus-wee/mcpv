# SubAgent 可观测性与 Caller TTL 收敛

本 ExecPlan 是一个持续更新的活文档，`Progress`、`Surprises & Discoveries`、`Decision Log`、`Outcomes & Retrospective` 四个章节必须在执行过程中保持最新。

本计划遵循仓库根目录的 `.agent/PLANS.md` 约束，且必须随实现进度同步修订。

## Purpose / Big Picture

完成本计划后，SubAgent 的 LLM 过滤行为可被量化观测（token 消耗、延迟、过滤比例），生命周期在 initialize 阶段具备有限的静默自愈能力，控制面将引入 caller TTL 淘汰机制以避免“僵尸 caller”长期占用资源。用户可以通过 Prometheus 指标观察 SubAgent 的成本与时延，通过 runtime 配置收敛 caller 活跃窗口，并在初始化偶发失败时看到成功重试。

## Progress

- [x] (2025-03-09T08:40:00Z) 新建 ExecPlan 文档并完成任务拆解。
- [x] (2025-03-09T08:40:00Z) 为 SubAgent 增加指标接口与 Prometheus 实现。
- [x] (2025-03-09T08:40:00Z) 切换 SubAgent 至 ToolCallingChatModel 并记录 token/延迟/过滤比例。
- [x] (2025-03-09T08:40:00Z) 引入 caller TTL 配置与淘汰逻辑，补齐测试。
- [x] (2025-03-09T08:40:00Z) Lifecycle initialize 增加静默重试并补齐测试。
- [x] (2025-03-09T08:40:00Z) 清理 catalog.example 目录与相关引用，更新运行配置示例。
- [x] (2025-03-09T08:55:00Z) 运行格式化、静态检查与测试，记录结果。
- [x] (2025-03-09T09:10:00Z) 补充 mcpv-overview 仪表盘中的 SubAgent 指标面板。

## Surprises & Discoveries

- Observation: `go test` 在 `internal/ui` 链接阶段出现 macOS 版本警告，但测试仍通过。
  Evidence: `ld: warning: object file ... was built for newer 'macOS' version (26.0) than being linked (11.0)`

## Decision Log

- Decision: 新增 runtime 字段 `callerInactiveSeconds` 作为 TTL 配置，并提供默认值 `300` 秒。
  Rationale: TTL 行为需要可配置与可追溯的默认值，避免硬编码并便于未来调优。
  Date/Author: 2025-03-09 / Codex
- Decision: `mcpv_subagent_filter_precision` 记录“终选 / 初选”比例，初选为 LLM 选择后的工具列表，终选为去重后实际返回的工具列表。
  Rationale: 该比例更能反映 dedup 与限额对输出的压缩程度，符合“初选 vs 终选”的语义。
  Date/Author: 2025-03-09 / Codex
- Decision: Initialize 重试采用“3 次重试 + 1 次初始尝试”的总计 4 次尝试策略。
  Rationale: 满足“3 次静默重试”的要求，同时保留首次尝试的失败信号。
  Date/Author: 2025-03-09 / Codex

## Outcomes & Retrospective

暂无。

## Context and Orientation

SubAgent 位于 `internal/infra/subagent`，通过 `internal/app/app.go` 初始化并注入控制面；其指标实现位于 `internal/infra/telemetry`，接口定义在 `internal/domain/metrics.go`。控制面的 caller 注册、心跳与淘汰逻辑在 `internal/app/control_plane_registry.go` 中完成，运行时配置结构位于 `internal/domain/types.go`，默认值在 `internal/domain/constants.go`，解析与校验由 `internal/infra/catalog/loader.go` 与 `internal/infra/catalog/schema.json` 负责。生命周期初始化逻辑在 `internal/infra/lifecycle/manager.go`。

“Profile store” 指一个目录结构，包含 `runtime.yaml`、`callers.yaml` 与 `profiles/*.yaml`；Core 仅接受目录模式作为配置入口。

## Plan of Work

首先扩展 runtime 配置：在 `internal/domain/types.go` 增加 `CallerInactiveSeconds` 字段，并在 `internal/domain/constants.go` 增加默认值。同步更新 `internal/infra/catalog/loader.go` 的默认值、解析与校验逻辑，并在 `internal/infra/catalog/schema.json` 中声明该字段，随后更新 `internal/ui/types.go` 与 `internal/ui/mapping.go` 以向前端暴露该字段。

其次，更新 caller 淘汰策略：在 `internal/app/control_plane_registry.go` 的 `reapDeadCallers` 中引入 TTL 判定，优先依据 `CallerInactiveSeconds` 淘汰超时 caller，即便 `pidAlive` 仍返回 true。补齐 `internal/app/control_plane_test.go` 以覆盖 TTL 行为。

然后，为 SubAgent 添加可观测性与新接口：扩展 `internal/domain/metrics.go` 增加 SubAgent 指标采集方法；在 `internal/infra/telemetry/prometheus.go` 中新增计数器与直方图，实现 `mcpv_subagent_tokens_total`、`mcpv_subagent_latency_seconds`、`mcpv_subagent_filter_precision`，并更新 `internal/infra/telemetry/metrics.go` 与 `internal/infra/telemetry/prometheus_test.go`。

随后切换 SubAgent 模型接口：`internal/infra/subagent/model.go` 返回 `model.ToolCallingChatModel`，`internal/infra/subagent/subagent.go` 改用该接口，避免 deprecated ChatModel 路径。在 `SelectToolsForCaller`/`filterWithLLM` 中记录 token、延迟与过滤比例。Metrics 的 provider/model label 取自 runtime SubAgent 配置。

最后为 lifecycle initialize 加入重试：在 `internal/infra/lifecycle/manager.go` 中为 initialize 包装 3 次静默重试，并使用轻量延迟与上下文取消判定，避免阻塞或泄漏。补充或调整 `internal/infra/lifecycle/manager_test.go` 相关用例。

清理历史示例配置：移除 `docs/catalog.example` 目录，更新 `Makefile` 的默认 CONFIG 指向当前目录，并在文档中删除对 catalog.example 的引用（不修改既有 ExecPlans）。

## Concrete Steps

在仓库根目录执行如下步骤：

1) 新建 ExecPlan 文件，并在每次完成子任务后更新 `Progress` 与 `Decision Log`。

2) 修改 runtime 配置与 schema，并同步测试与 UI 映射：

   - 编辑 `internal/domain/constants.go`、`internal/domain/types.go`、`internal/infra/catalog/loader.go`、`internal/infra/catalog/schema.json`、`internal/infra/catalog/loader_test.go`、`internal/infra/catalog/profile_store_test.go`、`internal/ui/types.go`、`internal/ui/mapping.go`。

3) 修改 caller TTL 淘汰逻辑与测试：

   - 编辑 `internal/app/control_plane_registry.go` 与 `internal/app/control_plane_test.go`。

4) SubAgent 指标与模型接口升级：

   - 编辑 `internal/domain/metrics.go`、`internal/infra/telemetry/metrics.go`、`internal/infra/telemetry/prometheus.go`、`internal/infra/telemetry/prometheus_test.go`、`internal/infra/subagent/model.go`、`internal/infra/subagent/subagent.go`、`internal/app/app.go`。

5) Lifecycle initialize 重试：

   - 编辑 `internal/infra/lifecycle/manager.go` 与 `internal/infra/lifecycle/manager_test.go`。

6) 移除 catalog.example 目录并更新文档/脚本：

   - 删除 `docs/catalog.example` 目录。
   - 更新 `Makefile` 默认 CONFIG，更新 `docs/CONFIG_VISUALIZATION_DESIGN.md`、`docs/PRD.md` 等对 catalog.example 的引用（不触及 `docs/EXEC_PLANS/*`）。

7) 运行格式化、静态检查与测试：

   - `GOCACHE=./.cache/go-build make fmt`
   - `GOCACHE=./.cache/go-build go vet ./...`
   - `GOCACHE=./.cache/go-build make test`

## Validation and Acceptance

- 运行 `make test`，所有测试通过；新增 TTL 测试在改动前失败、改动后通过。
- 运行 `mcpv serve --config .`，Prometheus `/metrics` 输出包含：

  - `mcpv_subagent_tokens_total{provider="...",model="..."}`
  - `mcpv_subagent_latency_seconds_bucket{provider="...",model="..."}`
  - `mcpv_subagent_filter_precision_bucket{provider="...",model="..."}`

- 在 SubAgent 处理一次 `automatic_mcp` 后，指标的计数与直方图有增量。

## Idempotence and Recovery

所有修改均可重复应用。若 TTL 或 metrics 相关逻辑导致编译失败，可先回滚新增字段与接口，再逐步恢复。删除 `docs/catalog.example` 后，默认示例改用仓库根目录的 `runtime.yaml`、`callers.yaml` 与 `profiles/*.yaml`。

## Artifacts and Notes

无。

## Interfaces and Dependencies

- 在 `internal/domain/metrics.go` 中新增：

  ObserveSubAgentTokens(provider string, model string, tokens int)
  ObserveSubAgentLatency(provider string, model string, duration time.Duration)
  ObserveSubAgentFilterPrecision(provider string, model string, ratio float64)

- 在 `internal/domain/types.go` 中新增 `RuntimeConfig.CallerInactiveSeconds int`。
- 在 `internal/infra/subagent/model.go` 中返回 `model.ToolCallingChatModel`。
- 在 `internal/infra/telemetry/prometheus.go` 中新增 Prometheus 指标：

  - mcpv_subagent_tokens_total (CounterVec, labels: provider, model)
  - mcpv_subagent_latency_seconds (HistogramVec, labels: provider, model)
  - mcpv_subagent_filter_precision (HistogramVec, labels: provider, model)

Plan Update Note (2025-03-09T00:00:00Z): 初始计划创建。
Plan Update Note (2025-03-09T08:40:00Z): 记录实现进展与新增决策。
Plan Update Note (2025-03-09T08:55:00Z): 更新测试执行状态并记录 macOS 链接警告。
Plan Update Note (2025-03-09T09:10:00Z): 补充 SubAgent 指标的 Grafana 仪表盘配置。
