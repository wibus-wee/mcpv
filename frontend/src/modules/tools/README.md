<!-- Once this directory changes, update this README.md -->

# Modules/Tools

工具模块，负责工具列表的取数与展示。  
依赖 Wails bindings 获取数据，并提供按服务器分组的结果。  
子目录承载工具页面的 UI 组件。

## Files

- **hooks.ts**: 工具列表与运行状态的 SWR 拉取与分组 hook。
- **components/**: 工具网格、服务器卡片与详情抽屉的 React 组件集合。
