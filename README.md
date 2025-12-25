<p align="center">
  <p align="center">
    <img src="./.github/mcpd.png" alt="Preview" width="128" />
  </p>
  <h1 align="center"><b>mcpd</b></h1>
  <p align="center">
    <b>Lightweight Elastic Orchestrator for Model Context Protocol Servers</b>
    <br />
    <br />
  </p>
</p>

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

> This project is currently **on hold**. 
>
> While I have many different ideas for its development, I noticed that **Mozilla AI** has also introduced their own version of `mcpd`.
>
> Although our concepts differ, they share striking similarities. Perhaps this project will wait for a new beginning, blooming again in the future.

## 🚀 What is mcpd?

**mcpd** is a lightweight elastic control plane for [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) servers, providing on-demand startup, auto-scaling, and scale-to-zero capabilities. A separate **mcpd-gateway** process exposes the MCP protocol and bridges requests to the core.

## 💡 Why mcpd?

As AI assistants integrate more MCP servers for extended capabilities, developers face growing resource management challenges:

- **Resource Waste**: Running multiple MCP servers simultaneously consumes significant memory and CPU, even when idle
- **Manual Complexity**: Starting, stopping, and monitoring each server individually becomes tedious and error-prone
- **No Elasticity**: Traditional setups lack dynamic scaling—servers either run continuously or require manual intervention
- **Poor Visibility**: Tracking health, performance, and tool availability across distributed servers is difficult

**mcpd solves these problems** by acting as a smart orchestrator that:
- Launches servers only when needed, keeping your system lightweight
- Automatically recycles idle instances, achieving true scale-to-zero efficiency  
- Provides a gateway entry point for MCP interactions with intelligent routing
- Aggregates tools from multiple servers into a unified, discoverable interface
- Ensures service reliability through health monitoring and graceful lifecycle management

Think of it as **"Kubernetes for MCP servers on your laptop"**—bringing cloud-native elasticity and observability to local AI development workflows.

## ✨ Core Features

- **⚡ On-Demand Startup**: Automatically launch MCP server instances on request without manual process management
- **📉 Auto-Scaling**: Idle timeout-based instance recycling with scale-to-zero support for resource efficiency
- **🎯 Unified Routing**: Single entry point for multiple MCP servers with sticky session and concurrency control
- **🏥 Health Management**: Built-in health probes and instance lifecycle management for service stability
- **🔧 Tool Aggregation**: Dynamically collect and expose unified tool lists from all downstream MCP servers
- **⚙️ Flexible Configuration**: Declarative YAML configuration with environment variable overrides and hot-reload support
- **📊 Observability**: Structured JSON logging with reserved Prometheus metrics interface

## 🏗️ Architecture Overview

```
┌──────────────────────────────────────────────────────────┐
│                       MCP Client                         │
│              (VS Code, Claude, etc.)                     │
└────────────┬─────────────────────────────────────────────┘
             │ MCP Protocol (stdio)
             v
┌──────────────────────────────────────────────────────────┐
│                       MCP Client                         │
│              (VS Code, Claude, etc.)                     │
└────────────┬─────────────────────────────────────────────┘
             │ MCP Protocol (stdio)
             v
┌──────────────────────────────────────────────────────────┐
│                      mcpd-gateway                        │
│            MCP Server + Tool Registry Bridge             │
└────────────┬─────────────────────────────────────────────┘
             │ gRPC
             v
┌────────────────────────────────────────────────────────────┐
│                         mcpd-core                          │
│  ┌────────────┐  ┌────────────┐  ┌────────────────────┐  │
│  │   Router   │  │ Scheduler  │  │   Tool Index       │  │
│  └────┬───────┘  └────┬───────┘  └────────────────────┘  │
│       │               │                                    │
│  ┌────v───────────────v─────┐     ┌────────────────┐     │
│  │   Lifecycle Manager      │────>│     Probe      │     │
│  └────────┬─────────────────┘     └────────────────┘     │
│           │                                                │
│  ┌────────v─────────────────────────────────────────┐    │
│  │            Transport Layer (stdio)               │    │
│  └────────┬─────────────────────────────────────────┘    │
└───────────┼─────────────────────────────────────────────┘
            │
    ┌───────┴───────┬───────────┬───────────┐
    v               v           v           v
┌─────────┐   ┌─────────┐  ┌─────────┐  ┌─────────┐
│ Server  │   │ Server  │  │ Server  │  │ Server  │
│ Type A  │   │ Type A  │  │ Type B  │  │ Type C  │
│Instance1│   │Instance2│  │Instance1│  │Instance1│
└─────────┘   └─────────┘  └─────────┘  └─────────┘
```

### Core Components

- **Gateway**: MCP protocol entry point, bridges tools/list and tools/call to core over gRPC
- **RPC Control Plane**: gRPC API for tool snapshots, tool calls, and log streaming
- **Router**: Request routing that selects or creates instances based on serverType
- **Scheduler**: Instance scheduling with sticky session and concurrency limits
- **Lifecycle Manager**: Handles instance startup, handshake, state transitions, and shutdown
- **Probe**: Periodic health checks with automatic failure instance removal
- **Tool Index**: Collects and exposes unified tool lists from downstream MCP servers
- **Transport**: Currently supports stdio, with future HTTP/SSE expansion

## 📄 License

[MIT License](LICENSE)

## 🔗 Resources

- [Model Context Protocol Specification](https://modelcontextprotocol.io/)
- [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)
