# App-managed core with caller profiles and mcpdmcp entry

本 ExecPlan 为可演进文档，必须在实施过程中持续更新 `Progress`、`Surprises & Discoveries`、`Decision Log`、`Outcomes & Retrospective`。本文件需遵循仓库根目录 `.agent/PLANS.md` 的要求。

## Purpose / Big Picture（目标与大局）

目标是让用户只需运行 `mcpdmcp <caller>` 就能获得可用的 MCP 会话，并在 App 中看到 tools、logs、resources、prompts 的状态与内容。App 成为唯一入口，负责 core 的生命周期与体验；core 负责调度与路由；gateway 负责 MCP 协议会话。完成后可通过启动 App、触发 `mcpdmcp` 与 URL scheme 来验证“零配置启动、工具可见、日志可控、资源与 prompts 可浏览”的端到端体验。

## Progress（进度）

- [x] (2025-03-08 12:40Z) 定义 profile store 与 caller 解析，完成 core 的多 profile 装载与默认回退（新增 profile store loader、目录布局校验、default fallback，app/CLI 切换到 profile store 入口）。
- [x] (2025-03-08 15:30Z) 引入 spec fingerprint 与共享实例池，确保 identical spec 才复用（spec 指纹排除 Name，scheduler 按 specKey 池化实例，新增共享池测试与指标语义调整）。
- [x] (2025-03-08 18:10Z) 将 control plane 全量引入 caller，新增 mcpdmcp CLI 与 gateway 贯通（proto/cp/gateway caller 贯通，profile 级 tool index，scheduler 使用 specKey 池化）。
- [x] (2025-03-08 21:20Z) 增加 resources/prompts 控制面与 gateway 透传，完成分页与 list-changed 语义（资源/提示索引、read/get 路由、分页 cursor、gateway registry 同步）。
- [ ] 引入 App 托管 core 的生命周期与 URL scheme，提供最小 UI（tools/logs/resources/prompts）。
- [ ] 运行测试与最小端到端验证，补齐文档与示例。

## Surprises & Discoveries（意外与发现）

- Observation: listAllResources/listAllPrompts 中空 ETag 需要参与一致性校验，否则分页可能混合不同快照。
  Evidence: 增加 etagSet 并在分页中严格比较。
- Observation: ErrMethodNotAllowed 需要清理 server cache，否则旧快照会在能力撤回后残留。
  Evidence: 对应 serverType 的缓存删除并触发快照重建。

## Decision Log（决策记录）

- Decision: Caller 未配置时回退到 `default` profile。
  Rationale: 保证一键启动的可用性，避免空状态阻塞。
  Date/Author: 2025-03-08 / Codex.
- Decision: macOS 优先，初期不做 daemon，core 跟随 App 生命周期。
  Rationale: 降低权限与后台服务复杂度，先验证体验闭环。
  Date/Author: 2025-03-08 / Codex.
- Decision: ServerSpec 完全一致才复用实例池，spec fingerprint 排除 Name。
  Rationale: 避免隐性共享导致行为不透明。
  Date/Author: 2025-03-08 / Codex.
- Decision: 项目处于 WIP，允许破坏性重构。
  Rationale: 以长期架构一致性优先，不为兼容性增加复杂度。
  Date/Author: 2025-03-08 / Codex.
- Decision: logging/setLevel 由 gateway 处理，并通过 StreamLogs 的 min_level 控制与 UI 过滤共同实现。
  Rationale: 遵循 MCP 语义且避免新增不必要的控制面 RPC。
  Date/Author: 2025-03-08 / Codex.
- Decision: profile store loader 同时支持目录结构与单一 catalog 文件（文件路径视为 default profile）。
  Rationale: 降低迁移成本，允许平滑过渡到 profiles 目录结构。
  Date/Author: 2025-03-08 / Codex.
- Decision: CLI 默认 config path 改为当前目录，profile store 默认布局为 `./profiles` + `./callers.yaml`。
  Rationale: 避免默认为 `catalog.yaml` 时触发目录误创建，并与 profile store 结构对齐。
  Date/Author: 2025-03-08 / Codex.
