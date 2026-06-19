# Helix CLI

> 双螺旋 · 多模型 · 可扩展

<p align="center">
  <img src="https://img.shields.io/badge/status-active-success?style=flat-square" alt="Status: Active">
  <img src="https://img.shields.io/badge/version-dev-informational?style=flat-square" alt="Version: dev">
  <img src="https://img.shields.io/badge/language-Go-00ADD8?style=flat-square&logo=go" alt="Language: Go">
  <img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="License: MIT">
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey?style=flat-square" alt="Platform: macOS | Linux | Windows">
  <img src="https://img.shields.io/badge/tests-343%20passing-brightgreen?style=flat-square" alt="Tests: 343 passing">
</p>

**Helix** 是一个纯 CLI 形态、基于 Go 语言的可扩展多模型 Agent 编程工具。融合 [DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix) 和 [MiMo-Code](https://github.com/XiaomiMiMo/MiMo-Code) 的核心优点，为 DeepSeek V4 和 Xiaomi MiMo 大模型提供深度优化，同时支持任意 OpenAI 兼容厂商通过配置文件一键接入。

---

## 快速开始

```bash
# 构建
make build

# 配置 API Key
cp helix.example.toml helix.toml
export DEEPSEEK_API_KEY="sk-..."

# 启动 TUI
./bin/helix

# 或单次任务
./bin/helix run "创建一个 hello.go"
echo "解释这段代码" | ./bin/helix run
```

---

## CLI 命令

```bash
helix                              # 启动交互式 TUI（默认）
helix --provider deepseek          # 指定 Provider（deepseek/mimo/openai）
helix --model deepseek-v4-pro     # 指定默认模型
helix --session <id>               # 恢复历史会话
helix --env-file custom.env       # 加载自定义 .env
helix --version                    # 显示版本
helix run <task>                   # 单次任务
helix setup                        # 配置向导
helix dashboard [addr]             # 启动 Web Dashboard（默认 :8080）
```

## TUI 交互

| 操作 | 说明 |
|------|------|
| 直接输入文字 | 发送任务给 AI |
| Tab | 切换 Agent 模式（build/plan/compose/max） |
| 输入 `/` 后 Tab | 命令自动补全 |
| ↑↓ / Enter / Esc | 交互式选择器（模型选择等） |

### TUI 命令

| 命令 | 说明 |
|------|------|
| `/help` | 显示帮助 |
| `/mode` | 显示当前模式和模型 |
| `/build` `/plan` `/compose` `/max` | 切换 Agent 模式 |
| `/model` | 交互式选择模型（↑↓ 选择，Enter 确认） |
| `/model <name>` | 直接切换模型 |
| `/skills` | 显示内置工具和外部 skills |
| `/env` | 查看环境变量 |
| `/env set <KEY> <VALUE>` | 设置环境变量 |
| `/env unset <KEY>` | 移除环境变量 |
| `/goal` | 设置/查看/清除停止条件 |
| `/sessions` | 查看会话列表 |
| `/sessions new <name>` | 创建新会话 |
| `/sessions switch <ID>` | 切换会话 |
| `/cost` | 显示成本统计 |
| `/clear` | 清空聊天记录 |
| `/quit` | 退出 |

---

## 环境变量配置

支持三级 `.env` 加载（后加载覆盖前加载）：

```
1. ~/.helix/.env        ← 全局配置
2. ./.env                ← 项目配置
3. ./.env.local          ← 本地覆盖（不提交 git）
4. --env-file custom.env ← CLI 参数（最高优先级）
```

```bash
# .env 示例
DEEPSEEK_API_KEY=sk-xxxxxxxxxxxxxxxx
MIMO_API_KEY=
OPENAI_API_KEY=
```

---

## Skills 自动加载

启动时自动从以下目录加载 skills：

```
~/.agents/skills/*/SKILL.md   ← 低优先级
~/.helix/skills/*/SKILL.md    ← 高优先级（同名覆盖）
```

每个 skill 是一个包含 `SKILL.md` 的目录。使用 `/skills` 查看所有已加载的 skills。

---

## 技术栈

| 层次 | 技术 |
|------|------|
| 语言 | Go（CGO_ENABLED=0） |
| 配置 | TOML |
| TUI | Bubble Tea + Lip Gloss |
| 记忆存储 | SQLite FTS5 |
| 插件协议 | MCP（stdio + HTTP） |
| LSP | JSON-RPC 2.0 |
| API 协议 | OpenAI 兼容 |

---

## 项目状态

当前处于 **活跃开发阶段**。

| 阶段 | 状态 |
|------|------|
| Phase 1 核心框架 MVP | 已完成 |
| Phase 2 缓存与成本优化 | 已完成 |
| Phase 3 多 Agent 与记忆 | 已完成 |
| Phase 4 生态与进化 | 已完成 |

**343 个测试全部通过**，17 个模块覆盖完整。

---

## 文档

- [CODEBUDDY.md](CODEBUDDY.md) — 项目入口指南
- [CONTRIBUTING.md](CONTRIBUTING.md) — 贡献指南
- [文档导航](docs/README.md) — 全部文档索引

---

## 许可证

MIT
