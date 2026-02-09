<p align="center">
  <p align="center">
    <img src="./build/appicon.png" alt="mcpv-logo" width="128" />
  </p>
  <h1 align="center"><b>mcpv</b></h1>
  <p align="center">
    <b>Elastic Control Plane & Runtime for Model Context Protocol (MCP)</b>
    <br />
    <br />
    <a href="#-key-features">Features</a> •
    <a href="#-architecture">Architecture</a> •
    <a href="#-quick-start">Quick Start</a> •
    <a href="#-observability">Observability</a>
  </p>
</p>

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev)
[![Wails](https://img.shields.io/badge/UI-Wails3-red.svg)](https://wails.io)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

## 🚀 What is mcpv?

**mcpv** is a lightweight orchestration control plane for [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) servers. It solves the problems of resource waste, configuration fragmentation, and poor visibility when running multiple MCP servers locally.

With `mcpv`, you can manage MCP servers like containers: **On-demand startup, Scale-to-Zero hibernation, and unified routing**, all wrapped in a modern visual interface.

## ✨ Key Features

- **⚡️ Elastic Runtime**: Automatically launches MCP server instances upon request and shuts them down after idle timeouts, significantly reducing local CPU and memory usage.
- **🎯 Unified Gateway (`mcpvmcp`)**: Provides a single entry point for all your MCP servers. Supports sticky sessions and concurrency control for high-frequency AI interactions.
- **🧠 Smart SubAgent**: Built-in intelligent filtering powered by `CloudWeGo/Eino`. The `automatic_mcp` tool dynamically selects relevant tools based on task context, minimizing context window bloat and token costs.
- **🖼 GUI Support (`mcpvui`)**: A desktop client built with Wails 3. Features real-time log streaming, tool inspection, resource browsing, and intuitive configuration editing.
- **📁 Single Config File**: Server-centric configuration with tag-based visibility. Configure MCP servers with optional tags and filter toolsets for clients like VSCode, Cursor, or specific projects based on tag matching.
- **📊 Observability**: Native Prometheus metrics and structured logging. Includes a pre-configured Grafana dashboard to monitor latency, cold-start times, and error rates.

## 🏗 Architecture

The project is designed with a three-layer architecture for maximum decoupling:

1.  **Core (`mcpv`)**: The central control plane managing instance lifecycles, scheduling algorithms, and aggregation indexes.
2.  **Gateway (`mcpvmcp`)**: The protocol bridge. Acts as a standard MCP server to communicate with AI clients (e.g., Claude Desktop, Cursor).
3.  **App (`mcpvui`)**: The Wails-driven GUI for configuration, real-time monitoring, and core lifecycle hosting.

## 🛠 Quick Start

WIP.

## 📊 Observability

We believe the control plane should be transparent.
- **Metrics**: Access raw data at `http://localhost:9090/metrics`.
- **Dashboard**: After running `make dev`, visit `http://localhost:4000` for a Grafana dashboard visualizing success rates and cold-start latency.
- **Health**: Check internal loop status at `http://localhost:9090/healthz`.

## 🔄 Hot Reload Behavior

When the config file changes, mcpv classifies updates and applies the smallest safe action:

- **Runtime-only**: Changes only to runtime settings (timeouts, refresh intervals, etc.) update the in-memory runtime configuration without restarting instances.
- **Tools-only**: Changes only to tool visibility fields (`name`, `tags`, `exposeTools`) refresh tool/resource/prompt indexes and visibility without restarting instances.
- **Restart-required**: Any other server spec change triggers a drain of existing instances so they restart on demand with the new spec.

If multiple categories change in one update, mcpv combines the actions (for example, runtime updates plus a restart-required drain).

## 🚧 Roadmap (WIP)

The project is under active development:
- [x] Core Lifecycle & Scale-to-Zero
- [x] Multi-Profile & Caller Mapping
- [x] Eino-based SubAgent Tool Filtering
- [x] Wails UI & Log Streaming
- [x] Hot Reload
- [ ] **Auto-discovery & Config Import - Planned**

## 🔗 References

- [Model Context Protocol (MCP)](https://modelcontextprotocol.io/)
- [mozilla-ai/mcpd](https://github.com/mozilla-ai/mcpd)

## 📄 License

This project is licensed under the [Apache License 2.0](LICENSE).

## ✏️ Author

mcpv © Wibus, Released under Apache License 2.0. Created on Dec 21, 2025.

> [Personal Website](http://wibus.ren/) · [Blog](https://blog.wibus.ren/) · GitHub [@wibus-wee](https://github.com/wibus-wee/) · Telegram [@wibus✪](https://t.me/wibus_wee)
