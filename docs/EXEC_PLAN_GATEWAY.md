# Split MCP Gateway From Control Plane With gRPC

This ExecPlan is a living document. The sections Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective must be kept up to date as work proceeds.

This document must be maintained in accordance with .agent/PLANS.md from the repository root.

## Purpose / Big Picture

完成本次变更后，mcpd 作为控制面核心服务（core）只负责编排、调度与路由，并通过 gRPC 暴露稳定的控制面 API；新增独立二进制 mcpd-gateway，作为唯一 MCP Server 入口，通过 RPC 与 core 交互，桥接 MCP 协议。用户将通过 stdio 连接 mcpd-gateway 调用 tools/list 与 tools/call，工具列表来自 core 的聚合结果，工具调用由 core 路由到下游 MCP servers。验证方式为运行单测与一个最小端到端示例，确认 gateway 能列出工具并成功调用，且 core 不再直接运行 MCP Server。

## Progress

- [x] (2025-12-25 09:32Z) 创建 Gateway 拆分 ExecPlan，明确 gRPC 传输与独立二进制方向。
- [x] (2025-12-25 10:18Z) 定义 gRPC API 与配置结构，补齐 proto 与生成工具链。
- [x] (2025-12-25 10:18Z) 抽出控制面服务与工具聚合索引，移除 core 对 MCP Server 的直接依赖。
- [x] (2025-12-25 10:18Z) 实现 gRPC server/client 与 gateway MCP Server 桥接。
- [ ] 补齐测试、文档与示例，完成端到端验证（完成：catalog/toolIndex/gateway/rpc 测试与 README/结构文档更新；剩余：e2e）。

## Surprises & Discoveries

- Observation: go fmt/test 需要将 GOCACHE 指向工作区，否则默认缓存目录无权限。
  Evidence: make fmt 报错 open /Users/wibus/Library/Caches/go-build/... operation not permitted，已通过设置 GOCACHE 解决。

## Decision Log

- Decision: 采用独立二进制 mcpd-gateway，并以 gRPC 作为 core 与 gateway 间 RPC 传输。
  Rationale: 彻底解耦 MCP 协议与控制面逻辑，便于演进与多入口复用，同时 gRPC 提供流式能力与标准化健康检查。
  Date/Author: 2025-12-25 / Codex.

- Decision: gRPC 只传输 MCP 类型的 JSON 载荷，不在 proto 中复刻 MCP schema。
  Rationale: 避免协议重复实现与版本漂移，gateway 与 core 继续使用 go-sdk 解析 MCP 类型。
  Date/Author: 2025-12-25 / Codex.

## Outcomes & Retrospective

未开始。

## Context and Orientation

当前 mcpd 入口为 `cmd/mcpd/main.go`，`internal/app/app.go` 负责加载 catalog、初始化 lifecycle/scheduler/router/tool index，并启动 gRPC 控制面服务。`internal/infra/gateway` 提供 MCP Server 入口（stdio），通过 gRPC 与 core 通讯并维护 tool registry。日志通过 core 内部 log broadcaster 出站，再由 gateway 发送 MCP logging 通知。

本次改造需要引入“core / gateway”分层：

- core：仅负责 catalog、scheduler、router、lifecycle、aggregator 与控制面 API。它不直接提供 MCP 协议服务，只暴露 gRPC。
- gateway：独立进程，只负责 MCP 协议处理（stdio），通过 gRPC 访问 core，桥接 tools/list、tools/call 与 logging。
- control plane：core 暴露的稳定 API，提供工具列表、工具调用、日志流与健康状态。

关键术语：

- MCP Server：对外提供 MCP 协议的服务，要求支持 stdio，处理 tools/list 与 tools/call。
- Gateway：MCP Server 的唯一入口进程，负责与 core 通讯并维护 MCP tool registry。
- Control Plane：core 对外暴露的 gRPC API，用于聚合工具与路由请求。

## Plan of Work

先定义 gRPC API 与配置结构，确保 core 与 gateway 的通信契约稳定。为保持类型安全与演进能力，新增 proto 文件并引入 gRPC 与 Protobuf 依赖，同时在 Makefile 中加入 proto 生成命令。随后重构 tool 聚合器，使其成为 core 内的纯工具索引（不再直接依赖 MCP Server），并提供快照与订阅能力。再重构日志管道，替换现有 MCPLogSink，使 core 将日志事件输出为可订阅流，供 gRPC 服务流式传输。

完成核心抽象后，实现 gRPC server（core 端）与 gRPC client（gateway 端），提供工具快照、工具调用与日志流式传输。随后新增 `cmd/mcpd-gateway` 作为独立进程，并将原有 MCP Server 逻辑迁移至 gateway 包中。最后更新文档与示例，并补齐覆盖 core/gateway 交互的测试，确保新的分层与接口具备工程完备性。

## Concrete Steps

在 /Users/wibus/dev/mcpd 执行以下命令并观察输出。

1) 阅读与定位现有 MCP Server 入口与 tool 聚合逻辑。
    rg -n "mcp_server|server.Run|ToolAggregator" internal docs

2) 完成 proto 定义与生成后，执行：
    make proto

3) 完成 core/gateway 改造后，执行：
    make fmt
    make vet
    make test

4) 运行最小端到端示例（core + gateway），观察 MCP tools/list 与 tools/call 成功。
    go run ./cmd/mcpd serve --config docs/catalog.example.yaml
    go run ./cmd/mcpd-gateway --rpc unix:///tmp/mcpd.sock

