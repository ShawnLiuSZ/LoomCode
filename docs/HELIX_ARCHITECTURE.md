# Helix CLI 架构设计文档

> 版本: v1.0  
> 日期: 2026-06-18  
> 形态: 纯 CLI 工具（不做桌面应用）  
> 基于: [HELIX_PLAN.md](./HELIX_PLAN.md)

---

## 1. 架构总览

### 1.1 分层架构

Helix 采用经典的分层架构，从上到下分为四层：

```
┌─────────────────────────────────────────────────────────────┐
│                     表示层 (Presentation)                    │
│          CLI 入口 · TUI 界面 · 语音输入 · 管道模式           │
├─────────────────────────────────────────────────────────────┤
│                     应用层 (Application)                     │
│       Agent 引擎 · 推理循环 · 多Agent编排 · 会话管理         │
├─────────────────────────────────────────────────────────────┤
│                     领域层 (Domain)                          │
│    Provider适配 · 工具系统 · 上下文管理 · 记忆系统 · 权限    │
├─────────────────────────────────────────────────────────────┤
│                   基础设施层 (Infrastructure)                 │
│      配置加载 · SQLite存储 · MCP通信 · HTTP客户端            │
└─────────────────────────────────────────────────────────────┘
```

### 1.2 核心设计原则

| 原则 | 说明 |
|------|------|
| **Provider First** | 所有模型能力通过 Provider 抽象，零硬编码厂商 |
| **Capability-Driven** | Agent 行为由 Provider 的 Capabilities 声明驱动，而非 if-else 判断厂商 |
| **Single Binary** | CGO_ENABLED=0 静态编译，一个文件即可部署 |
| **Config Over Code** | 模型、工具、插件全部通过 TOML 配置文件声明 |
| **Compose Over Inherit** | 通过接口组合构建复杂行为，避免深层继承 |

---

## 2. 模块架构

### 2.1 模块依赖关系

```
cmd/helix/main.go
    │
    ├── internal/config/          # 配置系统（被所有模块依赖）
    │
    ├── internal/provider/        # Provider 层
    │   ├── registry.go           #   ← 适配器注册
    │   ├── adapter.go            #   ← Adapter 接口
    │   ├── provider.go           #   ← Provider 接口
    │   ├── capabilities.go       #   ← 能力声明
    │   ├── deepseek/             #   ← DeepSeek 适配器
    │   ├── mimo/                 #   ← MiMo 适配器
    │   └── openai/               #   ← 通用 OpenAI 适配器
    │
    ├── internal/tool/            # 工具系统
    │   ├── registry.go           #   ← 工具注册中心
    │   ├── executor.go           #   ← 工具执行引擎
    │   ├── repair.go             #   ← 修复流水线
    │   ├── parallel.go           #   ← 并行调度
    │   └── tools/                #   ← 具体工具实现
    │
    ├── internal/context/         # 上下文管理
    │   ├── partition.go          #   ← 三层分区
    │   ├── cache.go              #   ← 缓存管理
    │   ├── checkpoint.go         #   ← 检查点
    │   └── compress.go           #   ← 压缩剪枝
    │
    ├── internal/agent/           # Agent 引擎
    │   ├── loop.go               #   ← 推理循环
    │   ├── modes.go              #   ← Agent 模式
    │   ├── subagent.go           #   ← 子Agent
    │   ├── judge.go              #   ← 停止判定
    │   └── workflow.go           #   ← 工作流编排
    │
    ├── internal/memory/          # 记忆系统
    │   ├── sqlite.go             #   ← 存储引擎
    │   ├── layers.go             #   ← 分层管理
    │   ├── writer.go             #   ← 独立Writer
    │   ├── dream.go              #   ← 知识提取
    │   └── distill.go            #   ← 工作流打包
    │
    ├── internal/control/         # 控制层
    │   ├── cost.go               #   ← 成本控制
    │   ├── permission.go         #   ← 权限沙箱
    │   └── gate.go               #   ← 编辑门控
    │
    ├── internal/session/         # 会话管理
    │   ├── session.go
    │   └── storage.go
    │
    ├── internal/mcp/             # MCP 插件
    │   ├── client.go
    │   └── transport.go
    │
    └── internal/ui/              # TUI 界面
        ├── app.go
        ├── input.go
        ├── chat.go
        ├── diff.go
        ├── cost.go
        └── voice.go
```

