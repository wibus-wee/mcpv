<!-- Once this directory changes, update this README.md -->

# Hooks

共享的 React hooks，封装跨模块的状态与服务交互。
用于集中管理核心状态、日志流和通用设备能力。
优先保持 hooks 的单一职责，避免混入具体页面逻辑。

## Files

- **use-core-state.ts**: Core 状态的 SWR hooks 与启动/停止/重启操作封装
- **use-active-clients.ts**: 活跃 client 状态的 SWR hook
- **use-logs.ts**: 日志流缓存 hooks 与日志类型定义
- **use-mobile.ts**: 设备尺寸判断的响应式 hooks