- Decision: profile store 缺失时自动创建 `profiles/default.yaml` 与 `callers.yaml` 占位文件。
  Rationale: 保障首次启动有明确可编辑入口，避免空目录导致无从下手。
  Date/Author: 2025-03-08 / Codex.
- Decision: 实例池以 spec fingerprint 作为 specKey，fingerprint 排除 ServerSpec.Name 并对 env/exposeTools 做稳定排序。
  Rationale: 仅当运行配置完全一致时共享实例池，避免命名差异导致的冗余实例。
  Date/Author: 2025-03-08 / Codex.
- Decision: 实例生命周期与指标统一按 specKey 记录，route 仍按 serverType 统计。
  Rationale: 实例池是 spec 维度的资源，避免多个 serverType 共享池时指标误读。
  Date/Author: 2025-03-08 / Codex.
- Decision: control plane 所有 RPC 均显式携带 caller，gateway/mcpdmcp 负责传递，core 负责 caller->profile fallback。
  Rationale: 明确调用方语义，避免核心层隐式状态。
  Date/Author: 2025-03-08 / Codex.
- Decision: scheduler 改为直接以 specKey 作为 pool key，router/ToolIndex 负责 serverType->specKey 映射。
  Rationale: 共享池与路由分离，避免 scheduler 承担 profile 语义。
  Date/Author: 2025-03-08 / Codex.
- Decision: RPC/Observability 配置要求跨 profile 一致，Ping 间隔取最小正值。
  Rationale: 全局服务只有一份，强制一致避免隐式覆盖；最小间隔确保探活不缺失。
  Date/Author: 2025-03-08 / Codex.
- Decision: prompts 使用与 tools 相同的 namespace 策略处理冲突，resources 保持 URI 原样并在冲突时保留先到者。
  Rationale: prompt 名称需要跨 serverType 唯一，URI 语义不可变更且不适合加前缀。
  Date/Author: 2025-03-08 / Codex.
- Decision: resources/prompts 列表分页使用固定 page size=200，cursor 为最后一项的 URI/Name。
  Rationale: 稳定可预测的 keyset 分页，避免引入额外配置。
  Date/Author: 2025-03-08 / Codex.
- Decision: 增加 resources/read 与 prompts/get 的控制面与 gateway 透传。
  Rationale: gateway 需要 read/get handler 才能正确暴露 resources/prompts 能力。
  Date/Author: 2025-03-08 / Codex.

## Outcomes & Retrospective（结果与回顾）

尚未开始。

## Context and Orientation（上下文与定位）

当前 core 通过 `cmd/mcpd` 启动，编排逻辑在 `internal/app/app.go`，控制面 gRPC 定义在 `proto/mcpd/control/v1/control.proto` 与 `internal/infra/rpc`，gateway 在 `internal/infra/gateway` 且 CLI 入口为 `cmd/mcpd-gateway`。现有 tools 聚合仅覆盖 tools（`internal/infra/aggregator`），resources/prompts 尚无索引。`internal/ui` 为 Wails 预留桥接层，`docs/WAILS_STRUCTURE.md` 描述了 Wails 结构。

本计划中的关键术语：caller 指 MCP client 的调用方标识；profile 是内部配置单元，包含一组 ServerSpec 与运行参数；profile store 指 `callers.yaml` 与 `profiles/*.yaml` 的组合；spec fingerprint 是对 ServerSpec 的稳定哈希，用于实例池复用。

## Plan of Work（工作计划）

第一阶段完成 profile store 与 caller 解析，核心进入多 profile 模式。profile store 使用 `callers.yaml` 将 caller 映射到 profile 名，profiles 目录按文件名定义 profile；未知 caller 回退到 `default`。core 的加载入口改为 profiles 目录路径，必要时自动生成 default profile 与 callers 映射。CLI 与 App 统一使用 profile store，以降低模型分叉。

