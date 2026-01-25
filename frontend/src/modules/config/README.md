<!-- Once this directory changes, update this README.md -->

# Modules/Config

配置管理模块，负责展示并编辑 servers/tags/clients 与配置入口。
页面通过 Wails UI bindings 拉取配置与运行状态，编辑操作写回单一 config 文件。
提供列表与详情视图，保存后可触发 Core 重载。

## Files

- **config-page.tsx**: 配置主页面，组织头部与标签页布局
- **atoms.ts**: 配置相关的 Jotai 状态容器
- **hooks.ts**: 配置数据获取 hooks，负责 servers/clients/details/runtime
- **components/**: 配置模块的 UI 子组件集合

## Components

- **components/servers-list.tsx**: 左侧 servers 列表与 tag 过滤
- **components/server-detail-panel.tsx**: server 详情面板，包含运行状态与操作
- **components/clients-list.tsx**: 活跃 client 列表与 tag 展示
- **components/import-mcp-servers-sheet.tsx**: MCP JSON 导入入口与流程
- **components/server-runtime-status.tsx**: server pool 运行状态指示器
