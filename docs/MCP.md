# MCP 协议要点（2025-11-25）

本文汇总 Model Context Protocol（MCP）2025-11-25 版的实现要点，聚焦 `mcpv` MVP（Go、stdio transport 子进程、JSON-RPC 2.0）。正文中文，代码块内为 English。

## 基础与版本
- 目标协议版本：`2025-11-25`，`initialize` 协商时需拒绝其他版本。
- 消息格式：JSON-RPC 2.0，默认 JSON Schema 方言为 2020-12；若使用其它方言需在 schema 中显式 `$schema`。
- 内容类型：text / blob 资源皆用 MIME 标注；资源、工具、提示等对象均可带 `icons`、`annotations`（audience/priority/lastModified）。

## 握手与能力协商
- 首次调用必须是 `initialize`，客户端参数：
  - `protocolVersion`: string（客户端支持的最高版本，例 `2025-11-25`）
  - `capabilities`: `ClientCapabilities`（roots/sampling/elicitation/tasks/experimental 等，子能力如 `listChanged`）
  - `clientInfo`: `Implementation`（name 必填，version 必填，可选 title/description/icons/websiteUrl）
- 服务端成功响应包含：
  - `protocolVersion`: 协商结果
  - `capabilities`: `ServerCapabilities`（prompts/resources/tools/logging/tasks/completions/experimental；子能力如 `listChanged`、`subscribe`）
  - `serverInfo`: `Implementation`
  - 可选 `instructions`
- 版本或能力不匹配应返回 JSON-RPC error（常用 `-32602` unsupported protocol version，附 supported/requested）。

### 能力表（核心）
- Client: `roots`（提供根路径、listChanged 通知）、`sampling`、`elicitation`（form/url）、`tasks`（list/cancel/requests.*）、`experimental`
- Server: `prompts`、`resources`（subscribe/listChanged）、`tools`（listChanged）、`logging`、`completions`、`tasks`（list/cancel/requests.tools.call）、`experimental`

## 传输与会话
- `mcpv` MVP 使用 stdio 子进程：建议逐行（`\n`）分隔 JSON-RPC 消息，启动后立即发送 `initialize`。
- HTTP 规范：新版本为 Streamable HTTP（可 SSE 下行、POST 上行，支持会话 ID `MCP-Session-Id`）；老版 HTTP+SSE 兼容策略可忽略。
- `mcpv` 现支持连接外部 Streamable HTTP MCP server（不负责启动本地 HTTP server），以 `transport: streamable_http` + `http.endpoint` 配置。
- 会话 ID 规则（仅 HTTP 相关）：服务器可在 `initialize` 响应头返回 `MCP-Session-Id`，后续请求需回传；服务器终止会话后返回 404。

## 资源（Resources）
- 支持该能力时必须声明 `capabilities.resources`，可选 `subscribe`、`listChanged`。
- 发现：`resources/list`（支持 cursor 分页）；`resources/templates/list` 发现可参数化的 URI 模板。
- 订阅：`resources/subscribe` 针对单个资源更新；服务器主动推送 `resources/updated` 通知。
- 资源对象字段：`uri`（必填）、`name`（必填）、`mimeType`、`title`、`description`、`size`、`icons`、`annotations`。内容可为 text 或 base64 `blob`。

## 工具（Tools）
- 发现：`tools/list`（cursor 分页）。工具定义：`name`、`title`、`description`、`inputSchema`、可选 `outputSchema`、`icons`、`execution.taskSupport`（required/optional/forbidden）。
- 调用：`tools/call`，参数 `name` + `arguments`（符合 inputSchema）。响应 `result` 含 `content`（文本或结构化字符串），可带 `structuredContent`；`isError: true` 表示业务错误。未知工具应返回 `-32602 Unknown tool`。
- 任务化：若服务端声明 `capabilities.tasks.requests.tools.call`，客户端可在请求加入 `task` 字段创建异步任务，否则不得附带任务。

## 提示（Prompts）
- 能力声明 `prompts`，可带 `listChanged`。
- 发现：`prompts/list`（分页）。
- 获取：`prompts/get`（`name`，可附 `arguments`，支持 completion 自动补全）。
- 变更通知：若声明 `listChanged`，服务器应发送 `notifications/prompts/list_changed`。

## 采样与 Tool Use（sampling/createMessage）
- `sampling/createMessage`：输入对话消息，可附 `tools`、`toolChoice` 控制生成；响应可含多个 `tool_use`。
- 序列约束：任意包含 `ToolUseContent` 的 assistant 消息，下一条 user 消息必须仅包含对应 `ToolResultContent`，且逐一匹配每个 tool use，避免“缺失结果”序列。

## Elicitation（信息征集）
- 服务器向客户端发起：`elicitation/create`，`mode` 支持 `form` 或 `url`。
  - form: `message` + 可选 `requestedSchema`（平面 JSON Schema，原语/enum/oneOf）。
  - url: `elicitationId` + `url` + `message`。
- 客户端响应 `result.action`（accept/decline/cancel）+ 可选 `content`。未完成 URL 交互时，服务器可返回 `URL_ELICITATION_REQUIRED`（示例 code `-32042`）错误。

## 任务（Tasks）
- 任务化请求：在支持的请求（如 `tools/call`）参数加入 `task`（可带 `ttl`），立即返回 `CreateTaskResult`，真实结果通过 `tasks/result` 轮询获取。

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "get_weather",
    "arguments": { "city": "New York" },
    "task": { "ttl": 60000 }
  }
}
```

- 查询：`tasks/get` 返回状态、`pollInterval`、`statusMessage`；`tasks/list` 支持 cursor 分页；`tasks/result` 取最终结果；`tasks/cancel`（若声明）取消任务。
- 状态：`working`/`completed`/`failed`/`cancelled`，失败时返回 `statusMessage`。

## 资源模板（Resource Templates）
- `resources/templates/list` 返回 `uriTemplate`、`name`、`title`、`description`、`mimeType`、`icons`，用于提示/自动补全参数化资源。

## 日志与观测
- 协议提供结构化日志能力（`capabilities.logging`），日志消息为 JSON-RPC notification（MVP 可先忽略，必要时映射到 zap）。
- Completion（`completion/complete`）提供参数自动补全；需要 `capabilities.completions`。

## 错误处理与代码
- 常见 JSON-RPC 错误码：`-32601 Method not found`（未声明能力时返回）、`-32602 Invalid params`（版本不匹配/参数非法）、`-32002 Resource not found`、`-32600 Task augmentation required` 等。
- 任务/工具错误可用业务层 `isError: true` 或任务状态 `failed` 携带 `statusMessage`。
- Elicitation 特殊错误：`URL_ELICITATION_REQUIRED`（示例 `-32042`），包含待完成的 elicitations 列表。

## 安全提示
- 配置/启动命令可能被注入恶意指令（示例含 `curl` 外传、`sudo rm -rf`）；加载 catalog 时需显式限制可执行命令来源与环境变量。
- 日志需过滤敏感 env/密钥；资源订阅/工具调用返回的数据默认可信度有限。

## 实现勾子（对 mcpv）
- 子进程启动后：发送 `initialize`（带客户端能力、版本），等待成功响应后才进入路由阶段。
- 路由层：仅允许声明过的 method（prompts/resources/tools/tasks/elicitation/sampling/completion/logging）；未声明能力直接返回 `-32601`。
- 缩容/重建：若 `initialize` 或后续 `ping`（可用 `tools/call` 健康探针或自定义资源读取）失败，标记实例 Failed 并清理。
