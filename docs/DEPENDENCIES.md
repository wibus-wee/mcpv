## 基础依赖选型（MVP）

- CLI：`github.com/spf13/cobra`（命令/子命令组织），`github.com/spf13/viper`（配置加载 + 环境覆盖）。
- 日志：`go.uber.org/zap`（结构化 JSON 日志，可对接 zapcore 配置）。
- 观测：`go.opentelemetry.io/otel`、`go.opentelemetry.io/otel/sdk`、`go.opentelemetry.io/otel/exporters/prometheus`（后续暴露 /metrics）。
- 并发：`golang.org/x/sync`（errgroup/semaphore）。
- 测试：`github.com/stretchr/testify`（断言/require）、`github.com/google/go-cmp/cmp`（结构 diff）。

版本目前在 `go.mod` 预先锁定，后续第一次联网 `go mod tidy` 时可根据兼容性微调。默认保持纯 Go、无 CGO，兼容 Wails 3 打包。
