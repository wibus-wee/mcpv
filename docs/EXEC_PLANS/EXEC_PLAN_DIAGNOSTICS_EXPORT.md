# 诊断导出与阶段追踪（非阻塞方案）

本 ExecPlan 是一份“可执行规范”，并且是活文档。实施过程中必须持续更新 `Progress`、`Surprises & Discoveries`、`Decision Log`、`Outcomes & Retrospective`。本计划需严格遵循仓库根目录的 `.agent/PLANS.md`。

## 目的 / 大局观

目标是让操作者在“服务卡在 init starting、没有实例、没有日志”的场景下，仍然能导出一份可判断卡点的诊断包。完成后，用户可以通过一次导出就看到“当前卡在哪一步、卡了多久、上一次错误是什么、最近发生过哪些关键事件”，且此能力不阻塞主流程、不引入破坏性重构。该方案强调旁路观察，不改动核心流程语义，也不让诊断收集阻塞调度或生命周期管理。

## Progress

- [x] (2026-02-09 23:50+08:00) 建立诊断事件与阶段追踪的数据结构与环形缓冲，实现非阻塞写入。
- [x] (2026-02-09 23:50+08:00) 在调度器、生命周期、init 管理器与 transport 中接入旁路探针，记录关键阶段与错误链。
- [x] (2026-02-09 23:50+08:00) 增加诊断包导出接口，包含 snapshot、事件时间线、metrics 与日志片段。
- [x] (2026-02-09 23:50+08:00) 新增验证用例并更新 wire 生成代码。

## Surprises & Discoveries

暂无。

## Decision Log

- Decision: 使用旁路 Diagnostics Probe + 环形缓冲，而不是改造核心状态机。
  Rationale: 保证非阻塞与最小侵入，同时仍能回答“卡在哪一步”。
  Date/Author: 2026-02-09 / Codex

- Decision: 诊断导出提供“默认安全模式 + 深度诊断可选模式”。
  Rationale: 默认避免泄露敏感数据，同时允许在需要时输出完整诊断线索。
  Date/Author: 2026-02-09 / Codex

- Decision: 诊断事件与日志在内存中保留敏感字段，但导出默认安全模式会进行脱敏。
  Rationale: 确保深度模式能还原关键线索，同时在默认导出中避免泄露。
  Date/Author: 2026-02-09 / Codex

- Decision: 仅在 dev 构建默认启用敏感捕获，其他环境默认关闭，可用环境变量覆盖。
  Rationale: 降低默认风险，同时为开发调试保留可用性。
  Date/Author: 2026-02-09 / Codex

## Outcomes & Retrospective

完成了诊断事件与日志的旁路采集、关键阶段追踪、诊断包导出与基础测试。CLI 导出路径尚未实现，后续如需无 UI 环境可补充。

## 现状与定位（Context and Orientation）

当前项目已经具备基础的可观测能力，但缺少“阶段级诊断”。已有能力主要包括：

- Prometheus metrics 服务：由 `internal/infra/telemetry` 提供 HTTP `/metrics`，启动入口在 `internal/app/application.go`。
- 日志广播：`internal/infra/telemetry/log_broadcaster.go` 提供订阅式日志流。
- Debug snapshot：`internal/ui/services/debug_snapshot.go` 汇总状态，但只有粗粒度（server init state、runtime status），无法说明“卡在哪一步”。
- Scheduler / Lifecycle / Server Init 的主流程分布在：
  - 调度器：`internal/infra/scheduler`，实例启动与 acquire 逻辑在 `acquire.go` 与 `lifecycle.go`。
  - 生命周期管理：`internal/infra/lifecycle/manager.go`，包含 transport connect 与 initialize。
  - init 管理器：`internal/app/bootstrap/serverinit/manager.go`，管理 minReady 初始化。

术语说明：

- “阶段”（step）指一个明确的处理步骤，例如 `set_min_ready` 或 `initialize_call`。
- “attempt” 指一次 init 尝试，包含一个连续的阶段序列。
- “旁路探针”（Diagnostics Probe）指只记录事件、永不影响核心流程的观察器。
- “诊断包”（Diagnostics Bundle）是一次性导出的 JSON 或 tar.gz，包含事件时间线、快照、metrics、日志片段。
- “深度诊断模式”指允许导出更细粒度的上下文数据（命令行、headers、握手内容），且必须脱敏或显式开启。
- “卡住”（stuck）指阶段在合理超时时间内不推进，并且错误/实例没有更新。

