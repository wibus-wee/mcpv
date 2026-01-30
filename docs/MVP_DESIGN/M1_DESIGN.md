## M1 设计说明（CLI MVP 骨架）

### 目标与范围
- 搭建可编译可运行的 CLI 骨架：`mcpv serve` / `mcpv validate`。
- 实现 catalog 加载与校验（YAML/JSON，支持环境变量覆盖）；校验失败即退出非 0。
- 提供 stdio transport + initialize/ping stub，贯通实例启动的基本生命周期接口，但不做真实路由闭环。
- 打通应用装配：app 层串起 catalog → scheduler/router/lifecycle stub（占位实现可返回未实现错误或日志），为 M2 闭环做准备。

### 非目标（留给 M2+）
- 真正的 Acquire/Release 路由闭环、并发控制、stateful/persistent/singleton 语义。
- idle 回收、minReady 维持、健康探测重建。
- /metrics、healthz 端点与完整观测字段。
- 真实的 JSON-RPC 入口和响应转发。

### 协议与安全补充
- 协议版本：catalog 校验时 `protocolVersion` 必须匹配 `YYYY-MM-DD`，默认期望 `2025-11-25`。initialize 发送客户端最高支持版本，若服务端响应版本不匹配或为空，返回 JSON-RPC `-32602`（unsupported protocol version）错误。
- 握手流程：Lifecycle.StartInstance 在 transport 建连后立刻发送 initialize，校验 `protocolVersion` 与 `serverInfo/capabilities`，成功后标记 Ready，失败标记 Failed 并清理进程。
- 能力白名单：Router 需要根据服务端声明的 capabilities 限定可转发的 method，未声明能力的请求直接返回 `-32601 Method not found`；M1 记录 TODO，M2 实现。
- 健康探针：Ping 可通过 `tools/call` 或资源读取实现，失败即标记 Failed→回收/重建（留给 M2）。
- 安全：catalog 命令/env 需提醒注入风险；日志需过滤敏感 env。M1 在校验/日志中添加警示，具体过滤逻辑留待实现。

### 分层与组件责任
- `cmd/mcpv`: cobra CLI，解析 `--config`，初始化 zap 日志，调用 app。
- `internal/app`: 用例编排，暴露 `Serve(ctx, ServeConfig)` 与 `ValidateConfig(ctx, ValidateConfig)`；装配 catalog loader、transport、lifecycle（stub）、scheduler/router（stub），注入 logger。
- `internal/domain`: 领域接口与模型（ServerSpec、InstanceState、Transport、Lifecycle、Scheduler、Router、CatalogLoader、HealthProbe）。
- `internal/infra/catalog`: 读取文件（viper），环境变量覆盖，基础校验（必填、范围、协议版本非空/格式、maxConcurrent>=1、数值非负）。
- `internal/infra/transport`: 首选复用 `github.com/modelcontextprotocol/go-sdk` 的 Stdio/Command transport 封装 JSON-RPC 读写；M1 可用简化 stub（不落地 JSON-RPC），但接口签名固定。当前实现基于子进程 stdio 行分隔 JSON，未接 initialize 语义。
- `internal/infra/lifecycle`: 调用 Transport.Start，发送 initialize 请求（go-sdk 协议类型），校验协议版本，返回 Instance 并持有 conn/stop。
- `internal/infra/router` / `scheduler`: 提供 Basic 实现；scheduler 维护实例表、stateful 绑定/MaxConcurrent，router 通过 scheduler 获取实例后直接转发 JSON-RPC（无能力过滤）；后续补全 idle/minReady/健康等策略。
- `internal/infra/telemetry`: zap logger 初始化辅助（可选），metrics 占位。

### 配置与校验
- 输入：`--config` 路径，支持 YAML/JSON；使用 viper 读取，支持 `${ENV}` 覆盖。
- 结构：`servers: []ServerSpec`，字段同 PRD（name、cmd、env、cwd、idleSeconds、maxConcurrent、strategy、sessionTTLSeconds、minReady、protocolVersion）。
- 校验规则（M1 必做）：
  - name 非空；cmd 至少一项；maxConcurrent >=1；idleSeconds >=0；minReady >=0。
  - protocolVersion 非空，匹配 `YYYY-MM-DD` 简单格式（符合 2025-11-25）。
  - env 可为空；cwd 可为空；strategy 为字符串，sessionTTLSeconds 为整数。
- `mcpv validate`：加载配置并校验，错误打印并 exit 1；成功 exit 0。

### 数据与接口约定（保持稳定，后续实现填充）
- `ServerSpec`：与 PRD 对齐字段。
- `Instance`：包含 ID、Spec、State、BusyCount、LastActive、StickyKey（暂不使用）。
- `Transport.Start(ctx, spec) (Conn, StopFn, error)`：M1 返回 stub Conn，可直接错误或 no-op；为后续 stdio 实现占位。
- `Lifecycle.StartInstance(ctx, spec) (*Instance, error)`：调用 transport、做 initialize（stub），生成 Instance（State=ready）。
- `Router.Route` / `Scheduler.Acquire`：M1 可返回 `errors.New("not implemented")`，但保持签名。

### 运行流程（M1）
- `mcpv validate`: 读取 config → 校验 → 日志输出结果 → exit code。
- `mcpv serve`: 读取 config → 校验 → 初始化 transport/lifecycle/router/scheduler → 通过 stdin 接收 JSON（字段 `serverType`/`routingKey`/`payload`），路由到实例后把响应写回 stdout，直到信号退出。

### 日志与错误
- 使用 zap JSON Production 配置；字段至少含 `msg`、`config` 路径、错误详情。
- 校验失败：打印错误并返回非 0；禁止 panic。
- 运行期 stub：清晰提示未实现的路由/调度，便于 M2 对齐。

### 测试计划（M1）
- 单元：catalog loader 校验（合法/非法配置、env 覆盖、协议格式）。
- app 层：validate 正常返回 0；错误配置返回错误。
- smoke：`go test ./...` 确保 stub 代码可编译。

### 交付物
- 编译通过的 CLI，可执行 `mcpv validate --config sample.yaml`。
- 设计中的接口/目录不再变动，仅在实现中填充逻辑。
- 文档：本设计 + 依赖/结构/约束已有文档。

### 后续衔接点（为 M2 预留）
- Router/Scheduler 接口已就绪，可直接替换 stub。
- Lifecycle/Transport 接口保持不变，可落地 stdio/握手。
- 在 app 层增加路由入口（stdin JSON-RPC）时，无需调整 CLI 层。