### 2.2 核心模块职责

| 模块 | 职责 | 关键抽象 |
|------|------|---------|
| **config** | TOML 配置加载、环境变量注入、配置优先级 | `Config` 结构体 |
| **provider** | 多厂商模型接入、能力声明、成本计算 | `Provider` 接口, `Adapter` 接口 |
| **tool** | 工具注册、执行、修复、并行调度 | `Tool` 接口, `ToolRegistry` |
| **context** | 三层上下文分区、缓存管理、检查点重建 | `ContextPartition` |
| **agent** | 推理循环、Agent模式、子Agent编排 | `Agent`, `AgentLoop` |
| **memory** | SQLite FTS5存储、分层记忆、Dream/Distill | `MemoryStore` |
| **control** | 成本控制、权限沙箱、编辑门控 | `CostController`, `PermissionGate` |
| **session** | 会话生命周期、JSONL持久化 | `Session` |
| **mcp** | MCP协议客户端 | `MCPClient` |
| **ui** | Bubble Tea TUI、聊天视图、成本仪表盘 | `App` |

---

## 3. Provider 层架构（核心可扩展设计）

### 3.1 接口体系

```
┌─────────────────────────────────────────────────────────────┐
│                     Provider Registry                        │
│  Registry.Register(adapter) → map[kind]Adapter              │
│  Registry.Create(kind, config) → (Provider, error)          │
└──────────────────────┬──────────────────────────────────────┘
                       │
         ┌─────────────┼─────────────┐
         ▼             ▼             ▼
   ┌──────────┐ ┌──────────┐ ┌──────────┐
   │DeepSeek  │ │  MiMo    │ │ OpenAI   │
   │ Adapter  │ │ Adapter  │ │ Adapter  │
   │          │ │          │ │          │
   │ Kind:    │ │ Kind:    │ │ Kind:    │
   │"deepseek"│ │ "mimo"   │ │ "openai" │
   └────┬─────┘ └────┬─────┘ └────┬─────┘
        │            │            │
        ▼            ▼            ▼
   ┌──────────┐ ┌──────────┐ ┌──────────┐
   │DeepSeek  │ │  MiMo    │ │ OpenAI   │
   │ Provider │ │ Provider │ │ Provider │
   │          │ │          │ │          │
   │ ·前缀缓存 │ │ ·OAuth   │ │ ·通用协议 │
   │ ·修复流水 │ │ ·语音ASR │ │          │
   └──────────┘ └──────────┘ └──────────┘
```

### 3.2 Provider 接口

```go
type Provider interface {
    // 核心推理
    Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    Stream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error)

    // 元信息
    Name() string
    Models() []ModelInfo
    Capabilities() Capabilities

    // 成本
    Cost(modelID string, usage Usage) Cost

    // 特殊能力（可选实现，通过接口断言获取）
    // OAuthProvider   — 支持 OAuth 认证
    // VoiceProvider   — 支持语音输入
    // CacheProvider   — 支持前缀缓存管理
}
```

### 3.3 Adapter 工厂接口

```go
type Adapter interface {
    Kind() string
    Create(cfg ProviderConfig) (Provider, error)
    ValidateConfig(cfg ProviderConfig) error
}
```

### 3.4 Capabilities 能力声明

```go
type Capabilities struct {
    SupportsReasoning    bool          // 是否支持 reasoning_content
    SupportsToolCall     bool          // 是否支持原生工具调用
    SupportsPrefixCache  bool          // 是否支持前缀缓存
    SupportsStreaming    bool          // 是否支持流式输出
    SupportsVision       bool          // 是否支持图片输入
    SupportsVoice        bool          // 是否支持语音输入
    SupportsOAuth        bool          // 是否支持 OAuth 认证
    NeedsToolRepair      bool          // 是否需要工具调用修复
    CacheTTL             time.Duration // 缓存有效期
    MaxToolCallsPerRound int           // 单轮最大工具调用数
}
```

