<!-- Once this directory changes, update this README.md -->

# 配置模块组件

配置管理功能的专用组件集合。
保持主从布局：左侧列表，右侧详情与拓扑区域。
跨模块通用组件请放在 `components/common` 或 `components/ui`。

## Files

- **profiles-list.tsx**: Profiles 列表与选中态交互
- **profile-detail-panel.tsx**: Profile 详情面板，展示 server 信息与 SubAgent 状态
- **callers-list.tsx**: Caller 与 profile 的映射列表
- **import-mcp-servers-sheet.tsx**: MCP JSON 导入面板（profiles 级别）
- **server-runtime-status.tsx**: Server pool 运行状态概览与实例详情展示
- **config-flow.tsx**: Profiles/callers/servers 的拓扑视图与自定义节点
