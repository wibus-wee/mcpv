<!-- Once this directory changes, update this README.md -->

# Modules/Config/Components/Profiles

Profile 相关的配置组件集合。

## Files

- **profile-overview-section.tsx**: Profile 概览与活跃 caller 统计
- **profile-subagent-section.tsx**: SubAgent 启用/禁用设置
- **profile-servers-section.tsx**: Servers 列表与 ServerItem 组件
- **index.ts**: 统一导出入口

## Design Pattern

各 section 采用轻量化的平铺布局：
- 无 Card 阴影与厚重边框
- 标题与描述分层展示
- section 之间使用分隔线
