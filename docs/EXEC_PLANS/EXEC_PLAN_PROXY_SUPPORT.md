# 为 Streamable HTTP 增加全局与按服务器 Proxy 支持

这份 ExecPlan 是一个持续更新的文档。`Progress`、`Surprises & Discoveries`、`Decision Log`、`Outcomes & Retrospective` 必须在实施过程中保持最新状态。

本计划遵循仓库根目录的 `.agents/PLANS.md` 约束，请在执行和更新时一并遵守。

## Purpose / Big Picture

目标是让使用者可以在 `runtime.yaml` 中配置全局代理，并在每个 `streamable_http` 服务器上选择“继承全局 / 禁用代理 / 自定义代理”。完成后，用户在需要 HTTP 或 SOCKS5 代理的网络环境中依然可以连接 MCP Streamable HTTP 服务。验证方式是：配置全局代理并运行 `mcpv serve`，然后对远端 Streamable HTTP MCP 服务器执行 `tools/list` 或 `ping` 成功；再切换单个服务器为“禁用代理”或“自定义代理”，并观察连接行为变化。

## Progress

- [x] (2026-02-11 00:00Z) 创建 ExecPlan 文档并记录已知约束。
- [x] (2026-02-11 00:20Z) 定义域模型与配置结构（runtime proxy + server override + effective proxy）。
- [x] (2026-02-11 00:25Z) 扩展 loader/normalizer/validator/schema 以解析与校验 proxy 配置。
- [x] (2026-02-11 00:35Z) 在 Streamable HTTP transport 中应用 proxy（含 socks5）。
- [x] (2026-02-11 00:50Z) 更新 UI 运行时设置与服务器编辑表单，支持 proxy 配置。
- [x] (2026-02-11 01:05Z) 更新示例与测试已完成，生成 bindings 已执行（有缓存权限警告）。
- [x] (2026-02-11 01:20Z) 使用可写缓存重跑 `make lint-fix` 与 `make test`，均成功通过。

## Surprises & Discoveries

- Observation: `http.Transport` 的代理解析依赖 `golang.org/x/net/http/httpproxy`，并原生支持 socks5/socks5h scheme。
  Evidence: `/Users/wibus/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.darwin-arm64/src/net/http/transport.go` 引用 `golang.org/x/net/http/httpproxy` 并在注释中列出 socks5/socks5h。
- Observation: `make wails-bindings` 与 `make test` 在当前环境遇到 go build cache 权限警告/错误。
  Evidence: `operation not permitted` 访问 `/Users/wibus/Library/Caches/go-build/*`。
- Observation: 设置 `GOCACHE=/tmp/go-build` 与 `GOLANGCI_LINT_CACHE=/tmp/golangci-lint` 后，`make lint-fix` 与 `make test` 可正常通过。
  Evidence: `make lint-fix` 输出 `0 issues.`，`make test` 全部包通过。

## Decision Log

- Decision: 在 loader 中计算 per-server 的 effective proxy，并写入 `ServerSpec.HTTP.EffectiveProxy`，transport 只读 effective proxy。
  Rationale: 不改 transport 接口即可引入 runtime 级别的行为变化，同时保证 spec fingerprint 可感知全局 proxy 的变化。
  Date/Author: 2026-02-11 / Codex

- Decision: 复用 `net/http` 的原生 socks5 支持，使用 `httpproxy.Config.ProxyFunc()` 统一生成代理选择逻辑。
  Rationale: Go 1.25 的 `http.Transport` 已支持 socks5/socks5h，避免引入自定义 dialer 与额外复杂度。
  Date/Author: 2026-02-11 / Codex

- Decision: 使用 `golang.org/x/net/http/httpproxy` 实现 proxy 解析与 no_proxy 处理。
  Rationale: `net/http` 内部也依赖该实现，可保持行为一致并支持 socks5/socks5h。
  Date/Author: 2026-02-11 / Codex

## Outcomes & Retrospective

- 代理支持链路已完整打通（runtime + per-server + socks5），lint/test 均通过；剩余风险主要在真实网络环境验证（代理可用性、no_proxy 行为）。

## Context and Orientation

本仓库是 mcpv/mcpd 的 Go + Wails 应用。`streamable_http` 服务器通过 `internal/infra/transport/streamable_http.go` 建立 HTTP/SSE 连接。配置由 `runtime.yaml` 与 `profiles/*.yaml` 组成，解析与校验链路为：

- 结构定义：`internal/domain/types.go`（`RuntimeConfig` 与 `StreamableHTTPConfig`）。
- Loader：`internal/infra/catalog/loader/loader.go`。
- 归一化：`internal/infra/catalog/normalizer/runtime.go` 与 `internal/infra/catalog/normalizer/server.go`。
- 校验：`internal/infra/catalog/validator/server.go` 与 `internal/infra/catalog/validator/schema.json`。
- Spec 指纹：`internal/domain/spec_fingerprint.go`。
- UI 映射：`internal/ui/types/types.go`、`internal/ui/mapping/mapping.go`、`internal/ui/services/config_service.go`。
- 前端设置页：`frontend/src/modules/settings/*`。
- 服务器编辑表单：`frontend/src/modules/servers/components/server-edit-sheet.tsx` 与 `frontend/src/modules/servers/lib/server-form-content.ts`。

术语说明：