第二阶段引入 spec fingerprint 与共享实例池。对 ServerSpec 做稳定归一化后计算 hash，实例池以 fingerprint 作为 key，profile 内 serverType 仅作为展示与工具命名空间。调度器改为“profile 维度路由 + specKey 维度实例池”的两层结构，确保 identical spec 才复用。

第三阶段改造控制面与 gateway。proto 与 domain 全量引入 caller 维度，tools/list、tools/call、watch tools、stream logs 都必须显式携带 caller。新增 `cmd/mcpdmcp` 作为官方 MCP 入口，参数只接受 `<caller>`。gateway 根据 MCP client 的 `logging/setLevel` 调整 StreamLogs 的 `min_level`，并在工具调用时传递 caller。

第四阶段补齐 resources 与 prompts。新增 ResourceIndex 与 PromptIndex（或与 ToolIndex 同级的索引模块），支持 list/get 与 list-changed 订阅，分页逻辑必须稳定并可缓存。gateway 透传 resources/list 与 prompts/list/get，UI 以只读视图呈现并支持分页与刷新提示。

第五阶段接入 Wails。`cmd/mcpd-wails` 负责启动 App，core 以 in-process background service 形式运行；App 关闭时拒绝新请求并等待 in-flight 完成后停止。URL scheme `mcpd://start?caller=<name>` 在 Wails 注册，通过事件回调解析后进入 UI 流程。UI 至少包含 profile 选择、tools 列表、logs 流与 resources/prompts 视图。

文档更新贯穿各阶段，包括 `docs/PRD.md`、`docs/STRUCTURE.md`、`docs/WAILS_STRUCTURE.md` 与 `docs/UX_REDUCTION.md`，确保协议边界与体验路径一致。

## Concrete Steps（具体步骤）

在 `/Users/wibus/dev/mcpd` 执行。

完成 profile store 与 CLI 入口改造后，补齐默认配置生成与验证逻辑。profile store 期望结构如下：

    profiles/default.yaml
    profiles/vscode.yaml
    callers.yaml

完成 proto 变更后执行生成：

    make proto

完成核心改造后格式化与测试：

    make fmt
    make test

手动验证 mcpdmcp 与 URL scheme：

    go run ./cmd/mcpdmcp vscode
    open "mcpdmcp://start?caller=vscode"

## Validation and Acceptance（验证与验收）

Profile store 与 caller 解析完成后，core 能加载多个 profile，并对未知 caller 回退到 default；同一 profile 的 tools/list 输出稳定且可复现。

Spec fingerprint 与共享实例池完成后，两个 profile 引用相同 ServerSpec 时只产生一套实例池，实例数与复用行为可通过日志与 metrics 验证。

control plane 引入 caller 后，`mcpdmcp <caller>` 的 tools/list 与 tools/call 仅反映对应 profile，日志按 caller 归因；`logging/setLevel` 会改变 MCP 客户端接收的日志等级。

resources/prompts 补齐后，UI 能分页显示资源与 prompts，list-changed 更新能触发 UI 刷新；prompts/get 返回内容可展示且与参数一致。

Wails 接入后，通过 URL scheme 拉起 App 并建立 MCP 会话，关闭 App 会优雅终止 core，且新请求被拒绝、旧请求完成后退出。

## Idempotence and Recovery（幂等与恢复）

所有步骤可重复执行。若 gRPC unix socket 残留导致启动失败，删除 socket 文件后重试。若 profile 文件损坏，删除并重新生成 default profile。proto 生成可重复运行且不会破坏既有生成代码。

## Artifacts and Notes（产出与记录）

保留关键日志片段、工具列表与 list-changed 更新示例、resources/prompts 分页示例，并在本节以简短缩进片段记录，便于后续回溯与验证。

## Interfaces and Dependencies（接口与依赖）

