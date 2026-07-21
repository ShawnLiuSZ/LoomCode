# MCP 插件协议参考

> 本文档描述 LoomCode 的 MCP (Model Context Protocol) 插件集成。

## 概述

MCP 是一种用于 AI 模型与外部工具/服务交互的协议。LoomCode 支持通过 MCP 协议加载外部插件。

## 配置

在 `settings.json`（或 `models.json`）中通过 `plugins` 数组配置 MCP 服务器。canonical 键为 `plugins`；也接受 `mcpServers` 对象形式（会自动并入 `plugins`）。插件 `kind` 由 `type` 字段决定（`stdio` / `sse`），若省略则按 `command`（→ stdio）或 `url`（→ sse）推断：

```json
{
  "plugins": [
    {
      "name": "my-plugin",
      "command": "path/to/plugin",
      "args": ["--flag"]
    },
    {
      "name": "remote-plugin",
      "type": "sse",
      "url": "https://mcp.example.com/sse"
    }
  ]
}
```

`plugins` 字段说明：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | 插件名称（唯一） |
| `type` | string | ❌ | 传输类型：`stdio` / `sse`（http/sse 均归为 sse） |
| `command` | string | ❌ | stdio 模式下的命令路径（与 `url` 二选一） |
| `args` | array | ❌ | 命令参数 |
| `env` | object | ❌ | 子进程环境变量 |
| `url` | string | ❌ | sse 模式下的服务地址（与 `command` 二选一） |

## 协议支持

- **stdio**: 标准输入/输出通信
- **HTTP SSE**: HTTP Server-Sent Events

## 参考实现

- `internal/mcp/` - MCP 客户端实现

---

*此文档为存根页，完整内容待补充。*
