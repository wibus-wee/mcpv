# 插件化治理管线（Gateway 全请求）落地

本 ExecPlan 是一个动态文档。`Progress`、`Surprises & Discoveries`、`Decision Log`、`Outcomes & Retrospective` 需要随实现过程持续更新。

本仓库的 ExecPlan 规范在 `./.agent/PLANS.md`，实现与维护必须严格遵循。

## Purpose / Big Picture

目标是把所有来自 gateway 的 MCP 请求统一接入“插件化治理管线”，让认证、授权、限流、校验、内容处理、审计、观测成为可插拔、可扩展、可热更新的系统能力。完成后，用户可以通过配置显式启用插件列表，不修改 core 代码也能控制安全策略与审计行为，并且对工具调用、资源读取、提示获取、列表与订阅等请求有一致的治理入口。可见效果包括：插件可拒绝请求并返回 MCP 业务错误、content 插件可安全改写请求/响应、观测与审计能覆盖所有 MCP 请求路径。

## Progress

- [x] (2026-01-30 08:20Z) 创建 ExecPlan，并记录治理管线需求与选定方案 B。
- [x] (2026-01-30 08:45Z) 完成插件配置与 catalog/schema 扩展：新增 domain 类型、配置解析与 schema、示例配置与插件 diff 支持。
- [x] (2026-01-30 09:30Z) 实现插件 gRPC 协议与插件进程管理器（Unix socket、Configure/CheckReady、commit hash 校验）并接入生成代码路径。
- [x] (2026-01-30 09:40Z) 实现治理管线引擎（类别顺序、并行/串行、required/optional、content 修改、optional rejection 策略）。
- [x] (2026-01-30 10:20Z) 引入统一治理执行器，并在 RPC 控制面接入工具调用/列表/订阅/日志等网关请求。
- [x] (2026-01-30 10:25Z) 打通热重载：插件列表变化触发 manager/pipeline 更新，reload 日志包含插件变更统计。
- [x] (2026-01-30 11:40Z) 增加治理管线的指标与日志，并完成单元/集成测试，包括 pipeline metrics、plugin manager+executor harness 等覆盖。

## Surprises & Discoveries

- Observation: （待补充）
  Evidence: （待补充）
- Observation: `make wire` 依赖的 wire 下载在当前环境出现 TLS handshake timeout，改为手动更新 `internal/app/wire_gen.go` 以保持可编译。
  Evidence: `go install github.com/google/wire/cmd/wire@latest` 请求 proxy.golang.org 超时。
- Observation: `go test ./...` 需要 `frontend/dist` 有可嵌入文件，已添加占位 `frontend/dist/index.html` 以满足 embed。
  Evidence: `embed.go:5:12: pattern frontend/dist: cannot embed directory frontend/dist: contains no embeddable files`.
- Observation: 插件 Unix socket 路径不能超过平台限制（macOS 约 108 字节），长路径会导致 `bind: invalid argument`，测试中改用 `/tmp/mcpv-harness-*` 作为 root 以保持路径简洁。
  Evidence: harness 测试使用 `t.TempDir()` 生成路径时触发 `plugin dial: context deadline exceeded`，日志显示 `listen unix .../harness-plugin-.../plugin.sock: bind: invalid argument`。

## Decision Log

- Decision: 采用方案 B（统一治理执行器 + 结构化请求上下文）并覆盖所有 gateway MCP 请求。
  Rationale: 在不进入“非常大”底层重构的前提下，提供统一的治理语义与最佳实践扩展性。
  Date/Author: 2026-01-30 / Codex

- Decision: 插件配置来源为显式列表，插件进程通过 Unix socket + gRPC 管理。
  Rationale: 对齐 MCPD 语义，并简化配置可控性与可审计性。
  Date/Author: 2026-01-30 / Codex
- Decision: 创建 `proto/mcpv/...` 目录并复制现有 control.proto，以保持 Makefile 与文档路径一致。
  Rationale: 现有 proto 文件路径与 Makefile/文档不一致，新增目录避免生成失败并为新增 plugin.proto 提供一致入口。
  Date/Author: 2026-01-30 / Codex

## Outcomes & Retrospective

- Observation: 治理管线现在输出跳出/拒绝指标，添加的插件 harness 提供进程级回放保障，整个管线（包括 manager、pipeline、executor）都有可重复的验证路径。
- Evidence: `go test ./...`（带新 harness）通过，pipeline/pipeline_test 与 plugin/manager_harness 覆盖新增语义，metrics 评估点可在 `/metrics` 观测。

- （待补充，完成阶段后填写）

## Context and Orientation

当前系统的 MCP 请求路径是 gateway → gRPC → controlplane → discovery → aggregator → router → scheduler/lifecycle/transport。核心调用入口在 `internal/infra/aggregator/*_index.go` 与 `internal/infra/router/router.go`。配置加载在 `internal/infra/catalog/loader.go` 与 `internal/infra/catalog/schema.json`。热重载在 `internal/app/controlplane/reload.go`。