- “全局 proxy” 指 `runtime.yaml` 中的 `proxy` 字段，对所有 `streamable_http` 服务器生效。
- “per-server override” 指某个 `streamable_http` 服务器配置中的 `http.proxy` 字段，用于覆盖或禁用全局 proxy。
- “effective proxy” 指最终用于建立连接的代理配置，合并了全局与 per-server 配置。
- “Streamable HTTP” 指 MCP 的 HTTP/SSE 传输方式，当前实现位于 `internal/infra/transport/streamable_http.go`。

## Plan of Work

先扩展域模型与配置结构：为 runtime 增加 `proxy` 字段，并为 `StreamableHTTPConfig` 增加 per-server proxy 与 `EffectiveProxy`。随后完善 loader/normalizer/validator/schema，使 `runtime.yaml` 与 server 配置均可表达并校验 `proxy`。然后在 Streamable HTTP transport 中使用 `httpproxy.Config.ProxyFunc()` 生成代理选择函数，支持 `http/https/socks5/socks5h`，并尊重 `noProxy`。接着更新 UI：运行时设置页新增全局代理配置区块，服务器编辑表单新增 per-server proxy 模式选择和自定义字段。最后更新示例与测试，并运行 lint/test 验证。

## Concrete Steps

以下命令均在仓库根目录执行，除非另有说明。

1) 列出并检查现有配置与 transport 相关实现：

    rg -n "streamable_http|StreamableHTTP|proxy" internal/infra internal/domain frontend

2) 修改后端配置与 loader：

    rg -n "RuntimeConfig|StreamableHTTPConfig" internal/domain/types.go
    rg -n "NormalizeRuntimeConfig" internal/infra/catalog/normalizer/runtime.go
    rg -n "normalizeStreamableHTTPConfig" internal/infra/catalog/normalizer/server.go
    rg -n "ValidateServerSpec" internal/infra/catalog/validator/server.go
    rg -n "spec_fingerprint" internal/domain/spec_fingerprint.go

3) 更新 transport 并补充测试：

    rg -n "buildStreamableHTTPTransport" internal/infra/transport/streamable_http.go
    rg -n "StreamableHTTPTransport_" internal/infra/transport/streamable_http_test.go

4) 更新 UI 与 bindings：

    rg -n "RuntimeConfigDetail|UpdateRuntimeConfigRequest" internal/ui/types/types.go
    rg -n "MapRuntimeConfigDetail" internal/ui/mapping/mapping.go
    rg -n "RuntimeFormState" frontend/src/modules/settings/lib/runtime-config.ts
    rg -n "RuntimeSettingsCard" frontend/src/modules/settings/components/runtime-settings-card.tsx
    rg -n "ServerEditSheet" frontend/src/modules/servers/components/server-edit-sheet.tsx

5) 生成 bindings 并运行测试：

    make wails-bindings
    make lint-fix
    make test

## Validation and Acceptance

- 配置全局 proxy：在 `runtime.yaml` 中设置 `proxy.mode: custom` 与 `proxy.url: socks5://127.0.0.1:1080`。启动 `mcpv serve` 后，连接一个外部 `streamable_http` MCP server，能成功 `ping` 或 `tools/list`。
- 设置 per-server 禁用代理：在某个 server 的 `http.proxy.mode: disabled` 后重载配置，该 server 直连成功，而依赖代理的 server 继续通过全局 proxy 连接。
- 设置 per-server 自定义代理：在某个 server 的 `http.proxy.mode: custom` 且 `http.proxy.url` 指向不同代理，连接行为应按 server 变化。
- 运行 `make test` 应全部通过；新增或修改的测试在变更前失败、变更后通过。

## Idempotence and Recovery

所有步骤均为配置或代码的增量修改，可重复执行。若 `make wails-bindings` 失败，可先保留 Go 改动并在环境准备好后重跑。若代理字段导致配置解析失败，可临时移除 `proxy` 配置并回退到默认（system）模式以恢复运行。

## Artifacts and Notes

示例配置片段：

    proxy:
      mode: custom
      url: socks5://127.0.0.1:1080
      noProxy: "localhost,127.0.0.1"

    servers:
      - name: "remote"
        transport: streamable_http
        http:
          endpoint: "https://example.com/mcp"
          proxy:
            mode: inherit

## Interfaces and Dependencies

需要新增并稳定以下类型与字段：

- `internal/domain/types.go`
  - `type ProxyMode string` with `system`, `custom`, `disabled`, `inherit`.
  - `type ProxyConfig struct { Mode ProxyMode; URL string; NoProxy string }`.
  - `RuntimeConfig.Proxy ProxyConfig`.
  - `StreamableHTTPConfig.Proxy *ProxyConfig` and `StreamableHTTPConfig.EffectiveProxy *ProxyConfig`.

- `internal/infra/transport/streamable_http.go`
  - `buildStreamableHTTPTransport` must clone `http.DefaultTransport` and apply `httpproxy.Config.ProxyFunc()`.
  - Supported proxy URL schemes: `http`, `https`, `socks5`, `socks5h`.

- `internal/ui/types/types.go`
  - `ProxyConfigDetail` for UI bindings.
  - `RuntimeConfigDetail` and `StreamableHTTPConfigDetail` expose proxy fields.

当计划被修改时，需要在文末追加“Plan Revision Note”段落，说明修改原因。

Plan Revision Note: 更新了进度与发现，记录 bindings/lint/test 执行结果与缓存权限问题，标注剩余的验证步骤。
