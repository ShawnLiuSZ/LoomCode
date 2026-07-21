# LoomCode CLI 开发计划

## 项目概述

**LoomCode** 是一个基于 Go 语言的可扩展多模型 AgentCLI，融合 DeepSeek-Reasonix 和 MiMo-Code 的核心优点，专门为 DeepSeek V4 和 Xiaomi MiMo 大模型提供深度优化，同时支持任意 OpenAI 兼容厂商的插件化接入。

- **形态**: 纯 CLI 工具（暂不做桌面应用）
- **语言**: Go（CGO_ENABLED=0 单二进制分发）
- **配置**: JSON 驱动（settings.json + models.json）
- **插件协议**: MCP（stdio + HTTP）
- **记忆存储**: SQLite FTS5
- **TUI**: Bubble Tea + Lip Gloss

---

## Phase 1: 核心框架 MVP

### 目标
可交互的命令行 Agent，支持 DeepSeek V4 和 MiMo 的基本对话与工具调用。

### MVP 策略：可交互优先

采用"可交互优先"策略——尽快做出最小可用版本，能真正跑起来后再逐步叠加优化层。

**Phase 1 交付物**：`loomcode run "读取 main.go 并解释这段代码"` 这个最小闭环能跑通：
- 加载配置 → 选择 Provider → 构建消息 → 模型推理 → 工具调用 → 流式返回结果

### 开发顺序

按以下顺序逐步实现，每个步骤完成后立即可验证：

```
1. go mod init + 目录结构           → `go build` 成功
2. JSON 配置加载                    → 打印配置验证
3. OpenAI 通用 Provider             → curl 式调用 API 成功
4. DeepSeek Provider                → 流式推理输出
5. 工具注册 + 执行                  → read_file/write_file/bash 可调用
6. Agent 推理循环                   → 多轮对话跑通
7. CLI 入口 (loomcode run)             → 端到端可用的命令行
8. Bubble Tea TUI                   → 交互式 REPL
9. install.sh + .goreleaser.yaml    → 可分发的二进制
```

### 1.1 环境准备

- [ ] 安装 Go（通过 Homebrew: `brew install go`）
- [ ] 初始化 Go module: `go mod init github.com/loomcode/loomcode`
- [ ] 创建基础目录结构
- [ ] 编写 Makefile（build/test/lint/release）

### 1.2 配置系统

**文件**: `internal/config/config.go`

- [ ] JSON 解析（encoding/json）
- [ ] Provider 配置结构定义
- [ ] Model 配置结构（含 cost、context_window、capabilities）
- [ ] 配置加载优先级：先合并 `~/.loomcode/{models.json, settings.json}`（global），再叠加 `<project>/.loomcode/{settings.json, settings.local.json}`（project > global，local > shared）
- [ ] 环境变量密钥注入（`api_key: "${ENV_VAR}"`，旧 `api_key_env` 已弃用）
- [ ] 示例配置文件 `settings.example.json` + `models.example.json`

### 1.3 Provider 层（可扩展设计）

**目录**: `internal/provider/`

核心接口：

```go
// Provider 实例接口
type Provider interface {
    Chat(ctx, req) (*ChatResponse, error)
    Stream(ctx, req) (<-chan StreamEvent, error)
    Models() []ModelInfo
    Capabilities() Capabilities
    Cost(model, usage) Cost
    Name() string
}

// 适配器工厂接口
type Adapter interface {
    Kind() string
    Create(cfg ProviderConfig) (Provider, error)
    ValidateConfig(cfg ProviderConfig) error
}
```

**内置适配器**:

- [ ] `internal/provider/registry.go` — 适配器注册中心
- [ ] `internal/provider/openai/adapter.go` — 通用 OpenAI 兼容适配器（任何厂商的基础）
- [ ] `internal/provider/deepseek/adapter.go` — DeepSeek 深度适配（推理内容处理、前缀缓存）
- [ ] `internal/provider/mimo/adapter.go` — MiMo 深度适配（OAuth、语音 ASR）

**Capabilities 能力声明**:

```go
type Capabilities struct {
    SupportsReasoning    bool
    SupportsToolCall     bool
    SupportsPrefixCache  bool
    SupportsStreaming    bool
    SupportsVision       bool
    SupportsVoice        bool
    SupportsOAuth        bool
    NeedsToolRepair      bool
    CacheTTL             time.Duration
    MaxToolCallsPerRound int
}
```

### 1.4 Agent 推理循环

**文件**: `internal/agent/loop.go`

- [ ] 基础推理循环：用户输入 → 构建消息 → 调用 Provider → 解析响应 → 处���工具调用 → 返回结果
- [ ] 流式输出支持
- [ ] 工具调用解析（OpenAI tool_calls 格式）
- [ ] 多轮对话上下文管理
- [ ] 就绪检查（检测模型是否完成最终答案）

### 1.5 工具系统

