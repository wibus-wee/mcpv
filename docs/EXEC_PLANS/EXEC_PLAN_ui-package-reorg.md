# UI Package Reorg into Services, Events, Mapping, Types

This ExecPlan is a living document. The sections Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective must be kept up to date as work proceeds.

This plan follows the requirements in /Users/wibus/dev/mcpd/.agent/PLANS.md and must be maintained in accordance with that file.

## Purpose / Big Picture

把内部 Wails UI 桥接层从单层扁平包重组为职责清晰的子包（services / events / mapping / types / transfer），消除文件堆叠和隐性耦合。完成后，服务实现与事件发射、数据映射、DTO 类型分别归位，依赖关系单向（services → mapping/types/events → domain），并且 Wails 绑定与前端调用路径更新到新的包路径。可见的成功标准是：Go 编译通过、`go test ./internal/ui/...` 通过，`make wails-bindings` 生成的新 bindings 能被前端正常 import，前端 typecheck 通过。

## Progress

- [x] (2026-02-09 10:02Z) 创建 ExecPlan 并记录目标与重组策略。
- [x] (2026-02-09 10:34Z) 创建新的目录结构并移动 UI 相关文件。
- [x] (2026-02-09 10:55Z) 更新 Go 包名与 import 路径，补齐导出映射函数与事件发射方法。
- [x] (2026-02-09 11:03Z) 更新 Wails bindings 与前端导入路径（services/types）。
- [x] (2026-02-09 11:07Z) 运行 gofmt 与 `go test ./internal/ui/...` 并记录结果。

## Surprises & Discoveries

- Observation: `go test ./internal/ui/...` 仍出现 macOS 链接版本警告，但测试通过。
  Evidence: `ld: warning: object file ... was built for newer 'macOS' version (26.0) than being linked (11.0)`

## Decision Log

- Decision: 采用 services/events/mapping/types 子包，并新增 types 子包以避免 Go 包循环依赖。
  Rationale: services 与 events/mapping 都需要复用 DTO 类型；把 DTO 下沉到 types 可保证依赖单向，避免 ui ↔ events/mapping 循环。
  Date/Author: 2026-02-09 / Codex

## Outcomes & Retrospective

- 已完成 UI 子包重组、Wails bindings 重建、前端导入更新，`go test ./internal/ui/...` 通过（仅有 macOS 链接警告）。仍建议在有前端依赖环境时运行 `pnpm -C frontend typecheck` 以确认类型检查。

## Context and Orientation

当前 UI 桥接层全部集中在 /Users/wibus/dev/mcpd/internal/ui 目录，包含 Manager、Error、UpdateChecker、事件定义、映射函数、DTO 类型以及十余个 Wails 服务实现文件。Wails 入口位于 /Users/wibus/dev/mcpd/app.go，通过 ServiceRegistry 注册 services。前端 bindings 位于 /Users/wibus/dev/mcpd/frontend/bindings/mcpv/internal/ui，前端代码通过 `@bindings/mcpv/internal/ui` 导入。

这次重组将创建以下子包：

- /Users/wibus/dev/mcpd/internal/ui/services: 所有 Wails service 实现与 ServiceRegistry。
- /Users/wibus/dev/mcpd/internal/ui/events: 事件常量、事件 payload 类型、以及 emit helpers。
- /Users/wibus/dev/mcpd/internal/ui/mapping: domain → UI 类型映射与请求映射。
- /Users/wibus/dev/mcpd/internal/ui/types: Wails 绑定所需的 DTO 类型。
- /Users/wibus/dev/mcpd/internal/ui/transfer: MCP transfer 逻辑（保持现有）。

Manager、Error、UpdateChecker、deep link、config path、shared state 保留在 /Users/wibus/dev/mcpd/internal/ui 根包中。services 子包可以依赖 ui 根包（用于 Manager 与 Error），但 ui 根包不依赖 services，以避免循环。

## Plan of Work

先创建新的子包目录并移动文件：所有 *_service.go、service.go、service_deps.go 与对应测试移动到 services；events.go 与 events_test.go 移动到 events；mapping.go 与 mcp_transfer_mapping.go 移动到 mapping；types.go 移动到 types；transfer 相关保持在 transfer 包。mcp_transfer_filter.go 与 catalog_helpers.go 作为服务内部辅助函数移动到 services。

