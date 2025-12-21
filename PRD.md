下面是 “mcpd” 的详细 PRD（产品需求文档），聚焦弹性 MCP 聚合器，单机 CLI MVP，可演进到 Wails3 桌面端。正文中文，所有代码/标识符/CLI 命令使用 English。

一、目标与定位
- 目标：为 MCP servers 提供按需启动、自动缩容、scale-to-zero 的弹性运行时，统一入口路由，降低空闲资源浪费，简化配置管理。
- 场景：本地开发/CI/轻量单机部署；后续可扩展到远端/容器/K8s。
- 约束：MVP 以 stdio transport 子进程方式管理 MCP server；兼容 MCP 2025-11-25 规范；Go 实现，纯 CLI，后续 Wails3 共用核心。

二、用户画像
- 主要：前端/全栈/AI 工程师，需要快速组合多个 MCP server，降低本机资源占用，避免手工启动/重启。
- 次要：DevOps/平台同学，希望在小型环境下获得“轻量 k8s for MCP”的体验。

三、范围（MVP）
- 支持 stdio transport 启动 MCP server（子进程）。
- 请求路由：单入口接受 JSON-RPC payload，指定 serverType，选择实例，转发请求并返回响应。
- 弹性策略：按需启动，无流量/空闲超时自动回收；支持标记 sticky/persistent 跳过回收；支持 MinReady 保温。
- Catalog 配置：声明 server types（命令、env、cwd、idleSeconds、maxConcurrent、sticky、persistent、minReady、protocolVersion），启动时加载并校验。
- 健康与握手：启动后执行 initialize 协商，校验 protocolVersion；支持 ping 探活。
- 观测：结构化日志；基础 metrics（启动耗时、启动失败计数、活跃实例、回收计数、请求延迟/失败率）。
- CLI：`mcpd serve`（运行）、`mcpd validate`（校验配置）；输出 JSON 结构化日志。
- 错误与 backpressure：并发超限可快速失败；启动中可返回“启动中”错误（不排队为 MVP）。

四、超出范围（MVP 后）
- Streamable HTTP/SSE transport；MCP Authorization。
- 远端/容器/K8s runtime 适配。
- 预热池高级策略、优先级队列、任务排队。
- 多租户隔离、ACL、鉴权。
- UI（Wails3）——仅在后续阶段。

五、功能需求（详细）
1) Catalog
- 输入：YAML/JSON 文件，支持环境变量覆盖。
- 字段：name, cmd[], env{key:val}, cwd, idleSeconds, maxConcurrent, sticky, persistent, minReady, protocolVersion (预期版本)。
- 校验：必填 name/cmd；数值非负；protocolVersion 必须与 MCP 规范列表匹配（至少检查非空+格式）；maxConcurrent >=1。
- 运行时：加载一次，失败即退出；可选热加载（非 MVP）。

2) 启动与握手
- Lifecycle.StartInstance：调用 Transport.Start → 建立 Conn → 发送 initialize 请求，校验响应 protocolVersion 与 serverInfo/capabilities；握手失败则终止进程，计入启动失败。
- 状态机：Starting → Ready → Busy → Draining → Stopped/Failed。
- 超时：启动与握手有可配置超时时间；超时视为失败。

3) 路由与调度
- Router.Route(serverType, routingKey, payload):
  - Acquire：优先选择 Ready 且 Busy < maxConcurrent 的实例；没有则启动新实例（受并发保护）。
  - Sticky：若 spec.Sticky=true，按 routingKey 绑定实例；找不到时可新建。
  - 失败策略：无可用且启动失败 → 返回错误；不实现排队（MVP）。
- Release：请求完成后 Busy--，更新 LastActive，若 Busy=0 → Ready。

4) 弹性缩容
- IdleManager：周期扫描实例表，若非 sticky/persistent 且 lastActive 超过 idleSeconds → Draining → StopInstance。
- MinReady：保持至少 minReady 个 Ready 实例（不回收）。

5) 健康检查
- Probe：对 Ready/Busy 实例定期 ping；失败标记 Failed → 可尝试重建（MVP 可简单停止）。

6) 观测
- 日志：zap JSON，字段至少包含 timestamp, level, msg, serverType, instanceID, state, error。
- Metrics（Prometheus 端点，默认可选）：启动耗时 histogram，启动失败 counter，活跃实例 gauge，回收 counter，请求延迟 histogram，请求失败 counter。
- 健康端点：可选 `healthz`（检查 goroutine 活性/内部错误）。

