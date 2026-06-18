# Helix CLI

> 双螺旋 · 多模型 · 可扩展

<p align="center">
  <img src="https://img.shields.io/badge/status-planning-blue?style=flat-square" alt="Status: Planning">
  <img src="https://img.shields.io/badge/version-0.1.0-informational?style=flat-square" alt="Version: 0.1.0">
  <img src="https://img.shields.io/badge/language-Go-00ADD8?style=flat-square&logo=go" alt="Language: Go">
  <img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="License: MIT">
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey?style=flat-square" alt="Platform: macOS | Linux | Windows">
</p>

**Helix** 是一个纯 CLI 形态、基于 Go 语言的���扩展多模型 Agent 编程工具。融合 [DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix) 和 [MiMo-Code](https://github.com/XiaomiMiMo/MiMo-Code) 的核心优点，为 DeepSeek V4 和 Xiaomi MiMo 大模型提供深度优化，同时支持任意 OpenAI 兼容厂商通过配置文件一键接入。

---

## 设计哲学

- **双模型原生优化** — 不是简单的"支持多个模型"，而是针对 DeepSeek 的缓存特性和 MiMo 的推理/语音特性分别深度适配
- **Provider 插件化** — Adapter 工厂模式，任何 OpenAI 兼容厂商通过 TOML 配置接入，零代码修改
- **单二进制分发** — CGO_ENABLED=0 静态编译，一个文件即可部署到 6 个平台
- **可交互优先** — MVP 阶段优先保证端到端可用，再逐步叠加缓存优化、多 Agent、记忆系统

---

## 快速开始

```bash
# 安装（规划中）
curl -fsSL https://raw.githubusercontent.com/ShawnLiuSZ/Helix/main/scripts/install.sh | bash

# 配置
helix setup

# 使用
helix run "解释当前项目的架构"
```

---

## 技术栈

| 层次 | 技术 |
|------|------|
| 语言 | Go（CGO_ENABLED=0） |
| 配置 | TOML |
| TUI | Bubble Tea + Lip Gloss |
| 记忆存储 | SQLite FTS5 |
| 插件协议 | MCP（stdio + HTTP） |
| API 协议 | OpenAI 兼容 |

---

## 项目状态

当前处于 **规划与设计阶段**（v0.1.0），正在推进 Phase 1 MVP 开发。

| 阶段 | 状态 |
|------|------|
| 项目规划与文档 | 已完成 |
| Phase 1 核心框架 MVP | 待开始 |
| Phase 2 缓存与成本优化 | 规划中 |
| Phase 3 多 Agent 与记忆 | 规划中 |
| Phase 4 生态与进化 | 规划中 |

---

## 文档

- [CODEBUDDY.md](CODEBUDDY.md) — 项目入口指南
- [CONTRIBUTING.md](CONTRIBUTING.md) — 贡献指南
- [文档导航](docs/README.md) — 全部文档索引

---

## 许可证

MIT
