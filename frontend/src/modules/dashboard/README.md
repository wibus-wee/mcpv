<!-- Once this directory changes, update this README.md -->

# Modules/Dashboard

仪表盘业务模块，展示核心状态与系统概览。
包含页面层与模块内的数据获取逻辑。
核心状态与日志等跨模块数据通过共享 hooks 提供。

## Files

- **dashboard-page.tsx**: 仪表盘主页面，组合头部、标签页与内容区；展示核心状态与操作
- **hooks.ts**: 仪表盘数据获取 hooks，包含 `useAppInfo`、`useTools`、`useResources`、`usePrompts`、`useBootstrapProgress`

## Components

- **components/bootstrap-progress.tsx**: 服务器启动进度面板，支持完整卡片和内联两种形式
- **components/status-cards.tsx**: 状态概览卡片，展示核心状态、运行时长与数量统计，支持本地/远程视图切换
- **components/local-core-daemon-card.tsx**: 本地 core daemon 状态与启动引导卡片
- **components/remote-core-connection-card.tsx**: Core 连接状态与远程模式入口卡片
- **components/tools-table.tsx**: 可搜索的工具表格，含详情弹窗
- **components/resources-list.tsx**: 资源列表与折叠详情
- **components/logs-panel.tsx**: 实时日志面板，支持等级/来源/Server 过滤与自动滚动开关
- **components/settings-sheet.tsx**: 设置面板，包含主题/刷新间隔/通知/日志级别
- **components/index.ts**: 仪表盘组件的统一导出
