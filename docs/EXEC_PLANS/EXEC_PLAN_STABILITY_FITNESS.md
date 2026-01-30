# 稳定性与边界约束修复

这是一个可执行计划（ExecPlan），它是一个活文档。必须在执行过程中持续更新 `Progress`、`Surprises & Discoveries`、`Decision Log` 和 `Outcomes & Retrospective`。

仓库内存在 `.agent/PLANS.md`，本计划必须符合其要求并持续维护。

## Purpose / Big Picture

完成本计划后，服务初始化重试不再在循环中分配短生命周期定时器，SubAgent 只接受严格 JSON 输出并拒绝模糊匹配，生命周期管理拥有明确的根上下文，配置环境变量缺失会在启动阶段被标记告警，domain 接口按职责拆分为更易维护的文件。用户可观察到的行为保持一致，但在高负载、边界输入与长期运行时更稳定、可诊断。

## Progress

- [x] (2025-03-07 14:40Z) 创建 ExecPlan 并完成现状梳理。
- [x] (2025-03-07 15:10Z) 修复 `server_init_manager.go` 中的循环定时器分配，并减轻 caller registry 的锁内排序工作。
- [x] (2025-03-07 15:10Z) 强制 SubAgent 使用 JSON 响应格式并移除模糊匹配回退。
- [x] (2025-03-07 15:20Z) 拆分 domain 接口文件并为 lifecycle Manager 注入根上下文。
- [x] (2025-03-07 15:20Z) 在 env 展开流程中记录缺失变量告警并更新 loader。
- [x] (2025-03-07 15:30Z) 运行 `make fmt`、`go vet ./...` 与 `make test`。

## Surprises & Discoveries

- Observation: 运行 `make fmt` 需要设置 `GOCACHE` 到工作区，默认缓存目录不可写。
  Evidence: `open /Users/wibus/Library/Caches/go-build/...: operation not permitted`
- Observation: `make test` 时 `internal/ui` 链接出现 macOS 版本提示但不影响通过。
  Evidence: `ld: warning: object file ... was built for newer 'macOS' version (26.0) than being linked (11.0)`

## Decision Log

- Decision: 子模块重构仅聚焦问题点，不引入外部依赖或兼容层。
  Rationale: 目标是降低风险并保持实现可维护性。
  Date/Author: 2025-03-07 / Codex
- Decision: SubAgent 仅接受 JSON 数组输出，解析失败或包含无效工具名时触发回退。
  Rationale: 避免模糊匹配导致误执行。
  Date/Author: 2025-03-07 / Codex
- Decision: lifecycle Manager 使用根上下文作为启动父级，并在启动阶段仅监听调用方上下文取消。
  Rationale: 既保持可取消启动，又避免进程生命周期脱离应用上下文。
  Date/Author: 2025-03-07 / Codex

## Outcomes & Retrospective

完成了初始化重试定时器复用、SubAgent JSON 强约束、domain 接口拆分、生命周期根上下文注入与 env 缺失告警；核心行为保持一致，边界与可诊断性更清晰。

## Context and Orientation

`internal/app/server_init_manager.go` 的 `runSpec` 在循环中使用 `time.After`，会在高频重试时产生短期对象压力。`internal/infra/subagent/subagent.go` 的 `parseSelectedTools` 存在文本包含回退，可能误匹配或导致不可控执行。`internal/infra/lifecycle/manager.go` 的 `StartInstance` 使用 `context.Background` 作为根上下文，导致启动与关闭链路不清晰。`internal/infra/catalog/env_expand.go` 使用 `os.ExpandEnv`，但对缺失变量没有明确告警策略。`internal/domain/types.go` 中汇集了多个接口，缺少按领域拆分。

## Plan of Work

先为 `ServerInitializationManager` 引入可复用 `time.Timer`，并在 `callerRegistry` 中将排序从锁内移到锁外，确保锁内只做最小数据写入。接着调整 SubAgent 的提示与解析逻辑，强制 JSON 数组输出并移除字符串包含回退。然后拆分 domain 接口为独立文件（transport、lifecycle、scheduler、router、catalog_loader、health_probe），并让 lifecycle Manager 在构造时接收根上下文，避免 `context.Background` 漏出。随后扩展 env 展开逻辑，记录缺失变量并在 loader 内统一告警。最后运行格式化、静态检查与测试，并记录结果。

## Concrete Steps

在 `/Users/wibus/dev/mcpv` 下工作。

1) 修复 `server_init_manager.go` 的定时器使用与 caller registry 的锁内排序。

    rg -n "runSpec|time.After" internal/app/server_init_manager.go
    rg -n "snapshotActiveCallers" internal/app/control_plane_registry.go

2) 强制 SubAgent JSON 输出并移除模糊匹配回退。

    rg -n "parseSelectedTools|defaultFilterSystemPrompt" internal/infra/subagent/subagent.go

3) 拆分 domain 接口并调整 lifecycle Manager 的 root context。

    rg -n "type Conn|type Transport|type Scheduler|type Router" internal/domain/types.go
    rg -n "StartInstance|Background" internal/infra/lifecycle/manager.go

4) 为 env 展开缺失变量告警并更新 loader。

    rg -n "expandConfigEnv" internal/infra/catalog

5) 运行验证命令。

    make fmt
    go vet ./...
    make test

## Validation and Acceptance

运行 `make fmt`、`go vet ./...` 与 `make test` 均通过。SubAgent 的工具筛选在 LLM 返回非 JSON 时直接报错并回退到全量工具列表（记录告警），不再基于 `strings.Contains` 执行模糊匹配。`ServerInitializationManager` 的重试循环不再分配新定时器。启动时如果配置包含缺失的 `${VAR}`，日志会输出缺失变量列表。

## Idempotence and Recovery

所有步骤可重复执行，不引入不可逆迁移。若拆分接口文件导致编译失败，可临时回退到 `internal/domain/types.go` 并逐步迁移。

## Artifacts and Notes

记录关键日志片段，例如 env 缺失变量告警、SubAgent JSON 解析失败的告警，以及 `go vet`/`make test` 的输出摘要。

## Interfaces and Dependencies

新增或调整的接口与入口：

- 在 `internal/infra/catalog/env_expand.go` 中的 `expandConfigEnv` 返回缺失变量列表。
- 在 `internal/infra/catalog/loader.go` 中记录缺失变量告警。
- 在 `internal/infra/lifecycle/manager.go` 中调整 `NewManager` 以接收根上下文。
- 在 `internal/domain/transport.go`、`internal/domain/lifecycle.go`、`internal/domain/scheduler.go`、`internal/domain/router.go`、`internal/domain/catalog_loader.go`、`internal/domain/health_probe.go` 中定义对应接口与类型。

Plan Change Note: 完成本计划实现与验证，补充测试与告警观察记录。