### 3.5 自适应行为

Agent 引擎根据 Capabilities 自动调整策略，而非硬编码厂商判断：

```go
func (a *Agent) configureLoop(p provider.Provider) {
    caps := p.Capabilities()

    // 前缀缓存：仅支持缓存的 Provider 启用
    if caps.SupportsPrefixCache {
        a.ctxPartition = NewCacheStablePartition(caps.CacheTTL)
    } else {
        a.ctxPartition = NewStandardPartition()
    }

    // 工具修复：仅 DeepSeek 等需要修复的 Provider 启用
    if caps.NeedsToolRepair {
        a.toolRepair = NewRepairPipeline()
    }

    // 语音输入：仅 MiMo 等支持语音的 Provider 启用
    if caps.SupportsVoice {
        a.voiceInput = NewVoiceInput(p)
    }
}
```

### 3.6 扩展新厂商

只需 3 步，无需修改任何代码：

```toml
# 步骤1: 编辑 helix.toml
[[providers]]
name         = "qwen"
display_name = "通义千问"
kind         = "openai"         # 使用通用 OpenAI 适配器
base_url     = "https://dashscope.aliyuncs.com/compatible-mode/v1"
api_key_env  = "DASHSCOPE_API_KEY"

  [[providers.models]]
  id    = "qwen-max"
  name  = "Qwen Max"
  context_window = 32768
  capabilities   = { tool_call = true }

# 步骤2: 设置环境变量
export DASHSCOPE_API_KEY="sk-xxx"

# 步骤3: 使用
helix --provider qwen --model qwen-max
```

---

## 4. Agent 引擎架构

### 4.1 推理循环

```
                    ┌─────────────┐
                    │  用户输入    │
                    └──────┬──────┘
                           ▼
              ┌────────────────────────┐
              │   构建消息上下文         │
              │   (不可变前缀 + 对话历史) │
              └───────────┬────────────┘
                          ▼
              ┌────────────────────────┐
              │   Provider.Stream()     │
              │   流式推理              │
              └───────────┬────────────┘
                          ▼
                   ┌─────────────┐
                   │ 有工具调用？  │
                   └──┬──────┬───┘
                  Yes │      │ No
                      ▼      ▼
         ┌──────────────┐  ┌──────────────┐
         │ 工具修复流水线 │  │  就绪检查     │
         │ (自适应启用)  │  │  (todos完成?) │
         └──────┬───────┘  └──────┬───────┘
                ▼                 ▼
         ┌──────────────┐  ┌──────────────┐
         │ 并行工具执行   │  │  返回最终答案  │
         │ (只读并行)    │  └──────────────┘
         └──────┬───────┘
                ▼
         ┌──────────────┐
         │ 结果追加到日志 │
         │ 上下文压缩检查 │
         └──────┬───────┘
                │
                └──────→ 回到"构建消息上下文"
```

### 4.2 Agent 模式

| 模式 | 权限 | 适用场景 | 触发方式 |
|------|------|---------|---------|
| **Build** | 完整工具权限 | 日常开发（默认） | 默认模式 |
| **Plan** | 只读分析 | 代码探索、方案设计 | `/plan` 命令 |
| **Compose** | 编排模式 | 规格驱动开发 | `/compose` 命令 |
| **Max** | 并行选优 | 高难度任务 | 配置 `experimental.maxMode` |

### 4.3 子 Agent 系统

```
主 Agent
  │
  ├── Explore Agent (只读)     ← 代码库探索
  ├── Writer Agent (独立)      ← 状态管理解耦
  ├── Judge Agent (独立)       ← 停止条件评估
  ├── Dream Agent (离线)       ← 知识提取
  └── Distill Agent (离线)     ← 工作流打包
```

---

## 5. 上下文管理架构

### 5.1 三层分区模型

