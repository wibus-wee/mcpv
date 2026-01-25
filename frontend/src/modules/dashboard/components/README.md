<!-- Once this directory changes, update this README.md -->

# Dashboard Components

仪表盘模块内的可复用组件集合。
每个组件负责仪表盘 UI 的一个独立区域。
跨模块复用的组件请放在 components/common 或 components/ui。

## Files

- **index.ts**: 组件统一导出
- **sparkline.tsx**: 可视化基础组件，包含 `Sparkline` (迷你折线图)、`MiniGauge` (环形进度)、`StackedBar` (堆叠条形图)、`AnimatedNumber` (动画数字)、`TrendIndicator` (趋势指示器)
- **server-health-overview.tsx**: 服务器健康概览面板，显示实例池状态分布、利用率、健康诊断和运行指标
- **activity-insights.tsx**: 活动洞察面板，展示工具使用分析、吞吐量和延迟指标
- **active-clients-panel.tsx**: 活跃客户端面板，显示已连接的 IDE/客户端及其状态
- **bootstrap-progress.tsx**: 服务器启动进度面板，显示 bootstrap 状态、进度条、错误信息；包含 `BootstrapProgressPanel` (完整卡片) 和 `BootstrapProgressInline` (内联紧凑版)
- **status-cards.tsx**: 状态概览卡片，展示核心状态、运行时长与数量统计，含动画效果
- **tools-table.tsx**: 工具表格，支持搜索与详情弹窗
- **resources-list.tsx**: 资源列表与折叠详情
- **logs-panel.tsx**: 实时日志面板，支持等级/来源/Server 过滤与自动滚动开关
