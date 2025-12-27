# App 范围与职责

## 核心定位

App 作为 MCP Host 的唯一入口，负责下游 server 的生命周期、权限、可观测性与用户体验。用户只与 App 交互，不直接接触 catalog 与协议细节。

## 角色边界

- App：产品入口与策略中心，承接 UI 与运行控制。
- Core：运行时编排，负责路由、调度、实例池与日志聚合。
- Gateway (mcpdmcp)：对外 MCP server，连接 MCP client 并转发到 core。

## 功能模块拆分

### 运行控制

- 启动/停止 core
- 启动状态与错误提示
- 退出时的优雅关闭策略（拒绝新请求，等待 in-flight 完成）

### Profile 管理

- caller -> profile 映射
- default profile 回退
- profile 列表与切换
- profile 的可视化配置编辑

### Tools 管理

- tools/list 拉取与分页
- list-changed 更新提示
- tools 搜索、过滤与分组

### Logs 与观测

- logging/setLevel 控制最小日志等级
- 日志流展示与过滤
- caller 维度归因与聚合

### 资源与 prompts（P1 起）

- resources/list 展示与分页
- prompts/list 展示与触发
- prompts/get 结果展示与复制

### 安全与合规

- 高敏感 tool 调用的确认
- 日志脱敏与最小化
- 资源链接的安全校验

### 稳定性与协议对齐

- 初始化与版本协商流程正确执行
- tools/prompts/resources 的分页与 listChanged 订阅
- logging/setLevel 控制与等级映射
- 连接中断的重试与退避

### 诊断与支持

- 一键导出日志与配置快照
- 运行状态与版本信息

## 非目标（初期）

- daemon 或系统后台服务
- 远程 core 连接
- 自动安装外部 MCP server

## 演进节奏

- P0：运行控制 + tools + logs + profile 选择
- P1：资源与 prompts 展示 + 可视化配置编辑
- P2：专业模式与诊断增强