```
┌─────────────────────────────────────────┐
│ IMMUTABLE PREFIX                        │ ← 会话固定，缓存命中候选
│   system prompt                          │
│   + tool_specs                          │
│   + 项目记忆 (MEMORY.md)                 │
│   + few_shot_examples                    │
├─────────────────────────────────────────┤
│ APPEND-ONLY LOG                         │ ← 单调增长，保留前缀
│   [assistant_msg₁]                      │
│   [tool_call₁ + tool_result₁]           │
│   [assistant_msg₂]                      │
│   ...                                   │
├─────────────────────────────────────────┤
│ VOLATILE SCRATCH                        │ ← 每轮重置，不发送给模型
│   R1 thought                            │
│   transient plan state                  │
│   draft response                        │
└─────────────────────────────────────────┘
```

**不变量**:
1. 前缀每会话计算一次，哈希后固定不变
2. 日志条目按追加顺序序列化，不可重写
3. 草稿区信息需经蒸馏后才能写入日志

### 5.2 工具调用修复流水线

专门处理 DeepSeek 已知的失败模式：

```
工具调用响应
  │
  ├── flatten      → 深层嵌套参数扁平化（参数>10 或 深度>2）
  ├── scavenge     → 从 reasoning_content 回收遗漏的工具调用
  ├── truncation   → 检测截断 JSON，补全括号
  └── storm        → 滑动窗口检测重复调用，抑制风暴
```

### 5.3 Cycle 无限会话（借鉴 MiMo-Code）

```
逻辑会话（无上限）
  Cycle 1 ──→ Cycle 2 ──→ Cycle 3 ──→ ...
      │           │           │
      ▼           ▼           ▼
  checkpoint  checkpoint  checkpoint
      │           │           │
      ▼           ▼           ▼
  rebuild     rebuild     rebuild
  (上下文重建) (上下文重建) (上下文重建)
```

- 提前提取策略：在 20%、45%、70% 窗口处触发 checkpoint
- 独立 Writer 子代理维护状态，解耦主推理循环
- Token 预算控制，按重要性排序注入记忆内容

---

## 6. 记忆系统架构

### 6.1 四层记忆体系

```
精炼度 ↑
  ┌──────────────────┐
  │ checkpoint.md    │  ← 会话级，结构化状态快照
  ├──────────────────┤
  │ MEMORY.md        │  ← 项目级，跨会话持久化知识
  ├──────────────────┤
  │ 全局记忆          │  ← 跨项目用户偏好与规则
  ├──────────────────┤
  │ SQLite 历史记录   │  ← 完整原始对话文本 (FTS5 索引)
  └──────────────────┘
```

### 6.2 存储方案

| 层级 | 存储方式 | 用途 |
|------|---------|------|
| checkpoint.md | 文件系统 | 当前会话状态快照 |
| MEMORY.md | 文件系统 | 项目知识、架构决策、规则 |
| 全局记忆 | `~/.helix/memory.db` | 用户偏好、跨项目规则 |
| SQLite 历史 | `~/.helix/sessions.db` | FTS5 全文搜索原始对话 |

### 6.3 Dream & Distill 机制

```
┌──────────────────┐     ┌──────────────────┐
│     Dream        │     │     Distill       │
��  (每7天自动触发)  │     │  (每30天自动触发)  │
├──────────────────┤     ├──────────────────┤
│ 扫描会话轨迹      │     │ 识别重复工作流     │
│ 提取持久知识      │     │ 打包为可复用组件   │
│ 更新 MEMORY.md   │     │ 生成 skill/command │
│ 清理过时条目      │     │ 注册到工具系统     │
└──────────────────┘     └──────────────────┘
```

---

## 7. 工具系统架构

### 7.1 工具接口

```go
type Tool interface {
    Name() string
    Description() string
    Schema() ToolSchema          // JSON Schema for LLM tool_call
    Execute(ctx context.Context, args map[string]any) (*ToolResult, error)
    IsReadOnly() bool            // 用于并行调度
}
```

### 7.2 工具注册中心

