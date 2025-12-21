## 目录结构（当前骨架）

```
cmd/mcpd            # CLI 入口（cobra）
internal/app        # 应用编排层，供 CLI/Wails 复用
internal/domain     # 领域模型与接口
internal/infra      # 适配器实现（catalog/router/scheduler/transport/...）
internal/ui         # 预留 Wails 接入层
pkg/api             # 可选的公共导出类型
docs                # 规则、依赖、约束文档
PRD.md
INITIAL_DESIGN.md
go.mod
```

- `cmd/mcpd`：只做参数解析与日志初始化。
- `internal/app`：组合 catalog/scheduler/router/lifecycle/telemetry，用例编排。
- `internal/domain`：ServerSpec、Instance、Transport、Scheduler 等接口与状态机常量。
- `internal/infra`：具体实现按子目录分布（catalog loader、stdio transport、router、scheduler、lifecycle、probe、telemetry）。
- `internal/ui`：未来 Wails 桥接，不侵入核心。
- `pkg/api`：若需要对外复用类型或接口，从这里暴露。
