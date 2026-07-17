---
tags:
  - 智能体索引
  - AGENTS
  - LoomCode
created: 2026-07-14
updated: 2026-07-14
aliases:
  - 智能体知识库索引
  - Agent Entry
---

# AGENTS.md — 智能体知识库索引

> 🤖 智能体优先阅读此文档
> 📅 最后更新：2026-07-14

---

## 🤖 智能体快速访问

_专为 ClaudeCode、Codex、LoomCode 等智能体的快速访问：_

- **[[00-Index|知识库首页]]** — 全部文档导航入口
- **[[architecture/架构总览|架构总览]]** — 分层架构与模块依赖
- **项目根目录**：`/Users/liushizhao/dev/Helix`
- **CLI 入口**：`cmd/loomcode/main.go`
- **Go Module**：`github.com/ShawnLiuSZ/loomcode`

---

## 核心模块速查

| 模块 | 文档 | 代码路径 | 关键抽象 |
|------|------|----------|----------|
| Agent 引擎 | [[modules/agent-引擎\|Agent 引擎]] | `internal/agent/` | `Agent`, `AgentLoop` |
| Provider 层 | [[modules/provider-层\|Provider 层]] | `internal/provider/` | `Provider`, `Adapter` |
| 工具系统 | [[modules/tool-系统\|工具系统]] | `internal/tool/` | `Tool`, `Registry`, `Executor` |
| 配置系统 | [[modules/config-系统\|配置系统]] | `internal/config/` | `Config` |
| 控制层 | [[modules/control-层\|控制层]] | `internal/control/` | `Permission`, `CostController` |
| 会话管理 | [[modules/session-管理\|会话管理]] | `internal/session/` | `Session`, `Manager` |
| MCP 插件 | [[modules/mcp-插件\|MCP 插件]] | `internal/mcp/` | `PluginManager` |
| TUI 界面 | [[modules/ui-TUI\|TUI 界面]] | `internal/ui/` | `App` |
| Dashboard | [[modules/dashboard\|Dashboard]] | `internal/dashboard/` | `Server` |
| LSP 集成 | [[modules/lsp-集成\|LSP 集成]] | `internal/lsp/` | `Client` |
| Skills 管理 | [[modules/skills-管理\|Skills 管理]] | `internal/skills/` | `Manager` |

---

## 接口协议速查

| 接口 | 文档 | 定义文件 |
|------|------|----------|
| Provider 接口 | [[interfaces/provider-接口\|Provider 接口]] | `internal/provider/provider.go` |
| Tool 接口 | [[interfaces/tool-接口\|Tool 接口]] | `internal/tool/tool.go` |
| MCP 协议 | [[interfaces/mcp-协议\|MCP 协议]] | `internal/mcp/protocol.go` |

---

## 设计原则

| 原则 | 说明 |
|------|------|
| **Provider 插件化** | Adapter 工厂模式，配置驱动接入新厂商，无需修改代码 |
| **Capability-Driven** | Agent 行为由 Provider 的 `Capabilities` 声明驱动，非 if-else 判断厂商 |
| **Config Over Code** | 模型、工具、插件全部 TOML 声明 |
| **Compose Over Inherit** | 接口组合构建复杂行为，避免深层继承 |
| **Single Binary** | CGO_ENABLED=0，零依赖部署 |

---

## 常见任务路径

| 任务 | 入口 | 关键调用链 |
|------|------|-----------|
| 新增 Provider | `loomcode.toml` | 配置 `[[providers]]` → `kind="openai"` → 设置 `api_key_env` |
| 新增工具 | `internal/tool/` | 实现 `Tool` 接口 → 在 `RegisterDefaults()` 注册 |
| 新增 MCP 插件 | `loomcode.toml` | 配置 `[[plugins]]` → stdio(command) 或 SSE(url) |
| 恢复会话 | CLI `--session <id>` | `session.Manager.Get()` → `App.RestoreSession()` |
| 编辑回退 | TUI `/rewind` | `CheckpointManager` → `~/.loomcode/checkpoints/` |