```go
type ToolRegistry struct {
    tools map[string]Tool
}

func (r *ToolRegistry) Register(t Tool)
func (r *ToolRegistry) Get(name string) (Tool, bool)
func (r *ToolRegistry) List() []Tool
func (r *ToolRegistry) Schemas() []ToolSchema
```

### 7.3 工具执行引擎

```
ToolExecutor.Execute(toolCalls []ToolCall)
  │
  ├── 1. 分区 (Partition)
  │     ├── 连续只读工具 → ReadGroup₁
  │     └── 写工具        → WriteSeq₁, WriteSeq₂
  │
  ├── 2. 执行守卫链 (每个工具独立)
  │     ├── 工具存在检查
  │     ├── 重复成功阻断（同一写操作 ≥2 次）
  │     ├── Plan 模式阻断（非只读工具）
  │     ├── 权限门控（用户确认）
  ���     └── 沙箱执行
  │
  ├── 3. 并行调度
  │     ├── ReadGroup 内工具并行执行
  │     └── WriteSeq 间串行，与其他并行
  │
  └── 4. 结果收集与截断（单结果最大 32KB）
```

### 7.4 基础工具清单

| 工具名 | 类型 | 说明 |
|--------|------|------|
| `read_file` | 只读 | 读取文件内容 |
| `write_file` | 写入 | 创建或覆盖文件 |
| `edit_file` | 写入 | 精确字符串替换 |
| `bash` | 写入 | 执行 Shell 命令 |
| `grep` | 只读 | 内容搜索（ripgrep） |
| `glob` | 只读 | 文件模式匹配 |
| `web_search` | 只读 | 网络搜索（多引擎） |
| `web_fetch` | 只读 | 抓取网页内容 |
| `git_status` | 只读 | Git 状态查询 |
| `git_diff` | 只读 | Git 差异查看 |
| `memory_read` | 只读 | 读取记忆 |
| `memory_write` | 写入 | 写入记忆 |

---

## 8. 配置系统架构

### 8.1 配置优先级

```
CLI 标志 (-p deepseek)       ← 最高优先级
    │
./helix.toml                 ← 项目级配置
    │
~/.helix/config.toml         ← 用户级配置
    │
内置��认值                    ← 最低优先级
```

### 8.2 配置结构

```toml
# 默认 Provider
default_provider = "deepseek"

# Provider 定义（可无限扩展）
[[providers]]
name         = "deepseek"
display_name = "DeepSeek"
kind         = "deepseek"
base_url     = "https://api.deepseek.com"
api_key_env  = "DEEPSEEK_API_KEY"

  [[providers.models]]
  id             = "deepseek-v4-flash"
  name           = "DeepSeek V4 Flash"
  cost           = { input = 0.14, cached_input = 0.014, output = 0.28 }
  context_window = 131072

# 插件 (MCP 服务器)
[[plugins]]
name    = "my-tool"
command = "node"
args    = ["./mcp-server.js"]

# 权限
[permissions]
shell_allowlist = ["git", "npm", "go", "ls", "cat"]

# 搜索
[search]
engine = "bing"  # bing|baidu|searxng|tavily|perplexity

# 实验性功能
[experimental]
maxMode      = false
batchTool    = false
```

---

## 9. 数据流

### 9.1 单次任务数据流

```
用户输入 "读取 main.go 并解释"
    │
    ▼
CLI 入口 (cmd/helix/main.go)
    │ 解析参数、加载配置
    ▼
Config System
    │ 选择 Provider、模型
    ▼
Agent Loop (agent/loop.go)
    │ 构建消息上下文
    ▼
Context Manager (context/partition.go)
    │ 注入不可变前缀 + 工具 Schema
    ▼
Provider.Stream() (provider/deepseek/provider.go)
    │ SSE 流式推理
    ▼
模型响应 (reasoning_content + tool_calls)
    │
    ├─ 有 tool_calls → Tool Repair Pipeline → Tool Executor → 结果追加 → 继续循环
    │
    └─ 无 tool_calls → 就绪检查 → 流式输出最终答案 → 返回
```

### 9.2 会话持久化

