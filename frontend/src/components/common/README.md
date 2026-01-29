<!-- Once this directory changes, update this README.md -->

# Components/Common

应用级共享组件，跨多个功能模块复用。
这些组件包含应用特定逻辑，但仍需保持职责清晰。
通用 UI 原子组件请放在 `components/ui/`。

## Files

- **active-clients-indicator.tsx**: 展示当前连接客户端数量的指示器
- **app-sidebar.tsx**: 应用主侧边栏与导航入口
- **app-topbar.tsx**: 顶部栏，展示核心状态与主题切换
- **client-chip-group.tsx**: 以标签形式展示客户端信息的组件
- **connect-ide-sheet.tsx**: IDE 连接配置抽屉与预设输出
- **error-boundary.tsx**: 组件级错误边界与回退 UI
- **main-content.tsx**: 主内容区域容器，负责布局与顶栏集成
- **nav-item.tsx**: 侧边栏导航项呈现
- **router-error-component.tsx**: 路由错误展示组件
- **universal-empty-state.tsx**: 通用空状态/错误态组件
