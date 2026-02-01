<!-- Once this directory changes, update this README.md -->

# Logs Components

日志模块的 UI 组件集合，覆盖列表、详情与辅助展示。
组件仅关注渲染与交互，数据由外层 hooks 提供。
保持组件职责清晰，避免跨模块依赖。

## Files

- **index.ts**: 组件出口汇总
- **log-detail-drawer.tsx**: 日志详情抽屉视图
- **log-detail-panel.tsx**: 右侧详情面板视图
- **log-level-badge.tsx**: 日志级别徽标
- **log-metadata.tsx**: 日志元信息展示组件
- **log-row-expanded.tsx**: 日志行展开内容
- **log-row.tsx**: 基础日志行渲染
- **log-table-row.tsx**: 表格样式日志行渲染
- **log-toolbar.tsx**: 过滤与操作工具条
- **log-trace-timeline.tsx**: 执行链路时间线展示
- **logs-bottom-panel.tsx**: 底部日志详情面板
- **trace-node-icon.tsx**: Trace 节点图标与序列组件
