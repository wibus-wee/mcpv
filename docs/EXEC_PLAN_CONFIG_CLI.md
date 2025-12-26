# Fix config env expansion, observability port, and unix socket permissions

This ExecPlan is a living document. The sections Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective must be kept up to date as work proceeds.

This document must be maintained in accordance with .agent/PLANS.md from the repository root.

## Purpose / Big Picture

本次变更解决三类配置/CLI 瑕疵：环境变量展开不再破坏配置解析，observability HTTP 端口从配置读取而非硬编码，Unix socket 在创建后显式 chmod。完成后，含双引号的环境变量不会导致 JSON/YAML 语法崩溃；metrics/healthz 端口可在 catalog 中配置；gateway 连接 unix socket 不再受 umask 随机影响。验证方式是运行 catalog loader 测试并检查示例配置与日志输出。

## Progress

- [x] (2025-12-26 03:06Z) 设计并实现 YAML AST 级环境变量展开，避免文本替换破坏语法。
- [x] (2025-12-26 03:06Z) 为 observability HTTP 服务新增配置项并接入 app 层。
- [x] (2025-12-26 03:06Z) 为 RPC unix socket 增加可配置 chmod，并补充 schema/校验/文档/测试。
- [x] (2025-12-26 03:08Z) 运行相关测试并更新 ExecPlan 记录。

## Surprises & Discoveries

暂无。实现过程中记录任何意外行为或兼容性风险。

## Decision Log

Decision: 使用 YAML AST 展开环境变量并在 plain scalar 上做标量类型回落。
Rationale: 避免文本替换导致语法崩溃，同时保留通过 env 注入数值的行为。
Date/Author: 2025-12-26 / Codex.

Decision: observability.listenAddress 从 catalog 读取，保持 env 开关不变。
Rationale: 解决端口硬编码冲突，同时不改变现有启动开关策略。
Date/Author: 2025-12-26 / Codex.

Decision: rpc.socketMode 采用字符串配置并在 unix listen 后执行 chmod。
Rationale: 允许使用 0660/0o660 格式表达权限，避免 umask 导致的连接失败。
Date/Author: 2025-12-26 / Codex.

## Outcomes & Retrospective

尚未完成。完成后总结达成效果与残留项。

## Context and Orientation

配置加载位于 `internal/infra/catalog/loader.go`，已改为 YAML AST 级 env 展开，再做 schema 校验与 Viper 解析。observability HTTP 服务由 `internal/infra/telemetry/server.go` 启动，地址通过 catalog 的 `observability.listenAddress` 提供。RPC 服务在 `internal/infra/rpc/server.go` 里监听 unix socket，并在 listen 后执行 chmod。配置默认值定义在 `internal/domain/constants.go`，catalog schema 在 `internal/infra/catalog/schema.json`，示例配置在 `docs/catalog.example.yaml`。

## Plan of Work

首先用 YAML AST 方式实现环境变量展开：读取配置为 `yaml.Node`，只对标量值节点做 `os.ExpandEnv`，对 plain style 的值尝试按 YAML 标量规则回落成 `int/bool/float/null`，其余保持字符串。展开后再进行 schema 校验与 Viper 解析，以保证包含引号的环境变量不会破坏语法，同时保留通过 env 注入数值的行为。

然后在 `domain.RuntimeConfig` 中加入 observability HTTP 地址配置（例如 `observability.listenAddress`），在 catalog loader 中设置默认值并校验，在 app 启动时将该地址传给 `telemetry.StartHTTPServer`。保留 `MCPD_METRICS_ENABLED` 和 `MCPD_HEALTHZ_ENABLED` 作为开关。

最后在 `domain.RPCConfig` 中加入 `socketMode` 配置，catalog loader 负责解析并校验（支持 `0660`/`0o660` 格式），RPC server 在 unix listen 后执行 `os.Chmod`。更新 schema、示例配置与测试断言。

## Concrete Steps

在仓库根目录修改以下文件并保持 gofmt：

1) `internal/infra/catalog/loader.go`：引入 AST 级 env 展开，替换 `os.ExpandEnv`。
2) `internal/domain/types.go` / `internal/domain/constants.go`：新增 observability 与 rpc.socketMode 默认值与结构定义。
3) `internal/app/app.go`：使用 catalog 中的 observability.listenAddress。
4) `internal/infra/rpc/server.go`：在 unix socket listen 后 chmod。
5) `internal/infra/catalog/schema.json`：新增 observability 与 rpc.socketMode 配置描述。
6) `internal/infra/catalog/loader_test.go`：补充 env 引号、socketMode 校验与默认值断言。
7) `docs/catalog.example.yaml` 与涉及 9090 的文档：更新配置示例与说明。

格式化命令：

    gofmt -w internal/infra/catalog/loader.go internal/domain/types.go internal/domain/constants.go internal/app/app.go internal/infra/rpc/server.go internal/infra/catalog/loader_test.go

## Validation and Acceptance

运行 `go test ./internal/infra/catalog`，应全部通过。验证点包括：含双引号的 env 值不会导致 parse/config/schema 错误；`observability.listenAddress` 被正确读取并用于启动 HTTP 服务；`rpc.socketMode` 无效值会在 Load 时报错，Unix socket 在 listen 后具备指定权限。

## Idempotence and Recovery

变更为代码与配置层调整，可重复执行。若出现兼容性问题，可暂时回退 env 展开逻辑为旧的 `os.ExpandEnv` 路径，同时保留新配置字段以便逐步迁移。

## Artifacts and Notes

应在此记录关键测试输出或重要 diff 片段，确保后续排查可复现。

    go test ./internal/infra/catalog
    ok  	mcpd/internal/infra/catalog	0.340s

## Interfaces and Dependencies

新增的配置结构应保持在 `internal/domain/types.go` 中定义：

- `RuntimeConfig.Observability`（含 `ListenAddress` 字段）
- `RPCConfig.SocketMode`（string，格式为 `0660`/`0o660`）

RPC server 在 `internal/infra/rpc/server.go` 中新增 chmod 逻辑，catalog loader 在 `internal/infra/catalog/loader.go` 中新增 env 展开与 socketMode 解析。

Plan Update Note: Initial creation of the config/CLI refinement ExecPlan (2025-12-26 03:02Z).
Plan Update Note: Marked implementation progress and recorded decisions after code updates (2025-12-26 03:06Z).
Plan Update Note: Recorded catalog test execution output (2025-12-26 03:08Z).