## 方案总览（Plan of Work）

本方案分三层：

第一层：定义诊断事件与阶段追踪的数据结构，并提供非阻塞的环形缓冲实现。这个层只负责“记录”，不负责任何业务决策。

第二层：在调度器、生命周期、init 管理器等关键边界点插入“旁路探针”，仅记录“阶段进入/退出、错误、耗时、上下文”。不改变控制流，不等待，不加锁阻塞关键路径。

第三层：提供导出接口，将以下内容组成诊断包：

- Debug snapshot（现有汇总）。
- 诊断事件时间线（按 server/spec 分组）。
- 当前 metrics 文本导出。
- 最近一段时间或最近 N 条日志。

导出接口放在 UI DebugService（Wails 服务），同时提供一个可选的 CLI 导出路径（用于无 UI 环境）。

## 详细设计与文件修改点（Plan of Work 细化）

### 1) 诊断事件数据结构与旁路探针

在 `internal/infra/telemetry` 新增子模块 `internal/infra/telemetry/diagnostics`，包含：

- `DiagnosticsEvent`：包含 `specKey`、`serverName`、`attemptId`、`step`、`phase`（enter/exit/error）、`timestamp`、`duration`、`error`、`cause`、`attributes`。
- `RingBuffer`：固定容量环形缓冲，支持 `Add(event)` 与 `Snapshot()`，并维护丢弃计数 `Dropped()`。
- `Probe`：实现 `DiagnosticsProbe` 接口，仅暴露 `Record(event)`。该方法必须是非阻塞写入，失败时仅增加 dropped 计数。

性能要求：

- `Record` 不得阻塞主流程，且不持有全局锁超过一次原子或短锁。
- 事件大小需控制，避免包含大块 payload 或敏感信息。
- 环形缓冲默认容量建议为 2048 条（可在此计划中确认为常量）。

### 2) 阶段枚举与 attempt 追踪

定义统一阶段枚举（字符串即可）：

- `launcher_start`
- `transport_connect`
- `initialize_call`
- `initialize_response`
- `notify_initialized`
- `instance_ready`
- `set_min_ready`
- `snapshot_done`

Attempt 生成策略：`attemptId = specKey + startTimeUTC`，在开始 `set_min_ready` 或 `launcher_start` 时创建并贯穿后续阶段。Attempt 的“当前阶段”由最新 `phase=enter` 事件推断。导出时由诊断模块计算 `currentStep` 与 `stepStartedAt`。

### 3) 插入记录点（不侵入核心流程）

- Launcher 级 (`internal/infra/transport/command_launcher.go`)：
  - `launcher_start enter/exit/error`。
  - 记录命令行、工作目录、环境变量摘要（默认脱敏）。

- Transport 级 (`internal/infra/transport/mcp_transport.go`, `internal/infra/transport/streamable_http.go`)：
  - stdio：记录 IO streams 可用性（reader/writer nil 或关闭）。
  - streamable_http：记录 URL、headers 摘要、连接状态。

- Scheduler (`internal/infra/scheduler/acquire.go`, `internal/infra/scheduler/lifecycle.go`)：
  - 启动前记录 `launcher_start enter`。
  - 启动错误处记录 `launcher_start error` + err。
  - 启动成功后记录 `launcher_start exit`。
  - Acquire 失败记录 `acquire_failure`（含 reason 与 routingKey 是否存在）。
  - 记录 pool 状态：starting 计数、waiters、maxConcurrent、minReady。

- Lifecycle (`internal/infra/lifecycle/manager.go`)：
  - `transport_connect enter/exit/error`。
  - `initialize_call enter/exit/error`，记录 attempt number 与错误。
  - `initialize_response error`（解析失败或协议不匹配）。
  - 记录 initialize 重试次数与最终错误。

- Server Init (`internal/app/bootstrap/serverinit/manager.go`)：
  - `set_min_ready enter/exit/error`。
  - snapshot 采样完成记录 `snapshot_done`。