7) CLI
- `mcpd serve --config path`: 加载配置，启动调度器，启动 metrics/health HTTP（可选端口）。
- `mcpd validate --config path`: 仅校验配置（CUE/JSONSchema），返回 0/1。
- 输入/输出：路由入口为标准输入的 JSON-RPC 请求（或后续 HTTP 入口，非 MVP）。

8) 错误处理与 backpressure
- 并发超限：返回错误 `busy`。
- 启动中：可返回 `starting` 错误。
- 连接中断：标记实例 Failed，释放资源。

六、非功能需求
- 语言：Go 1.22+。
- 依赖：cobra, viper, zap, opentelemetry (prom exporter), cue/jsonschema, testify/go-cmp, x/sync（errgroup/semaphore），纯 Go。
- 可移植性：无 CGO；兼容 Wails3 打包。
- 性能：单机支撑数十并发请求、数十实例；冷启动主要由下游 server 决定。
- 安全：MVP 运行在本地；不暴露远端 HTTP 入口；日志不泄漏敏感 env（需显式过滤）。

七、架构与分层
- cmd/mcpd: CLI 入口。
- internal/app: 应用服务编排（装配 scheduler/router/lifecycle/telemetry）。
- internal/domain: 领域模型与接口（ServerSpec, InstanceState, Transport, Scheduler, Router, Lifecycle, Probe, Logger, Metrics）。
- internal/infra: 适配器实现
  - transport/stdio
  - catalog/loader（Viper + CUE/JSONSchema）
  - scheduler/basic
  - router/router
  - lifecycle/manager
  - probe/ping
  - telemetry/logging (zap), metrics (OTel+Prom)
- pkg/api: 对外可复用类型（可选）。
- internal/ui: 预留 Wails 入口（后续）。

八、配置示例（catalog.yaml）
```yaml
servers:
  - name: git-helper
    cmd: ["./mcp-git-helper"]
    cwd: "/path/to/git"
    env:
      TOKEN: "${GIT_TOKEN}"
    idleSeconds: 60
    maxConcurrent: 4
    sticky: false
    persistent: false
    minReady: 0
    protocolVersion: "2025-11-25"

  - name: vector-store
    cmd: ["./mcp-vector"]
    idleSeconds: 300
    maxConcurrent: 2
    sticky: true
    persistent: true
    minReady: 1
    protocolVersion: "2025-11-25"
```

九、接口契约（关键）
- Transport.Start(ctx, spec) → (Conn, StopFn, error)
- Conn.Send/Recv/Close：发送/接收 JSON-RPC 消息（换行分隔），需阻塞/超时。
- Lifecycle.StartInstance：封装 Transport + initialize 握手 + probe。
- Scheduler.Acquire/Release：实例选择与状态更新。
- Router.Route：对外入口（serverType, routingKey, payload）→ response。

十、状态机与时序
- StartInstance: exec → initialize → Ready → Acquire → Busy → Release → Ready → Idle timeout → Draining → Stop.
- 失败：initialize/ping 失败 → Failed → 清理/可重建。

十一、日志与事件
- 关键事件：start_attempt, start_success, start_failure, initialize_failure, ping_failure, route_error, idle_reap, stop_success, stop_failure。
- 字段：serverType, instanceID, state, duration_ms, error。

十二、测试计划
- 单元：catalog 验证；scheduler Acquire/Release；lifecycle（模拟 transport）；router 路由。
- 集成：假 MCP server（回显 + ping），验证启动、initialize、route、idle 回收。
- 负面：启动超时、initialize 失败、ping 失败、并发超限。

十三、里程碑（建议）
- M1：项目骨架 + catalog loader + stdio transport + initialize stub + CLI validate。
- M2：scheduler/router/lifecycle 完成，基础路由闭环；日志。
- M3：idle 回收 + metrics + ping probe；集成测试。
- M4：prom/health 端点，配置校验增强，稳定性打磨。
- M5（后续）：HTTP transport + auth + Wails UI 适配。

十四、风险与缓解
- 冷启动慢：提供 minReady/idleSeconds 配置；允许 prewarm。
- 状态粘性：sticky/persistent 标记防误回收；默认非粘性。
- 配置错误：启动前校验并阻断；错误信息清晰。
- 进程泄漏：StopFn 实现需优雅终止，超时强杀；注册退出钩子。

十五、验收标准（MVP）
- `mcpd serve --config catalog.yaml` 能加载并运行，无 panic；能处理至少一个 serverType 的 JSON-RPC 请求并返回响应。
- idleSeconds 到期自动回收实例（非 sticky/persistent）。
- 日志包含关键事件，metrics 能暴露（可选）。
- `mcpd validate` 对合法配置返回 0，对非法配置输出错误并返回非 0。
