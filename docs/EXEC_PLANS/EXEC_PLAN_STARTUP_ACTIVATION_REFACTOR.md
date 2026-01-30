# 拆分启动策略与激活策略，消除 minReady 与 lazy 冲突

这是一个持续更新的 ExecPlan。必须遵循仓库根目录 `.agent/PLANS.md` 的规范，并在执行过程中持续更新 `Progress`、`Surprises & Discoveries`、`Decision Log`、`Outcomes & Retrospective` 四个章节。

仓库根目录包含 `.agent/PLANS.md`，本计划必须遵循其全部要求并保持自洽。

## Purpose / Big Picture

完成后，配置将不再把“bootstrap 元数据流程”和“实例保活策略”混在同一字段中。用户可以明确地选择：是否要在启动时抓取元数据、以及某个服务在没有 caller 时是否仍保持运行。验证方式是：在 `bootstrapMode: metadata` 且 `defaultActivationMode: on-demand` 时，`minReady > 0` 不会触发无 caller 的常驻；只有显式配置 `activationMode: always-on` 的服务才会在启动后保持运行。

## Progress

- [x] (2026-01-01 10:00Z) 创建 ExecPlan，确定配置拆分与行为目标。
- [x] (2026-01-01 10:35Z) 完成 domain/config/schema 与 runtime/profile 编辑器更新，引入 bootstrapMode + activationMode。
- [x] (2026-01-01 10:40Z) 调整 bootstrap/init/caller 流程，确保只对 always-on 预热且 bootstrap 不改写 minReady。
- [x] (2026-01-01 10:50Z) 更新 UI 绑定、前端展示与示例配置、文档。
- [x] (2026-01-01 11:10Z) 运行 `make test`（GOCACHE 指向 frontend/.cache）完成，记录链接器警告。

## Surprises & Discoveries

- Observation: `make wails-bindings` 使用 go1.24 生成时对 go1.25+ 包提示警告，但绑定仍生成成功。
  Evidence: `package requires newer Go version go1.25 (application built with go1.24)`。
- Observation: `make test` 输出 macOS linker 版本警告但测试通过。
  Evidence: `ld: warning: object file ... was built for newer 'macOS' version (26.0) than being linked (11.0)`。

## Decision Log

- Decision: 用 `bootstrapMode` 替代 `startupStrategy`，将元数据 bootstrap 与实例保活解耦。
  Rationale: 现有 `startupStrategy` 同时影响 bootstrap 与实例常驻，和 `minReady` 语义冲突且难以推导。
  Date/Author: 2026-01-01 / Codex.

- Decision: 新增 `activationMode`（默认 `on-demand`），只有 `always-on` 才会在没有 caller 时保持运行。
  Rationale: 让“是否常驻”成为显式意图，避免 `minReady` 产生隐性常驻。
  Date/Author: 2026-01-01 / Codex.

- Decision: bootstrap 使用 `scheduler.Acquire` 拉起实例并抓取元数据，不再通过 `SetDesiredMinReady` 改写目标值。
  Rationale: 避免 bootstrap 与 warm pool 目标互相覆盖，保持目标只由 activation/caller 决定。
  Date/Author: 2026-01-01 / Codex.

- Decision: 不再在 bootstrap 完成后强制 StopSpec，交由 activation/idle 策略收敛。
  Rationale: 强制停止可能与 caller 激活竞争，且 bootstrap 的实例释放后应由 idle 与 activation 决定生命周期。
  Date/Author: 2026-01-01 / Codex.

## Outcomes & Retrospective

待完成后补充。

## Context and Orientation

本改动涉及配置解析、启动流程与 UI 展示。关键路径如下：