预期：gateway 启动后可通过 MCP 客户端看到聚合工具列表，并能成功调用其中任意工具。

## Validation and Acceptance

验收标准：

1) `cmd/mcpd` 启动后仅提供 gRPC 控制面，不再直接运行 MCP Server。
2) `cmd/mcpd-gateway` 能通过 gRPC 拉取工具列表并注册为 MCP tools。
3) `tools/list` 返回的工具集合与 core 聚合结果一致（命名策略与过滤策略生效）。
4) `tools/call` 通过 gateway 调用能正确路由到下游并返回结果，错误遵循 MCP 语义（tool error vs protocol error）。
5) 日志桥接可用：core 产生日志后，gateway 能向 MCP session 发送 logging 通知（遵循 client 设置的 level）。
6) `make test` 全量通过，并包含新增 gRPC 与 gateway 测试。

## Idempotence and Recovery

所有变更为新增或可重复执行的更新。gRPC Unix socket 监听需要在启动前清理旧文件，若启动失败可删除 socket 文件后重试。proto 生成可重复执行，重复执行不会破坏现有生成代码。若 gateway 无法连接 core，应提供退避重试并在日志中明确提示。

## Artifacts and Notes

示例 proto 结构（仅用于理解，真实内容以仓库文件为准）：
    syntax = "proto3";
    package mcpd.control.v1;
    option go_package = "mcpd/pkg/api/control/v1;controlv1";

    service ControlPlaneService {
      rpc GetInfo(GetInfoRequest) returns (GetInfoResponse);
      rpc ListTools(ListToolsRequest) returns (ListToolsResponse);
      rpc WatchTools(WatchToolsRequest) returns (stream ToolsSnapshot);
      rpc CallTool(CallToolRequest) returns (CallToolResponse);
      rpc StreamLogs(StreamLogsRequest) returns (stream LogEntry);
    }

message ToolDefinition {
  string name = 1;
  bytes tool_json = 2;
}

    message ToolsSnapshot {
      string etag = 1;
      repeated ToolDefinition tools = 2;
    }

    message CallToolRequest {
      string name = 1;
      bytes arguments_json = 2;
      string routing_key = 3;
    }

message CallToolResponse {
  bytes result_json = 1;
}

    enum LogLevel {
      LOG_LEVEL_UNSPECIFIED = 0;
      LOG_LEVEL_DEBUG = 1;
      LOG_LEVEL_INFO = 2;
      LOG_LEVEL_NOTICE = 3;
      LOG_LEVEL_WARNING = 4;
      LOG_LEVEL_ERROR = 5;
      LOG_LEVEL_CRITICAL = 6;
      LOG_LEVEL_ALERT = 7;
      LOG_LEVEL_EMERGENCY = 8;
    }

message LogEntry {
  string logger = 1;
  LogLevel level = 2;
  int64 timestamp_unix_nano = 3;
  bytes data_json = 4;
}

## Interfaces and Dependencies

新增依赖：

- google.golang.org/grpc
- google.golang.org/protobuf
- go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc
- google.golang.org/grpc/health/grpc_health_v1 (grpc-go 内置)

新增或调整接口（示例）：

1) core 控制面接口，建议定义在 `internal/domain/controlplane.go`：
    type ControlPlane interface {
        Info(ctx context.Context) (ControlPlaneInfo, error)
        ListTools(ctx context.Context) (ToolSnapshot, error)
        WatchTools(ctx context.Context) (<-chan ToolSnapshot, error)
        CallTool(ctx context.Context, name string, args json.RawMessage, routingKey string) (json.RawMessage, error)
        StreamLogs(ctx context.Context, minLevel LogLevel) (<-chan LogEntry, error)
    }

2) tool 聚合索引（替换当前 ToolAggregator 对 MCP Server 的依赖）：
    type ToolIndex interface {
        Start(ctx context.Context)
        Stop()
        Snapshot() ToolSnapshot
        Subscribe(ctx context.Context) <-chan ToolSnapshot
        Resolve(name string) (ToolTarget, bool)
        CallTool(ctx context.Context, name string, args json.RawMessage, routingKey string) (json.RawMessage, error)
    }

3) gRPC server/client：
    internal/infra/rpc/server.go: type Server struct { ... }
    internal/infra/rpc/client.go: type Client struct { ... }

4) gateway MCP Server：
    internal/infra/gateway/gateway.go: type Gateway struct { ... }

配置扩展（示例）：

- 在 `internal/domain/types.go` 的 `RuntimeConfig` 增加 `RPC` 子结构，包含：
  - ListenAddress (string, 默认 unix:///tmp/mcpd.sock)
  - TLS (enabled, certFile, keyFile, caFile, clientAuth)
  - MaxRecvMsgSize / MaxSendMsgSize
  - Keepalive 参数

更新点（需明确替换）：

- 删除或弃用 `internal/infra/server/mcp_server.go`，其职责迁移到 gateway。
- `internal/app/app.go` 不再调用 `server.Run`，改为启动 gRPC server。
- 新增 `cmd/mcpd-gateway/main.go`，仅负责 MCP Server + gRPC client 桥接。
- 更新 `docs/STRUCTURE.md`、`docs/INITIAL_DESIGN.md`、`docs/DRAFT_PLAN.md` 与 `docs/catalog.example.yaml` 以反映新入口与配置。

Plan Update Note: 记录 core/gateway 拆分实施进度、接口变更与文档同步（完成 gRPC 与 tool index 落地，待补齐测试与 e2e）。
