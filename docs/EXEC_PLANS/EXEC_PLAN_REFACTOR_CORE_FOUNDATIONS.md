# 重构聚合索引、配置编辑、控制面板与常量体系

这是一个可执行计划（ExecPlan），它是一个活文档。必须在执行过程中持续更新 `Progress`、`Surprises & Discoveries`、`Decision Log` 和 `Outcomes & Retrospective`。

仓库内存在 `.agent/PLANS.md`，本计划必须符合其要求并持续维护。

## Purpose / Big Picture

完成本计划后，聚合索引（工具/资源/提示）将共享一个泛型核心，配置编辑逻辑从 UI 桥接层迁移到 catalog 基础设施中，ControlPlane 被拆分为多个职责清晰的子模块但保持对外接口不变，同时映射与常量归一化以减少重复与硬编码。用户可观察到的行为应保持一致，但工程结构更易维护、可复用与可测试。除此之外，系统将移除“单文件配置模式”，统一以 profile store 目录作为唯一配置入口。

## Progress

- [x] (2025-03-07 10:30Z) 引入 aggregator 的泛型索引核心，并重构 ToolIndex/ResourceIndex/PromptIndex 使用该核心，保持行为与测试一致。
- [x] (2025-03-07 11:10Z) 移除单文件配置模式，仅支持 profile store 目录；更新 loader、CLI 文案、测试与文档示例。
- [x] (2025-03-07 13:00Z) 将 UI 层配置编辑逻辑迁移至 `internal/infra/catalog` 的 Editor，WailsService 仅做转发。
- [x] (2025-03-07 13:00Z) 拆分 ControlPlane 为职责明确的服务模块，保持 `domain.ControlPlane` 接口不变。
- [x] (2025-03-07 13:00Z) 统一映射与常量：提取通用映射函数与文件权限/缓冲区/时间常量。
- [x] (2025-03-07 13:10Z) 运行 `make fmt`、`go vet ./...` 与 `make test`（`make vet` 目标不存在）。

## Surprises & Discoveries

- Observation: 初次执行 `make test` 受 Go build cache 权限限制，需要指定 `GOCACHE` 到工作区目录。
  Evidence: `open /Users/wibus/Library/Caches/go-build/...: operation not permitted`
- Observation: `make vet` 目标不存在，需改用 `go vet ./...`。
  Evidence: `make: *** No rule to make target 'vet'.  Stop.`

## Decision Log

- Decision: 泛型索引核心放在 `internal/infra/aggregator` 包内，而非新增包。
  Rationale: 保持现有包边界与依赖关系稳定，减少迁移成本。
  Date/Author: 2025-03-07 / Codex
- Decision: 不引入外部自动映射依赖，改用内部通用映射函数。
  Rationale: 避免新增依赖与运行时成本，同时保持可读性与一致性。
  Date/Author: 2025-03-07 / Codex
- Decision: 移除单文件配置模式，并将默认示例调整为 profile store 目录。
  Rationale: 简化配置路径与 UI/CLI 逻辑，减少分支与维护成本。
  Date/Author: 2025-03-07 / Codex
- Decision: ControlPlane 拆分为 registry/discovery/observability/automation 四个模块，公共依赖汇总到 `controlPlaneState`。
  Rationale: 将调用者注册、资源发现、可观测性与自动化工具筛选解耦，降低维护耦合度。
  Date/Author: 2025-03-07 / Codex
- Decision: 文件权限与日志缓冲区等常量集中到 `internal/infra/fsutil` 与 `internal/infra/telemetry`。
  Rationale: 降低散落硬编码，便于后续统一调整默认值。
  Date/Author: 2025-03-07 / Codex

## Outcomes & Retrospective

- 2025-03-07：完成聚合索引泛型化、UI 配置编辑迁移、ControlPlane 拆分与常量/映射归一，配置入口已统一为 profile store 目录；后续如需新增配置示例，仅需在 `docs/catalog.example/` 扩展。

## Context and Orientation