**目录**: `internal/tool/`

- [ ] `internal/tool/registry.go` — 工具注册中心
- [ ] `internal/tool/executor.go` — 工具执行引擎
- [ ] 基础工具实现：
  - `read_file` — 读取文件
  - `write_file` — 写入文件
  - `edit_file` — 精确编辑文件
  - `bash` — 执行 Shell 命令
  - `grep` — 内容搜索
  - `glob` — 文件匹配
- [ ] 工具 Schema 定义（JSON Schema，用于模型工具调用）

### 1.6 CLI 入口

**文件**: `cmd/loomcode/main.go`

- [ ] 交互式 REPL 模式（默认）
- [ ] 单次任务模式：`loomcode run "任务描述"`
- [ ] 管道模式：`echo "代码" | loomcode run`
- [ ] 模型选择：`loomcode --provider deepseek --model deepseek-v4-pro`
- [ ] 配置向导：`loomcode setup`

### 1.7 安装与分发

- [ ] curl 一行安装脚本 `install.sh`
- [ ] Homebrew formula 模板
- [ ] `.goreleaser.yaml` 多平台构建配置

---

## Phase 2: 缓存与成本优化

### 目标
集成 DeepSeek 前缀缓存优化，实现低成本长会话。

### 2.1 三层上下文分区

**文件**: `internal/context/partition.go`

- [ ] 不可变前缀（system + tool_specs + 记忆）
- [ ] 追加日志（assistant + tool 消息，单调增长）
- [ ] 易变草稿（每轮重置的临时状态）
- [ ] 前缀哈希计算与缓存命中追踪

### 2.2 工具调用修复流水线

**文件**: `internal/tool/repair.go`

- [ ] `flatten` — 深层嵌套参数扁平化
- [ ] `scavenge` — 从 reasoning_content 回收遗漏的工具调用
- [ ] `truncation` — 截断 JSON 补全
- [ ] 修复流水线按 Provider Capabilities 自适应启用

### 2.3 成本控制

**文件**: `internal/control/cost.go`

- [ ] flash-first 分层默认（便宜模型优先）
- [ ] 辅助调用（摘要、子代理）强制使用低成本模型
- [ ] 工具结果自动压缩（超长结果摘要化）
- [ ] 成本实时可视化（绿/黄/红分级）

### 2.4 并行工具调度

**文件**: `internal/tool/parallel.go`

- [ ] 连续只读工具自动分组并行
- [ ] 写工具保持串行
- [ ] 最大并行数可配置（默认 3）

---

## Phase 3: 多 Agent 与记忆系统

### 目标
MiMo 式的多 Agent 系统和持久化记忆。

### 3.1 Agent 模式

**文件**: `internal/agent/modes.go`

- [ ] Build Agent — 默认模式，完整工具权限
- [ ] Plan Agent — 只读分析模式
- [ ] Compose Agent — 编排模式，规格驱动开发
- [ ] Max Mode — 并行 N 候选 + judge 选最优（实验性）

### 3.2 子 Agent 系统

**文件**: `internal/agent/subagent.go`

- [ ] 按需创建子 Agent
- [ ] 上下文共享
- [ ] 并行执行支持
- [ ] 生命周期管理（跟踪、取消、超时）

### 3.3 记忆系统

**目录**: `internal/memory/`

- [ ] SQLite FTS5 全文搜索存储
- [ ] 四层记忆体系：
  - `checkpoint.md` — 会话级检查点
  - `MEMORY.md` — 项目级持久记忆
  - 全局记忆 — 跨项目用户偏好
  - SQLite 历史 — 完整原始对话
- [ ] 独立 Writer 子代理（状态管理解耦）
- [ ] 提前提取策略（20%/45%/70% 窗口触发）

### 3.4 Cycle 无限会话

**文件**: `internal/context/checkpoint.go`

- [ ] 基于上下文窗口的自动检查点
- [ ] 从检查点 + 记忆 + 任务进度重建上下文
- [ ] Token 预算控制，按重要性注入

### 3.5 Goal 停止条件

**文件**: `internal/agent/judge.go`

- [ ] 用户设置自然语言停止条件
- [ ] 独立 judge 模型评估是否满足
- [ ] 防止 AI 乐观提前停止

---

## Phase 4: 自我进化与生态

### 4.1 Dream 机制

**文件**: `internal/memory/dream.go`

- [ ] 扫描近期会话轨迹
- [ ] 提取持久知识到 MEMORY.md
- [ ] 清理过时条目

### 4.2 Distill 机制

**文件**: `internal/memory/distill.go`

- [ ] 识别重复手动工作流
- [ ] 打包为可复用 skill/subagent/command

### 4.3 MCP 插件

**目录**: `internal/mcp/`

- [ ] MCP 客户端（stdio transport）
- [ ] MCP 客户端（HTTP SSE transport）
- [ ] 工具服务器注册与生命周期

