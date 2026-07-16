# LoomCode CLI

> 双螺旋 · 多模型 · 可扩展

<p align="center">
  <img src="https://img.shields.io/badge/status-active-success?style=flat-square" alt="Status: Active">
  <img src="https://img.shields.io/badge/version-dev-informational?style=flat-square" alt="Version: dev">
  <img src="https://img.shields.io/badge/language-Go-00ADD8?style=flat-square&logo=go" alt="Language: Go">
  <img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="License: MIT">
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey?style=flat-square" alt="Platform: macOS | Linux | Windows">
  <img src="https://img.shields.io/badge/tests-343%20passing-brightgreen?style=flat-square" alt="Tests: 343 passing">
</p>

[English](README_EN.md) | 简体中文

> **更名公告（2026-07-13）**：本项目原名 **Helix**，于今日正式更名为 **LoomCode**。命令行由 `helix` 变更为 `loom` / `loomcode`（双命令等价），配置目录由 `~/.helix/` 变更为 `~/.loomcode/`。仓库地址暂未 rename，后续将迁移至 `ShawnLiuSZ/loomcode`。

**LoomCode** 是一个纯 CLI 形态、基于 Go 语言的可扩展多模型 Agent 编程工具。融合 [DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix) 和 [MiMo-Code](https://github.com/XiaomiMiMo/MiMo-Code) 的核心优点，为 DeepSeek V4 和 Xiaomi MiMo 大模型提供深度优化，同时支持任意 OpenAI 兼容厂商通过配置文件一键接入。

---

## 特性

- **多模型支持** — DeepSeek V4、Xiaomi MiMo、任意 OpenAI 兼容厂商
- **四种 Agent 模式** — Build（构建）/ Plan（规划）/ Compose（编排）/ Max（最大）
- **跨平台原生支持** — macOS / Linux / Windows，Grep/Glob 由 Go 原生实现，无系统命令依赖
- **编辑快照安全网** — 写文件前自动快照，`/rewind` 一键回退
- **跨会话上下文** — `list_sessions` / `read_session` 工具，让 Agent 访问历史会话
- **工具调用修复** — RepairPipeline 自动修复 JSON 格式错误的工具调用
- **配置向导** — `loomcode setup` 交互式生成配置 + `.env`
- **配置 Schema** — `loomcode schema` 输出 JSON Schema Draft 7，支持编辑器自动补全
- **MCP 插件协议** — stdio + HTTP 双通道，接入外部工具
- **长期记忆** — SQLite FTS5，`/remember` 写入项目知识与用户偏好
- **成本控制** — 实时统计 Token 成本，可设预算上限
- **Skills 自动加载** — `~/.loomcode/skills/` 目录自动加载
- **Prefix Cache** — 充分利用 DeepSeek/MiMo 的前缀缓存能力降低成本

---

## 安装

**方式一：一键脚本（推荐，适合终端用户）**

```bash
curl -fsSL https://raw.githubusercontent.com/ShawnLiuSZ/LoomCode/main/scripts/install.sh | bash
```

脚本自动检测操作系统与架构（macOS/Linux/Windows × amd64/arm64），下载预编译二进制并配置 PATH。

**方式二：Go 源码安装（适合 Go 开发者）**

```bash
go install github.com/ShawnLiuSZ/loomcode/cmd/loomcode@latest
```

二进制安装到 `$GOPATH/bin/loomcode`，确保 `$GOPATH/bin` 在 PATH 中。

**方式三：本地构建（适合贡献者）**

```bash
git clone https://github.com/ShawnLiuSZ/loomcode.git
cd LoomCode
make build
./bin/loomcode --version
```

---

## 快速开始

### 首次使用：交互式配置向导

```bash
loomcode setup
```

五步引导：选择 Provider → 输入 API Key → 选择模型 → 生成 `~/.loomcode/models.toml` + `.env` → 输出 JSON Schema。（手动配置推荐使用 `~/.loomcode/models.json`）

### 启动 TUI

```bash
loomcode                       # 启动交互式 TUI（默认）
loomcode --provider deepseek   # 指定 Provider
loomcode --model deepseek-v4-pro  # 指定默认模型
loomcode --session <id>        # 恢复历史会话
```

### 单次任务

```bash
loomcode run "创建一个 hello.go"
echo "解释这段代码" | ./bin/loomcode run
```

### 输出配置 Schema

```bash
loomcode schema > ~/.loomcode/schema.json
# 编辑器（VS Code / Vim 等）加载此文件即可获得 loomcode.toml 的自动补全与校验
```

---

## 接入大模型

LoomCode 通过 Provider 适配器接入大模型。系统启动或执行 `/model` 切换时，**只读取已在配置文件中配置的模型**；未配置的模型不会出现在可选列表中。接入模型即是把它声明到配置文件里。

### 方式一：交互式配置向导（推荐）

```bash
loomcode setup
```

向导会：

1. 选择内置 Provider（DeepSeek / MiMo / OpenAI）或自定义 OpenAI 兼容厂商
2. 输入 API Key
3. 选择默认模型
4. 自动写入 `~/.loomcode/models.toml`（TOML）和 `~/.loomcode/.env`
5. 生成 JSON Schema 供编辑器补全

完成后再运行 `loomcode chat` 或 `loomcode run` 即可使用已配置的模型。

### 方式二：手动编辑配置

配置文件支持 **TOML** 与 **JSON** 两种格式，位置按以下优先级查找：

1. `--config <path>`（CLI 参数，最高优先级）
2. `./loomcode.toml`（项目级，已弃用，向后兼容）
3. `~/.loomcode/models.json`（全局用户配置，JSON，推荐）
4. `~/.loomcode/models.toml`（全局用户配置，TOML）
5. `./models.json` / `./models.toml`（项目级）
6. `~/.loomcode/loomcode.toml` / `~/.loomcode/config.toml`（已弃用，向后兼容）

JSON 完整示例见 [`models.example.json`](models.example.json)，TOML 完整示例见 [`loomcode.example.toml`](loomcode.example.toml)。最小可运行配置（JSON）：

```json
{
  "default_provider": "deepseek",
  "providers": [
    {
      "name": "deepseek",
      "display_name": "DeepSeek",
      "kind": "deepseek",
      "base_url": "https://api.deepseek.com",
      "api_key_env": "DEEPSEEK_API_KEY",
      "default_model": "deepseek-v4-flash",
      "models": [
        {
          "id": "deepseek-v4-flash",
          "name": "DeepSeek V4 Flash",
          "context_window": 131072,
          "cost": {
            "input": 0.14,
            "cached_input": 0.014,
            "output": 0.28
          },
          "capabilities": {
            "tool_call": true,
            "prefix_cache": true
          }
        }
      ]
    }
  ]
}
```

### 支持的 Provider 类型

| `kind` | 厂商 | 认证方式 | 说明 |
|--------|------|----------|------|
| `deepseek` | DeepSeek | API Key (`Authorization: Bearer`) | 支持 reasoning_content、prefix cache、工具调用修复 |
| `mimo` | 小米 MiMo | API Key (`Authorization: Bearer`) | 支持语音、推理、prefix cache；支持「按量付费 API」与「Token Plan」两种接入方式 |
| `openai` | OpenAI 及任意 OpenAI 兼容厂商 | API Key | 通用适配，可用于 GPT 系列、本地兼容服务等 |

自定义 OpenAI 兼容厂商时，只需把 `kind` 设为 `openai` 并填写正确的 `base_url`。

### 小米 MiMo 接入方式

MiMo 当前仅支持 API Key 接入（OAuth 暂未实现），提供两种接入方式，配置时只需把对应的 `base_url` 和 `api_key_env` 填对即可：

| 接入方式 | 说明 | `base_url` | API Key 格式 |
|----------|------|------------|--------------|
| 按量付费 API | 按实际使用量计费，适合轻度使用 | `https://api.xiaomimimo.com/v1` | `sk-xxxxx` |
| Token Plan | 固定订阅费，按套餐限量调用 | `https://token-plan-cn.xiaomimimo.com/v1` | `tp-xxxxx` |

按量付费示例：

```json
{
  "providers": [
    {
      "name": "mimo",
      "kind": "mimo",
      "base_url": "https://api.xiaomimimo.com/v1",
      "api_key_env": "MIMO_API_KEY",
      "default_model": "mimo-v2.5-pro"
    }
  ]
}
```

Token Plan 示例：

```json
{
  "providers": [
    {
      "name": "mimo-token-plan",
      "kind": "mimo",
      "base_url": "https://token-plan-cn.xiaomimimo.com/v1",
      "api_key_env": "MIMO_TOKEN_PLAN_KEY",
      "default_model": "mimo-v2.5-pro"
    }
  ]
}
```

> 提示：两种接入方式可以同时在配置中声明为两个 provider，启动后通过 `/model` 切换使用。

### 离线 Token 计数

DeepSeek provider 内置了纯 Go 实现的 DeepSeek V3 tokenizer，可在本地精确计算 prompt token 数，用于上下文窗口预估和压缩策略。无需 Python 或 `transformers` 依赖。

```bash
# 默认使用内置 tokenizer
loomcode --provider deepseek

# 使用自定义 tokenizer.json（可选）
LOOMCODE_TOKENIZER_PATH=/path/to/tokenizer.json loomcode --provider deepseek
```

其他 provider（MiMo / OpenAI）暂时使用字符数粗略估算，后续可按需扩展。

### 模型配置字段

| 字段 | 必填 | 说明 |
|------|------|------|
| `id` | 是 | 模型在 API 请求中使用的 ID，如 `deepseek-v4-pro` |
| `name` | 是 | 展示名称 |
| `context_window` | 是 | 上下文窗口大小（token 数） |
| `cost.input` | 否 | 输入单价（每百万 token） |
| `cost.cached_input` | 否 | 缓存命中输入单价（每百万 token） |
| `cost.output` | 否 | 输出单价（每百万 token） |
| `capabilities.reasoning` | 否 | 是否支持推理/thinking 模式 |
| `capabilities.tool_call` | 否 | 是否支持函数/工具调用 |
| `capabilities.prefix_cache` | 否 | 是否支持前缀缓存 |
| `capabilities.vision` | 否 | 是否支持视觉输入 |
| `capabilities.voice` | 否 | 是否支持语音输入 |

### API Key

配置文件中 `api_key_env` 指定环境变量名，实际密钥从 `.env` 或系统环境变量读取。也可以直接写 `api_key`（优先级高于 `api_key_env`）：

```json
{
  "providers": [
    {
      "name": "deepseek",
      "api_key": "sk-xxxxxxxxxxxxxxxx"
    }
  ]
}
```

> 安全建议：优先使用 `api_key_env` + `.env`，避免把密钥明文写入配置文件。

```bash
# ~/.loomcode/.env 或 ./.env
DEEPSEEK_API_KEY=sk-xxxxxxxxxxxxxxxx
MIMO_API_KEY=sk-xxxxxxxxxxxxxxxx
OPENAI_API_KEY=sk-xxxxxxxxxxxxxxxx
```

`.env` 加载优先级：

```
1. ~/.loomcode/.env
2. ./.env
3. ./.env.local
4. --env-file custom.env（最高优先级）
```

### 切换模型

在 TUI 中：

```bash
/model           # 交互式选择（仅列出已配置模型）
/model <id>      # 直接切换到指定已配置模型
```

CLI 启动时指定：

```bash
loomcode --model deepseek-v4-pro
```

---

## CLI 命令

### 子命令

| 命令 | 说明 |
|------|------|
| `loomcode` / `loomcode chat` / `loomcode tui` | 启动交互式 TUI（默认） |
| `loomcode run <task>` | 单次任务，支持从 stdin 读取任务内容 |
| `loomcode setup` | 交互式配置向导，生成 `~/.loomcode/models.toml` + `.env` |
| `loomcode schema` | 输出 JSON Schema（配置校验） |
| `loomcode dashboard [addr]` | 启动 Web Dashboard（默认 `:8080`） |

### 全局选项

| 选项 | 说明 |
|------|------|
| `--provider <name>` | 指定 Provider（`deepseek` / `mimo` / `openai`） |
| `--model <id>` | 指定模型 ID |
| `--config <path>` | 指定配置文件路径 |
| `--env-file <path>` | 加载自定义 `.env` |
| `--session <id>` | 恢复历史会话 |
| `--version` | 显示版本 |

### 示例

```bash
loomcode                                  # 启动 TUI
loomcode run "解释这段代码"               # 单次任务
loomcode --provider deepseek --model deepseek-v4-pro
loomcode --session session_xxxxxxxx       # 恢复会话
loomcode dashboard :9090                  # 在 9090 端口启动 Dashboard
```

---

## TUI 交互

| 操作 | 说明 |
|------|------|
| 直接输入文字 | 发送任务给 AI |
| Tab | 切换 Agent 模式（build / plan / compose / max） |
| 输入 `/` 后 Tab | 命令自动补全 |
| ↑↓ / Enter / Esc | 交互式选择器（模型选择、会话恢复等） |
| Shift+Enter | 换行 |
| Ctrl+C 两次 | 退出（3 秒内二次确认） |

### TUI 命令

| 命令 | 说明 |
|------|------|
| `/help` | 显示帮助 |
| `/mode` | 显示当前模式和模型 |
| `/build` / `/plan` / `/compose` / `/max` | 切换 Agent 模式 |
| `/goal` | 查看当前停止条件 |
| `/goal "<条件>"` | 设置停止条件 |
| `/goal clear` | 清除停止条件 |
| `/steps` | 查看当前最大步数 |
| `/steps <n>` | 设置最大步数 |
| `/model` | 交互式选择模型（↑↓ 选择，Enter 确认） |
| `/model <name>` | 直接切换模型 |
| `/skills` | 显示内置工具和外部 skills |
| `/clear` | 清空聊天记录 |
| `/cost` | 显示成本统计 |
| `/budget` | 查看当前预算 |
| `/budget <amount>` | 设置会话预算上限 |
| `/budget clear` | 清除预算 |
| `/env` | 查看环境变量 |
| `/env set <KEY> <VALUE>` | 设置环境变量 |
| `/env unset <KEY>` | 移除环境变量 |
| `/env reload` | 重新加载环境变量 |
| `/sessions` | 列出会话 |
| `/sessions new <name>` | 创建新会话 |
| `/sessions switch <ID>` | 切换到指定会话 |
| `/sessions rename <ID> <新名称>` | 重命名会话 |
| `/resume` | 列出并恢复历史会话 |
| `/resume <ID>` | 恢复指定会话 |
| `/compact` | 压缩上下文 |
| `/queue` | 查看任务队列 |
| `/remember <text>` | 写入长期记忆（项目知识 / 用户偏好） |
| `/rewind` | 列出最近的编辑快照 |
| `/rewind last` | 回退到最近一次编辑前的状态 |
| `/rewind <id>` | 按 ID 回退到指定快照 |
| `/quit` / `/exit` | 退出 |

---

## 编辑快照安全网（/rewind）

LoomCode 在每次写文件/编辑文件前自动创建快照副本，存储在 `~/.loomcode/checkpoints/`，最多保留 100 个。被覆盖或删除的文件可通过 `/rewind` 命令恢复。

```bash
# 查看最近 20 个快照
/rewind

# 恢复最近一次编辑
/rewind last

# 恢复指定快照
/rewind 1700000000000_001
```

快照元数据（`meta.json`）记录：原路径、快照时间、触发工具（`write_file` / `edit_file`）、原文件是否存在、文件大小。

---

## 跨会话上下文

Agent 在执行任务时可调用以下工具访问历史会话：

| 工具 | 说明 |
|------|------|
| `list_sessions` | 列出最近会话（可按 limit/role 过滤） |
| `read_session` | 读取指定会话的完整消息历史 |

这让 Agent 能"记得"之前讨论过的方案、约定和决策，避免重复提问。

---

## 配置

### 配置文件

推荐将 provider 配置放在 `~/.loomcode/models.json`（JSON）；也支持 `~/.loomcode/models.toml`（TOML）。项目级可使用 `./models.json` / `./models.toml`。旧的 `loomcode.toml` / `~/.loomcode/loomcode.toml` 仍兼容，但已弃用。

JSON 示例见 [`models.example.json`](models.example.json)，TOML 示例见 [`loomcode.example.toml`](loomcode.example.toml)。

```json
{
  "default_provider": "deepseek",
  "providers": [
    {
      "name": "deepseek",
      "display_name": "DeepSeek",
      "kind": "deepseek",
      "base_url": "https://api.deepseek.com",
      "api_key_env": "DEEPSEEK_API_KEY",
      "default_model": "deepseek-v4-flash",
      "models": [
        {
          "id": "deepseek-v4-flash",
          "name": "DeepSeek V4 Flash",
          "context_window": 131072,
          "cost": {
            "input": 0.14,
            "cached_input": 0.014,
            "output": 0.28
          },
          "capabilities": {
            "tool_call": true,
            "prefix_cache": true
          }
        }
      ]
    }
  ]
}
```

### JSON Schema 校验

```bash
loomcode schema > ~/.loomcode/schema.json
```

VS Code 在 TOML 配置顶部添加：

```toml
#:schema ~/.loomcode/schema.json
```

即可获得字段补全、类型校验、枚举提示。JSON 配置同理可通过编辑器关联 `schema.json`。

### 环境变量

支持四级 `.env` 加载（后加载覆盖前加载）：

```
1. ~/.loomcode/.env        ← 全局配置
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

## MCP 插件

在配置文件（推荐 `~/.loomcode/models.json`，也支持 `~/.loomcode/models.toml` 或项目级对应文件）中配置 MCP 服务器，扩展工具能力：

```toml
# stdio 模式
[[plugins]]
name    = "my-tool"
command = "node"
args    = ["./mcp-server.js"]

# HTTP/SSE 模式
[[plugins]]
name = "remote-tool"
url  = "https://mcp.example.com/sse"
```

---

## Skills 自动加载

启动时自动从以下目录加载 skills：

```
~/.agents/skills/*/SKILL.md   ← 低优先级
~/.loomcode/skills/*/SKILL.md    ← 高优先级（同名覆盖）
```

每个 skill 是一个包含 `SKILL.md` 的目录。使用 `/skills` 查看所有已加载的 skills。

---

## 技术栈

| 层次 | 技术 |
|------|------|
| 语言 | Go（CGO_ENABLED=0） |
| 配置 | TOML + JSON Schema |
| TUI | Bubble Tea + Lip Gloss |
| 记忆存储 | SQLite FTS5 |
| 插件协议 | MCP（stdio + HTTP） |
| LSP | JSON-RPC 2.0 |
| API 协议 | OpenAI 兼容 |
| 跨平台 | Unix + Windows（build tags 分流） |

---

## 项目结构

```
cmd/loomcode/main.go          ← CLI 入口
internal/
  config/                  ← 配置加载、向导、Schema 生成
  provider/                ← 多厂商模型接入
    ├── deepseek/          ← DeepSeek V4 适配器
    ├── mimo/              ← MiMo V2.5 适配器
    └── openai/            ← 通用 OpenAI 兼容适配器
  agent/                   ← Agent 引擎（loop/modes/subagent/judge）
  tool/                    ← 工具系统
    ├── platform_*.go      ← 跨平台进程管理
    ├── checkpoint.go      ← 编辑快照管理器
    ├── command_tools.go   ← Bash/Grep/Glob 工具
    ├── file_tools.go       ← Read/Write/Edit 工具
    ├── git_tools.go       ← Git 工具
    ├── session_tools.go   ← 跨会话上下文工具
    └── registry.go        ← 工具注册中心
  context/                 ← 三层分区上下文管理
  memory/                  ← 长期记忆（SQLite FTS5）
  control/                 ← 权限/成本/门控
  session/                 ← 会话管理 + JSONL 持久化
  mcp/                     ← MCP 客户端
  ui/                      ← Bubble Tea TUI
```

---

## 开发

```bash
make build          # 构建
make test           # 运行测试
make lint           # 代码检查
make install        # 安装到 $GOPATH/bin
make release        # 发布构建
```

交叉编译验证：

```bash
GOOS=windows go build ./...    # 验证 Windows 编译
GOOS=linux   go build ./...    # 验证 Linux 编译
```

详见 [CONTRIBUTING.md](CONTRIBUTING.md)。

---

## 项目状态

当前处于 **活跃开发阶段**。

| 阶段 | 状态 |
|------|------|
| Phase 1 核心框架 MVP | 已完成 |
| Phase 2 缓存与成本优化 | 已完成 |
| Phase 3 多 Agent 与记忆 | 已完成 |
| Phase 4 生态与进化 | 已完成 |
| Phase 5 跨平台与安全网 | 已完成 |

**343 个测试全部通过**，18 个模块覆盖完整。

---

## 文档

- [CODEBUDDY.md](CODEBUDDY.md) — 项目入口指南
- [CONTRIBUTING.md](CONTRIBUTING.md) — 贡献指南
- [文档导航](docs/README.md) — 全部文档索引

---

## 致谢

本项目融合了以下项目的核心设计思想：

- [DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix) — Planner/Executor 分离 session 架构
- [MiMo-Code](https://github.com/XiaomiMiMo/MiMo-Code) — Prefix Cache 调度与工具能力声明

---

## 许可证

MIT
