<!-- Once this directory changes, update this README.md -->

# Dashboard Components

仪表盘模块内的可复用组件集合。
每个组件负责仪表盘 UI 的一个独立区域。
跨模块复用的组件请放在 components/common 或 components/ui。

## Files

- **index.ts**: 组件统一导出
- **bootstrap-progress.tsx**: 服务器启动进度面板，显示 bootstrap 状态、进度条、错误信息；包含 `BootstrapProgressPanel` (完整卡片) 和 `BootstrapProgressInline` (内联紧凑版)
- **status-cards.tsx**: 状态概览卡片，展示核心状态、运行时长与数量统计
- **tools-table.tsx**: 工具表格，支持搜索与详情弹窗
- **resources-list.tsx**: 资源列表与折叠详情
- **logs-panel.tsx**: 实时日志面板，支持等级/来源/Server 过滤与自动滚动开关
- **settings-sheet.tsx**: 设置面板，包含主题/刷新间隔/通知/日志级别
