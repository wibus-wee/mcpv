## 基础依赖选型（MVP）

- CLI：`github.com/spf13/cobra`（命令/子命令组织），`github.com/spf13/viper`（配置加载 + 环境覆盖）。
- 日志：`go.uber.org/zap`（结构化 JSON 日志，可对接 zapcore 配置）。
- 观测：`go.opentelemetry.io/otel`、`go.opentelemetry.io/otel/sdk`、`go.opentelemetry.io/otel/exporters/prometheus`（后续暴露 /metrics）。
- RPC：`google.golang.org/grpc`、`google.golang.org/protobuf`、`go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc`（core/gateway 控制面通信与观测）。
- 并发：`golang.org/x/sync`（errgroup/semaphore）。
- 测试：`github.com/stretchr/testify`（断言/require）、`github.com/google/go-cmp/cmp`（结构 diff）。
- MCP 协议与传输：`github.com/modelcontextprotocol/go-sdk`（内置 Stdio/Command transport、initialize/ping 协商、工具/资源/提示等协议对象）。

版本目前在 `go.mod` 预先锁定，后续联网可按需 `go mod tidy`/升级。保持纯 Go、无 CGO，兼容 Wails 3 打包。由于 go-sdk 目标 `go 1.24`，项目 go 版本提升至 1.24（工具链 1.24.10）以兼容。