“治理管线”指对请求/响应的插件化处理链，按固定类别顺序执行：observability → authentication → authorization → rate_limiting → validation → content → audit。observability 类别并行执行且忽略 optional rejection，content 类别允许改写请求/响应，其他类别串行、默认阻塞。

“gateway MCP 请求”是指 gateway 所暴露的 MCP 服务收到的全部方法（例如 tools/list、tools/call、resources/list、resources/read、prompts/list、prompts/get、logging/subscribe 等）。本方案要求所有这些入口至少经过“请求管线”，并对非流式请求进行“响应管线”。

## Plan of Work

首先扩展 catalog 配置结构，新增 `plugins` 显式列表，并在 `internal/infra/catalog/schema.json` 与 `internal/infra/catalog/loader.go` 支持解析、规范化与校验。为插件定义 domain 类型（类别、流程、required/optional、进程启动信息、commit hash、超时等）。更新 `internal/domain/catalog_summary.go` 与 `internal/domain/catalog_diff.go`，让插件列表纳入 summary 与 reload diff。更新 `internal/app/controlplane/reload.go` 以响应插件变更。

其次设计插件协议与管理器。新增 `proto/mcpv/plugin/v1/plugin.proto`，定义 `PluginService`（GetMetadata、Configure、CheckReady、HandleRequest、HandleResponse、Shutdown）。生成 Go bindings 并在 `internal/infra/plugin` 实现管理器：负责 Unix socket 路径生成、进程启动、gRPC 连接、元数据校验、配置下发与就绪检测，失败时具备超时与强制停止逻辑。

然后实现治理管线引擎。新增 `internal/infra/pipeline`，实现类别排序、并行/串行调度、optional/required 语义、ignore optional rejection、以及 content 改写约束。治理管线以结构化 `RequestContext` 与 `ResponseContext` 运行，支持生成“继续/拒绝”的决策对象，拒绝时携带 MCP 业务错误信息。

接着引入统一治理执行器（治理入口）。新增 `internal/infra/governance`（或 `internal/infra/executor`）作为统一调用入口，向上提供 `Execute` 能力：对所有 gateway 请求先执行 request pipeline，再执行实际业务（router 或 discovery），再执行 response pipeline。将 Tool/Resource/Prompt 调用统一接入该执行器，并在 `internal/infra/rpc/control_service.go` 对 list/watch/logging 等 gateway RPC 请求接入治理管线。

最后完成热重载与可观测性。reload 时更新插件列表、重启/停止插件进程，并在 metrics 与日志中体现治理管线耗时、拒绝原因与插件异常。补充单元测试和集成测试，覆盖类别语义、optional 行为、content 改写与拒绝路径。

## Concrete Steps

在仓库根目录执行下列步骤。所有命令都在 `/Users/wibus/Desktop/plugin` 下运行。

1) 扩展配置与 domain 类型。

   - 编辑 `internal/infra/catalog/schema.json`，新增顶层 `plugins` 字段与结构定义。
   - 编辑 `internal/infra/catalog/loader.go`，解析并规范化 `plugins`。
   - 新增 `internal/domain/plugin.go`（或 `internal/domain/governance.go`）定义 `PluginSpec`、`PluginCategory`、`PluginFlow`、`PluginPolicy`、`PipelineDecision` 等类型。
   - 编辑 `internal/domain/catalog_summary.go` 与 `internal/domain/catalog_diff.go`，将插件信息纳入摘要与 diff。

2) 引入插件 gRPC 协议与插件管理器。

   - 新增 `proto/mcpv/plugin/v1/plugin.proto` 并更新生成脚本。
   - 运行 `make proto` 生成 `pkg/api/plugin/v1/*`。
   - 新增 `internal/infra/plugin/manager.go` 实现插件进程管理与 gRPC 客户端。
   - 新增 `internal/infra/plugin/manager_test.go`，使用 Unix socket + 本地 gRPC server 模拟插件。

3) 实现治理管线引擎。

   - 新增 `internal/infra/pipeline/engine.go`（或分文件），实现类别执行语义。
   - 新增 `internal/infra/pipeline/engine_test.go`，覆盖并行、optional、content 改写、拒绝短路等行为。

4) 引入统一治理执行器与接入点。

   - 新增 `internal/infra/governance/executor.go`，提供 `Execute`/`Handle` 接口。
   - 修改 `internal/infra/aggregator/aggregator.go`、`internal/infra/aggregator/resource_index.go`、`internal/infra/aggregator/prompt_index.go`，统一走治理执行器。
   - 修改 `internal/infra/rpc/control_service.go`，对 list/watch/get/prompt/call 等请求接入治理执行器。