聚合索引目前位于 `internal/infra/aggregator/aggregator.go`、`internal/infra/aggregator/resource_index.go`、`internal/infra/aggregator/prompt_index.go`，三者拥有高度重复的启动/停止、刷新并发、订阅与快照逻辑。

UI 配置编辑逻辑位于 `internal/ui/service.go`，包含配置模式判断（单文件/目录）、路径解析、文件写入等逻辑；catalog 已有 `profile_editor.go` 与 `store_editor.go` 但缺乏一个统一的上层 Editor。

ControlPlane 位于 `internal/app/control_plane.go`，承担 caller 注册、tools/resources/prompts 聚合、日志流、运行时状态等多重职责；接口定义在 `internal/domain/controlplane.go`。

RPC 映射在 `internal/infra/rpc/mapping.go`，UI 侧转换分散在 `internal/ui/service.go`。文件权限与缓冲区大小散落于 `internal/infra/catalog/profile_store.go`、`internal/infra/rpc/server.go` 等。

配置入口通过 `ProfileStoreLoader.Load` 处理，当前仅支持目录模式。配置路径必须为 profile store 目录，包含 `profiles/*.yaml`、`callers.yaml`、以及可选 `runtime.yaml`。

## Plan of Work

首先新增 aggregator 泛型索引核心，统一 Start/Stop/Subscribe/Refresh 等逻辑。ToolIndex/ResourceIndex/PromptIndex 仅保留各自的 fetch、合并、冲突处理与特定 RPC 调用。保持 ToolIndex 的 `refresh` 可用于现有测试调用。

随后移除单文件配置模式：`ProfileStoreLoader` 不再接受 YAML 文件路径；CLI flag 文案与错误信息更新；测试从“加载单文件成功”改为“单文件报错”；示例配置迁移为 profile store 目录并更新 Makefile 与文档。

然后实现 catalog Editor，负责配置模式检查、目录路径解析、写入 ProfileUpdate 等；WailsService 改为调用 Editor 的方法，不再直接读写文件或判断模式。

接着拆分 ControlPlane：定义 core 结构，拆出 caller registry、discovery、observability 等子服务，ControlPlane 作为 facade 组合这些服务并实现 `domain.ControlPlane`。

最后整理映射与常量：新增通用 slice 映射函数；为 UI 映射建立集中函数；提取文件权限、缓冲区大小、时间默认值为命名常量并更新调用点。

## Concrete Steps

在 `/Users/wibus/dev/mcpd` 下工作。

1) 引入 aggregator 泛型核心并重构三类索引。

    rg -n "ToolIndex|ResourceIndex|PromptIndex" internal/infra/aggregator

2) 移除单文件模式并更新 CLI/测试/示例文档。

    rg -n "catalog.example|single|LoadFromFile" -S docs internal cmd Makefile

3) 新增 catalog Editor，更新 WailsService 以委派。

    rg -n "ImportMcpServers|SetServerDisabled|DeleteServer|CreateProfile|SetCallerMapping|SetProfileSubAgentEnabled|GetConfigMode" internal/ui/service.go

4) 拆分 ControlPlane 为子服务。

    rg -n "ControlPlane" internal/app

5) 统一映射与常量。

    rg -n "0o644|0o755|make\\(chan|time\\.Second|time\\.Millisecond" internal/app internal/infra

6) 运行格式化与测试。

    make fmt
    make vet
    make test

## Validation and Acceptance

运行 `make fmt vet test`，所有现有测试应通过。新增/调整后的测试应覆盖：

1) ProfileStoreLoader 对单文件路径返回错误。
2) 目录模式加载与自动创建逻辑保持一致。
3) 新的 catalog Editor 覆盖导入、禁用、删除等路径。

手工验证：

1) `make validate CONFIG=docs/catalog.example` 成功（该目录包含 `runtime.yaml`、`callers.yaml` 与 `profiles/default.yaml`）。
2) `make serve CONFIG=docs/catalog.example` 能启动并加载默认 profile。

