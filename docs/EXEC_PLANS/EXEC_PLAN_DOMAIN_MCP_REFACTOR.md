# 领域模型解耦、MCP 变更通知与传输解耦重构

这是一个可执行计划（ExecPlan），它是一个活文档。必须在执行过程中持续更新 `Progress`、`Surprises & Discoveries`、`Decision Log` 和 `Outcomes & Retrospective`。

仓库内存在 `.agent/PLANS.md`，本计划必须符合其要求并持续维护。

## Purpose / Big Picture

完成本计划后，领域层不再直接依赖 `json.RawMessage` 表达工具/资源/提示等核心对象，业务逻辑能够直接访问结构化字段；工具/资源/提示的刷新将优先响应 MCP `list_changed` 通知并保留定时兜底；当下游连续刷新失败达到阈值时，聚合视图将自动下线对应服务；`SpecFingerprint` 仅依赖运行环境字段保持稳定；`stdio` 连接的进程启动与数据传输完全拆分，为后续扩展到容器或其他启动方式铺平路径。用户应能观察到工具列表更新更即时、故障服务不再“假在线”、工具调度更稳定。

## Progress

- [x] 记录并确认当前重构目标与边界（领域模型去 JSON、通知监听、熔断、指纹稳定、传输解耦）。
- [x] 设计并落地领域模型结构化类型与 JSON 编解码适配层，修复相关业务逻辑与 UI/RPC 映射。
- [x] 重构连接模型与传输层，建立 list_changed 通知分发，并接入索引刷新。
- [x] 引入刷新失败熔断逻辑与稳定化 SpecFingerprint，更新测试。
- [x] 完成全量测试并回填计划进度与产出。

## Surprises & Discoveries

- Observation: JSON-RPC 的 ID 在 decode 过程中可能以 int64 进入，导致基于类型的请求映射失败。
  Evidence: transport round-trip 测试首次失败并提示 unsupported id type int64。
- Observation: 进程 stop 时可能返回 signal: killed，需要在 stop 流程中视为正常终止。
  Evidence: CommandLauncher 的 stop 测试在等待进程结束时返回 killed。

## Decision Log

- Decision: 领域层对工具/资源/提示的结构化建模落地，并以 mcpcodec 作为 JSON 边界适配层。
  Rationale: 保持 domain 纯净，避免业务逻辑频繁解析 JSON。
  Date/Author: 2025-12-30 / Codex
- Decision: list_changed 采用能力门控的通知触发 + 兜底轮询。
  Rationale: 加速更新体验并兼容不支持通知的下游。
  Date/Author: 2025-12-30 / Codex
- Decision: GenericIndex 增加失败阈值熔断，达到阈值清空缓存并等待恢复。
  Rationale: 避免“假在线”工具影响交互与调用错误。
  Date/Author: 2025-12-30 / Codex
- Decision: SpecFingerprint 仅基于 Cmd/Env/Cwd 生成稳定哈希。
  Rationale: 降低无关字段变动引发重启的风险。
  Date/Author: 2025-12-30 / Codex
- Decision: 传输层拆分 Launcher 与 Transport，Conn 仅负责 JSON-RPC Call。
  Rationale: 解耦进程管理与 IO，便于扩展到其他启动方式。
  Date/Author: 2025-12-30 / Codex

## Outcomes & Retrospective

- 领域层模型完成去 JSON，工具/资源/提示/日志在 domain 内可直接访问结构化字段。
- list_changed 通知已接入索引刷新，并保留轮询兜底；刷新失败具备熔断下线机制。
- SpecFingerprint 稳定化完成，测试覆盖关键字段与非关键字段变更。
- 传输层已拆分 Launcher 与 Transport，连接具备通知分发能力。
- 全量测试已运行通过（go test ./...）；macOS 链接器警告不影响测试结果。

## Context and Orientation

领域模型位于 `internal/domain/controlplane.go` 与 `internal/domain/subagent.go`，其中 `ToolDefinition/ResourceDefinition/PromptDefinition/LogEntry` 等类型当前使用 `json.RawMessage` 存储 MCP JSON。工具、资源与提示聚合索引分别位于 `internal/infra/aggregator/aggregator.go`、`internal/infra/aggregator/resource_index.go` 与 `internal/infra/aggregator/prompt_index.go`，刷新逻辑由 `internal/infra/aggregator/index_core.go` 中的 `GenericIndex` 驱动。`SpecFingerprint` 位于 `internal/domain/spec_fingerprint.go`。

传输与生命周期管理相关逻辑位于 `internal/infra/transport/stdio.go` 与 `internal/infra/lifecycle/manager.go`，当前 `StdioTransport` 同时负责进程启动与连接数据读写。路由逻辑位于 `internal/infra/router/router.go`，以 `domain.Conn` 的 Send/Recv 模式调用下游。

UI 侧数据映射位于 `internal/ui/mapping.go` 与 `internal/ui/events.go`，RPC 侧映射位于 `internal/infra/rpc/mapping.go`。SubAgent 逻辑位于 `internal/infra/subagent/subagent.go`。

## Plan of Work

首先定义领域层结构化类型（工具、资源、提示、日志），并引入一个 MCP JSON 编解码适配层，保证 UI/RPC 输出仍满足既有协议格式。随后将聚合索引与自动化逻辑改为直接访问结构化字段，并统一快照哈希逻辑以避免重复实现。

