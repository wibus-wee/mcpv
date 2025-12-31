# Wire DI 重构与配置访问器抽象

这是一个持续更新的 ExecPlan。必须遵循仓库根目录下的 `.agent/PLANS.md` 规范，并在执行过程中持续更新 `Progress`、`Surprises & Discoveries`、`Decision Log`、`Outcomes & Retrospective` 四个章节。

## Purpose / Big Picture

完成后，Core 的依赖注入将由 Wire 生成并在代码层面可验证，`app.Serve` 与 `app.NewControlPlane` 中的手写组装将被彻底替换。配置读取将被抽象为访问器接口，组件不再直接依赖 YAML 解析结果，从而为未来热重载改造铺路。验证方式是：CLI 与 Wails 启动行为保持一致，配置加载日志与 RPC/Observability 行为与当前一致，且 Wire 生成文件存在并能通过构建。

## Progress

- [x] (2025-12-30 16:47Z) 编写 ExecPlan 并确认目标依赖图与接口列表。
- [x] (2025-12-30 17:25Z) 实现配置访问器与配置汇总重构，更新 Application/ControlPlane/ServerInitializationManager 的依赖签名。
- [x] (2025-12-30 17:25Z) Wire ProviderSet 与注入器落地（含 `cmd/mcpd/wire.go` 与生成文件），并更新 CLI/Wails/UI 的调用链。
- [ ] 通过编译与测试验证，补齐必要文档或注释。

## Surprises & Discoveries

暂无。

## Decision Log

- Decision: 在 `internal/app` 引入 `Application` 作为运行期容器，保留 `App` 作为启动入口，但将 `Serve` 改为调用 Wire 注入器。
  Rationale: 兼顾现有 UI 调用路径与“消除手写 DI”的要求，同时便于 CLI 使用 Wire 直接构建。
  Date/Author: 2025-12-30 / Codex

- Decision: 在 `internal/domain` 定义 `CatalogAccessor` 接口，并提供 `StaticCatalogAccessor` 实现，以 `ProfileStore` 作为唯一的配置源。
  Rationale: 以最小接口解耦 YAML 值与组件构建，同时不引入 domain 对 app 的耦合。
  Date/Author: 2025-12-30 / Codex

- Decision: 引入 `CatalogSnapshot` 作为配置快照对象，Wire 图中仅注入快照对象而非 map/struct 配置值。
  Rationale: 满足“组件依赖访问器而非静态值”的约束，同时避免重复计算 profile summary。
  Date/Author: 2025-12-30 / Codex

- Decision: CLI 入口不保留独立 Wire 注入器，直接调用 `app.InitializeApplication`。
  Rationale: CLI 不需要额外注入边界，减少样板文件与生成负担。
  Date/Author: 2025-12-30 / Codex

## Outcomes & Retrospective

待完成后补充。

## Context and Orientation

当前 Core 的组装逻辑集中在 `internal/app/app.go` 的 `Serve` 方法中：它负责加载配置、构建 scheduler/router/aggregator/control-plane/rpc server，并启动 observability 与后台管理协程。`internal/app/control_plane.go` 中的 `NewControlPlane` 还会直接创建内部服务（registry/discovery/observability/automation），属于手写 DI。CLI 入口在 `cmd/mcpd/main.go`，Wails 入口在仓库根目录的 `app.go`，UI 管理器在 `internal/ui/manager.go` 调用 `App.Serve`。

本次改造的核心点是：在 `internal/domain` 定义 `CatalogAccessor` 接口，使用静态实现加载配置，并让应用层的构建流程以访问器为入口；在 Wire 的 ProviderSet 中将 Core 生命周期组件与配置生命周期组件区分为 `CoreInfraSet` 与 `ReloadableAppSet`，并在 `cmd/mcpd/wire.go` 提供 `InitializeApp` 注入器。

关键文件与模块：

- `internal/app/app.go`: 当前 `App.Serve` 与 `ValidateConfig` 的实现位置。
- `internal/app/control_plane.go`: `NewControlPlane` 负责创建内部服务。
- `internal/app/server_init_manager.go`: `NewServerInitializationManager` 依赖配置值。
- `internal/infra/*`: scheduler/router/transport/lifecycle/telemetry 等基础设施实现。
- `cmd/mcpd/main.go`: CLI 入口，将改为调用 `InitializeApp`。
- `app.go`: Wails 入口，仍通过 `App` 入口启动 Core。

## Plan of Work

第一步引入配置访问器接口与静态实现。`CatalogAccessor` 放在 `internal/domain`，静态实现放在 `internal/app`，通过 `catalog.ProfileStoreLoader` 加载一次配置并缓存 `ProfileStore`。然后将 `buildProfileSummary` 及相关辅助函数移动到独立文件，保持纯函数特性以便复用。

