## M2.5 设计：mcpd 作为标准 MCP Server 暴露

> 该设计已被 core/gateway 拆分方案替代：mcpd 仅提供 gRPC 控制面，MCP Server 入口迁移至 mcpd-gateway。

### 目标
- 将当前路由器（stdin JSON-RPC `route`）改为标准 MCP Server，实现 MCP initialize/ping/方法调用，由 go-sdk 管理会话与协议，客户端（MCP Client）可直接连接。
- 继续复用现有 Scheduler/Router/Lifecycle/Transport，对外协议符合 MCP 2025-11-25。

### 方案概述
- 使用 go-sdk `mcp.NewServer` 创建 MCP Server；使用 go-sdk `mcp.StdioTransport` 或 HTTP transport（后续）。
- initialize、ping、会话管理等由 go-sdk 处理；我们在工具/资源/prompt/任务方法中，将请求转发到当前 Router。
- 在 initialize 过程中加载 catalog，启动 Scheduler/Lifecycle；Server 会话结束时调用 Scheduler.StopAll 清理实例。

### 数据流
1) MCP Client 连接 → go-sdk Server 接管 stdio → initialize 协商（go-sdk 自动）。
2) MCP 方法调用（例如 tools/call 或自定义 method）进入 handler → 解析参数中的 `serverType`/`routingKey`/`payload` → 调用 Router.Route → 返回结果作为 MCP 响应。
3) ping/keepalive 由 go-sdk 处理；我们可将 Router 的 send/recv 超时与健康探测融入。

### Handler 设计
- 定义一个工具（例如 `route_tool` 或专用 method）参数：
  ```json
  { "serverType": "echo", "routingKey": "", "payload": { "jsonrpc":"2.0","id":1,"method":"ping" } }
  ```
  Handler 将参数转发给 Router.Route 并返回响应 JSON 作为工具结果。
- 可同时暴露一个简单的 `ping` tool 直接调用 Router.Route。
- 能力声明：在 ServerCapabilities 中声明 tools；若需要资源/prompt，可留空或实现空列表。

### 生命周期与清理
- Server 启动时：加载 catalog，构建 Scheduler/Lifecycle/Router。
- 会话关闭时：调用 Scheduler.StopAll（清理子进程）。
- 进程退出信号：同样 StopIdleManager + StopAll。

### 日志与 IO
- 保持日志输出到 stderr；go-sdk 会向客户端发送 notifications/logging（可默认关闭或映射）。
- stdout 仅用于 MCP 协议通信。

### 超时与错误
- Router 已有超时；可在 handler 层设置上限（例如 10s）。
- 能力白名单：基于 catalog/initialize 返回的 capabilities 决定允许的 methods；无权限返回 `-32601`.

### 步骤拆解（实现计划）
1) 在 app 层新增 mode：使用 go-sdk `mcp.Server` + `StdioTransport`；移除自定义 stdin decoder。
2) 在 infra 层添加 MCP handler 适配器，将 tools/call 代理到 Router.Route。
3) 初始化时加载 catalog，创建 Scheduler/Lifecycle/Router，注册 handler 后运行 server.Run(ctx, transport)。
4) 关闭时 StopIdleManager + StopAll。
5) 调整文档/示例，指导 MCP Client 直接连接 mcpd。

### 测试计划
- 单元：handler 转发参数解析正确；无权限方法返回 `-32601`。
- 集成：使用假 MCP server（cat echo），客户端通过 go-sdk Client 连接 mcpd，调用工具成功返回 echo；会话关闭后实例被 StopAll。

### 注意事项（后续合并工具列表）
- 需要支持动态抓取下游 MCP Server 的 `tools/list`，合并到 mcpd 自身的 tool list（通过 go-sdk Server 注册工具）。合并时要处理命名冲突（可加前缀或命名空间）、权限过滤，以及缓存/过期策略以避免每次调用都广播。
