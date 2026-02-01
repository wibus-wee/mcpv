<!-- Once this directory changes, update this README.md -->

# Providers

应用级 Provider 与全局桥接逻辑集中于此。
负责主题、动画、状态容器以及与后端事件的接入。
避免在此目录放入业务逻辑与页面组件。

## Files

- **root-provider.tsx**: 根级 Provider 组合与 Wails 事件桥接，负责批量更新日志流、解析事件并支持日志流重启触发