## Idempotence and Recovery

所有步骤均可重复执行。若中途失败，可按模块回滚到上一个稳定点再逐步重试。目录示例与测试使用临时路径，不会污染用户配置。

## Artifacts and Notes

记录关键 diff 与测试输出，优先保留能证明行为一致性的片段。

## Interfaces and Dependencies

在 `internal/infra/aggregator/index_core.go` 定义：

    type GenericIndex[Snapshot any, Target any, Cache any] struct { ... }
    type GenericIndexOptions[Snapshot any, Target any, Cache any] struct {
        Name string
        Specs map[string]domain.ServerSpec
        Config domain.RuntimeConfig
        Logger *zap.Logger
        Health *telemetry.HealthTracker
        Gate *RefreshGate
        EmptySnapshot func() Snapshot
        CopySnapshot func(Snapshot) Snapshot
        SnapshotETag func(Snapshot) string
        BuildSnapshot func(cache map[string]Cache) (Snapshot, map[string]Target)
        Fetch func(ctx context.Context, serverType string, spec domain.ServerSpec) (Cache, error)
        OnRefreshError func(serverType string, err error) (shouldContinue bool)
        ShouldStart func(cfg domain.RuntimeConfig) bool
    }
    func NewGenericIndex[Snapshot any, Target any, Cache any](opts GenericIndexOptions[Snapshot, Target, Cache]) *GenericIndex[Snapshot, Target, Cache]
    func (g *GenericIndex[Snapshot, Target, Cache]) Start(ctx context.Context)
    func (g *GenericIndex[Snapshot, Target, Cache]) Stop()
    func (g *GenericIndex[Snapshot, Target, Cache]) Snapshot() Snapshot
    func (g *GenericIndex[Snapshot, Target, Cache]) Subscribe(ctx context.Context) <-chan Snapshot
    func (g *GenericIndex[Snapshot, Target, Cache]) Resolve(key string) (Target, bool)
    func (g *GenericIndex[Snapshot, Target, Cache]) Refresh(ctx context.Context) error

在 `internal/infra/catalog/editor.go` 定义：

    type ConfigInfo struct {
        Path string
        IsWritable bool
    }
    type Editor struct { ... }
    func NewEditor(path string, logger *zap.Logger) *Editor
    func (e *Editor) Inspect(ctx context.Context) (ConfigInfo, error)
    func (e *Editor) ImportServers(ctx context.Context, req ImportRequest) error
    func (e *Editor) SetServerDisabled(ctx context.Context, profileName, serverName string, disabled bool) error
    func (e *Editor) DeleteServer(ctx context.Context, profileName, serverName string) error
    func (e *Editor) CreateProfile(ctx context.Context, profileName string) error
    func (e *Editor) DeleteProfile(ctx context.Context, profileName string) error
    func (e *Editor) SetCallerMapping(ctx context.Context, caller, profile string) error
    func (e *Editor) RemoveCallerMapping(ctx context.Context, caller string) error
    func (e *Editor) SetProfileSubAgentEnabled(ctx context.Context, profileName string, enabled bool) error

在 `internal/infra/mapping/slice.go` 定义：

    func MapSlice[S any, D any](src []S, fn func(S) D) []D

在 `internal/ui/mapping.go` 定义 UI 映射函数，并在 `internal/ui/service.go` 中替换手写循环。

在 `internal/infra/fsutil/permissions.go` 定义：

    const DefaultFileMode = 0o644
    const DefaultDirMode = 0o755

Plan Change Note: 新建计划，覆盖索引泛型化、配置编辑迁移、ControlPlane 拆分、映射与常量整理，并移除单文件配置模式。
Plan Change Note: 完成单文件模式移除与示例目录落地（profile store 目录、CLI 文案、测试与 UI 限制同步更新）。
Plan Change Note: 完成 ControlPlane 拆分、UI 配置 Editor 迁移、映射/常量归一化，并补充 go fmt/vet/test 结果记录。
