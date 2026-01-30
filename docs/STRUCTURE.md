## 目录结构（当前骨架）

```
cmd/
  mcpv/             # Core CLI 入口（cobra）
  mcpv-gateway/     # MCP Gateway 入口（cobra）
  mcpv-wails/       # Wails GUI 入口
internal/
  app/              # 应用编排层，供 CLI/Wails 复用
  domain/           # 领域模型与接口
  infra/            # 适配器实现（catalog/router/scheduler/transport/...）
  ui/               # Wails 适配层（已实现）
pkg/api             # 可选的公共导出类型
frontend/           # Wails 前端资源
build/              # Wails 平台构建配置
docs/               # 规则、依赖、约束文档
PRD.md
INITIAL_DESIGN.md
go.mod
Makefile
```

- `cmd/mcpv`：只做参数解析与日志初始化，启动 core 控制面 RPC。
- `cmd/mcpv-gateway`：只做参数解析与日志初始化，启动 MCP gateway 并连接 core。
- `cmd/mcpv-wails`：Wails 应用入口，通过 `internal/ui` 桥接核心功能。
- `internal/app`：组合 catalog/scheduler/router/lifecycle/telemetry，用例编排。
- `internal/domain`：ServerSpec、Instance、Transport、Scheduler 等接口与状态机常量。
- `internal/infra`：具体实现按子目录分布（catalog loader、stdio transport、router、scheduler、lifecycle、probe、telemetry、rpc、gateway）。
- `internal/ui`：Wails 桥接层，封装核心 app 为可绑定服务，不侵入核心。
- `frontend/`：Wails 前端 UI 资源（HTML/CSS/JS）。
- `build/`：Wails 各平台构建配置与资源。
- `pkg/api`：若需要对外复用类型或接口，从这里暴露。