```
Session Start
    │
    ├── 创建 JSONL 文件 (sessions/<id>.jsonl)
    ├── 每轮追加: [user, assistant, tool_call, tool_result]
    ├── 定期 checkpoint (20%/45%/70% 窗口)
    │     └── Writer 子代理写入 checkpoint.md
    └── Session End
          └── Dream Agent 异步提取知识到 MEMORY.md
```

---

## 10. 安全架构

### 10.1 多层守卫

```
工具调用请求
  │
  ├── 守卫1: 工具存在检查         → 未知工具直接拒绝
  ├── 守卫2: 重复成功阻断         → 同一写操作 ≥2 次成功 → 阻止
  ├── 守卫3: Plan 模式阻断        → Plan Agent 禁止写工具
  ├── 守卫4: 权限门控             → 敏感操作需用户确认
  ├── 守卫5: 风暴检测             → 连续3次相同失败 → 注入反思提示
  └── 守卫6: 沙箱执行             → 隔离执行环境
```

### 10.2 编辑门控

| 模式 | 行为 | 适用场景 |
|------|------|---------|
| **review** | 每个编辑块弹出确认弹窗 | 谨慎模式（默认） |
| **auto** | 自动应用，5秒内可撤销 | 信任模式 |
| **yolo** | 跳过所有确认 | 完全自主（需显式启用） |

### 10.3 权限白名单

```toml
[permissions]
# Bash 命令白名单（精细到参数级别）
shell_allowlist = [
    "git status",
    "git diff",
    "git log",
    "npm test",
    "go build",
    "go test",
    "ls",
    "cat",
    "find . -name",     # 允许 find -name，拒绝 find -exec
]
```

---

## 11. 技术选型理由

| 技术 | 理由 |
|------|------|
| **Go** | 单二进制分发、零依赖部署、高性能、Reasonix 验证 |
| **TOML** | 比 JSON 更人性化、支持注释、Reasonix 验证 |
| **Bubble Tea** | Go 生态最成熟 TUI 框架、Elm 架构 |
| **SQLite FTS5** | 全文搜索、嵌入式、零运维、MiMo 验证 |
| **MCP** | 标准插件协议、stdio+HTTP 双传输、生态成熟 |
| **OpenAI 协议** | 事实标准、DeepSeek/MiMo 原生兼容 |
| **GoReleaser** | 多平台交叉编译、自动发布 |

---

## 12. 部署架构

```
┌─────────────────────────────────────────────┐
│                  分发渠道                     │
├──────────────┬──────────────┬───────────────┤
│  curl 安装    │   Homebrew   │  GitHub Release│
│  install.sh  │   formula    │  预编译二进制   │
└──────┬───────┴──────┬───────┴──────┬────────┘
       │              │              │
       ▼              ▼              ▼
┌─────────────────────────────────────────────┐
│           CGO_ENABLED=0 静态二进制            │
│  darwin/amd64  darwin/arm64                  │
│  linux/amd64   linux/arm64                   │
│  windows/amd64 windows/arm64                 │
└─────────────────────────────────────────────┘
```

---

## 附录: 与参考项目的架构对比

| 维度 | DeepSeek-Reasonix | MiMo-Code | Helix |
|------|-------------------|-----------|-------|
| **语言** | Go (v2) | TypeScript + Bun | Go |
| **配置** | TOML | JSON/JSONC | TOML |
| **Provider** | 配置驱动 | 配置驱动 | Adapter 工厂模式 |
| **Agent模式** | 单/双模型 | Build/Plan/Compose | Build/Plan/Compose/Max |
| **记忆** | 文件层级 | SQLite FTS5 | SQLite FTS5 + 文件层级 |
| **上下文** | 三层分区(缓存优先) | Cycle(checkpoint) | 三层分区 + Cycle |
| **工具修复** | 四道流水线 | 受限命令行语法 | Capability自适应 |
| **TUI** | 自研 | 自研 | Bubble Tea |
| **分发** | npm/Homebrew/二进制 | curl/npm | curl/Homebrew/二进制 |
| **许可证** | MIT | MIT | MIT |
