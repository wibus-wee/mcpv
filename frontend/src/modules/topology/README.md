<!-- Once this directory changes, update this README.md -->

# Topology Module

拓扑可视化模块，用于展示客户端、标签与服务器的关系图。
基于 React Flow 渲染与布局计算。
页面入口为 `/topology`。

## Files

- **components.tsx**: 空状态与骨架屏组件
- **config-flow.tsx**: 拓扑图主体与数据加载逻辑（包含视图与节点聚焦埋点）
- **layout.ts**: 拓扑布局计算与节点定位
- **topology-page.tsx**: Topology 页面入口
- **types.ts**: 拓扑节点与布局的类型定义

## Directories

- **nodes/**: 拓扑节点渲染与样式定义
