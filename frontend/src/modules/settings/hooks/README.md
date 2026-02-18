<!-- Once this directory changes, update this README.md -->

# Settings Hooks

Settings 模块的数据获取与保存逻辑集中于此。
负责运行时与 SubAgent 配置的加载、表单状态与保存。
避免在此目录编写 UI 组件。

## Files

- **use-runtime-settings.ts**: Runtime 配置加载与保存处理（含 observability 与保存埋点）
- **use-gateway-settings.ts**: Gateway UI settings 表单与保存处理（含路由基址校验）
- **use-core-connection-settings.ts**: Core 连接 UI settings 表单与保存处理
- **use-subagent-settings.ts**: SubAgent 配置加载、模型拉取与保存处理（包含埋点）
- **use-update-settings.ts**: Update 偏好读取与保存处理（含 pre-release 开关）
