# 修复 Lazy 启动下的元数据可用性与标识字段一致性

这是一个持续更新的 ExecPlan。必须遵循仓库根目录 `.agent/PLANS.md` 的规范，并在执行过程中持续更新 `Progress`、`Surprises & Discoveries`、`Decision Log`、`Outcomes & Retrospective` 四个章节。

## Purpose / Big Picture

当前 Lazy 启动流程在 bootstrap 完成后会停止实例，但索引刷新仍要求 ready 实例，导致 tools/resources/prompts 列表为空。新增的 MetadataCache 只写不读，SpecKey/ServerName 在实时聚合路径里缺失。完成本计划后，bootstrap 缓存会被索引读取，Lazy 模式下列表稳定可用；prompt/resource 的 SpecKey/ServerName 在所有路径一致；eager 模式下 init manager 仍保证 minReady 目标，不被 bootstrap 覆盖。

## Progress

- [x] (2025-03-15 10:12Z) 创建 ExecPlan 并完成目标拆解。
- [x] (2025-03-15 10:40Z) 迁移 MetadataCache 到 domain 层并更新依赖与测试。
- [x] (2025-03-15 10:40Z) 聚合索引读取缓存并补齐 SpecKey/ServerName，bootstrap 完成后触发刷新。
- [x] (2025-03-15 10:40Z) 修复 eager 启动下 init manager 生命周期与 minReady 覆盖问题。
- [x] (2026-01-01 09:10Z) 修复 bootstrap waiter 设置导致的初始化死锁，并补充索引级回归测试。
- [x] (2026-01-01 09:20Z) lazy 模式下也启动 init manager，避免 caller 注册阻塞与警告噪音。
- [ ] (2025-03-15 10:50Z) 增补单测并完成 make test 验证（已新增单测并运行 go test 覆盖 domain/aggregator/app，尚未执行 make test）。

## Surprises & Discoveries

- Observation: bootstrap manager 使用 spec registry 与 serverType specKeys 的键空间不一致，导致目标列表可能为空。
  Evidence: `internal/app/bootstrap_manager.go` 需要根据 specKeys 反查 spec registry。
- Observation: 在沙箱环境运行 `go test` 需要写入 `~/Library/Caches/go-build`，当前权限受限导致聚合与 app 包测试无法执行。
  Evidence: `open /Users/wibus/Library/Caches/go-build/...: operation not permitted`。
- Observation: 使用 `GOCACHE` 指向工作区目录可绕开缓存权限限制并完成局部测试。
  Evidence: `GOCACHE=/Users/wibus/dev/mcpv/.cache/go-build go test ./internal/domain ./internal/infra/aggregator ./internal/app`。
- Observation: SetBootstrapWaiter 在持有写锁时调用 startBootstrapRefresh 触发同锁 RLock，导致初始化路径死锁。
  Evidence: 栈追踪卡在 `ToolIndex.startBootstrapRefresh` 等读取 bootstrapWaiterMu。

## Decision Log

- Decision: MetadataCache 迁移到 `internal/domain` 并直接被聚合索引持有与读取。
  Rationale: 避免 infra -> app 的循环依赖，同时保持 domain 仅依赖标准库。
  Date/Author: 2025-03-15 / Codex.

- Decision: 索引在 fetch 遇到 `ErrNoReadyInstance` 时回退到 MetadataCache，并在 bootstrap 完成后触发一次 refresh。
  Rationale: Lazy 模式不允许列表触发实例启动，但需要可用快照；refresh 间隔为 0 时也能加载缓存。
  Date/Author: 2025-03-15 / Codex.

- Decision: eager 策略下仍启动 ServerInitializationManager，bootstrap 使用 max(1, minReady) 作为目标，不覆盖 init manager 的 minReady。
  Rationale: 保持 eager 启动的最小实例保障与状态追踪，避免 bootstrap 将目标回落到 1。
  Date/Author: 2025-03-15 / Codex.

- Decision: bootstrap 目标列表由 specKeys 反向映射到 spec registry 并去重，避免 serverType 与 specKey 键空间错配。
  Rationale: spec registry 以 specKey 为主键，直接迭代会导致 specKey 查找失败。
  Date/Author: 2025-03-15 / Codex.

## Outcomes & Retrospective

