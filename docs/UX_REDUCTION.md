# 减少用户使用负担的产品与技术方案

## 背景与目标

我们的目标是让普通用户在不理解 MCP 架构与 catalog 的情况下完成“开箱即用”，同时保留专业用户的可控性与可扩展性。UI 应该承担主入口职责，完成配置、启动、可观测性展示，并与 MCP 生态的使用习惯保持一致。

## MCP 规范中的典型使用方式（用户视角）

基于 MCP 2025-11-25 规范，典型用户在 Host 应用中使用 MCP 的方式可抽象为以下事实：

- Host 负责创建并管理多个 client，每个 client 与一个 server 形成 1:1 会话连接，由 Host 统一编排与生命周期管理。
- tools 发现是通过 `tools/list` 请求进行，支持分页，server 如果支持 tools 必须在 capabilities 中声明并可通过 list-changed 通知提示更新。
- tools 调用通过 `tools/call` 完成，响应包含结果与错误标记。
- 初始化阶段需要进行协议版本协商；若客户端不支持服务器返回版本，应断开；初始化完成后 client 需要发送 `initialized`。
- Host 在安全与权限上负责管理用户授权与敏感操作确认，并记录工具使用行为。

这些约束决定了：我们应把 App 设计为 Host，承担编排、权限、可观测性与体验入口的职责，而不是让用户直接接触底层 server 或 catalog。

## MCP 交互基线（从协议视角提炼）

- 初始化必须先于任何业务请求：client 先发 `initialize`，server 回应能力与版本，随后 client 发 `initialized`。
- capabilities 是功能边界：tools、resources、prompts、logging 等均通过 capabilities 声明是否支持以及 listChanged/subscribe 等能力。
- tools/list 与 prompts/list、resources/list 都支持分页，客户端需要分页拉取并缓存。
- logging 使用 `logging/setLevel` 设置最小等级，服务端通过 `notifications/message` 推送日志。
- stdio 是本地常见的 transport，stdout 只能输出 JSON-RPC 消息，stderr 仅作日志，不能视为错误通道。

## MCP 角色映射到当前产品

- App = Host：管理多个下游 MCP server 的生命周期、权限与可观测性。
- core = Host runtime：执行调度与路由，隐藏 catalog，暴露统一控制面。
- mcpdmcp/gateway = MCP server：对外提供 MCP tools/list 和 tools/call，会话面向外部 MCP client。

## 用户负担的关键来源

- 多 server 组合需要手工配置与启动，缺乏统一入口。
- tools/list 与 logs 分散在不同 server，需要手工定位。
- 初始化/版本协商/能力边界对用户不可见，错误难以理解。

## 减负原则

- 默认隐藏 catalog，仅暴露“caller profile”这一层。
- “一键启动”优先于“可配置能力”，高级能力延后在 Pro 模式提供。
- UI 是唯一入口，mcpdmcp 只作为 MCP 客户端入口和 URL scheme 触发器。
- 所有操作都应可解释、可恢复，不把用户置于不可逆的状态。

## 用户体验路径

### 1. 一键启动

用户在 MCP 客户端执行：

```
mcpdmcp <caller>
```

如果 App 未运行，mcpdmcp 通过 URL scheme 拉起 App：

```
mcpdmcp://start?caller=vscode
```

App 启动后根据 caller 选择 profile（未配置则回退到 default），启动 core，并向 mcpdmcp 提供 MCP 会话。

### 2. 工具可见性与日志

- UI 在启动后展示 tools 列表与日志流。
- tools 列表遵循 `tools/list` 的分页与 list-changed 机制；列表更新时 UI 自动刷新。
- 日志按 caller 归因，可快速定位调用方问题。

### 3. 配置管理

- 普通用户：仅看到 profile 的可视化配置项（模板、开关、路径等）。
- 高级用户：允许导入/导出 catalog，并查看完整 YAML。

### 4. 空状态与失败路径

- caller 未映射时明确提示已回退到 `default`，并提供一键创建映射入口。
- core 启动失败时展示错误摘要与重试按钮，同时提供日志入口。
- tools 列表为空时提示可能原因（未启动 server、权限不足、配置为空）并给出下一步操作。
- 连接中断时 UI 显示重连状态与退避时间，避免用户误以为卡死。

## 关键产品机制

### Profile 与 Caller

- caller 是唯一用户输入；profile 是内部配置实体。
- caller 未映射到 profile 时回退到 `default`。
- profile 内部是 catalog 的集合，但对用户隐藏。

### Spec 复用

- 仅当 ServerSpec 完全一致时复用实例池。
- 任何参数不同即创建新池，避免隐性共享导致的可观测性混乱。

### 启停策略

- 初期不支持 daemon。
- core 跟随 App 生命周期。
- App 关闭时拒绝新请求，等待 in-flight 请求完成后停止 core。

### MCP 能力覆盖策略

- P0 仅覆盖 tools 与 logging，满足最常见使用路径。
- resources/prompts 作为 P1 能力补齐，先以只读展示为主。
- 统一 listChanged 订阅，确保 UI 自动刷新。

## 安全与合规

- tools/call 前进行必要的参数展示与确认（针对高敏感操作）。
- 日志与工具使用记录默认开启，便于审计与故障定位。
- 如果 tool 输入含路径或外部资源，UI 应提示风险与权限。
- 日志必须过滤敏感信息；对 tools 输出与资源链接进行基本校验。

## 交互设计要点

- 核心操作不超过 2 步：选择 profile -> 启动。
- 状态必须显式：未启动 / 启动中 / 运行中 / 出错。
- 失败时提供一键重试与“查看日志”入口。

## App 应具备的核心功能

- Profile 管理：caller -> profile 映射与默认回退。
- 运行控制：启动、停止、重启、运行状态与错误提示。
- Tools 视图：列表、搜索、按 profile 过滤与更新提示。
- Logs 视图：实时流、等级过滤、按 caller 归因。
- 安全提示：高敏感 tool 调用需显式确认。
- 诊断导出：最小日志与配置快照用于问题定位。
- 空状态与错误引导：未映射、启动失败、无工具时给出下一步操作。

## 实施计划（产品角度）

1. P0：一键启动闭环
   - mcpdmcp + URL scheme 拉起 App
   - caller -> default profile fallback
   - tools list + logs UI

2. P1：可视化配置
   - profile 编辑表单
   - 模板与预设
   - 基于 profile 的工具列表过滤

3. P2：专业模式
   - catalog 导入/导出
   - 高级运行参数
   - diagnostics 导出

## 与 MCP 规范对齐的实现建议

- 初始化时严格进行协议版本协商并记录结果。
- tools/list 做分页与缓存，避免 UI 阻塞。
- list-changed 通知触发 UI 刷新。
- tools/call 结果与错误标记分开展示。

## 预期结果

- 普通用户不需要理解 MCP 协议或 catalog 格式即可使用。
- MCP 客户端的使用方式保持不变，只替换入口为 `mcpdmcp`。
- 在不牺牲可观测性的情况下显著降低上手成本。
