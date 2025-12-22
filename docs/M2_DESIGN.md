## M2 技术设计（调度闭环 + 弹性 + 健康 + 观测）

### 目标
- 打通完整路由闭环：CLI stdin JSON-RPC 入口 → Router → Scheduler → Lifecycle → MCP server → 响应返回。
- 引入弹性与健康：Idle 回收、minReady 预热、persistent/sticky 保护、ping 健康探测。
- 增强协议正确性：initialize 结果校验（protocolVersion/serverInfo/capabilities）、方法能力白名单、错误码语义。
- 基础观测：结构化日志关键字段；预留 metrics 钩子。

### 范围内
- Router：能力检查、错误语义（starting/busy/failed）、request/response JSON-RPC 转发。
- Scheduler：实例表 + 状态机（Ready/Busy/Draining/Failed/Stopped），sticky 绑定、maxConcurrent 硬限制、minReady 维护、idleSeconds 驱动回收（定时扫描）。
- Lifecycle：用 go-sdk 协议完成 initialize（校验 protocolVersion/serverInfo/capabilities），暴露 Ping；Stop 优雅→超时→强杀；实例状态更新。
- Transport：沿用 go-sdk CommandTransport；StopFn 使用 go-sdk Close（终止管道）。
- 健康：周期 ping Ready/Busy 实例，失败→标记 Failed→回收/重建策略（M2 可先停止，重建留待 M3）。
- I/O 契约：stdin JSON-RPC 2.0（method `route`），error code 语义化。
- 测试：单元 + 集成（假 MCP server echo+ping），覆盖超限、初始化失败、ping 失败、idle 回收、sticky/persistent 行为。

### 关键设计

#### Router
- 接口：`Route(ctx, serverType, routingKey, payload json.RawMessage) (json.RawMessage, error)`
- 能力检查：基于 catalog + initialize 结果（server capabilities）确定允许 method 列表；未声明能力返回 `-32601 Method not found`。
- 错误码映射：并发超限 → `busy` (`-32002` 自定义)；启动中/无实例 → `starting`; sticky 满载 → `busy`; 下游错误透传 JSON-RPC error。
- 实现：Acquire → 用 go-sdk `jsonrpc.DecodeMessage/EncodeMessage` 转发（避免手写 JSON）→ Release；在 Acquire 失败时按错误分类返回。

#### Scheduler
- 状态机：Starting → Ready ↔ Busy → Draining → Stopped/Failed。
- 数据结构：`instances map[serverType][]*trackedInstance`，`sticky map[serverType]map[routingKey]*trackedInstance`。
- Acquire：
  - sticky: 命中实例且 BusyCount<maxConcurrent → markBusy；否则若满载 → 返回 sticky busy。
  - 非 sticky：优先 Ready 且 BusyCount<maxConcurrent；否则启动新实例（并发保护，后续可用 semaphore）。
  - markBusy: BusyCount++，State=Busy，LastActive=now。
- Release：BusyCount--，BusyCount=0→State=Ready，更新 LastActive。
- IdleManager：定时扫描（ticker），对非 sticky/persistent 且 IdleSeconds 超时的 Ready 实例 → Draining → StopInstance；MinReady：每个 serverType 至少保留 N 个 Ready（不回收）。
- Failed 实例：Release 时若发现 State=Failed，则直接 Stop 清理。

#### Lifecycle
- StartInstance：
  - Transport.Start → 使用 go-sdk `mcp.InitializeParams` + `jsonrpc.EncodeMessage` 发送 initialize，校验 protocolVersion == catalog & supported list，校验 serverInfo/capabilities 非空。
  - 可选：直接使用 go-sdk `Client.Connect` 建立 `ClientSession`（若路由需要更高层 API），但为通用转发可继续用低层 `Connection`。
- Ping：优先用 go-sdk `jsonrpc.NewCall` 构造 ping 或 `ClientSession.Ping`（若使用 session），失败标记 Failed。
- StopInstance：关闭 Conn → StopFn；State=Stopped；记录 reason。

#### Transport
- 继续使用 go-sdk `CommandTransport`；StopFn 调用 Close（内部 SIGTERM/SIGKILL）。
- Env/Cwd 由 catalog 提供；日志需过滤敏感 env（调用方处理）。

#### CLI / I/O 契约
- stdin 请求：
```json
{"jsonrpc":"2.0","id":1,"method":"route","params":{"serverType":"echo","routingKey":"","payload":{"jsonrpc":"2.0","id":1,"method":"ping"}}}
```
- stdout 响应：`{"jsonrpc":"2.0","id":1,"result":{...}}` 或 `{"jsonrpc":"2.0","id":1,"error":{"code":<code>,"message":"..."}}`
- 错误码建议：`-32600` invalid request，`-32601` method not found（能力未声明），`-32001` route failed，`-32002` busy，`-32003` starting。

### 日志与观测（M2 底线）
- zap JSON：字段至少 serverType、instanceID、state、event（start_success/start_failure/route_error/idle_reap/ping_failure/stop_success）、duration_ms、error。
- 预留 metrics 接口（计数/直方图），实现可放在 M3。

### 测试计划
- 单元：
  - lifecycle: initialize 校验（成功/协议不匹配/错误响应）、ping 失败标记 Failed。
  - scheduler: sticky 绑定、MaxConcurrent、idle 回收（借助手动调用 IdleManager）、minReady 保温。
  - router: 能力拒绝（模拟 capabilities），busy/starting/unknown server 错误路径。
- 集成：
  - 假 MCP server（/bin/sh -c cat + stub initialize/ping）：Route echo 成功。
  - 初始化失败（假 server 返回错误）→ StartInstance 失败。
  - idle 回收：设置 idleSeconds=0，Release 后触发 IdleManager 停止。

### 演进留白
- 重建策略：Failed/Stopped 后自动重建（可选标记）留到 M3。
- Metrics/healthz 完整实现留到 M3。
- 多入口（HTTP/streamable）后续版本扩展。
