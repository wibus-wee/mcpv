# Import MCP JSON Into Profiles (No Hot Reload)

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.agent/PLANS.md` in the repository root.

## Purpose / Big Picture

用户可以在配置界面粘贴 `mcpServers` JSON（Claude/Cursor 风格），选择目标 profiles 后导入为 MCP servers，并写入对应的 profile 配置文件。导入成功后 UI 明确提示“需要重启 Core 才生效”。用户无需手动编辑 YAML 即可完成增量配置，并能在 UI 中看到新增 server。

## Progress

- [x] (2025-12-29T13:22Z) 设计并实现后端导入接口与配置落盘逻辑，支持 profile store 模式并保留运行时字段。
- [x] (2025-12-29T13:22Z) 在前端实现 JSON 解析、预览、重命名与 profile 多选流程，校验冲突与必填项。
- [x] (2025-12-29T13:22Z) 将导入能力接入 Config 页面入口，并在成功后提示重启与刷新 UI 数据。
- [x] (2025-12-29T13:22Z) 补充后端单测覆盖 YAML 合并、冲突处理与文件选择逻辑。
- [x] (2025-12-29T13:22Z) 更新设计与文档说明导入流程与已知限制。

## Surprises & Discoveries

- Observation: None yet.
  Evidence: N/A.

## Decision Log

- Decision: 仅支持 `mcpServers` JSON，且导入目标仅限 profiles，不提供 caller 级别策略与热更新。
  Rationale: 降低复杂度，保持与当前 profile store 模型一致，避免改动控制面与运行时合成逻辑。
  Date/Author: 2025-12-29 / Codex.

- Decision: 导入成功后只提示“重启 Core 生效”，不触发热更新。
  Rationale: 当前 Core 启动时加载配置，热更新涉及运行时重编排，超出本阶段范围。
  Date/Author: 2025-12-29 / Codex.

- Decision: 采用前端解析 JSON、后端接收规范化的 server 列表进行落盘；后端仍做最小校验。
  Rationale: UI 需要预览与重命名，后端保持安全边界与稳定输出。
  Date/Author: 2025-12-29 / Codex.

- Decision: 导入的 server 默认使用 idleSeconds=60、maxConcurrent=1、protocolVersion=2025-11-25。
  Rationale: 避免 idleSeconds=0 导致实例即时回收，并与现有 catalog 默认值保持一致。
  Date/Author: 2025-12-29 / Codex.

## Outcomes & Retrospective

已完成 MCP JSON 导入到 profiles 的端到端流程：前端解析与校验、后端合并写回 YAML、UI 成功提示与数据刷新。当前限制为重启 Core 后生效，且写回会重新格式化 YAML（不保留注释）。后续若需要热更新或更细粒度编辑，可在此基础上扩展。

## Context and Orientation

配置管理 UI 位于 `frontend/src/modules/config`，当前仅提供只读展示与刷新入口。后端 Wails 服务定义在 `internal/ui/service.go`，通过 `WailsService` 暴露 UI 方法。配置落盘使用 profile store 模式：配置目录包含 `profiles/*.yaml` 和 `callers.yaml`（加载逻辑在 `internal/infra/catalog/profile_store.go`）。现有逻辑仅在 Core 启动时读取配置，不支持热更新。

“Profile” 指包含运行时配置与 server 列表的 YAML 文件；“profile store” 指以目录存储多个 profile 的结构；“mcpServers JSON” 指 MCP 客户端常见的 JSON 配置格式，形如 `{\"mcpServers\": {\"name\": {\"command\": \"node\", \"args\": [\"...\"]}}}`。

## Plan of Work

先在后端增加导入入口与 YAML 合并能力，再在前端实现导入流程和校验，最后补测试与文档。后端新增一个导入请求类型（profiles + servers），并在 `internal/infra/catalog` 新增合并函数：读取 profile YAML、解析 servers 列表、检查重复名称、追加新 servers 后写回文件。写回时保留原有 runtime 配置字段，但会重新格式化 YAML（不会保留注释或原始顺序）。

前端在 Config 页新增导入入口（Sheet），支持粘贴 JSON、解析为 server 列表、允许重命名并选择目标 profiles。若目标 profile 中已存在同名 server，必须先改名才可提交。提交后调用 `@wailsio/runtime` 的 `Call.ByName(\"ui.WailsService.ImportMcpServers\")` 执行导入，并显示“Restart Core to apply changes”的提示。

## Concrete Steps

1) 后端导入与落盘

- 在 `internal/ui/types.go` 增加导入请求与 server 定义类型。
- 在 `internal/infra/catalog` 新增 `BuildProfileUpdate`（读取 + 合并 + 生成 YAML bytes）与 `ResolveProfilePath`（处理 .yaml/.yml）等辅助函数。
- 在 `internal/ui/service.go` 增加 `ImportMcpServers` 方法：校验请求、解析配置模式、对目标 profiles 构建更新，再统一写回磁盘。

2) 前端导入 UI

- 新增 `frontend/src/modules/config/components/import-mcp-servers-sheet.tsx`，包含 JSON 输入、解析预览、重命名、profiles 选择与提交按钮。
- 新增 `frontend/src/modules/config/lib/mcp-import.ts`（或同层文件）实现 `parseMcpServersJson`，仅支持 `mcpServers` JSON。
- 在 `frontend/src/modules/config/config-page.tsx` 的 header 区域添加导入按钮入口。
- 成功后展示提示文案并刷新 `useProfiles` / `useProfileDetails` 数据。

3) 测试与文档

- 在 `internal/infra/catalog` 增加 `_test.go`，覆盖合并写回、重复名称、.yml 文件选择。
- 更新 `docs/CONFIG_VISUALIZATION_DESIGN.md` 增补导入入口与“需重启生效”的限制说明。

## Validation and Acceptance

- 运行 `go test ./...`，确保新增 catalog 合并逻辑的测试通过。
- 手动验证：启动应用，进入 Config 页面，粘贴示例 JSON，选择一个 profile 导入后保存，确认 profile YAML 文件新增 server 且运行时字段仍存在。UI 显示“Restart Core to apply changes”。重启 Core 后 tools 列表与 topology 反映新增 server。

## Idempotence and Recovery

- 导入过程不会修改 `callers.yaml`，只追加 servers 到已存在的 profile 文件。
- 若导入失败，不写回任何文件。若写回失败，提示错误并保持原文件不变。
- 重复导入同名 server 会被拒绝并要求重命名，避免覆盖或破坏已有配置。

## Artifacts and Notes

- Example `mcpServers` JSON for manual validation:

    {
      "mcpServers": {
        "weather": {
          "command": "node",
          "args": ["./weather-demo-mcp/build/index.js"],
          "env": {
            "API_KEY": "demo"
          }
        }
      }
    }

## Interfaces and Dependencies

- Go: `internal/ui/service.go` must expose `ImportMcpServers(ctx context.Context, req ImportMcpServersRequest) error`.
- Go: `internal/infra/catalog` must provide a helper to resolve profile file paths and build updated YAML bytes.
- TS: UI must call `Call.ByName(\"ui.WailsService.ImportMcpServers\", request)` using `@wailsio/runtime` to avoid regenerating bindings.
- TS: JSON parsing accepts only `mcpServers` objects with `command` (string), optional `args` (string[]), optional `env` (record string), optional `cwd` (string). Non-stdio transport should be rejected if provided.

Plan Update Note (2025-12-29): Marked all steps complete, added idleSeconds default decision, and recorded outcomes after implementation.
