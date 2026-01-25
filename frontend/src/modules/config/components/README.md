<!-- Once this directory changes, update this README.md -->

# 配置模块组件

配置管理功能的专用组件集合。
保持主从布局：左侧列表，右侧详情与拓扑区域。
跨模块通用组件请放在 `components/common` 或 `components/ui`。

## Files

- **servers-list.tsx**: Server 列表与选中态交互（含 tag 过滤）
- **server-detail-panel.tsx**: Server 详情面板，展示配置与运行状态
- **clients-list.tsx**: 活跃 client 列表（含 tag chips）
- **import-mcp-servers-sheet.tsx**: MCP JSON 导入面板（server 级别）
- **server-runtime-status.tsx**: Server pool 运行状态概览与实例详情展示
