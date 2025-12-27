<p align="center">
  <p align="center">
    <img src="./.github/mcpd.png" alt="Preview" width="128" />
  </p>
  <h1 align="center"><b>mcpd</b></h1>
  <p align="center">
    <b>Lightweight MCP server orchestration core</b>
    <br />
    <br />
  </p>
</p>

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## 🚀 What is mcpd?

**mcpd** is a lightweight control plane for [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) servers. It starts servers on demand, scales them elastically, and supports scale-to-zero. The **mcpdmcp** gateway is the MCP entry point that bridges MCP requests into the mcpd core.

## 💡 Why do we need it?

As the number of MCP servers grows, local setups often face:

- **Resource waste**: idle MCP servers consume CPU and memory
- **Operational complexity**: start/stop/observe flows are fragmented
- **Lack of elasticity**: no on-demand startup or idle reclamation
- **Poor visibility**: no unified view for tools, resources, and prompts

**mcpd** addresses these issues by:

- Starting instances on demand and reclaiming them when idle
- Exposing a single MCP entry point
- Aggregating tools/resources/prompts with list-changed semantics
- Using health probes and lifecycle management for stability

## ✨ Core capabilities

- **On-demand startup**: requests trigger instance launch
- **Elastic scaling**: idle reclamation + scale-to-zero
- **Unified routing**: one entry point for multiple MCP servers, sticky and concurrency limits
- **Resource and prompt aggregation**: unified view with list-changed updates
- **Observability**: structured logs and Prometheus metrics
- **Profile Store**: caller -> profile mapping with multi-profile support

## ✅ Quick start

1) Create profile directory and a default profile:

```yaml
# profiles/default.yaml
servers:
  - name: weather
    cmd: ["node", "./weather-demo-mcp/build/index.js"]
    idleSeconds: 60
    maxConcurrent: 1
```

2) Create caller mapping:

```yaml
# callers.yaml
callers:
  vscode: default
```

3) Start the core:

```bash
go run ./cmd/mcpd serve --config .
```

4) Start the MCP gateway:

```bash
go run ./cmd/mcpdmcp vscode
```

5) In your MCP client, launch `mcpdmcp` as a stdio server.

## 🧩 Configuration layout

Profile Store layout:

```
<store-root>/
  callers.yaml
  profiles/
    default.yaml
    vscode.yaml
```

- `callers.yaml`: defines the `caller -> profile` mapping
- `profiles/*.yaml`: each profile is a full catalog (servers + runtime)

## 🏗️ Architecture overview

```
┌──────────────────────────────────────────────┐
│                  MCP Client                  │
└───────────────┬──────────────────────────────┘
                │ MCP Protocol (stdio)
                v
┌──────────────────────────────────────────────┐
│               mcpdmcp (gateway)              │
└───────────────┬──────────────────────────────┘
                │ gRPC
                v
┌──────────────────────────────────────────────────────────┐
│                         mcpd core                        │
│  ┌──────────┐  ┌────────────┐  ┌──────────────────────┐ │
│  │  Router  │  │ Scheduler  │  │ Tool/Resource/Prompt │ │
│  └────┬─────┘  └────┬───────┘  │       Indexes        │ │
│       │            │          └──────────────────────┘ │
│  ┌────v────────────v─────┐      ┌────────────────────┐ │
│  │    Lifecycle Manager   │────>│       Probe        │ │
│  └────────┬────────────────┘      └──────────────────┘ │
│           │                                              │
│  ┌────────v───────────────────────────────────────────┐ │
│  │                 Transport (stdio)                 │ │
│  └────────┬───────────────────────────────────────────┘ │
└───────────┼────────────────────────────────────────────┘
            │
    ┌───────┴───────┬───────────┬───────────┐
    v               v           v           v
┌─────────┐   ┌─────────┐  ┌─────────┐  ┌─────────┐
│ Server  │   │ Server  │  │ Server  │  │ Server  │
│ Type A  │   │ Type A  │  │ Type B  │  │ Type C  │
│Instance1│   │Instance2│  │Instance1│  │Instance1│
└─────────┘   └─────────┘  └─────────┘  └─────────┘
```

## 📄 License

[MIT License](LICENSE)

## 🔗 References

- [Model Context Protocol Specification](https://modelcontextprotocol.io/)
- [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)
