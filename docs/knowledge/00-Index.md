---
tags:
  - 知识库首页
  - Index
  - LoomCode
created: 2026-07-14
updated: 2026-07-14
aliases:
  - 知识库首页
  - Knowledge Index
---

# LoomCode 知识库

> 📚 纯 CLI 形态、基于 Go 的可扩展多模型 Agent 编程工具
> 📅 最后更新：2026-07-14

---

## 🤖 智能体快速访问

_专为 ClaudeCode、Codex、LoomCode 等智能体的快速访问：_

- **[[AGENTS|智能体知识库索引]]** — 智能体优先阅读此文档
- **[[architecture/架构总览|架构总览]]** — 分层架构与模块依赖
- **项目根目录**：`/Users/liushizhao/dev/Helix`

---

## 项目简介

**LoomCode**（原 Helix，2026-07-13 更名）是纯 CLI 形态、基于 Go 的可扩展多模型 Agent 编程工具。融合 DeepSeek-Reasonix（Planner/Executor 分离 session）和 MiMo-Code（Prefix Cache 调度）的设计思想。

- **语言**：Go（CGO_ENABLED=0 单二进制分发）
- **配置**：JSON + JSON Schema
- **TUI**：Bubble Tea + Lip Gloss
- **记忆存储**：SQLite FTS5
- **插件协议**：MCP（stdio + HTTP）
- **API 协议**：OpenAI 兼容

---

## 文档导航

### 📐 技术架构

| 文档 | 说明 |
|------|------|
| [[architecture/架构总览\|架构总览]] | 分层架构、模块依赖、核心设计原则 |

### 🧩 核心模块

| 模块 | 文档 | 职责 |
|------|------|------|
| Agent 引擎 | [[modules/agent-引擎\|Agent 引擎]] | 推理循环、模式切换、子Agent编排 |
| Provider 层 | [[modules/provider-层\|Provider 层]] | 多厂商模型接入、能力声明 |
| 工具系统 | [[modules/tool-系统\|工具系统]] | 工具注册、执行、修复、并行 |
| 配置系统 | [[modules/config-系统\|配置系统]] | JSON 加载、向导、Schema |
| 控制层 | [[modules/control-层\|控制层]] | 权限、成本、门控 |
| 会话管理 | [[modules/session-管理\|会话管理]] | 生命周期、JSONL 持久化 |
| MCP 插件 | [[modules/mcp-插件\|MCP 插件]] | 外部工具扩展 |
| TUI 界面 | [[modules/ui-TUI\|TUI 界面]] | 交互式终端界面 |
| Dashboard | [[modules/dashboard\|Dashboard]] | Web 监控面板 |
| LSP 集成 | [[modules/lsp-集成\|LSP 集成]] | 语言服务器协议 |
| Skills 管理 | [[modules/skills-管理\|Skills 管理]] | 自动加载技能 |

### 🔌 接口协议

| 接口 | 文档 | 说明 |
|------|------|------|
| Provider 接口 | [[interfaces/provider-接口\|Provider 接口]] | 模型提供者抽象 |
| Tool 接口 | [[interfaces/tool-接口\|Tool 接口]] | 工具抽象与 Schema |
| MCP 协议 | [[interfaces/mcp-协议\|MCP 协议]] | 插件通信协议 |

### 📑 MOC 索引

| MOC | 说明 |
|-----|------|
| [[MOC/MOC-全部\|全部文档]] | 全部知识库文档列表 |
| [[MOC/MOC-技术架构\|技术架构]] | 架构相关文档 |
| [[MOC/MOC-核心模块\|核心模块]] | 模块实现文档 |
| [[MOC/MOC-接口协议\|接口协议]] | 接口与协议文档 |

---

## 外部资源

| 资源 | 链接 |
|------|------|
| 项目 README | [README.md](../../README.md) |
| 文档导航 | [docs/README.md](../README.md) |
| 架构设计文档 | [LOOMCODE_ARCHITECTURE.md](../LOOMCODE_ARCHITECTURE.md) |
| GitHub | [ShawnLiuSZ/LoomCode](https://github.com/ShawnLiuSZ/loomcode) |