proto 需要引入 caller 字段，并新增 resources/prompts 相关接口。示例形态如下：

    message ListToolsRequest { string caller = 1; }
    message WatchToolsRequest { string caller = 1; string last_etag = 2; }
    message CallToolRequest { string caller = 1; string name = 2; bytes arguments_json = 3; string routing_key = 4; }
    message StreamLogsRequest { string caller = 1; LogLevel min_level = 2; }

    rpc ListResources(ListResourcesRequest) returns (ListResourcesResponse);
    rpc WatchResources(WatchResourcesRequest) returns (stream ResourcesSnapshot);
    rpc ReadResource(ReadResourceRequest) returns (ReadResourceResponse);
    rpc ListPrompts(ListPromptsRequest) returns (ListPromptsResponse);
    rpc WatchPrompts(WatchPromptsRequest) returns (stream PromptsSnapshot);
    rpc GetPrompt(GetPromptRequest) returns (GetPromptResponse);

    message ListResourcesRequest { string caller = 1; string cursor = 2; }
    message ListResourcesResponse { ResourcesSnapshot snapshot = 1; string next_cursor = 2; }
    message WatchResourcesRequest { string caller = 1; string last_etag = 2; }
    message ResourcesSnapshot { string etag = 1; repeated ResourceDefinition resources = 2; }
    message ResourceDefinition { string uri = 1; bytes resource_json = 2; }
    message ReadResourceRequest { string caller = 1; string uri = 2; }
    message ReadResourceResponse { bytes result_json = 1; }

    message ListPromptsRequest { string caller = 1; string cursor = 2; }
    message ListPromptsResponse { PromptsSnapshot snapshot = 1; string next_cursor = 2; }
    message WatchPromptsRequest { string caller = 1; string last_etag = 2; }
    message PromptsSnapshot { string etag = 1; repeated PromptDefinition prompts = 2; }
    message PromptDefinition { string name = 1; bytes prompt_json = 2; }
    message GetPromptRequest { string caller = 1; string name = 2; bytes arguments_json = 3; }
    message GetPromptResponse { bytes result_json = 1; }

domain 控制面接口需要 caller 维度，并新增资源与 prompts 类型：

    ListTools(ctx context.Context, caller string) (ToolSnapshot, error)
    WatchTools(ctx context.Context, caller string) (<-chan ToolSnapshot, error)
    CallTool(ctx context.Context, caller, name string, args json.RawMessage, routingKey string) (json.RawMessage, error)
    StreamLogs(ctx context.Context, caller string, minLevel LogLevel) (<-chan LogEntry, error)

    ListResources(ctx context.Context, caller string, cursor string) (ResourcePage, error)
    WatchResources(ctx context.Context, caller string) (<-chan ResourceSnapshot, error)
    ReadResource(ctx context.Context, caller, uri string) (json.RawMessage, error)
    ListPrompts(ctx context.Context, caller string, cursor string) (PromptPage, error)
    WatchPrompts(ctx context.Context, caller string) (<-chan PromptSnapshot, error)
    GetPrompt(ctx context.Context, caller, name string, args json.RawMessage) (json.RawMessage, error)

    type ResourceDefinition struct { URI string; ResourceJSON json.RawMessage }
    type ResourceSnapshot struct { ETag string; Resources []ResourceDefinition }
    type ResourcePage struct { Snapshot ResourceSnapshot; NextCursor string }
    type PromptDefinition struct { Name string; PromptJSON json.RawMessage }
    type PromptSnapshot struct { ETag string; Prompts []PromptDefinition }
    type PromptPage struct { Snapshot PromptSnapshot; NextCursor string }

profile store 定义应集中在 `internal/domain/profile.go`，并由 `internal/infra/catalog/profile_store.go` 负责加载。

Plan Update Note: Marked phase 3 complete with caller-aware control plane and mcpdmcp entry, recorded specKey routing and shared runtime rules, and updated progress to reflect RPC/gateway/app refactor.
Plan Update Note: Marked phase 4 complete with resources/prompts control plane + gateway passthrough, added pagination/read/get interfaces, and recorded pagination and naming decisions.
Plan Update Note: Addressed pagination ETag consistency, capability removal cache cleanup, and added pagination/cursor tests based on review feedback.
