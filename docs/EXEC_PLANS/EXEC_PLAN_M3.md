# Implement M3 stability, health, capability gating, and tool aggregation

This ExecPlan is a living document. The sections Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective must be kept up to date as work proceeds.

This document must be maintained in accordance with .agent/PLANS.md from the repository root.

## Purpose / Big Picture

完成本次变更后，mcpd 作为 MCP Server 对外暴露统一的工具视图，并具备稳定性、健康检查与能力白名单。用户可以通过 stdio 连接 mcpd，调用 tools/list 获取聚合后的工具集合，调用 tools/call 触发下游工具，同时路由会按照下游 initialize 能力进行方法过滤，超时和健康检查参数可配置。验证方式是运行测试与一个最小端到端示例，观察 tools/list 与 tools/call 的返回符合预期。

## Progress

- [x] (2025-12-22 03:57Z) 拟定并确认 M3 ExecPlan。
- [x] (2025-12-22 04:25Z) 完成运行时配置与 catalog 扩展，并在 app/router/scheduler 中注入。
- [x] (2025-12-22 04:25Z) 完成能力模型与方法白名单，并实现路由超时配置。
- [x] (2025-12-22 04:40Z) 完成 ping 探测与健康循环，同时修正 scheduler 状态安全。
- [x] (2025-12-22 04:40Z) 完成工具聚合与刷新机制，并接入 MCP Server。
- [x] (2025-12-22 05:05Z) 增加 metrics 钩子与 Noop 实现，完成 router/scheduler 接入。
- [x] (2025-12-22 05:25Z) 补充测试与文档更新，验证完整链路（新增 e2e 测试与 M3 设计文档更新）。
- [x] (2025-12-22 06:05Z) 补充 MCP logging 通知桥接与对应测试。

## Surprises & Discoveries

- Observation: Go build cache 写入系统缓存目录在当前沙箱内不可用。
  Evidence: make test 报错 open /Users/wibus/Library/Caches/go-build/... operation not permitted，已通过设置 GOCACHE 到工作区解决。

## Decision Log

- Decision: 以 domain.ServerCapabilities 表达下游能力，不直接暴露 go-sdk 类型。
  Rationale: domain 层需保持纯净，能力判断只需要布尔字段。
  Date/Author: 2025-12-22 / Codex.

- Decision: 工具聚合使用 mcp.Server.AddTool/RemoveTools 动态更新工具集合。
  Rationale: go-sdk 原生支持变更通知与工具替换，避免自建列表协议。
  Date/Author: 2025-12-22 / Codex.

- Decision: 工具命名策略默认使用 serverType 前缀，提供 flat 作为兼容选项。
  Rationale: 前缀可避免冲突并明确归属，flat 保留旧习惯。
  Date/Author: 2025-12-22 / Codex.

## Outcomes & Retrospective

未开始。

## Context and Orientation

当前入口位于 internal/app/app.go，负责加载 catalog、初始化 lifecycle/scheduler/router，并调用 internal/infra/server/mcp_server.go 启动 MCP Server。现有 MCP Server 仅注册 route 工具并转发 JSON-RPC payload。Router 位于 internal/infra/router/router.go，固定 10 秒超时且能力白名单为 noop。Scheduler 位于 internal/infra/scheduler/basic.go，支持 idle 回收但缺少健康探测与并发安全。Lifecycle 位于 internal/infra/lifecycle/manager.go，会发送 initialize 并校验协议版本，但不会持久化 capability 结果。

术语说明：能力白名单指依据 initialize 返回的 capabilities 决定允许转发的方法；工具聚合指从下游 tools/list 拉取工具并暴露为 mcpd 的工具集合；ping 探测指周期性向下游发送 JSON-RPC ping 以检测失败实例。

## Plan of Work

先扩展 catalog 结构，增加运行时配置字段并提供默认值，再调整 Load 返回结构与调用方适配。接着定义 domain 层的能力结构并在 lifecycle 初始化时填充，router 在路由时基于实例能力判断方法是否允许，超时使用配置注入。随后引入 ping 探测实现与 scheduler 健康循环，保证状态安全并避免路由到失效实例。然后实现工具聚合：定期调用 tools/list 合并工具，按命名策略注册到 MCP Server，并为每个聚合工具提供 tools/call 转发处理。最后补充测试与文档，确保工程完备性与可验证性。

## Concrete Steps

在 /Users/wibus/dev/mcpd 执行以下命令并观察输出。

1) 查找 catalog 与 app 入口变更点。
    rg -n "Load\(" internal
    rg -n "ServeConfig|ValidateConfig" internal/app

2) 实现配置扩展与 router 超时配置后，运行：
    go test ./internal/infra/catalog ./internal/infra/router

3) 实现能力白名单与 lifecycle 映射后，运行：
    go test ./internal/infra/lifecycle ./internal/infra/router

4) 实现 ping 探测与 scheduler 健康循环后，运行：
    go test ./internal/infra/scheduler

5) 实现工具聚合并接入 MCP Server 后，运行：
    go test ./internal/infra/aggregator ./internal/infra/server

6) 完成后运行完整测试：
    make test

## Validation and Acceptance

验收标准：启动 mcpd 后，tools/list 返回聚合工具（默认前缀命名），tools/call 可以正确转发到下游，并在下游不支持的能力上返回 method not allowed。通过 go test ./... 与一个最小集成测试验证聚合工具列表与调用结果匹配。

## Idempotence and Recovery

所有变更为新增或可重复执行的更新。若测试失败，回滚单文件改动并重新运行对应包测试；后台 goroutine 需可在 context 取消后退出，避免测试残留。

## Artifacts and Notes

示例输出（用于比对）：
    tools/list response tools:
    - name: "echo.echo"

## Interfaces and Dependencies

不新增外部依赖。使用 go-sdk v1.1.0 现有能力。

新增或更新类型（示例）：
    internal/domain/types.go
        type RuntimeConfig struct { RouteTimeoutSeconds int; PingIntervalSeconds int; ToolRefreshSeconds int; ExposeTools bool; ToolNamespaceStrategy string }
        type ServerCapabilities struct { Tools bool; Resources bool; Prompts bool; Logging bool; Completions bool; Experimental bool }
        type Catalog struct { Specs map[string]ServerSpec; Runtime RuntimeConfig }
        type Instance struct { Capabilities ServerCapabilities }

新增或更新入口与构造：
    internal/infra/catalog/loader.go
        func (l *Loader) Load(ctx context.Context, path string) (domain.Catalog, error)

    internal/infra/router/router.go
        func NewBasicRouter(scheduler domain.Scheduler, opts Options) *BasicRouter

    internal/infra/scheduler/basic.go
        func NewBasicScheduler(lc domain.Lifecycle, specs map[string]domain.ServerSpec, opts Options) *BasicScheduler

    internal/infra/aggregator/aggregator.go
        func NewToolAggregator(rt domain.Router, specs map[string]domain.ServerSpec, cfg domain.RuntimeConfig, logger *zap.Logger) *ToolAggregator
        func (a *ToolAggregator) RegisterServer(s *mcp.Server)
        func (a *ToolAggregator) Start(ctx context.Context)
        func (a *ToolAggregator) Stop()

Plan Update Note: 创建 M3 ExecPlan 文档并对齐 .agent/PLANS.md 规范。
Plan Update Note: 更新进度与发现项，记录沙箱下 GOCACHE 处理与里程碑完成情况。
Plan Update Note: 记录 metrics 钩子落地与进度状态更新。
Plan Update Note: 记录 e2e 测试与 M3 设计文档同步。
Plan Update Note: 记录 MCP logging 通知桥接与测试补充。
