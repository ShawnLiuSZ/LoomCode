---
tags:
  - 接口协议
  - MCP
  - 插件通信
created: 2026-07-14
updated: 2026-07-14
aliases:
  - MCP Protocol
  - Model Context Protocol
---

# MCP 协议

> 🔌 MCP 插件通信协议（stdio + SSE）
> 📅 最后更新：2026-07-14

---

## 定义文件

`internal/mcp/protocol.go`

## 传输方式

| 方式 | 客户端文件 | 配置 | 适用场景 |
|------|-----------|------|----------|
| **stdio** | `client.go` | `command` + `args` | 本地子进程工具 |
| **SSE** | `sse_client.go` | `url` | 远程 HTTP 工具 |

## 协议概述

MCP（Model Context Protocol）是标准化的插件协议，让 LoomCode 能接入外部工具服务器。

```
LoomCode (MCP Client)
    │
    ├── stdio  ←→  MCP Server (子进程)
    │
    └── SSE    ←→  MCP Server (HTTP)
```

## PluginManager

```go
type PluginManager struct {
    tools  *tool.Registry
    plugins map[string]*Client
}

// 连接 stdio 插件
func (pm *PluginManager) Connect(name, command string, args ...string) error

// 连接 SSE 插件
func (pm *PluginManager) ConnectSSE(ctx context.Context, name, url string) error

// 断开所有插件
func (pm *PluginManager) DisconnectAll()
```

## 插件适配

MCP 插件的工具通过 `plugin.go` 适配为 `Tool` 接口，注册到 `tool.Registry`，与内置工具统一调度。

## 配置示例

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

## 连接容错

- 单个插件连接失败不影响启动
- 失败时打印告警并跳过该插件
- 无插件配置时返回 nil（不创建 PluginManager）

## 插件发现

插件启动后通过 MCP 协议声明自身提供的工具列表，PluginManager 自动将这些工具注册到工具系统。

## 相关文档

- [[../modules/mcp-插件|MCP 插件]] — 模块概览
- [[../interfaces/tool-接口|Tool 接口]] — 插件工具适配目标
- [[../modules/config-系统|配置系统]] — 插件配置
- [[../architecture/架构总览|架构总览]]