待完成后补充。

## Context and Orientation

- `internal/app/bootstrap_manager.go`: bootstrap 获取 metadata 并缓存，但当前停止实例后索引仍需 ready 实例。
- `internal/infra/aggregator/*.go`: tools/resources/prompts 索引刷新逻辑，使用 `AllowStart: false`。
- `internal/app/metadata_cache.go`: MetadataCache 位于 app 层且未被读取。
- `internal/app/application.go`: 有 bootstrap manager 时不会启动 init manager。
- `internal/app/providers.go` + `internal/app/wire_gen.go`: 负责注入各组件。
- `internal/app/reload_manager.go`: reload 时重新构建 profile runtime。

## Plan of Work

先把 MetadataCache 移到 domain，并更新 wire/provider、bootstrap 与测试引用，确保依赖关系清晰。随后在 ToolIndex/ResourceIndex/PromptIndex 增加 MetadataCache 读取路径：fetch 返回 `ErrNoReadyInstance` 时从 cache 构建 serverCache，并在 bootstrap 完成后触发一次 refresh，保证 refresh 间隔为 0 时也能加载缓存。同步在资源与 prompt 的 snapshot 构建处补齐 SpecKey/ServerName。最后在 Application 启动流程中按 eager 策略启动 init manager，并在 bootstrap 内部使用最小 ready 的上限以避免覆盖目标。补充覆盖这些行为的单测，并跑 `make test` 验证。

## Concrete Steps

1. 在仓库根目录执行 `rg -n "MetadataCache" internal` 确认当前使用点。
2. 将 `internal/app/metadata_cache.go` 与对应测试移动到 `internal/domain/metadata_cache.go`，更新 import 与 wire/provider 代码。
3. 修改 `internal/infra/aggregator/aggregator.go`、`resource_index.go`、`prompt_index.go`：
   - 构造函数新增 `*domain.MetadataCache` 参数并存储。
   - fetch 失败时回退到缓存。
   - bootstrap 完成后触发 refresh。
   - snapshot 构建补齐 SpecKey/ServerName。
4. 修改 `internal/app/application.go` 与 `internal/app/bootstrap_manager.go`，确保 eager 策略下 init manager 启动且 bootstrap 不覆盖目标 minReady。
5. 更新 `internal/app/providers.go`、`internal/app/wire_gen.go`、`internal/app/wire_sets.go`、`internal/app/reload_manager.go` 的依赖注入与参数传递。
6. 更新或新增聚合索引与 cache 的单测，检查 prompts/resources 的 SpecKey/ServerName。
7. 在仓库根目录运行 `make test`。

## Validation and Acceptance

- `make test` 全量通过。
- Lazy 策略下，bootstrap 完成后无需实例常驻即可通过 list 接口获得 tools/resources/prompts；refresh 间隔为 0 时也能得到非空快照。
- Prompt/Resource 的 `SpecKey` 与 `ServerName` 字段在快照中稳定填充，不依赖 bootstrap 路径。
- eager 策略下 init manager 会启动并保证 minReady 行为不被 bootstrap 回落。

## Idempotence and Recovery

所有变更均为可重复执行的代码修改，不涉及数据迁移。若某一步失败，回滚对应文件修改后可重新执行 `make test` 验证。

## Artifacts and Notes

完成后在此记录关键测试输出或日志片段。

## Interfaces and Dependencies

- `internal/domain.MetadataCache`: 提供 `GetTools/GetResources/GetPrompts` 等读取接口，供聚合索引使用。
- `internal/infra/aggregator.ToolIndex/ResourceIndex/PromptIndex`: 构造函数新增 `metadataCache *domain.MetadataCache`，并在 `fetchServerCache` 中回退到缓存。
- `internal/app.Application`: 根据 `RuntimeConfig.StartupStrategy` 决定是否启动 `ServerInitializationManager`。

Plan Update Note: 初始创建 ExecPlan（2025-03-15）。
Plan Update Note: 完成 MetadataCache 迁移、索引缓存回退与 eager 初始化修复，更新进度与决策记录（2025-03-15）。
Plan Update Note: 补充沙箱测试限制的发现记录（2025-03-15）。
Plan Update Note: 记录 GOCACHE 绕开权限并完成局部测试的进度更新（2025-03-15）。
