<!-- Once this directory changes, update this README.md -->

# Logs Hooks

日志模块的状态管理 hooks，负责过滤、选择与滚动控制。
输出给 LogsViewer 统一编排。
保持 hooks 纯粹，不做 UI 细节。

## Files

- **index.ts**: hooks 出口汇总
- **use-log-viewer.ts**: 过滤/选择/滚动组合 hooks