接着重构连接模型与传输层：拆分进程启动与数据传输，新增连接读循环以处理 `list_changed` 通知，将通知分发到索引刷新逻辑，并确保能力标识被实际使用。

然后在 `GenericIndex` 引入刷新失败的连续计数与阈值机制，当达到阈值时下线缓存，并在恢复成功后自动回归。同步调整 `SpecFingerprint` 为基于运行环境的稳定哈希。

最后更新测试、执行 `go test ./...` 验证行为，并回填计划进度、决策与产出。

## Concrete Steps

在 `/Users/wibus/dev/mcpv` 下工作。

1) 定义领域结构化类型并新增 MCP JSON codec。

    - 编辑 `internal/domain/controlplane.go`、`internal/domain/subagent.go`，引入结构化字段并移除 `ToolJSON/ResourceJSON/PromptJSON/DataJSON`。
    - 新增 `internal/domain/*` 辅助类型（Meta、Annotations、ToolAnnotations、PromptArgument、ListChangeEvent）与深拷贝工具。
    - 新增 `internal/infra/mcpcodec` 包，提供 MCP 结构与领域模型的双向映射与 JSON 编解码。
    - 更新聚合索引、自动化逻辑、SubAgent、UI/RPC 映射使用 codec。

2) 拆分进程启动与数据传输，并接入 list_changed。

    - 新增 `domain.Launcher` 与 `domain.Transport` 接口，`domain.Conn` 改为 `Call` 模式。
    - `internal/infra/transport` 中实现 `CommandLauncher` 与 `MCPTransport`，连接内部读循环接收通知并分发。
    - 新增 `internal/infra/notifications` 的 list change hub，连接启动时绑定 emitter。
    - `ToolIndex/ResourceIndex/PromptIndex` 在 `Start` 时订阅通知并触发 refresh。
    - 更新 router、probe、lifecycle、scheduler 与相关测试，适配新接口。

3) 引入熔断与稳定指纹。

    - 在 `internal/infra/aggregator/index_core.go` 增加失败计数与阈值触发下线。
    - 在 `internal/domain/spec_fingerprint.go` 改为基于 Cmd/Env/Cwd 的稳定哈希，并更新测试。

4) 测试与回填。

    - 更新相关单测（aggregator、transport、router、lifecycle、spec_fingerprint 等）。
    - 运行 `go test ./...`。
    - 更新 Progress、Decision Log、Outcomes。

## Validation and Acceptance

- 启动后在下游工具列表变化时，能够在一次通知到达后触发刷新（通过单测或日志验证）。
- 当某服务连续刷新失败达到阈值时，其工具/资源/提示从聚合视图中移除；恢复后重新出现。
- `SpecFingerprint` 在修改非运行环境字段时保持稳定，修改 Cmd/Env/Cwd 时变化。
- `go test ./...` 全部通过。

## Idempotence and Recovery

上述改动可重复执行；如出现编译错误，可根据错误提示逐一修复接口适配。若连接重构导致运行异常，可暂时回退到单一连接调用并禁用通知分发，确保最小可运行路径。

## Artifacts and Notes

- 变更点以文件为中心整理：`internal/domain/*`、`internal/infra/aggregator/*`、`internal/infra/transport/*`、`internal/infra/router/*`、`internal/infra/notifications/*`、`internal/ui/*`、`internal/infra/rpc/*`。

## Interfaces and Dependencies

- `internal/domain`:

    - `type ToolDefinition struct { Name string; Description string; InputSchema any; OutputSchema any; Title string; Annotations *ToolAnnotations; Meta Meta; SpecKey string; ServerName string }`
    - `type ResourceDefinition struct { URI string; Name string; Title string; Description string; MIMEType string; Size int64; Annotations *Annotations; Meta Meta }`
    - `type PromptDefinition struct { Name string; Title string; Description string; Arguments []PromptArgument; Meta Meta }`
    - `type LogEntry struct { Logger string; Level LogLevel; Timestamp time.Time; Data map[string]any }`
    - `type ListChangeEvent struct { Kind ListChangeKind; ServerType string; SpecKey string }`
    - `type Launcher interface { Start(ctx context.Context, specKey string, spec ServerSpec) (IOStreams, StopFn, error) }`
    - `type Transport interface { Connect(ctx context.Context, specKey string, spec ServerSpec, streams IOStreams) (Conn, error) }`
    - `type Conn interface { Call(ctx context.Context, payload json.RawMessage) (json.RawMessage, error); Close() error }`

- `internal/infra/mcpcodec`:

    - `func ToolFromMCP(*mcp.Tool) domain.ToolDefinition`
    - `func ResourceFromMCP(*mcp.Resource) domain.ResourceDefinition`
    - `func PromptFromMCP(*mcp.Prompt) domain.PromptDefinition`
    - `func MarshalToolDefinition(domain.ToolDefinition) ([]byte, error)`
    - `func MarshalResourceDefinition(domain.ResourceDefinition) ([]byte, error)`
    - `func MarshalPromptDefinition(domain.PromptDefinition) ([]byte, error)`
    - `func HashToolDefinition(domain.ToolDefinition) (string, error)` 等

- `internal/infra/notifications`:

    - `type ListChangeHub struct { ... }` 实现 `EmitListChange` 与 `Subscribe`。
