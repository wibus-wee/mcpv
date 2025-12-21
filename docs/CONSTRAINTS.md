## 约束与基线

- 语言/运行时：Go 1.22+，纯 CLI；保持纯 Go、无 CGO，兼容 Wails 3 打包。
- 传输/协议：MVP 仅支持 MCP stdio 子进程，遵循 MCP 2025-11-25 协议版本校验；初始化需完成 `initialize` 握手。
- 伸缩：按需启动、idle 超时回收；`sticky`/`persistent` 跳过回收；`minReady` 保温。
- 并发/回压：每实例 `maxConcurrent` 硬限制，超限快速失败；启动中可返回 `starting`。
- 配置：catalog YAML/JSON + 环境覆盖，启动时校验必填与数值范围；校验失败即退出。
- 观测：stdout/stderr JSON 日志（zap），暴露启动/回收/失败等关键事件；后续 /metrics（Prometheus exporter）和可选 `healthz`。
- 安全：本地运行，不暴露远端 HTTP 接口；日志需过滤敏感 env。
