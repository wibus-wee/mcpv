<!-- Once this directory changes, update this README.md -->

# Settings Lib

Settings 模块的配置结构、模型与文案集合。
包含 runtime/subagent 的表单结构与辅助工具。
该目录仅存放无 UI 的逻辑与常量。

## Files

- **runtime-config.ts**: 运行时配置表单结构与默认值（含 observability 与 proxy）
- **runtime-help.ts**: 运行时配置字段帮助文案（含 observability 与 proxy）
- **gateway-config.ts**: Gateway 设置表单结构、默认值与路由基址辅助
- **gateway-help.ts**: Gateway 设置字段帮助文案（含路由基址说明）
- **subagent-config.ts**: SubAgent 配置表单结构与默认值
- **subagent-help.ts**: SubAgent 配置字段帮助文案
- **subagent-models.ts**: SubAgent 模型列表与获取逻辑
