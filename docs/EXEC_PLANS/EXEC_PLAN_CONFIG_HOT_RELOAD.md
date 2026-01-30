# 配置热重载（Hot Reload）

这是一个持续更新的 ExecPlan。必须遵循仓库根目录下的 `.agent/PLANS.md` 规范，并在执行过程中持续更新 `Progress`、`Surprises & Discoveries`、`Decision Log`、`Outcomes & Retrospective` 四个章节。

## Purpose / Big Picture

完成后，Core 可以在不重启的情况下接收配置更新：修改 profile store（`profiles/*.yaml`、`runtime.yaml`、`callers.yaml`）会触发重新加载，新增或变更的 MCP server 会平滑切换到新的 specKey，旧实例进入 draining 并自动退出。运行期如果遇到无效配置，会保留旧配置继续运行并记录清晰的告警，避免服务中断。验证方式是：运行 `mcpv serve` 后修改配置，日志出现 reload 成功或失败的提示，RPC 监听不重建，已连接的客户端继续可用，工具/资源列表能反映最新配置。

## Progress

- [x] (2025-12-31 04:38Z) 起草配置热重载的 ExecPlan，与当前 Wire 结构对齐。
- [x] (2025-12-31 06:30Z) 引入 CatalogProvider/CatalogState/Diff 与 CatalogSummary（迁移到 domain），完成静态与动态 Provider。
- [x] (2025-12-31 06:30Z) 让 Scheduler、Index、ControlPlane 状态可更新，新增 ReloadManager 与 callerRegistry 重新对账逻辑。
- [x] (2025-12-31 06:30Z) Wire 注入链路改造为 Provider + State + ReloadManager，并更新相关测试。
- [x] (2025-12-31 07:10Z) 补齐运行期验证与剩余测试失败的定位与修复。

## Surprises & Discoveries

- `go test ./internal/...` 在沙箱内需要自定义 `GOCACHE`，否则访问系统缓存失败。
  Evidence: `open /Users/wibus/Library/Caches/go-build/...: operation not permitted`

- `mcpv/internal/infra/lifecycle` 的 `TestManager_StartInstance_DetachesFromCallerContext` 在本次验证中失败。
  Evidence: `manager_test.go:77: start context should not be canceled when caller context is canceled after startup`

## Decision Log

- Decision: 使用 `CatalogProvider` 统一配置读取与变更订阅，替代只读的 `CatalogAccessor`。
  Rationale: 保持组件依赖稳定的访问器接口，同时为热重载提供 Watch/Reload 能力。
  Date/Author: 2025-12-31 / Codex

- Decision: 以 `CatalogState`（包含 `ProfileStore` 与计算后的摘要）作为运行时单一真相，使用原子指针发布新版本。
  Rationale: 避免组件持有旧指针导致状态撕裂，提供一致的读视图并支持快速切换。
  Date/Author: 2025-12-31 / Codex

- Decision: 运行期暂不允许 `RuntimeConfig` 发生任何变更（包括 RPC/Observability 及刷新/超时相关字段）。
  Rationale: 避免在运行中重建 router/index/ticker 等组件；先保证热重载的正确性与可维护性，再逐步放开。
  Date/Author: 2025-12-31 / Codex

- Decision: 使用 specKey（`SpecFingerprint`）区分“可替换实例”与“原地更新”，specKey 变化触发旧实例 draining。
  Rationale: 让命令、环境、工作目录变更具备滚动更新语义，其余字段尽量原地生效。
  Date/Author: 2025-12-31 / Codex

- Decision: `ServerInitializationManager` 在 reload 时原地更新 spec/targets/statuses，而非重建。
  Rationale: 保持状态连续性并减少后台 goroutine 的抖动。
  Date/Author: 2025-12-31 / Codex

## Outcomes & Retrospective

热重载路径完成闭环：配置提供者、快照、diff、reload 编排、调度器与索引的动态更新已落地，且 `go test ./internal/...` 通过。
`internal/ui` 仍存在 macOS 链接版本告警，属于外部工具链问题，未影响测试通过。