1. `internal/domain/types.go` 定义 `ServerSpec` 与 `RuntimeConfig`，包含 `MinReady`、`ActivationMode`、`BootstrapMode`、`DefaultActivationMode`。
2. `internal/infra/catalog/loader.go` 解析 runtime.yaml 与 profiles/*.yaml，校验并归一化字段。
3. `internal/app/server_init_manager.go` 启动时仅对 always-on 目标设定 minReady。
4. `internal/app/bootstrap_manager.go` 使用 `scheduler.Acquire` 拉起实例并抓取元数据。
5. `internal/app/control_plane_registry.go` caller ref-count 激活/停用实例池。
6. `internal/ui/types.go` + `internal/ui/mapping.go` 决定 Wails 绑定字段，`frontend/` 使用这些字段渲染配置页与配置详情。
7. `internal/infra/catalog/schema.json` 与 `runtime.yaml`、`profiles/*.yaml` 为配置规范与示例。

术语说明：
 - Bootstrap：启动时为各 server 拉起实例以抓取 tools/resources/prompts 元数据的流程。
 - Activation：当没有 caller 时，是否仍保持实例池的策略。
 - Warm pool（minReady）：服务在激活状态下维持的最小 ready 实例数。

## Plan of Work

第一步是改造配置与 domain 模型：新增 `bootstrapMode` 与 `defaultActivationMode`，并在 `ServerSpec` 中新增 `activationMode`。更新 schema 与 loader 的归一化/校验逻辑，让 `activationMode` 允许为空（代表使用 default），并保持 `minReady` 为 warm pool 目标。

第二步调整启动流程：`BootstrapManager` 仅负责 metadata，不再改写 minReady；`ServerInitializationManager` 在启动与 catalog 更新时仅对 `always-on` 目标设定 minReady；`callerRegistry` 在 deactivate 时跳过 always-on，并在 activation 时统一使用 `max(1, minReady)`。

第三步更新 UI 与绑定：Wails types 增加 `bootstrapMode` 与 `defaultActivationMode`，`ServerSpecDetail` 增加 `activationMode`。前端设置页增加 bootstrap/activation 选择项，profile server 详情显示 activation 与 warm pool。

第四步同步示例与文档：更新 `runtime.yaml` 与 `profiles/*.yaml` 以显式标注 always-on；更新相关 docs 中的配置字段与语义说明。

最后更新测试并验证：更新 loader/runtime_editor/server_init_manager 等单测，运行 `go test ./...`，如需前端验证则运行 `pnpm lint`。

## Concrete Steps

1. 在仓库根目录执行 `rg -n "startupStrategy|minReady" internal frontend docs` 以定位所有使用点。
2. 修改 `internal/domain/types.go` 与相关常量定义，引入 `ActivationMode` 与 `BootstrapMode`，并更新 `RuntimeConfig` 字段。
3. 更新 `internal/infra/catalog/loader.go`、`schema.json`、`runtime_editor.go`、`profile_editor.go` 的字段解析与校验。
4. 更新 `internal/app/bootstrap_manager.go` 与 `internal/app/server_init_manager.go` 的启动流程。
5. 更新 `internal/app/control_plane_registry.go` 的 activate/deactivate 行为以尊重 activationMode。
6. 更新 `internal/ui/types.go`、`internal/ui/mapping.go`、`internal/ui/service.go`，并运行 `make wails-bindings` 同步 bindings。
7. 更新 `frontend/src/modules/settings/settings-page.tsx` 与 `frontend/src/modules/config/components/profile-detail/server-item.tsx`，并同步其目录 README。
8. 更新 `runtime.yaml` 与 `profiles/*.yaml`，以及 `docs/PRD.md` / `docs/CONSTRAINTS.md` 的字段说明。
9. 运行 `GOCACHE=/Users/wibus/dev/mcpv/.cache/go-build go test ./...` 与 `pnpm lint`（若环境支持）。

## Validation and Acceptance

需要满足以下可观察行为：

1. `runtime.yaml` 使用 `bootstrapMode: metadata` 且 `defaultActivationMode: on-demand` 时，`minReady: 1` 的服务在无 caller 状态下不再常驻。
2. 设置 `activationMode: always-on` 的服务在启动后保持运行，`ServerInitStatus` 中 minReady 目标为 `max(1, minReady)`。
3. bootstrap 期间不会覆盖运行中的 minReady 目标（观察日志或通过状态接口验证）。
4. `go test ./...` 通过；前端 `pnpm lint` 通过（如执行）。

## Idempotence and Recovery

所有改动均为代码与配置文件编辑，重复执行不会破坏状态。若某一步失败，可回滚对应文件并重新运行测试。若 `make wails-bindings` 不可用，可跳过并在完成后提示手动同步 bindings。

## Artifacts and Notes

完成后记录关键测试输出与必要的差异摘要。

## Interfaces and Dependencies

需要新增/更新的关键接口与类型：

- `internal/domain.ActivationMode`:
  - 取值 `on-demand` / `always-on`。
- `internal/domain.BootstrapMode`:
  - 取值 `metadata` / `disabled`。
- `internal/domain.RuntimeConfig`:
  - 新增 `BootstrapMode`、`DefaultActivationMode` 字段，移除 `StartupStrategy`。
- `internal/domain.ServerSpec`:
  - 新增 `ActivationMode` 字段。
- `internal/app.ServerInitializationManager`:
  - 启动时仅对 `always-on` 目标设定 minReady。

Plan Update Note: 初始创建 ExecPlan（2026-01-01）。
Plan Update Note: 完成配置与启动流程调整、UI/文档更新，并记录生成 bindings 的环境警告（2026-01-01）。
Plan Update Note: 记录 make test 完成与 linker 警告（2026-01-01）。
