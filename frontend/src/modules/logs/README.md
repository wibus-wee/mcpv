<!-- Once this directory changes, update this README.md -->

# Logs Module

日志浏览与检索模块，负责日志表格、详情与底部面板展示。
列表渲染使用低优先级更新以保持滚动流畅。
提供过滤、选择、虚拟滚动与面板联动能力。
路由通过 /logs 进入该模块。

## Files

- **index.ts**: 模块出口，统一导出 components/hooks/types/utils
- **logs-viewer.tsx**: 日志主视图（表格 + 详情面板 + 底部面板）

## Directories

- **components/**: 日志相关 UI 组件
- **hooks/**: 日志过滤、选择、滚动状态 hooks
- **types/**: 日志模块类型定义
- **utils/**: 格式化与过滤等工具函数
