# MCP 插件协议参考

> 本文档描述 Helix 的 MCP (Model Context Protocol) 插件集成。

## 概述

MCP 是一种用于 AI 模型与外部工具/服务交互的协议。Helix 支持通过 MCP 协议加载外部插件。

## 配置

在 `helix.toml` 中配置 MCP 服务器：

```toml
[[mcp_servers]]
name = "my-plugin"
command = "path/to/plugin"
args = ["--flag"]
```

## 协议支持

- **stdio**: 标准输入/输出通信
- **HTTP SSE**: HTTP Server-Sent Events

## 参考实现

- `internal/mcp/` - MCP 客户端实现

---

*此文档为存根页，完整内容待补充。*
