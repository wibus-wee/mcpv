# Wails3 Bindings 生成指南

## 核心概念

### Wails 如何发现 Services？

Wails 的 bindings 生成器会：
1. 扫描指定的 **Go package**（通常是包含 `main()` 的入口包）
2. 寻找 `application.New(application.Options{...})` 调用
3. 分析 `Options.Services` 中注册的服务
4. **只为注册的服务**生成 TypeScript bindings

虽然 Wails 会扫描所有依赖包（这是 Go 静态分析的必要过程），但**只有明确注册的服务才会生成 bindings**。

## 项目结构与 Bindings

```
cmd/mcpd-wails/
├── app.go          # 入口文件，注册 WailsService
└── embed.go        # 前端资源

internal/ui/
└── service.go      # WailsService（唯一被注册的服务）

internal/app/       # 核心层，不会生成 bindings
internal/domain/    # 领域层，不会生成 bindings
internal/infra/     # 基础设施层，不会生成 bindings
```

### 为什么 internal/app 等包会被扫描？

因为 `WailsService` 引用了 `internal/app.App`，Go 静态分析器需要理解完整的类型依赖树。但这**不会**为 `internal/app` 生成 bindings，因为它没有被注册为 Service。

## 使用方式

### 基础命令

```bash
# 生成 TypeScript bindings
wails3 generate bindings -ts ./cmd/mcpd-wails

# 隐藏警告信息
wails3 generate bindings -ts -silent ./cmd/mcpd-wails

# 或使用 Makefile
make wails-bindings
```

### 常用选项

```bash
# 生成 TypeScript interfaces（而非 classes）
wails3 generate bindings -ts -i ./cmd/mcpd-wails

# 自定义输出目录
wails3 generate bindings -ts -d ./frontend/src/bindings ./cmd/mcpd-wails

# 使用方法名而非 ID（更易调试）
wails3 generate bindings -ts -names ./cmd/mcpd-wails

# 不生成 index 文件
wails3 generate bindings -ts -noindex ./cmd/mcpd-wails
```

## 关于警告信息

### Go 版本警告（可忽略）

```
WARNING [warn] /path/to/file.go:1:1: package requires newer Go version go1.25 (application built with go1.24)
```

**原因**：Wails3 CLI 用 Go 1.24 编译，但项目要求 Go 1.25
**影响**：无，bindings 生成不受影响
**解决**：
- 方案 1：使用 `-silent` 隐藏警告
- 方案 2：等待 Wails3 更新或从源码用 Go 1.25 编译

### Assets 未定义错误

```
WARNING [warn] /Users/.../cmd/mcpd-wails/app.go:37:43: undefined: Assets
```

**原因**：`embed.go` 和 `app.go` 在同一个 package，但静态分析器可能暂时找不到
**影响**：无，运行时 Assets 正常可用
**解决**：确保 `embed.go` 存在且格式正确（已修复）

## 生成结果示例

```bash
$ make wails-bindings
INFO  Processed: 600 Packages, 1 Service, 4 Methods, 0 Enums, 13 Models, 0 Events
INFO  Output directory: /Users/wibus/dev/mcpd/frontend/bindings
```

**解读**：
- **600 Packages**：扫描的 Go 包总数（包括依赖）
- **1 Service**：注册的服务数量（WailsService）
- **4 Methods**：导出的方法数量（GetVersion, Ping, HandleURLScheme, SetWailsApp）
- **13 Models**：涉及的数据模型数量

生成的文件：
```
frontend/bindings/
├── index.ts                    # 总入口
├── models.ts                   # 数据模型
└── mcpd/ui/
    └── WailsService.ts         # WailsService 的 TS bindings
```

## 前端使用

```typescript
import { WailsService } from './bindings/mcpd/ui/WailsService'

// 调用 Go 方法
const version = await WailsService.GetVersion()
const pong = await WailsService.Ping()
```

## 开发流程

1. **修改 Go Service**（如 `internal/ui/service.go`）
2. **重新生成 bindings**：`make wails-bindings`
3. **前端使用新的 bindings**

Wails 会自动保留注释、参数名，并生成 JSDoc 类型提示。

## Makefile 集成

```bash
# 生成 bindings
make wails-bindings

# 启动开发服务器（自动重新生成 bindings）
make wails-dev

# 构建生产版本
make wails-build
```

## 最佳实践

1. **只在 `internal/ui` 中暴露前端需要的方法**
   - ✅ 导出方法：`GetVersion()`, `Ping()`
   - ❌ 不要暴露：内部逻辑、数据库操作

2. **保持 Service 薄**
   - `internal/ui/service.go` 只做桥接
   - 核心逻辑在 `internal/app`

3. **使用有意义的类型**
   - Go struct 会自动转换为 TypeScript class/interface
   - 注释会保留到 JSDoc

4. **定期重新生成 bindings**
   - 在 CI/CD 中添加检查，确保 bindings 是最新的

## 故障排查

### 问题：Service 没有被发现

**检查**：
- `application.NewService(service)` 是否正确调用？
- Service 是否是导出类型（首字母大写）？

### 问题：方法没有生成 binding

**检查**：
- 方法是否导出（首字母大写）？
- 方法签名是否符合规范（参数、返回值）？

### 问题：类型转换错误

**参考**：
- Go `[]byte` → JS `string` (base64)
- Go `time.Time` → JS `string` (RFC3339)
- Go `map[string]interface{}` → JS `Record<string, any>`
