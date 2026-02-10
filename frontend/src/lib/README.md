<!-- Once this directory changes, update this README.md -->

# Lib

共享工具与业务无关的基础能力集中于此。
包含解析、时间、状态、请求与分析等通用逻辑。
避免在此目录放入 UI 或业务模块代码。

## Files

- **analytics.ts**: Umami 统计封装，提供开关、事件追踪、统一上下文与页面浏览上报
- **analytics-events.md**: AnalyticsEvents 事件规范与字段说明（英文）
- **framer-lazy-feature.ts**: Motion LazyMotion 特性加载器
- **is-dev.ts**: 开发环境判断工具
- **jotai.ts**: Jotai store 与 atom hooks 辅助函数
- **mcpvmcp.ts**: 构建 mcpvmcp 客户端配置与 CLI 片段的工具函数
- **parsers.ts**: 输入解析与格式化工具（环境变量、逗号列表等）
- **parsers.test.ts**: parsers.ts 的单元测试
- **server-stats.ts**: 服务器运行状态统计与派生指标计算
- **spring.ts**: 动画弹簧参数预设
- **spring.test.ts**: spring.ts 的单元测试
- **swr-config.ts**: SWR 预设与共享配置
- **swr-keys.ts**: SWR key 常量定义（含 runtime config）
- **time.ts**: 时间格式化与时间差计算工具
- **time.test.ts**: time.ts 的单元测试
- **tool-names.ts**: 工具名展示与命名空间处理
- **tool-schema.ts**: Tool JSON schema 解析与格式化
- **utils.ts**: 通用基础工具函数（如 className 合并）