然后更新 Go package 声明与 import：services 包需要显式引用 ui 根包（Manager、Error、ErrCode）、types 包、mapping 包、events 包；events 包引用 types 与 domain；mapping 包引用 types 与 domain。所有映射函数改为导出版本，供 services 和 DebugSnapshot 使用。

接着更新 Manager 与 UpdateChecker：改用 events 包的 EmitX helpers 与事件常量；StartCoreOptions/UpdateCheckOptions 等 DTO 类型引用 types 包。

最后更新 app.go 的 service registry 引用路径，执行 `make wails-bindings` 重新生成前端 bindings，并批量替换前端 import 路径为 `@bindings/mcpv/internal/ui/services`（类型引用根据生成路径更新）。

## Concrete Steps

在 /Users/wibus/dev/mcpd 目录执行：

  mkdir -p internal/ui/services internal/ui/events internal/ui/mapping internal/ui/types
  git mv internal/ui/*_service.go internal/ui/debug_snapshot.go internal/ui/service.go internal/ui/service_deps.go internal/ui/catalog_helpers.go internal/ui/mcp_transfer_filter.go internal/ui/services/
  git mv internal/ui/*_service_test.go internal/ui/service_test.go internal/ui/service_deps_test.go internal/ui/mcp_transfer_filter_test.go internal/ui/services/
  git mv internal/ui/events.go internal/ui/events_test.go internal/ui/events/
  git mv internal/ui/mapping.go internal/ui/mcp_transfer_mapping.go internal/ui/mapping/
  git mv internal/ui/types.go internal/ui/types/

随后编辑 Go 文件调整包名与 import，运行格式化：

  gofmt -w internal/ui

生成 Wails bindings 并更新前端：

  make wails-bindings
  rg -n "@bindings/mcpv/internal/ui" frontend -g "*.ts" -g "*.tsx"
  # 按生成路径修改 import（见 Implementation 部分）。

## Validation and Acceptance

- 运行 `go test ./internal/ui/...`，期望看到 `ok   mcpv/internal/ui/...` 且无编译错误。
- 运行 `pnpm -C frontend typecheck`（如果前端依赖齐全），期望类型检查通过。
- 观察 Wails 前端可以正常调用服务（例如启动 app 后基础 API 可用）。

## Idempotence and Recovery

所有移动均通过 git 记录，若需要回滚，可使用 `git checkout -- <file>` 或 `git reset --hard`（仅在明确需要回滚且可接受丢失改动时使用）。重复执行 gofmt 与 make wails-bindings 是安全的。

## Artifacts and Notes

- 变更后的包层级应满足：ui 根包不依赖 services；services 依赖 ui 根包、events/mapping/types。
- 事件字符串值必须保持不变，确保前端事件监听不受影响。

## Interfaces and Dependencies

- `internal/ui/services` 必须导出 `ServiceRegistry`、所有 *Service 构造器与方法。
- `internal/ui/events` 必须导出事件常量（EventCoreState 等）与 Emit helpers（EmitCoreState / EmitError / EmitRuntimeStatusUpdated / EmitUpdateAvailable 等）。
- `internal/ui/mapping` 必须导出：
  - MapToolCatalogEntries
  - MapResourcePage
  - MapPromptPage
  - MapRuntimeStatuses
  - MapServerInitStatuses
  - MapActiveClients
  - MapServerSummary
  - MapServerSpecDetail
  - MapServerSpecDetailToDomain
  - MapRuntimeConfigDetail
  - MapTransferIssues / MapDomainToImportSpec / MapImportToDomainSpec
- `internal/ui/types` 负责所有 DTO 类型定义（ToolEntry/ServerSpecDetail/StartCoreOptions/UpdateRelease 等）。

Plan update note (2026-02-09 11:08Z): 记录重组完成、bindings 生成与测试结果，并补充 macOS 链接警告。原因是实际实现与验证已完成，需要更新 Progress/Outcomes/Discoveries 以保持 ExecPlan 可追溯。
