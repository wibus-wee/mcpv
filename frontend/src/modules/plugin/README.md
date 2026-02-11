<!-- Once this directory changes, update this README.md -->

# Plugin Module

插件管理模块，负责插件列表展示与编辑表单。
支持插件的新增、编辑、删除与启用/禁用。
主要入口为 `/plugins`（开发模式下可见）。

## Files

- **hooks.ts**: 插件列表与过滤相关 hooks
- **plugin-page.tsx**: 插件管理主页面（搜索、列表、编辑入口，含埋点）

## Directories

- **components/**: 插件管理相关 UI 组件
