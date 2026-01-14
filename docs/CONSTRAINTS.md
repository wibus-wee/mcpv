## 约束与基线

- 语言/运行时：Go 1.25+（工具链 1.25.0），纯 CLI；保持纯 Go、无 CGO，兼容 Wails 3 打包。
- 传输/协议：MVP 支持 MCP stdio 子进程与外部 streamable HTTP（仅连接，不管理本地 HTTP server），stdio 遵循 MCP 2025-11-25，streamable HTTP 支持 2025-06-18/2025-03-26/2024-11-05；初始化需完成 `initialize` 握手。优先复用 `github.com/modelcontextprotocol/go-sdk` 的 Stdio/Streamable transport 与协议类型。
- 伸缩：按需启动、idle 超时回收；`stateful` 在绑定未过期时跳过回收，`persistent`/`singleton` 永不回收；`minReady` 作为激活态 warm pool，`activationMode` 控制无 caller 是否常驻。
- 并发/回压：每实例 `maxConcurrent` 硬限制，超限快速失败；启动中可返回 `starting`。
- 配置：catalog YAML/JSON + 环境覆盖，启动时校验必填与数值范围；校验失败即退出。
- 观测：stdout/stderr JSON 日志（zap），暴露启动/回收/失败等关键事件；后续 /metrics（Prometheus exporter）和可选 `healthz`。
- 安全：本地运行，不暴露远端 HTTP 接口；日志需过滤敏感 env。
- 输入输出：`serve` 模式当前 stdin 接收 `{serverType,routingKey?,payload}` JSON，stdout 输出响应 JSON；后续可换 JSON-RPC 包装。
