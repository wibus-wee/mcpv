> 这是最初版的设计，可能已经过时  
> 当前实现已演进为 core/gateway 分离：`mcpd` 作为控制面通过 gRPC 暴露 API，`mcpd-gateway` 承担 MCP Server 入口并桥接 tools/list 与 tools/call。

可以用少量成熟三方库把工程基线做扎实，同时保持依赖收敛。推荐一套“核心必选 + 可选增强”清单，并注明作用与取舍：

核心必选（工程完备度高且稳定）  
- CLI：`spf13/cobra` + `spf13/viper`（命令组织 + 配置/环境覆盖，一致的 CLI UX）。  
- 日志：`go.uber.org/zap`（高性能结构化日志，生态成熟），如需标准库接口可加 `slog-zap` bridge。  
- 配置校验：`cuelang.org/go/cue`（声明式约束），或轻量 `github.com/xeipuuv/gojsonschema`。  
- Metrics/Tracing：OpenTelemetry Go SDK（`go.opentelemetry.io/otel/...`）+ Prometheus exporter（`go.opentelemetry.io/otel/exporters/prometheus`）。  
- 测试：`github.com/stretchr/testify`（断言/require），`github.com/google/go-cmp/cmp`（结构对比）。  
- 进程/JSON：标准库 `os/exec`、`encoding/json` 足够；如需更快序列化可选 `json-iterator/go`。
- MCP SDK：model context protocol 官方推出了 go-sdk 方便我们开发，在开发中一定要绝对能用它就用它

可选增强（看阶段和复杂度决定）  
- HTTP 路由（未来接 Streamable HTTP）：`github.com/go-chi/chi`（轻量、可组合中间件）或 `gorilla/mux`（更重但老牌）。  
- DI/生命周期管理：`go.uber.org/fx`（帮你组合模块、管理启动/关闭），但会增加抽象；若想保持简单，可用手写 wire-up 或 `google/wire` 生成器。  
- 并发工具：标准库足够；若要限流/信号量，可用 `golang.org/x/sync/semaphore`；若要 errgroup，用 `golang.org/x/sync/errgroup`。  
- 配置热加载：Viper 自带简单热加载，如需更强一致性可结合 fsnotify；不必引入更多库。  
- 结构化日志格式：如需 JSON 日志字段约束，可用 `zapcore.EncoderConfig` 自定义；无需额外库。

理由与取舍  
- 这些库都有较高采纳度、文档齐全，避免造轮子（CLI、配置、日志、观测、校验、测试）。  
- 依赖面保持小，便于长周期维护；未来加 HTTP/K8s/容器 runtime 再按需引入客户端 SDK。  
- CUE/JSON Schema 校验能在启动时阻断坏配置，对声明式 catalog 很关键。  
- OpenTelemetry + Prometheus 覆盖 metrics/trace，后续可无痛接观测平台。

兼容性没问题。这套依赖都是纯 Go（zap、cobra、viper、cue/jsonschema、otel/prometheus、x/sync、testify/go-cmp），用 Go modules，Wails 3 打包不会受限。建议：

- 分层清晰：把核心放 `/internal`（catalog/transport/scheduler/router/lifecycle/metrics），CLI 放 `/cmd/cli`，未来 Wails 3 直接复用核心包，换一个 UI 壳（前端调用 Go 方法）。
- 避免 CGO/重 ABI 依赖：目前推荐的库都是纯 Go，适合多平台打包。
- 前后端交互：Wails 调用导出 Go 方法；保持核心 API 纯函数/接口（接受 context、结构体入参，返回结果/错误），前端只做展示和配置。
- 进程模型：Wails 窗口生命周期与后台实例管理解耦；把实例管理作为后台 goroutine/service，UI 仅发命令/查状态。
- 观测输出：保留 stdout/stderr JSON 日志和 /metrics HTTP 暴露；Wails 内可嵌一个 metrics 面板或转发到外部。
- 配置：仍用 Viper/CUE 校验，Wails 端提供表单生成/校验提示即可。

如果确认走这路子，我可以给出目录/包划分草案和核心接口签名，让 CLI 与未来 Wails 共用同一核心。


基于可演进、可复用（CLI + 未来 Wails）的目标，建议采用“核心域 + 适配器”分层，偏六边形/洁净架构风格，最小必要抽象，保持 Go idiomatic：

