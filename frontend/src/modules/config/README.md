<!-- Once this directory changes, update this README.md -->

# Modules/Config

配置管理模块，负责展示 profiles/callers/servers 与配置入口。
页面通过 Wails UI bindings 拉取配置与运行状态。
提供列表、详情与拓扑视图的组合。

## Files

- **config-page.tsx**: 配置主页面，组织头部与标签页布局
- **atoms.ts**: 配置相关的 Jotai 状态容器
- **hooks.ts**: 配置数据获取 hooks，负责 profiles/callers/details/runtime
- **components/**: 配置模块的 UI 子组件集合

## Components

- **components/profiles-list.tsx**: 左侧 profiles 列表与选择状态
- **components/profile-detail-panel.tsx**: profile 详情面板，含 runtime 与 servers
- **components/callers-list.tsx**: caller 到 profile 的映射列表
- **components/import-mcp-servers-sheet.tsx**: MCP JSON 导入入口与流程
- **components/server-runtime-status.tsx**: server pool 运行状态指示器
- **components/config-flow.tsx**: profiles/callers/servers 拓扑关系图与节点渲染
