## M3 技术设计（稳定性 + 能力管控 + 健康 + Tool 聚合）

> 注：当前实现已拆分为 core/gateway，MCP Server 入口由 mcpd-gateway 提供，本文中涉及的 MCP Server 逻辑现已迁移到 gateway。

### 目标
- 增强稳定性与协议正确性：能力白名单、生效的超时配置、健康探测。
- 提升运行态弹性：完善 idle/minReady/persistent 回收策略和状态管理。
- Tool 聚合：动态抓取下游 MCP servers 的 tool list，暴露统一的 mcpd tool 视图。
- 运维/退出：优雅清理、可观测性基础（日志字段规范、metrics 钩子）。

### 范围与拆分
1) 能力白名单 & 超时配置
   - 基于 initialize 返回的 capabilities 决定允许转发的方法；未声明返回 `-32601`.
   - Router 超时从配置注入（`routeTimeoutSeconds`，默认 10s）。
   - 错误码规范：busy/starting/method not allowed/route failed。

2) 健康探测 & 状态
   - 在 Lifecycle 增加 Ping（go-sdk jsonrpc ping）接口。
   - IdleManager 扫描时对 Ready/Busy 实例可选 ping，失败标记 Failed→Stop。
   - 状态字段统一（Starting/Ready/Busy/Draining/Failed/Stopped）并在日志中体现。

3) Tool 聚合
   - 设计 Aggregator：定期/按需调用下游 `tools/list`，合并工具集合暴露给 go-sdk Server。
   - 命名冲突策略：前缀命名（`{serverType}.{tool}`）或命名空间字段；可配置。
   - 缓存/刷新：启动时预拉，之后按 `toolRefreshSeconds` 刷新；失败时保留上次可用。
   - 权限/过滤：可基于 catalog `servers[].exposeTools` 标记允许暴露的工具。

4) 优雅退出 & 清理
   - 退出时 StopIdleManager + StopAll + 关闭 go-sdk Server（Run ctx cancel）。
   - 确认所有 goroutine（idle/ping）可停止；避免泄漏。

5) 可观测性基础
   - 日志字段统一：event, serverType, instanceID, state, duration_ms, error。
   - Metrics 钩子接口预留（启动/失败计数、请求延迟直方图、活跃实例 gauge），默认 Noop 实现。
   - MCP logging 通知依赖客户端调用 `logging/setLevel`，否则不会发送通知；CLI 本地日志仍由 `--log-stderr` 控制。
   - 选做：healthz handler（自检 goroutine 活性）。

### 数据流调整
- MCP Server handler：从 Aggregated tools 里路由；route tool 继续存在，但优先暴露聚合工具列表。
- 初始化：加载 catalog → scheduler/lifecycle/router → tool aggregator preload → 启动 go-sdk server。

### 配置（实现与默认值）
- `routeTimeoutSeconds`：Router 调用超时，默认 10。
- `pingIntervalSeconds`：健康探测间隔，默认 30（0 关闭）。
- `toolRefreshSeconds`：工具列表刷新间隔，默认 60（0 关闭刷新）。
- `exposeTools`: bool，默认 true。
- `toolNamespaceStrategy`: `prefix|flat`，默认 `prefix`。
- `servers[].exposeTools`: 可选工具白名单，仅暴露列表内工具。

### 测试计划
- 单元：
  - 能力白名单拒绝未声明方法。
  - Router 超时配置生效。
  - Ping 失败标记 Failed，Stop 被调用。
  - Tool 聚合：合并列表、命名冲突策略、缓存刷新。
  - 优雅退出：StopAll/StopIdleManager 调用，goroutine 可退出。
- 集成：
  - 假 MCP server 返回 capabilities/tools，客户端通过 go-sdk Client 调用聚合工具成功。
  - 可选：本地 weather MCP server（通过 `MCPD_E2E_MCP_SERVER_CMD` 指定命令）用于 tools/list 聚合验证。
  - 健康探测失败 → 实例回收/重建（如实现重建）。
  - 超时：下游阻塞时 Router 返回超时错误。

### 迭代建议
- M3a：能力白名单 + 路由超时可配 + Ping 健康 + 优雅退出完善。
- M3b：Tool 聚合（tools/list 合并、命名冲突策略、缓存）。
- M3c：可观测性（日志字段统一 + metrics 钩子 + healthz）。