5) 热重载与可观测性。

   - 修改 `internal/app/controlplane/reload.go`，在 catalog 变更时更新插件列表与管线配置。
   - 修改 `internal/infra/telemetry/*` 记录治理管线耗时与拒绝原因。

6) 文档与示例。

   - 更新 `docs/catalog.example.yaml` 添加 `plugins` 示例。
   - 视需要添加最小示例插件实现与使用说明。

## Validation and Acceptance

- 运行 `go test ./...`，期望全部通过。
- 在示例配置中加入一个 `authentication` 插件，令其始终拒绝请求。启动 mcpv 与 gateway，调用 `tools/call` 时应收到 MCP 业务错误响应，且日志中记录拒绝原因。
- 将 `content` 插件设置为改写请求参数或响应内容，调用工具时应看到变更生效。
- 对 `tools/list`、`resources/list`、`prompts/list` 的请求，也应通过治理管线（至少 request pipeline），并可被 optional 插件观测到。

## Idempotence and Recovery

- 插件管理器在重启时应清理旧的 Unix socket 并重新建立连接。所有启动步骤可重复执行，不应产生重复进程或遗留 socket。
- 如果插件启动失败，系统应记录错误并根据 required/optional 策略决定是否阻断请求。失败后可通过重载配置触发重试。

## Artifacts and Notes

- （实现过程中补充最小日志或测试输出片段）

## Interfaces and Dependencies

在 `internal/domain/plugin.go` 定义以下核心接口与类型（名称可微调但需保持语义一致）：

    type PluginCategory string
    const (
        PluginCategoryObservability PluginCategory = "observability"
        PluginCategoryAuthentication PluginCategory = "authentication"
        PluginCategoryAuthorization  PluginCategory = "authorization"
        PluginCategoryRateLimiting   PluginCategory = "rate_limiting"
        PluginCategoryValidation     PluginCategory = "validation"
        PluginCategoryContent        PluginCategory = "content"
        PluginCategoryAudit          PluginCategory = "audit"
    )

    type PluginFlow string
    const (
        PluginFlowRequest  PluginFlow = "request"
        PluginFlowResponse PluginFlow = "response"
    )

    type PluginSpec struct {
        Name         string
        Category     PluginCategory
        Required     bool
        Cmd          []string
        Env          map[string]string
        Cwd          string
        CommitHash   string
        TimeoutMs    int
        ConfigJson   json.RawMessage
        Flows        []PluginFlow
    }

    type GovernanceRequest struct {
        Flow       PluginFlow
        Method     string
        Caller     string
        Server     string
        ToolName   string
        ResourceURI string
        PromptName string
        RoutingKey string
        RequestJson  json.RawMessage
        ResponseJson json.RawMessage
        Metadata    map[string]string
    }

    type GovernanceDecision struct {
        Continue     bool
        RequestJson  json.RawMessage
        ResponseJson json.RawMessage
        RejectCode   string
        RejectMessage string
    }

在 `proto/mcpv/plugin/v1/plugin.proto` 定义 `PluginService`：

    service PluginService {
        rpc GetMetadata(google.protobuf.Empty) returns (PluginMetadata);
        rpc Configure(PluginConfigureRequest) returns (PluginConfigureResponse);
        rpc CheckReady(google.protobuf.Empty) returns (PluginReadyResponse);
        rpc HandleRequest(PluginHandleRequest) returns (PluginHandleResponse);
        rpc HandleResponse(PluginHandleRequest) returns (PluginHandleResponse);
        rpc Shutdown(google.protobuf.Empty) returns (google.protobuf.Empty);
    }

实现 `internal/infra/plugin.Manager` 作为插件生命周期管理器，并在 `internal/infra/pipeline.Engine` 中依赖该管理器执行插件调用。

在 `internal/infra/governance.Executor` 中定义：

    type Executor interface {
        Execute(ctx context.Context, req GovernanceRequest, next func(context.Context, GovernanceRequest) (json.RawMessage, error)) (json.RawMessage, error)
    }

Executor 负责 request pipeline → next → response pipeline 的统一调用，并在拒绝时生成 MCP 业务错误结果或错误返回。

更新 `internal/app/providers.go` 注入插件管理器、治理管线与执行器，保持依赖单向：domain → infra → app。

---
Change Note: 初始创建 ExecPlan，用于实现“插件化治理管线（全 gateway MCP 请求）”的方案 B。
Change Note: 更新进度，记录插件配置与 catalog/schema 扩展已完成，并补充示例配置与插件 diff 信息。
Change Note: 更新进度，记录插件协议/manager 与 pipeline 引擎已完成；补充 wire 下载失败的发现与 proto 路径决策。
Change Note: 更新进度，记录治理执行器与 reload 接入完成，并补充 embed 占位文件的测试前提。
