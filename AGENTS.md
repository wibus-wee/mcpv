# Repository Guidelines

## 项目结构与模块职责
- `cmd/mcpd`：CLI 入口，仅做 flag 解析与日志开关，默认配置文件名为 `catalog.yaml`。
- `internal/app`：编排 catalog、scheduler、router、lifecycle、transport，供 CLI/Wails 复用。
- `internal/domain`：核心接口与状态模型，保持纯净无外部依赖；`pkg/api` 对外导出共享类型。
- `internal/infra`：适配器实现，子目录含 `catalog` loader、`scheduler`、`router`、`transport`(stdio)、`lifecycle`、`probe`、`telemetry`；`internal/ui` 预留；测试与被测代码同目录。
- `docs`：设计/约束文档与 `docs/catalog.example.yaml` 示例配置，修改配置或协议时同步更新。

## 构建、测试与开发命令
- `make build` 编译全部包；`make test` 运行所有单测。
- `make fmt` 触发 `go fmt ./...`；`make vet` 做静态检查；`make tidy` 清理/同步依赖。
- 运行服务：`make serve CONFIG=docs/catalog.example.yaml`；仅验证配置：`make validate CONFIG=docs/catalog.example.yaml`；也可 `go run ./cmd/mcpd serve --config <path>`.
- Go 版本要求 `go 1.25+`，使用 `go.mod` 中指定版本。

## 编码风格与命名
- 采用标准 Go 风格（tabs 缩进，`gofmt` 即规范），禁止自定义格式或手写对齐。
- 包/目录名使用小写无下划线；导出标识符用 `PascalCase`，非导出用 `camelCase`；接口命名偏向行为而非器件名。
- 保持 domain 层无耦合，infra 子目录隔离职责；新增文件紧贴所属模块放置，同目录补充测试。
- 避免防御式、过度抽象代码，优先清晰逻辑流；必要时用简短注释解释意图而非复述代码。

## 测试准则
- 框架：标准库 `testing` 搭配 `testify/assert`，优先表驱动覆盖。
- 测试文件与被测文件同目录，命名以 `_test.go` 结尾，函数名遵循 `TestXxx_Subcase`。
- 并发/时序路径（路由、生命周期、调度、idle 管理）需覆盖正常与错误分支，使用 `context.WithTimeout` 控制运行时长。
- 调整 CLI 或配置解析时附带 `make test` 结果，必要时在 `docs/catalog.example.yaml` 加入最小可运行示例。

## 提交与 PR 指南
- 提交信息遵循 Conventional Commits（示例：`feat: ...`、`fix: ...`），与现有历史保持一致。
- PR 描述需说明动机、主要接口/配置影响、测试范围，关联相关 issue/设计文档。
- PR 体量保持聚焦，避免跨域改动；引入新配置或协议时写明兼容/迁移策略与默认值。
- 提交前至少运行 `make fmt vet test`；触及运行路径时附上推荐命令示例（如 `--config` 用法）与验证结果。

## ExecPlans

When writing complex features or significant refactors, use an ExecPlan (as described in .agent/PLANS.md) from design to implementation.

## Additional AGENTS.md Files

if there is AGENTS.md in subdirectories, please also follow the guidelines described in those files.