### 4.4 语音输入

**文件**: `internal/ui/voice.go`

- [ ] 实时流式语音输入（基于 PortAudio）
- [ ] MiMo ASR 集成
- [ ] 跨平台音频支持

### 4.5 编辑器集成

- [ ] LSP 客户端（代码补全、诊断、跳转）

### 4.6 编辑门控

**文件**: `internal/control/gate.go`

- [ ] review 模式 — 编辑前弹出确认
- [ ] auto 模式 — 自动应用，可撤销
- [ ] yolo 模式 — 跳过所有确认

---

## 目录结构总览

```
loomcode/
├── cmd/loomcode/
│   └── main.go                    # CLI 主入口
├── internal/
│   ├── agent/
│   │   ├── loop.go               # 推理循环
│   │   ├── modes.go              # Build/Plan/Compose/Max
│   │   ├── subagent.go           # 子 Agent 管理
│   │   ├── judge.go              # Goal 裁判
│   │   └── workflow.go           # Dynamic Workflow
│   ├── provider/
│   │   ├── registry.go           # 适配器注册中心
│   │   ├── adapter.go            # Adapter 工厂接口
│   │   ├── provider.go           # Provider 实例接口
│   │   ├── capabilities.go       # 能力声明
│   │   ├── config.go             # Provider 配置结构
│   │   ├── deepseek/
│   │   │   ├── adapter.go
│   │   │   ├── provider.go
│   │   │   └── cache.go
│   │   ├── mimo/
│   │   │   ├── adapter.go
│   │   │   ├── provider.go
│   │   │   ├── oauth.go
│   │   │   └── voice.go
│   │   └── openai/
│   │       ├── adapter.go
│   │       └── provider.go
│   ├── context/
│   │   ├── partition.go          # 三层上下文分区
│   │   ├── cache.go              # 前缀缓存管理
│   │   ├── checkpoint.go         # 检查点/重建
│   │   └── compress.go           # 压缩/剪枝
│   ├── memory/
│   │   ├── sqlite.go             # SQLite FTS5 存储
│   │   ├── layers.go             # 四层记忆管理
│   │   ├── writer.go             # 独立 Writer
│   │   ├── dream.go              # 知识提取
│   │   └── distill.go            # 工作流打包
│   ├── tool/
│   │   ├── registry.go           # 工具注册
│   │   ├── executor.go           # 执行引擎
│   │   ├── repair.go             # 修复流水线
│   │   ├── parallel.go           # 并行调度
│   │   └── tools/
│   │       ├── file.go
│   │       ├── bash.go
│   │       ├── search.go
│   │       ├── git.go
│   │       └── web.go
│   ├── control/
│   │   ├── cost.go               # 成本控制
│   │   ├── permission.go         # 权限/沙箱
│   │   └── gate.go               # 编辑门控
│   ├── session/
│   │   ├── session.go
│   │   └── storage.go            # JSONL 持久化
│   ├── mcp/
│   │   ├── client.go
│   │   └── transport.go
│   ├── config/
│   │   ├─�� config.go
│   │   └── migrate.go            # 配置迁移
│   └── ui/
│       ├── app.go                # Bubble Tea 主程序
│       ├── input.go              # 输入组件
│       ├── chat.go               # 聊天视图
│       ├── diff.go               # Diff 视图
│       ├── cost.go               # 成本仪表盘
│       └── voice.go              # 语音输入
├── scripts/
│   └── install.sh                # 一键安装脚本
├── settings.example.json            # 示例主配置
├── models.example.json             # 示例模型配置
├── Makefile
├── go.mod
├── go.sum
├── .goreleaser.yaml
└── README.md
```

---

## 技术依赖

| 依赖 | 用途 | 版本 |
|------|------|------|
| `github.com/charmbracelet/bubbletea` | TUI 框架 | latest |
| `github.com/charmbracelet/lipgloss` | TUI 样式 | latest |
| `github.com/BurntSushi/toml` | TOML 解析（仅历史迁移输入，活配置已统一为 JSON） | v1.x |
| `github.com/mattn/go-sqlite3` | SQLite 驱动 | latest |
| `modernc.org/sqlite` | 纯 Go SQLite（备选） | latest |
| `github.com/sashabaranov/go-openai` | OpenAI API 客户端 | latest |

---

## 关键设计决策

1. **Go 语言** — 借鉴 Reasonix 的单二进制优势，性能优于 TypeScript
2. **Provider 插件化** — Adapter 工厂模式，任何厂商通过配置接入
3. **JSON 配置** — 结构化、编辑器 JSON Schema 自动补全友好，Reasonix 验证的选择
4. **Capabilities 自适应** — Agent 根据 Provider 能力自动调整策略
5. **SQLite FTS5** — 借鉴 MiMo 的全文搜索记忆
6. **MCP 协议** — 两者都支持的标准插件协议
