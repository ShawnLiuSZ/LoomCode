---
tags:
  - 核心模块
  - MCP
  - 插件
created: 2026-07-14
updated: 2026-07-14
aliases:
  - MCP
  - PluginManager
  - 插件协议
---

# MCP 插件

> 🔌 MCP 客户端、stdio+SSE 双通道、外部工具扩展
> 📅 最后更新：2026-07-14

---

## 概述

MCP（Model Context Protocol）插件系统让 LoomCode 能接入外部工具。支持 stdio（子进程）和 SSE（HTTP）两种传输方式。

**代码路径**：`internal/mcp/`

## 关键文件

| 文件 | 职责 |
|------|------|
| `manager.go` | `PluginManager` 插件管理器 |
| `client.go` | MCP 客户端（stdio） |
| `sse_client.go` | MCP 客户端（SSE/HTTP） |
| `protocol.go` | MCP 协议定义 |
| `plugin.go` | 插件适配（注册为 Tool） |

## 传输方式

| 方式 | 配置字段 | 说明 |
|------|----------|------|
| **stdio** | `command` + `args` | 启动子进程，通过 stdin/stdout 通信 |
| **SSE** | `url` | 连接 HTTP SSE 端点 |

## 配置方式

```json
// settings.json 或 models.json 的 plugins 数组
{
  "plugins": [
    {
      "name": "my-tool",
      "command": "node",
      "args": ["./mcp-server.js"]
    },
    {
      "name": "remote-tool",
      "url": "https://mcp.example.com/sse"
    }
  ]
}
```

## 连接流程

```
CLI 启动
    │
    ▼
connectPlugins(plugins, tools)
    │
    ├── 无插件 → 返回 nil
    │
    └── 有插件
          │
          ├── PluginManager.NewPluginManager(tools)
          │
          └── 遍历插件配置
                │
                ├── kind == "stdio" → pm.Connect(name, command, args...)
                ├── kind == "sse"   → pm.ConnectSSE(ctx, name, url)
                └── 连接失败 → 打印告警，跳过（不影响启动）
```

**设计要点**：单个插件连接失败不影响整体启动。

## 插件工具注册

MCP 插件的工具会自动注册到 `tool.Registry`，与内置工具统一调度。插件工具通过 `plugin.go` 适配为 `Tool` 接口。

## 关键方法

| 方法 | 说明 |
|------|------|
| `NewPluginManager(tools)` | 创建插件管理器 |
| `pm.Connect(name, cmd, args...)` | 连接 stdio 插件 |
| `pm.ConnectSSE(ctx, name, url)` | 连接 SSE 插件 |
| `pm.DisconnectAll()` | 断开所有插件 |

## 相关文档

- [[../interfaces/mcp-协议|MCP 协议]] — 协议定义
- [[tool-系统|工具系统]] — 插件工具注册
- [[config-系统|配置系统]] — 插件配置
- [[../architecture/架构总览|架构总览]]