## Context and Orientation

当前运行期配置由 `internal/app/catalog_provider_dynamic.go` 的 `DynamicCatalogProvider` 加载并发布 `domain.CatalogState`。`internal/domain/catalog_summary.go` 负责构建 `CatalogSummary`，`internal/app/providers.go` 根据当前状态构建 scheduler、profile runtimes、control plane 状态与 RPC server。`internal/app/control_plane_state.go` 使用读写锁持有 `specRegistry`、`profiles`、`callers` 与 `runtime`，而 `internal/infra/scheduler/basic.go`、`internal/infra/aggregator/*` 则新增了动态更新入口，确保 reload 时状态与索引能够平滑切换。

这意味着“配置加载 → 组件构建 → 运行”的路径是一次性的，运行期没有任何组件能被通知到新的配置快照。即便 profile 文件变更，scheduler 仍然在旧的 `specs` map 上工作，Index 仍然引用旧 specKey，ControlPlane 的 profileRuntime 也不会更新；因此热重载无法成立。

本计划将“配置值”与“配置提供者”彻底解耦，引入可订阅的配置源，并将变更应用到 scheduler、索引与控制面状态。关键术语：`ProfileStore` 是由 profile store 目录解析出的配置集合；`specKey` 是由 `SpecFingerprint` 生成的指纹；`CatalogState` 是包含 `ProfileStore`、摘要与版本号的只读快照。

## Plan of Work

第一阶段引入可热重载的配置提供者与快照构建器。新增 `CatalogProvider` 接口，提供 `Snapshot`、`Watch` 与 `Reload` 能力，并实现 `StaticCatalogProvider` 与 `DynamicCatalogProvider` 两种实现。`DynamicCatalogProvider` 使用文件系统监听与去抖动策略来检测变更，并在内存中执行“影子加载”与校验：先调用 `ProfileStoreLoader` 构建 `ProfileStore`，再计算摘要并执行语义校验，只有完全成功时才生成新的 `CatalogState`。这个阶段还需要引入 `CatalogDiff` 计算，用于描述新增、删除、替换与原地更新的 specKey，并记录 profile/caller 的变化。

第二阶段让运行时组件可响应更新。核心思路是将“动态配置”集中到一个 `ReloadManager`，它订阅 `CatalogProvider` 的更新事件，按固定顺序应用变更：先更新 spec registry 与 scheduler 可见的新 specKey，随后更新 profile runtime 的索引配置（保持订阅通道不变），最后将旧 specKey 标记为 draining 并触发 `StopSpec`。`BasicScheduler` 需要支持动态更新 spec 列表并保留旧 pool 直到 draining 完成。`GenericIndex` 增加 `UpdateSpecs` 能力，在不重建订阅通道的前提下刷新配置。`callerRegistry` 需要在配置变更时重新评估 active callers 的 profile/spec 归属，确保新旧 spec 计数一致且不会丢失活动状态。

第三阶段将热重载接入应用生命周期与 Wire。`Application` 在 `Run` 期间启动 `ReloadManager`，`App.Serve` 继续保持同步阻塞。Wire ProviderSet 将 `CatalogProvider` 注入 `ReloadManager`，并替换现有 `CatalogSnapshot` 构建路径。UI 侧提供手动 reload 入口（例如 `Manager.ReloadConfig`），直接触发 `CatalogProvider.Reload`，避免 Wails UI 只能依赖文件变更触发。

## Concrete Steps

在仓库根目录执行以下命令定位相关代码并验证修改范围：

    rg -n "CatalogAccessor|CatalogSnapshot|profileSummary" internal/app internal/domain
    rg -n "BasicScheduler|GenericIndex|ToolIndex|ResourceIndex|PromptIndex" internal/infra
    rg -n "callerRegistry|controlPlaneState" internal/app