分层与目录（示例）
- `/cmd/cli`: Cobra 入口，仅做参数解析→调用应用服务。
- `/internal/app`: 应用服务层（用例编排），暴露方法给 CLI/Wails；组合 scheduler/router/catalog/lifecycle。
- `/internal/domain`: 纯领域模型与接口（ServerSpec、InstanceState、Transport、Launcher、Scheduler、Router、HealthProbe、MetricsSink）。
- `/internal/infra`: 适配器实现
  - `transport/stdio.go`（MVP），预留 `transport/http.go`
  - `catalog/loader.go`（Viper + CUE/JSONSchema 校验）
  - `scheduler/basic.go`（idle 回收、minReady、sticky/persistent）
  - `router/router.go`（实例选择、并发计数、sticky key）
  - `lifecycle/manager.go`（启动/停止状态机，drain，重建）
  - `telemetry/logging.go`（zap）、`telemetry/metrics.go`（OTel/Prom）
  - `probe/ping.go`（MCP ping）
- `/internal/ui`（未来 Wails）: 仅包装 app 层 API，不侵入核心。
- `/pkg/api`（可选）: 对外暴露的公共类型/接口（若需要被其他模块导入）。

核心接口草案（domain）
- Transport:
  - `type Conn interface { Send(ctx context.Context, msg []byte) error; Recv(ctx context.Context) ([]byte, error); Close() error }`
  - `type Transport interface { Start(ctx context.Context, spec ServerSpec) (Conn, StopFn, error) }`
- Catalog:
  - `type ServerSpec struct { Name string; Cmd []string; Env map[string]string; Cwd string; IdleSeconds int; MaxConcurrent int; Sticky bool; Persistent bool; MinReady int; ProtocolVersion string }`
  - Loader 返回 `map[string]ServerSpec` + 校验。
- Router/Scheduler:
  - `Acquire(ctx, serverType, routingKey) (*Instance, error)` 选择/启动实例
  - `Release(instance)` 更新 lastActive/busy count
  - Scheduler 维护实例表，策略：空闲回收、最小副本、重建、不选择 draining/starting。
- Lifecycle:
  - `StartInstance(ctx, spec)` → spawn + initialize（握手校验版本/capabilities）
  - `StopInstance(ctx, inst, reason)` → drain + kill
- Probe:
  - `Ping(ctx, conn)` 健康探活（MCP `ping`）
- Metrics/Logging:
  - 事件：启动耗时、启动失败、活跃实例 gauge、回收计数、请求延迟/失败率、ping 失败率。
  - 日志：结构化，含 serverType/instanceID/phase。

状态机（实例）
- States: Starting → Ready → Busy → Draining → Stopped/Failed
- 转移：
  - Starting 超时/失败 → Failed → 可重建
  - Ready/Bizy idle 计时；Draining 拒绝新请求，完成后停止
  - Persistent/Sticky 跳过 idle 回收；MinReady 保温

调度与路由策略
- 首选 Ready 且未超 MaxConcurrent；若无可用则启动新实例（受并发上限保护）。
- Sticky：routingKey 绑定实例；无法满足时可新建或返回错误（策略可配置）。
- Backpressure：达并发上限时可排队或快速失败；CLI 先用快速失败+可选等待超时。

配置/校验
- Catalog YAML/JSON + CUE/JSONSchema 校验启动即阻断坏配置。
- 环境变量覆盖（Viper）。
- 可选热加载：fsnotify + 原子替换 catalog；实例策略由 scheduler 感知。

观测与可运维性
- `/metrics` HTTP 端点（即便 CLI 模式也可开启本地端口）。
- `healthz`（自检 scheduler/lifecycle goroutine 活性）。
- 事件日志（启动/停止/回收/重建）便于调试缩容策略。

测试策略
- 单元：scheduler/router/lifecycle 状态机、catalog 校验。
- 集成（带假 MCP server fixture）：启动→initialize→ping→route→idle 回收。
- 负面：启动超时、initialize 失败、ping 失败重建。

演进路径
- 二期：`transport/http` 支持 streamable HTTP + auth；多节点 runtime 适配（k8s/OCI）。
- 预热池：MinReady>0；优雅扩/缩：并发上限动态、延迟回收。
- 粘性增强：session lease/TTL、长任务 drain window。

这样分层可以确保：核心逻辑纯 Go，无 UI/CLI 绑死；Wails 仅是另一适配器。若 OK，我可以细化目录树和关键文件骨架（空接口/struct/注释），便于开 repo 即写。
