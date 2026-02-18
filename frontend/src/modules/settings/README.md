<!-- Once this directory changes, update this README.md -->

# Modules/Settings

全局运行时配置（含 observability 与 proxy）、Core 连接、Gateway 与 SubAgent 设置模块。
通过 bindings 读取与更新 runtime 配置，并触发配置重载或 UI settings 更新。
设置相关页面、组件与逻辑统一放在该模块中。

## Files

- **settings-page.tsx**: 设置主页面，整合 Runtime 与 SubAgent 卡片

## Directories

- **components/**: Settings 模块 UI 组件
- **hooks/**: Settings 相关数据与表单 hooks
- **lib/**: Settings 配置结构与文案常量