新增或调整文件时，按如下路径进行创建与编辑：

    internal/domain/catalog_provider.go
    internal/domain/catalog_state.go
    internal/app/catalog_provider_static.go
    internal/app/catalog_provider_dynamic.go
    internal/app/reload_manager.go
    internal/infra/aggregator/index_core.go
    internal/infra/scheduler/basic.go
    internal/app/control_plane_state.go
    internal/app/control_plane_registry.go
    internal/app/providers.go
    internal/app/wire_sets.go

## Validation and Acceptance

执行 `make test`，期望所有测试通过，并新增覆盖 reload 的单测。需要至少包含以下验证：

1) `CatalogDiff` 能区分新增、删除、specKey 变化与同 key 的字段更新。
2) `BasicScheduler` 对新增 specKey 可立即 `Acquire`，对移除 specKey 会进入 draining 并在完成后释放。
3) `GenericIndex` 在更新 specs 后订阅通道仍可收到新的 snapshot。

运行期验证流程如下，工作目录为仓库根：

    go run ./cmd/mcpv serve --config ./runtime

修改 `runtime/profiles/default.yaml` 新增一个 server，保存后观察日志出现类似 `config reload applied` 的提示，随后再删除该 server 并观察旧实例进入 draining 的日志。RPC 监听地址保持不变，已连接的 caller 不需要重连即可继续调用。

## Idempotence and Recovery

文件变更触发的 reload 是幂等的：相同配置会被 diff 识别为 no-op 并跳过应用。若影子加载或校验失败，系统继续使用旧配置并记录错误，修复配置后可再次触发 reload，不需要重启。任何单次 reload 失败都不应破坏 `CatalogState` 的读一致性。

## Artifacts and Notes

本阶段完成后，补充代表性的日志片段以辅助验证，例如：

    2025-12-31T05:10:02.345Z INFO config reload applied revision=12 profiles=3 servers=17 added=2 removed=1 replaced=1
    2025-12-31T05:12:07.221Z WARN config reload rejected error="runtime config changed: rpc.listenAddress"
    2025-12-31T05:12:09.541Z INFO scheduler drain started specKey=abc123 reason="spec removed"

## Interfaces and Dependencies

在 `internal/domain/catalog_provider.go` 定义配置提供者接口与事件类型：

    type CatalogProvider interface {
        Snapshot(ctx context.Context) (CatalogState, error)
        Watch(ctx context.Context) (<-chan CatalogUpdate, error)
        Reload(ctx context.Context) error
    }

    type CatalogUpdate struct {
        Snapshot CatalogState
        Diff     CatalogDiff
        Source   CatalogUpdateSource
    }

在 `internal/domain/catalog_state.go` 定义运行期快照与 diff：

    type CatalogState struct {
        Store    ProfileStore
        Summary  CatalogSummary
        Revision uint64
        LoadedAt time.Time
    }

    type CatalogDiff struct {
        AddedSpecKeys    []string
        RemovedSpecKeys  []string
        ReplacedSpecKeys []string
        UpdatedSpecKeys  []string
        AddedProfiles    []string
        RemovedProfiles  []string
        UpdatedProfiles  []string
        CallersChanged   bool
    }

在 `internal/infra/scheduler/basic.go` 增加动态更新接口，保留现有 pool 与 draining 逻辑：

    func (s *BasicScheduler) ApplySpecChanges(ctx context.Context, diff CatalogDiff, registry SpecRegistry) error

在 `internal/infra/aggregator/index_core.go` 增加热更新入口，确保订阅通道不变：

    func (g *GenericIndex[Snapshot, Target, Cache]) UpdateSpecs(specs map[string]domain.ServerSpec, cfg domain.RuntimeConfig)

在 `internal/app/reload_manager.go` 新增协调器，负责订阅更新并按顺序应用：

    type ReloadManager struct {
        Start(ctx context.Context)
        Reload(ctx context.Context) error
    }

本计划初稿创建于 2025-12-31，后续修改需在文末补充更新原因与影响范围。

2025-12-31 更新：完成 Provider/State/ReloadManager 与 scheduler/index/control-plane 的改造，并将 `RuntimeConfig` 的热重载限制为“完全禁止”，以降低动态更新复杂度与运行期风险。
