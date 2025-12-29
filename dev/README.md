# Docker Compose 开发环境

本目录提供基于 Docker Compose 的本地开发配置。

## 架构

- **dev**：MCP Inspector + Go 环境（Inspector 会启动 mcpdmcp）
- **core**：mcpd 控制面（gRPC + metrics）
- **prometheus**：指标收集与可视化 (http://localhost:9500)

## 快速开始

```bash
make dev
docker compose logs -f dev
make down
```

## 服务说明

### MCP Inspector（dev 服务）
- **UI**: http://localhost:6274
- **WebSocket**: ws://localhost:6277
- **用途**: 调试 MCP 协议、启动 mcpdmcp

### mcpd-core（core 服务）
- **Metrics**: http://localhost:9090/metrics
- **用途**: 控制面 + 编排

### Prometheus
- **UI**: http://localhost:9500
- **用途**: 指标查询

## 使用 MCP Inspector

1) 启动服务：`make dev`
2) 打开 Inspector UI：http://localhost:6274
3) 在 Inspector 里配置启动命令：

```bash
go run /app/cmd/mcpdmcp inspector --rpc core:9091
```

4) 点击 Connect，Inspector 会连接到 mcpdmcp
5) Inspector 会展示 MCP 交互日志

## 指标查看

1) 确保 core 已运行
2) 打开 Prometheus：http://localhost:9500
3) 查询 mcpd 指标：
   - `mcpd_route_duration_seconds`
   - `mcpd_instance_starts_total`
   - `mcpd_instance_stops_total`
   - `mcpd_active_instances`

## 配置文件

- `dev/profiles/default.yaml`: 默认 profile 配置
- `dev/callers.yaml`: caller 映射（默认包含 `inspector -> default`）
- `dev/prometheus.yaml`: Prometheus scrape 配置
- `dev/Dockerfile.dev`: 开发容器镜像
- `dev/runtime.yaml`: 全局 runtime 配置（路由超时、探活、RPC/observability 等）

## 端口

- `6274`: MCP Inspector UI
- `6277`: MCP Inspector WebSocket
- `9090`: mcpd-core metrics
- `9500`: Prometheus UI

## 排障

**Inspector 无法连接 mcpdmcp：**
- 确认命令路径：`/app/cmd/mcpdmcp`
- 确认 core 正在运行：`docker compose ps core`
- 确认 RPC 地址：`--rpc core:9091`

**Prometheus 无数据：**
- 查看 core 日志：`docker compose logs core`
- 检查 metrics：`curl http://localhost:9090/metrics`
- 查看 targets：http://localhost:9500/targets

**端口冲突：**
- 停止占用 6274/6277/9090/9500 的进程
- 或调整 `docker-compose.yml` 端口映射
