# Streamable HTTP Transport 支持

本 ExecPlan 是一个持续更新的执行文档，必须遵循仓库根目录的 `.agent/PLANS.md` 约束并在实现过程中保持同步更新。

## Purpose / Big Picture

让 mcpd 可以通过配置连接外部 Streamable HTTP MCP server（不启动本地 HTTP server），并保持现有实例状态机/调度策略。完成后，用户可在 profile 的 server 配置里声明 `transport: streamable_http` 与 `http.endpoint`，mcpd 会建立会话、完成 initialize 握手、维持 session 状态、按策略路由与回收，并继续对 tools/resources/prompts 做刷新与聚合。

## Progress

- [x] (done) 添加 transport/config 模型与 schema/loader 校验，支持 streamable_http。
- [x] (done) 实现 streamable HTTP transport 与复合 transport，调整 lifecycle 启动与握手。
- [x] (done) 更新指纹、导入/编辑流程与必要文档说明。
- [x] (done) 添加测试并运行 `go test ./internal/...`（`make test` 需先生成 frontend/dist）。

## Surprises & Discoveries

- Observation: go-sdk 的 StreamableClientTransport 依赖 MCP-Protocol-Version 头；默认行为与 mcpd 的 2025-11-25 不兼容，需要为 HTTP transport 单独允许 2025-06-18/2025-03-26/2024-11-05。
  Evidence: go-sdk `mcp/shared.go` 支持的版本列表与 Streamable transport 对协议头的要求。

## Decision Log

- Decision: streamable_http 仅连接外部端点，不管理本地 HTTP server。
  Rationale: 需求明确且复杂度更低，避免进程发现与端口管理。
  Date/Author: 2026-01-14 / Codex

- Decision: 对 streamable_http 使用默认协议版本 2025-06-18，stdio 仍为 2025-11-25。
  Rationale: go-sdk Streamable transport 不支持 2025-11-25。
  Date/Author: 2026-01-14 / Codex

## Outcomes & Retrospective

- 实现完成。已运行 `go test ./internal/...`，`make test` 仍需 frontend/dist。
- go-sdk 的 StreamableClientTransport 需要 MCP-Protocol-Version 头，已通过 headerRoundTripper 注入。
- 隐式 transport 检测（当 http 配置存在但 transport 未指定时）已添加警告日志。

## Context and Orientation

mcpd 的核心启动流程位于 `internal/infra/lifecycle/manager.go`，通过 `Launcher` 启动 stdio 子进程并由 `Transport` 建立 JSON-RPC 连接。配置解析在 `internal/infra/catalog/loader.go` 与 `internal/infra/catalog/schema.json`，ServerSpec 定义于 `internal/domain/types.go`。transport 现仅支持 stdio（`internal/infra/transport/mcp_transport.go` + `command_launcher.go`）。

## Plan of Work

先扩展 ServerSpec 支持 `transport` 与 `http` 配置，并在 loader/schema 中完成解析与跨字段校验。随后新增 Streamable HTTP transport，并用复合 transport 根据 `transport` 选择 stdio 或 HTTP 连接，同时调整 lifecycle 在 streamable_http 时跳过 Launcher。最后更新 spec fingerprint、导入/编辑路径与文档说明，并补齐单测与集成测试。

## Concrete Steps

1. 修改 `internal/domain/types.go` 与 `internal/domain/constants.go`，新增 transport/HTTP 配置与默认值。
2. 更新 `internal/infra/catalog/loader.go` 和 `internal/infra/catalog/schema.json`，加入 transport/http 配置解析与校验。
3. 新增 `internal/infra/transport/streamable_http.go` 与 `internal/infra/transport/composite_transport.go`，在 `internal/app/providers.go` 组合两种 transport。
4. 调整 `internal/infra/lifecycle/manager.go` 支持 streamable_http 启动路径。
5. 更新 `internal/domain/spec_fingerprint.go` 与相关测试。
6. 更新导入流程与前端解析（如 `frontend/src/modules/config/lib/mcp-import.ts`）。
7. 运行 `make test` 并记录结果。

## Validation and Acceptance

- 运行 `make test`，期望全部测试通过。
- 新增的 streamable_http 测试应证明：
  - transport 可连接 httptest 的 Streamable HTTP server；
  - lifecycle 能启动实例并完成 initialize 握手。
- 人工验证：配置一个 `transport: streamable_http` 的 server，启动 mcpd 后能正常 tools/list 与路由。

## Idempotence and Recovery

修改均为可重复运行的增量变更。若测试失败，可回滚到变更前版本或逐项撤销新增文件。

## Artifacts and Notes

暂无。

## Interfaces and Dependencies

- `internal/domain.ServerSpec` 新增字段：
  - `Transport`（string enum: stdio | streamable_http）
  - `HTTP`（endpoint, headers, maxRetries）
- 新增 transport：`internal/infra/transport.StreamableHTTPTransport`。
- 复合 transport：`internal/infra/transport.CompositeTransport`。
- 外部依赖：`github.com/modelcontextprotocol/go-sdk` 已在 go.mod 中存在。
