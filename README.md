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

## 💡 Why mcpd?

As the number of MCP servers grows, local setups often face:

- **Resource waste**: idle MCP servers consume CPU and memory
- **Operational complexity**: start/stop/observe flows are fragmented
- **Lack of elasticity**: no on-demand startup or idle reclamation
- **Poor visibility**: no unified view for tools, resources, and prompts

## ✨ Features

- **⚡ On-Demand Startup**: Automatically launch MCP server instances on request without manual process management
- **📉 Auto-Scaling**: Idle timeout-based instance recycling with scale-to-zero support for resource efficiency
- **🎯 Unified Routing**: Single entry point for multiple MCP servers with sticky session and concurrency control
- **📊 Observability**: Structured JSON logging with reserved Prometheus metrics interface
- **📷 Profile Store**: caller -> profile mapping with multi-profile support

## 📄 License

[MIT License](LICENSE)

## 🔗 References

- [Model Context Protocol Specification](https://modelcontextprotocol.io/)
- [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)
