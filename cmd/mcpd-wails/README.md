# Wails 应用入口说明

## 文件结构

```
cmd/mcpd-wails/
├── app.go      # Wails 应用入口（替代传统的 main.go）
└── embed.go    # 前端资源 embed（可复用）

internal/ui/
├── doc.go      # 包文档
└── service.go  # Wails 服务适配层
```

## 设计理念

### 1. 入口文件命名

- **不使用 `main.go`**：避免入口文件成为"大杂烩"
- **使用 `app.go`**：明确表示这是应用启动文件
- **`embed.go` 独立**：前端资源可能被测试、打包工具等复用

### 2. 职责分层

```
cmd/mcpd-wails/app.go
  ↓ (启动、注册、转发)
internal/ui/service.go
  ↓ (桥接、事件流)
internal/app/app.go
  ↓ (核心编排)
internal/domain + internal/infra
  (领域逻辑)
```

### 3. 符合架构约定

- ✅ Wails 入口**只做启动与依赖注入**
- ✅ **不在入口层堆积业务逻辑**
- ✅ URL Scheme **在入口层注册**，解析转发给 `internal/ui`
- ✅ 核心逻辑仍在 `internal/app`，与 CLI 共享

## 使用方式

### 开发模式

```bash
# 使用 Wails CLI 运行开发服务器
wails3 dev -config cmd/mcpd-wails

# 或使用 task（如果配置了 Taskfile）
task dev
```

### 构建

```bash
# 构建所有平台
wails3 build -config cmd/mcpd-wails

# 构建特定平台
wails3 build -config cmd/mcpd-wails -platform darwin/arm64
```

## URL Scheme 处理流程

1. **注册**：在 `app.go` 的 `application.Options.Protocols` 中注册
2. **接收**：通过 `ApplicationOpenedWithURL` 事件接收
3. **转发**：入口层调用 `wailsService.HandleURLScheme(url)`
4. **处理**：`internal/ui/service.go` 解析并发送前端事件

示例 URL：
```
mcpd://open/server?id=123
mcpd://settings/profiles
```

## 扩展点

### 添加新的导出方法

在 `internal/ui/service.go` 中添加**导出方法**（首字母大写）：

```go
// GetServerList 获取服务器列表
func (s *WailsService) GetServerList(ctx context.Context) ([]string, error) {
    // 调用核心层获取数据
    return []string{"server1", "server2"}, nil
}
```

Wails 会自动生成 JS 绑定到前端。

### 发送事件到前端

```go
s.wails.EmitEvent("server-status-changed", map[string]interface{}{
    "server": "server1",
    "status": "running",
})
```

## 参考

- `docs/WAILS_STRUCTURE.md` - Wails 工程结构建议
- `docs/STRUCTURE.md` - 整体项目结构