所有记录必须走 `DiagnosticsProbe`，不可引入阻塞等待。禁止在核心路径中进行 JSON 序列化、文件 IO 或网络 IO。

### 4) 日志与 metrics 采集策略（非阻塞）

- 日志：新增 `DiagnosticsLogBuffer`，由应用启动时订阅日志流并写入 ring buffer。导出时读取 ring buffer 快照，避免导出时阻塞等待日志。
- Metrics：通过 Prometheus registry `Gather()` 导出 text，导出时只做读取，不在核心路径更新。

### 5) 深度诊断可选模式

导出参数提供 `mode`（`safe` 或 `deep`），默认 `safe`：

- `safe`：命令行参数、环境变量、headers、握手消息仅保留脱敏摘要（例如键名、长度、hash）。
- `deep`：输出完整字段，但必须在导出 JSON 中标记 `containsSensitive=true`，并在 CLI 提示中输出警告。

脱敏策略建议：

- 键名包含 `token`, `secret`, `authorization`, `api_key`, `cookie` 的值全部替换为 `***`。
- 对握手消息内容保存摘要（例如截断前 N 字符 + SHA256 hash）。

### 6) 诊断包导出

在 `internal/ui/services` 新增 `debug_export.go`，实现 `ExportDiagnosticsBundle` 方法。导出内容包含：

- `snapshot`：现有 debug snapshot JSON。
- `events`：DiagnosticsBuffer 时间线（按 server/spec 分组）。
- `metrics`：当前 Prometheus registry 的 text dump。
- `logs`：DiagnosticsLogBuffer 最近 N 条或最近 X 秒日志。
- `stuck`：按 server/spec 计算的“卡住判定”，输出 `currentStep`、`stepStartedAt`、`duration` 与 `reason`。
- `redaction`：输出当前模式 `safe` 或 `deep` 以及是否包含敏感字段。

导出结构建议：

- 顶层 JSON: `generatedAt`, `snapshot`, `events`, `metrics`, `logs`, `stuck`, `dropped`, `redaction`。

可选加 CLI：在 `cmd/mcpv` 新增 `debug export` 子命令，将 JSON 写到文件。若文件已存在，写入前备份为 `.bak`。

### 7) 卡住判定规则

定义阈值（建议 30 秒或使用 runtime 配置的 bootstrap 超时作为上限），判断条件为：

- `currentStep` 在阈值内没有推进。
- 且 `lastError` 为空或 `ready` 未增加。

输出 `stuck` 字段包含 `step`, `since`, `durationMs`, `lastError`。

### 8) 安全与敏感信息控制

诊断包可能包含日志与错误链，必须避免直接泄露 secrets：

- 输出日志时对字段名包含 `token`, `secret`, `authorization`, `api_key` 的值进行掩码。
- snapshot 与 metrics 保持原样，但若 metrics 中存在敏感标签，需在导出层做过滤（本计划建议先不做过滤，仅在日志与 deep 模式字段中处理）。

## 具体步骤（Concrete Steps）

所有命令在仓库根目录 `/Users/wibus/dev/mcpd` 执行。

1) 创建诊断模块与 ring buffer：

   - 新建 `internal/infra/telemetry/diagnostics` 目录。
   - 添加 `buffer.go`, `probe.go`, `types.go`。

2) 创建日志 ring buffer：

   - 新建 `internal/infra/telemetry/diagnostics/log_buffer.go`。
   - 在应用启动后订阅 `LogBroadcaster` 并写入 ring buffer。

3) 注入探针：

   - 在 `internal/app/providers.go` 添加 `NewDiagnosticsProbe` 与 `NewDiagnosticsHub`，并通过 wire 注入到 Scheduler/Lifecycle/ServerInit。
   - 在 `internal/infra/scheduler`、`internal/infra/lifecycle/manager.go`、`internal/app/bootstrap/serverinit/manager.go` 调用 `probe.Record`。
   - 在 launcher/transport 层调用 `probe.Record` 并提供脱敏上下文。

4) 新增导出接口：

   - 在 `internal/ui/services` 添加 `debug_export.go`，实现 `ExportDiagnosticsBundle`。
   - 使用 `ServiceDeps.getCoreApp()` 获取 core app 与 registry，读取 metrics。
   - 使用 diagnostics buffers 读取 events/logs。