第二步重构应用组装层。新增 `Application` 结构体（运行期容器），负责封装 scheduler/control-plane/rpc server/metrics 等已注入的组件，并在 `Run` 方法中执行启动逻辑。`App.Serve` 变为调用 Wire 注入器 `InitializeApplication`，再执行 `Run`。`NewControlPlane` 改为纯构造器（只接收已注入的内部服务），内部服务的创建由独立 Provider 函数完成。`NewServerInitializationManager` 改为依赖 profile summary 或访问器，而不直接接受配置 map。

第三步加入 Wire ProviderSet 与注入器。新增 `internal/app/wire.go`（wireinject build tag）与生成文件，提供 `InitializeApplication` 供 `App.Serve` 与 CLI 使用。ProviderSet 需分成 `CoreInfraSet` 与 `ReloadableAppSet`，并明确 `CatalogAccessor` 的 `wire.Bind`。

第四步更新 CLI/Wails/UI 的调用链与测试。CLI 使用 `app.InitializeApplication` 构建 `*app.Application` 并调用 `Run`。Wails/UI 仍可使用 `App.Serve`，但其内部已切换到 Wire 注入。更新 `internal/app` 的相关测试以适配新的构造器签名。

## Concrete Steps

在仓库根目录执行如下命令进行编辑与验证：

    rg -n "Serve\\(|NewControlPlane|ServerInitializationManager" internal/app cmd/mcpd
    rg -n "ServeConfig|ValidateConfig" internal/ui cmd/mcpd

创建与编辑文件：

- `internal/domain/catalog_accessor.go` 新增 `CatalogAccessor` 接口。
- `internal/app/catalog_accessor.go` 新增 `StaticCatalogAccessor` 与构造函数。
- `internal/app/application.go` 新增 `Application` 与 `Run` 方法。
- `internal/app/wire.go` 与 `internal/app/wire_gen.go` 新增注入器。

如需生成 wire 代码（有 wire 工具时）：

    wire ./internal/app

## Validation and Acceptance

验证方式以行为为准：

- CLI: 运行 `go run ./cmd/mcpd serve --config .`，应看到与改造前一致的配置加载日志，并能正常启动 RPC 服务。
- UI: 运行 Wails 入口（若已有运行方式），`Start`/`Stop` 流程仍可驱动 Core。
- 测试: 运行 `make test` 或 `go test ./...`，预期全绿；若无法运行测试，需在 PR 说明中标注。

## Idempotence and Recovery

本改造主要为新增文件和构造器签名调整，可安全重复执行。若 Wire 生成文件出现问题，可通过重新运行 `wire` 生成，或将 `wire_gen.go` 回退到上一次已知可编译版本。若注入器签名调整导致编译失败，优先检查 ProviderSet 的依赖关系与函数参数顺序。

## Artifacts and Notes

示例变更片段会包含在最终提交中（`wire.go` 与 `wire_gen.go`）。如需记录执行日志或构建输出，附在本节的缩进块中。

## Interfaces and Dependencies

在 `internal/domain/catalog_accessor.go` 定义：

    type CatalogAccessor interface {
        GetProfileStore() (ProfileStore, error)
    }

在 `internal/app/catalog_accessor.go` 定义：

    type StaticCatalogAccessor struct {
        store domain.ProfileStore
    }

    func NewStaticCatalogAccessor(ctx context.Context, cfg ServeConfig, logger *zap.Logger) (*StaticCatalogAccessor, error)

在 `internal/app/application.go` 定义：

    type Application struct { /* fields for injected deps */ }

    func NewApplication(ctx context.Context, cfg ServeConfig, logger *zap.Logger, registry *prometheus.Registry, metrics domain.Metrics, health *telemetry.HealthTracker, snapshot *CatalogSnapshot, profiles map[string]*profileRuntime, scheduler domain.Scheduler, initManager *ServerInitializationManager, controlPlane *ControlPlane, rpcServer *rpc.Server) *Application

    func (a *Application) Run() error

在 `internal/app/catalog_snapshot.go` 定义：

    type CatalogSnapshot struct { /* store + summary */ }

    func NewCatalogSnapshot(accessor domain.CatalogAccessor) (*CatalogSnapshot, error)

在 `internal/app/control_plane.go` 中保留：

    func NewControlPlane(state *controlPlaneState, registry *callerRegistry, discovery *discoveryService, observability *observabilityService, automation *automationService) *ControlPlane

在 `internal/app/server_init_manager.go` 中更新：

    func NewServerInitializationManager(scheduler domain.Scheduler, snapshot *CatalogSnapshot, logger *zap.Logger) *ServerInitializationManager

并在 `internal/app/wire.go` 中定义：

    func InitializeApplication(ctx context.Context, cfg ServeConfig, logging LoggingConfig) (*Application, error)

更新说明（2025-12-30）：同步进度状态为已完成步骤，记录引入 `CatalogSnapshot` 与移除 CLI 注入器的决策，并更新接口签名以匹配当前实现。
