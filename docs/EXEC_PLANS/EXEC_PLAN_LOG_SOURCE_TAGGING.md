# 标注日志来源并镜像下游 stderr

This ExecPlan is a living document. The sections Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective must be kept up to date as work proceeds.

This document must be maintained in accordance with .agent/PLANS.md from the repository root.

## Purpose / Big Picture

完成这次改动后，Wails3 前端可以清晰区分日志来源：核心控制面自身、下游 MCP 子进程（stdio stderr），以及 Wails UI 自身。使用者不需要改动 MCP 协议或启动方式，只要在前端读取日志事件中的字段即可完成过滤与分组。验收方式是启动 core 或 Wails app 后观察日志流，能看到 log_source 字段与 server/stream 信息，并且下游 stderr 日志会被镜像出来。

## Progress

- [x] (2025-12-28 11:35Z) 创建 ExecPlan，明确日志来源字段与 stdio stderr 镜像目标。
- [x] (2025-12-28 11:50Z) 让 core 日志自动携带 log_source=core，并确保 Wails UI 日志使用 log_source=ui。
- [x] (2025-12-28 11:55Z) 在 stdio transport 中镜像下游 stderr，并附带 log_source=downstream 与 server/stream 字段。
- [x] (2025-12-28 12:00Z) 补充测试覆盖 stderr 镜像与字段标记。
- [x] (2025-12-28 12:05Z) 运行相关 go test 并记录结果。

## Surprises & Discoveries

暂无。

## Decision Log

- Decision: 使用 log_source 字段区分 core、downstream、ui 三类日志，并用 stream=stderr 标注下游输出通道。
  Rationale: 结构化字段可被 LogBroadcaster 捕获并在 Wails UI 中直接过滤，同时保持 stdout 纯 JSON-RPC 的 MCP 约束。
  Date/Author: 2025-12-28 / Codex.

- Decision: 下游日志仅镜像 stderr，不处理 stdout。
  Rationale: MCP stdio 规范要求 stdout 仅输出 JSON-RPC，镜像 stdout 可能破坏协议。
  Date/Author: 2025-12-28 / Codex.

## Outcomes & Retrospective

已完成 log_source 字段标注与下游 stderr 镜像，Wails UI 可按 core/downstream/ui 过滤日志并定位具体 server/stream。transport 测试新增 stderr 镜像断言，go test ./internal/infra/transport 通过。后续若需要支持更细粒度的日志等级或 stdout 调试镜像，可再扩展。

## Context and Orientation

当前日志通过 internal/infra/telemetry/log_broadcaster.go 进入 UI，DataJSON 中包含 message、timestamp、logger 以及 fields。核心 app 在 internal/app/app.go 构建 logger 并创建 stdio transport。stdio 子进程由 internal/infra/transport/stdio.go 启动，当前未捕获 stderr。Wails 入口在 app.go，WailsService 位于 internal/ui/service.go，日志使用传入的 zap.Logger。

## Plan of Work

先在 telemetry 侧定义 log_source 与 stream 字段常量和值，确保字段名统一。随后在 core app 初始化时为 logger 注入 log_source=core，保证所有 core 组件日志携带该字段。Wails 入口处为 WailsService 使用 log_source=ui 的 logger，避免与 core 混淆。最后在 stdio transport 中接管子进程 stderr，逐行镜像为日志，并在日志上追加 log_source=downstream、serverType 与 stream=stderr。补充 transport 测试，验证 stderr 被镜像且字段正确，再运行指定 go test。

## Concrete Steps

在仓库根目录执行下列命令。

1) 编辑 internal/infra/telemetry/log_fields.go，新增 log_source/stream 字段常量和值。

2) 编辑 internal/app/app.go，为 New 和 NewWithBroadcaster 中的 logger 注入 log_source=core。

3) 编辑 app.go，为 WailsService 使用 log_source=ui 的 logger，并将 UI 自身日志输出改为该 logger。

4) 编辑 internal/infra/transport/stdio.go，增加 logger 依赖与 stderr 镜像逻辑，并给下游日志加上 log_source=downstream、serverType、stream=stderr。

5) 编辑 internal/infra/transport/stdio_test.go，新增 stderr 镜像测试并更新构造函数调用。

## Validation and Acceptance

运行 go test ./internal/infra/transport，期望新测试通过且没有新的失败。启动 core 或 Wails app 后，日志事件的 DataJSON.fields 内包含 log_source 字段，且下游 stderr 输出被镜像为 log_source=downstream 的日志。

## Idempotence and Recovery

上述改动均为可重复执行的代码调整。若需要回滚，只需恢复涉及的 Go 文件到改动前版本并重新运行 go test。

## Artifacts and Notes

本次变更的关键输出包括 stderr 镜像测试的 go test 结果。

    go test ./internal/infra/transport
    ok    mcpv/internal/infra/transport  5.608s

## Interfaces and Dependencies

保持 zap 作为日志实现与 LogBroadcaster 作为日志分发机制。stdio transport 将新增 StdioTransportOptions 以传入 logger；下游日志将使用 zap.String 字段写入 log_source 与 stream。字段名以 internal/infra/telemetry/log_fields.go 为准。

Plan update note: 2025-12-28 12:00Z，更新进度以反映 log_source 注入、stderr 镜像与测试补充已完成。
Plan update note: 2025-12-28 12:05Z，记录 go test ./internal/infra/transport 的执行结果。
Plan update note: 2025-12-28 12:06Z，补充 Outcomes & Retrospective 以总结达成情况与后续空间。
Plan update note: 2025-12-28 12:07Z，补充 Artifacts and Notes 以记录测试输出。
Plan update note: 2025-12-28 12:08Z，修正 Artifacts 中的换行与示例格式。