5) 新增 CLI（可选）：

   - 在 `cmd/mcpv` 增加 `debug export --output <path> --mode safe|deep` 子命令。

6) 测试与验证：

   - 运行 `go test ./internal/infra/telemetry/...`。
   - 运行 `go test ./internal/infra/scheduler/...`。
   - 运行 `go test ./internal/app/bootstrap/serverinit/...`。
   - 运行 `go test ./internal/ui/services/...`。

## 验收与可观察性（Validation and Acceptance）

验收标准（人工可验证）：

1) 启动 core 后，调用 Wails DebugService 的 `ExportDiagnosticsBundle`，输出 JSON 中必须包含：
   - `events` 且包含 `step` 与 `timestamp`。
   - `metrics` 文本（包含 `# HELP` 或 `# TYPE` 行）。
   - `logs` 至少包含最近 N 条（当日志流可用时）。

2) 当某个 MCP server 卡在 init starting 时，诊断包必须能体现：
   - `currentStep`（例如 `transport_connect`）。
   - `stepStartedAt` 距今的时长（说明卡多久）。
   - `lastError`（如果存在）。

3) 当启动错误发生时（例如 streamable http endpoint refused），事件时间线中出现 `phase=error` 且 error 字符串包含拒绝连接。

4) 在 `deep` 模式下，导出包 `redaction.containsSensitive=true`，且包含命令行/headers/握手摘要或完整内容。

## 幂等性与恢复（Idempotence and Recovery）

- DiagnosticsBuffer 使用固定大小 ring，不依赖外部状态，可重复初始化。
- 导出过程仅读取状态，不修改核心数据；导出失败可直接重试。
- 如果 CLI 导出失败，保留 `.bak` 文件作为回滚。

## Artifacts and Notes

期望导出的 JSON 结构（示意）：

    {
      "generatedAt": "2026-02-09T08:20:00Z",
      "snapshot": { ... },
      "metrics": "# HELP ...",
      "logs": [ {"timestamp": "...", "level": "info", "message": "..."} ],
      "events": {
        "context7": [
          {"time": "...", "step": "set_min_ready", "phase": "enter"},
          {"time": "...", "step": "transport_connect", "phase": "error", "error": "dial tcp ..."}
        ]
      },
      "stuck": {
        "context7": {"step": "transport_connect", "since": "...", "durationMs": 12345, "lastError": "dial tcp ..."}
      },
      "redaction": {"mode": "safe", "containsSensitive": false},
      "dropped": {"events": 12, "logs": 3}
    }

## Interfaces and Dependencies

需要新增的接口与类型（在 `internal/infra/telemetry/diagnostics`）：

- `type DiagnosticsEvent struct { SpecKey string; ServerName string; AttemptID string; Step string; Phase string; Timestamp time.Time; Duration time.Duration; Error string; Attributes map[string]string }`
- `type DiagnosticsProbe interface { Record(event DiagnosticsEvent) }`
- `type RingBuffer struct { Add(event DiagnosticsEvent); Snapshot() []DiagnosticsEvent; Dropped() uint64 }`

日志缓冲接口建议：

- `type LogBuffer struct { Add(entry domain.LogEntry); Snapshot() []domain.LogEntry; Dropped() uint64 }`

Scheduler/Lifecycle/ServerInit 注入 `DiagnosticsProbe`（可选，为 nil 时不记录）。在 `internal/app/providers.go` 与 `internal/app/wire_sets.go` 加入 provider 与 wire 注入。

依赖说明：

- 依赖现有 `zap` 日志与 `LogBroadcaster` 进行日志采样。
- 依赖 Prometheus registry `Gather` 与 `expfmt` 进行 metrics 导出。

变更记录：初版创建，后续扩充诊断导出、深度模式与诊断点细节。

Plan change note (2026-02-09 / Codex): Expanded plan to include launcher/transport/init retry diagnostics, deep mode with redaction, and capacity diagnostics per review feedback.

Plan change note (2026-02-09 / Codex): Marked implementation as completed, added decision about sensitive in-memory capture, and noted CLI export remains optional.

Plan change note (2026-02-09 / Codex): Set sensitive capture default to dev-only with env override per product requirement.
