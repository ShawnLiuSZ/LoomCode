# LoomCode CLI 功能完整性审查

> 审查日期: 2026-06-18  
> 版本: v0.1.0  
> 参考项目: [MiMo-Code](https://github.com/XiaomiMiMo/MiMo-Code) | [DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix)

---

## 一、核心功能完成度

| 模块 | 功能 | 状态 | 测试 |
|------|------|------|------|
| **Provider** | OpenAI 通用适配器 | ✅ 完成 | 10 |
| | DeepSeek V4 适配器 | ✅ 完成 | 11 |
| | MiMo V2.5 适配器 | ✅ 完成 | 11 |
| **Agent** | 推理循环 | ✅ 完成 | 9 |
| | Build/Plan/Compose/Max 模式 | ✅ 完成 | 8 |
| | 子 Agent 系统 | ✅ 完成 | 10 |
| **Tool** | 文件读写/命令执行/搜索 | ✅ 完成 | 42 |
| | 工具修复流水线 | ✅ 完成 | 12 |
| | 并行调度 | ✅ 完成 | 10 |
| **Config** | TOML 配置 | ✅ 完成 | 5 |
| | .env 加载 | ✅ 完成 | 17 |
| **Context** | 三层分区 | ✅ 完成 | 8 |
| **Control** | 编辑门控 | ✅ 完成 | 16 |
| | 成本控制 | ✅ 完成 | 9 |
| **Memory** | SQLite FTS5 | ✅ 完成 | 15 |
| **Session** | JSONL 持久化 | ✅ 完成 | 10 |
| **MCP** | stdio 客户端 | ✅ 完成 | 14 |
| **LSP** | 协议客户端 | ✅ 完成 | 11 |
| **Voice** | ASR 接口 | ⚠️ 接口定义 | 10 |
| **Skills** | 自动加载 | ✅ 完成 | 5 |
| **CLI** | 入口/参数 | ✅ 完成 | 4 |
| **TUI** | 交互界面 | ✅ 完成 | - |

**总计: 235 测试，全部通过**

---

## 二、与官方项目功能对比

### MiMo-Code 功能对比

| 功能 | MiMo-Code | LoomCode CLI | 状态 |
|------|-----------|-----------|------|
| Multiple Agents (build/plan/compose) | ✅ | ✅ | 已实现 |
| Persistent Memory (SQLite FTS5) | ✅ | ✅ | 已实现 |
| Intelligent Context Management | ✅ 自动检查点、上下文重建、预算注入 | ⚠️ 基础分区 | 需完善 |
| Task Tracking (T1, T1.1, T1.2...) | ✅ 树形任务系统 | ❌ | 待实现 |
| Subagent System | ✅ 并行执行 | ✅ | 已实现 |
| **Goal / Stop Condition** | ✅ `/goal` 命令 + 独立 judge | ❌ | 待实现 |
| Compose Mode (规格驱动) | ✅ 内置 skills | ⚠️ 基础实现 | 需完善 |
| **Voice Input** | ✅ 实时流式 (TenVAD + MiMo ASR) | ⚠️ 接口定义 | 需实现 |
| **Dream & Distill** | ✅ 知识蒸馏 + 技能发现 | ❌ | 待实现 |
| Claude-format skills 兼容 | ✅ | ❌ | 待实现 |

### DeepSeek-Reasonix 功能对比

| 功能 | Reasonix | LoomCode CLI | 状态 |
|------|----------|-----------|------|
| **Cache-first Loop** | ✅ 99.82% cache hit | ⚠️ 基础 TTL | 需优化 |
| **Tool-call Repair** | ✅ | ⚠️ 基础实现 | 需完善 |
| Cost Control | ✅ | ✅ | 已实现 |
| **SEARCH/REPLACE → /apply review** | ✅ 编辑预览 | ❌ | 待实现 |
| **Web Dashboard** | ✅ 嵌入式 | ❌ | 待实现 |
| **Configurable Web Search** | ✅ Bing/Baidu/SearXNG/Tavily | ❌ | 待实现 |
| Persistent Sessions | ✅ per-workspace | ✅ | 已实现 |
| **Hooks 系统** | ✅ PreToolUse/PostToolUse | ❌ | 待实现 |
| Skills/Memory | ✅ | ✅ | 已实现 |
| **Semantic Index** | ✅ 语义索引 | ❌ | 待实现 |
| **/effort knob** | ✅ 思考强度控制 | ❌ | 待实现 |
| **Transcript replay** | ✅ 会话重放 | ❌ | 待实现 |
| **Event log** | ✅ 事件日志 | ❌ | 待实现 |
| **QQ Channel** | ✅ 远程通道 | ❌ | 待实现 |
| Desktop Client (Tauri) | ✅ | ❌ | 待实现 |

---

## 三、待实现功能清单

### 🔴 P0 - 核心竞争力（立即实现）

#### 1. Goal / Stop Condition
**来源**: MiMo-Code  
**描述**: `/goal` 命令设置停止条件，独立 judge 模型评估是否真正完成
**状态**: ✅ 已实现

```go
// agent/goal.go
type GoalStopCondition struct {
    goal    string
    judge   provider.Provider
    enabled bool
}

func (g *GoalStopCondition) Evaluate(ctx context.Context, messages []Message) (bool, string, error) {
    // 调用 judge 模型评估
    prompt := g.buildEvalPrompt(g.goal, messages)
    resp, _ := g.judge.Chat(ctx, &ChatRequest{
        Messages: []Message{
            {Role: "system", Content: judgeSystemPrompt},
            {Role: "user", Content: prompt},
        },
    })
    return g.parseEvaluation(resp.Content)
}
```

**TUI 命令**:
- `/goal "实现用户认证模块"` - 设置停止条件
- `/goal` - 显示当前停止条件
- `/goal clear` - 清除停止条件

**影响文件**: `internal/agent/goal.go` (新建), `internal/agent/loop.go`, `internal/agent/modes.go`, `internal/ui/app.go`

---

#### 2. Dream & Distill
**来源**: MiMo-Code  
**描述**: 知识蒸馏和技能自动发现
**状态**: ✅ 已实现

```go
// agent/dream.go
type DreamScheduler struct {
    agent      *Agent
    sessionMgr *session.Manager
    memoryDir  string
}

func (d *DreamScheduler) RunDream() error {
    // 1. 收集最近会话
    // 2. 分析模式
    // 3. 提取知识
    // 4. 保存到 MEMORY.md
}
```

**功能**:
- `/dream` - 扫描最近会话，提取知识到项目记忆
- `/distill` - 发现重复工作流，打包为 skills

**影响文件**: `internal/agent/dream.go` (新建)

---

#### 3. SEARCH/REPLACE → /apply
**来源**: DeepSeek-Reasonix  
**描述**: 编辑文件时显示 diff 预览，用户确认后才应用
**状态**: ✅ 已实现

```go
// tool/review.go
type ReviewTool struct {
    pending []PendingEdit
    enabled bool
}

func (t *ReviewTool) Apply(id int) error {
    // 用户确认后应用编辑
}

func (t *ReviewTool) Preview(edit PendingEdit) string {
    // 生成 unified diff 预览
}
```

**功能**:
- 编辑文件时自动进入预览模式
- `/apply` - 应用所有编辑
- `/apply <id>` - 应用指定编辑
- `/reject` - 拒绝所有编辑

**影响文件**: `internal/tool/review.go` (新建)

---

#### 4. Hooks 系统
**来源**: DeepSeek-Reasonix  
**描述**: 生命周期钩子，支持自定义脚本
**状态**: ✅ 已实现

```go
// agent/hooks.go
type HookManager struct {
    hooks map[HookType][]Hook
}

type HookType int
const (
    HookPreToolUse HookType = iota    // 工具执行前
    HookPostToolUse                    // 工具执行后
    HookUserPromptSubmit               // 用户输入后
    HookStop                           // Agent 停止时
)
```

**配置示例**:
```toml
[hooks]
pre_tool_use = "echo 'Executing: {{.ToolName}}'"
post_tool_use = "echo 'Completed: {{.ToolName}}'"
stop = "echo 'Session ended'"
```

**影响文件**: `internal/agent/hooks.go` (新建)

---

#### 2. Dream & Distill
**来源**: MiMo-Code  
**描述**: 知识蒸馏和技能自动发现

- `/dream` - 扫描最近会话，提取持久知识到项目记忆
- `/distill` - 发现重复工作流，打包为可复用 skills

```go
// agent/dream.go
type DreamScheduler struct {
    agent    *Agent
    interval time.Duration
}

func (s *DreamScheduler) Run() {
    // 1. 收集最近对话
    conversations := s.collectRecent()
    // 2. 分析成功/失败模式
    patterns := s.analyzePatterns(conversations)
    // 3. 提取知识
    knowledge := s.extractKnowledge(patterns)
    // 4. 保存到 MEMORY.md
    s.saveToMemory(knowledge)
}
```

**影响文件**: `internal/agent/dream.go` (新建), `internal/agent/distill.go` (新建)

---

#### 3. SEARCH/REPLACE → /apply review
**来源**: DeepSeek-Reasonix  
**描述**: 编辑文件时显示 diff 预览，用户确认后才应用

```go
// tool/review.go
type ReviewTool struct {
    pending []PendingEdit
}

type PendingEdit struct {
    File    string
    OldText string
    NewText string
    Diff    string
}

func (t *ReviewTool) Preview(edit PendingEdit) string {
    // 生成 unified diff
    return diff.Format(edit.OldText, edit.NewText)
}

func (t *ReviewTool) Apply(idx int) error {
    // 用户确认后应用
    edit := t.pending[idx]
    return os.WriteFile(edit.File, []byte(edit.NewText), 0644)
}
```

**影响文件**: `internal/tool/review.go` (新建), `internal/tool/file_tools.go`

---

#### 4. Hooks 系统
**来源**: DeepSeek-Reasonix  
**描述**: 生命周期钩子，支持自定义脚本

| Hook | 触发时机 | 用途 |
|------|----------|------|
| PreToolUse | 工具执行前 | 权限检查、日志 |
| PostToolUse | 工具执行后 | 结果处理、通知 |
| UserPromptSubmit | 用户输入后 | 输入过滤、增强 |
| Stop | Agent 停止时 | 清理、通知 |

```go
// agent/hooks.go
type HookManager struct {
    hooks map[HookType][]Hook
}

type HookType int
const (
    PreToolUse HookType = iota
    PostToolUse
    UserPromptSubmit
    Stop
)

type Hook interface {
    Execute(ctx HookContext) error
}

type ShellHook struct {
    Command string
}

func (h *ShellHook) Execute(ctx HookContext) error {
    cmd := exec.Command("bash", "-c", h.Command)
    cmd.Env = ctx.Env
    return cmd.Run()
}
```

**影响文件**: `internal/agent/hooks.go` (新建), `internal/tool/executor.go`

---

### 🟡 P1 - 高级功能（1-2 周）

#### 5. Web Dashboard
**来源**: DeepSeek-Reasonix  
**描述**: 嵌入式 Web 监控面板

**功能模块**:
- 实时会话监控
- 成本分析图表
- Provider 状态
- 会话历史管理
- Token 用量统计

**技术栈**:
- 后端: Go + Fiber
- 前端: React + TailwindCSS
- 实时通信: WebSocket

**影响文件**: `internal/dashboard/` (新建)

---

#### 6. Configurable Web Search
**来源**: DeepSeek-Reasonix  
**描述**: 多搜索引擎支持

| 引擎 | 说明 |
|------|------|
| Bing | 默认 |
| Baidu AI Search | 中文优化 |
| SearXNG | 自托管 |
| Tavily | API 搜索 |
| Perplexity | AI 搜索 |
| Exa | 语义搜索 |
| Brave | 隐私搜索 |

```go
// tool/websearch.go
type WebSearchTool struct {
    engine SearchEngine
}

type SearchEngine interface {
    Search(ctx context.Context, query string) ([]SearchResult, error)
}

type BingSearch struct { apiKey string }
type TavilySearch struct { apiKey string }
type SearXNGSearch struct { endpoint string }
```

**影响文件**: `internal/tool/websearch.go` (新建)

---

#### 7. Semantic Index
**来源**: DeepSeek-Reasonix  
**描述**: 语义索引搜索

```go
// memory/semantic.go
type SemanticIndex struct {
    embeddings EmbeddingProvider
    index      vectorstore.VectorStore
}

type EmbeddingProvider interface {
    Embed(ctx context.Context, text string) ([]float64, error)
}

func (s *SemanticIndex) Search(query string, topK int) ([]Document, error) {
    queryVec, _ := s.embeddings.Embed(context.Background(), query)
    return s.index.Search(queryVec, topK)
}
```

**影响文件**: `internal/memory/semantic.go` (新建)

---

#### 8. /effort knob
**来源**: DeepSeek-Reasonix  
**描述**: 思考强度控制

```
/effort low    # 快速响应
/effort medium # 平衡
/effort high   # 深度思考
```

```go
// agent/effort.go
type EffortLevel int
const (
    EffortLow EffortLevel = iota
    EffortMedium
    EffortHigh
)

func (a *Agent) SetEffort(level EffortLevel) {
    a.effort = level
    // 映射到 reasoning_effort 参数
    switch level {
    case EffortLow:
        a.reasoningEffort = "low"
    case EffortMedium:
        a.reasoningEffort = "medium"
    case EffortHigh:
        a.reasoningEffort = "high"
    }
}
```

**影响文件**: `internal/agent/effort.go` (新建), `internal/agent/loop.go`

---

#### 9. Transcript Replay
**来源**: DeepSeek-Reasonix  
**描述**: 会话重放功能

```go
// session/replay.go
type Replayer struct {
    session *Session
}

func (r *Replayer) Replay(ctx context.Context) error {
    for _, msg := range r.session.Messages {
        if msg.Role == "assistant" {
            // 重放 assistant 消息
            fmt.Println(msg.Content)
        } else if msg.Role == "tool" {
            // 显示工具调用
            fmt.Printf("[Tool: %s] %s\n", msg.ToolName, msg.Content)
        }
        time.Sleep(100 * time.Millisecond) // 模拟延迟
    }
    return nil
}
```

**影响文件**: `internal/session/replay.go` (新建)

---

#### 10. Event Log
**来源**: DeepSeek-Reasonix  
**描述**: 事件日志系统

```go
// agent/eventlog.go
type EventLog struct {
    events []Event
    mu     sync.Mutex
}

type Event struct {
    Timestamp time.Time
    Type      EventType
    Message   string
    Metadata  map[string]any
}

type EventType int
const (
    EventToolCall EventType = iota
    EventToolResult
    EventError
    EventCost
    EventCacheHit
)

func (l *EventLog) Log(event Event) {
    l.mu.Lock()
    defer l.mu.Unlock()
    event.Timestamp = time.Now()
    l.events = append(l.events, event)
}
```

**影响文件**: `internal/agent/eventlog.go` (新建)

---

### 🟢 P2 - 扩展功能（2-4 周）

#### 11. MCP HTTP SSE Transport
**来源**: MiMo-Code  
**描述**: 支持远程 MCP 服务器

```go
// mcp/sse_client.go
type SSEClient struct {
    url    string
    client *http.Client
    events chan SSEEvent
}

func (c *SSEClient) Connect(ctx context.Context) error {
    req, _ := http.NewRequestWithContext(ctx, "GET", c.url, nil)
    req.Header.Set("Accept", "text/event-stream")
    resp, err := c.client.Do(req)
    if err != nil {
        return err
    }
    go c.readStream(resp.Body)
    return nil
}
```

**影响文件**: `internal/mcp/sse_client.go` (新建)

---

#### 12. LSP 服务器自动发现
**来源**: MiMo-Code  
**描述**: 自动检测项目语言和 LSP 服务器

```go
// lsp/discovery.go
type LSPDiscovery struct {
    servers map[string]LSPServer
}

func (d *LSPDiscovery) Discover() error {
    // 扫描 PATH
    for _, path := range strings.Split(os.Getenv("PATH"), ":") {
        entries, _ := os.ReadDir(path)
        for _, entry := range entries {
            d.detectLanguage(entry.Name(), filepath.Join(path, entry.Name()))
        }
    }
    return nil
}
```

**影响文件**: `internal/lsp/discovery.go` (新建)

---

#### 13. Claude-format Skills 兼容
**来源**: MiMo-Code  
**描述**: 支持 `.claude/skills/*/SKILL.md` 格式

```go
// skills/loader.go
func (m *Manager) LoadClaudeSkills(dir string) error {
    entries, _ := os.ReadDir(dir)
    for _, entry := range entries {
        if entry.IsDir() {
            skillPath := filepath.Join(dir, entry.Name(), "SKILL.md")
            if _, err := os.Stat(skillPath); err == nil {
                m.loadSkill(skillPath, "claude")
            }
        }
    }
    return nil
}
```

**影响文件**: `internal/skills/manager.go`

---

#### 14. 语音输入硬件集成
**来源**: MiMo-Code  
**描述**: 集成 PortAudio 实现实时录音

```go
// voice/portaudio.go
/*
#cgo pkg-config: portaudio
#include <portaudio.h>
*/
import "C"

type PortAudioRecorder struct {
    stream *C.PaStream
}

func (r *PortAudioRecorder) Start() error {
    C.Pa_Initialize()
    C.Pa_OpenDefaultStream(&r.stream, 1, 0, C.paFloat32, 44100, 256, nil, nil)
    return nil
}
```

**影响文件**: `internal/voice/portaudio.go` (新建)

---

#### 15. Multi Provider 负载均衡
**来源**: 通用需求  
**描述**: 在多个 Provider 间分配请求

```go
// provider/loadbalancer.go
type LoadBalancer struct {
    providers []Provider
    strategy  Strategy
}

type Strategy int
const (
    RoundRobin Strategy = iota
    WeightedRandom
    LeastLatency
)

func (lb *LoadBalancer) Select() Provider {
    switch lb.strategy {
    case RoundRobin:
        return lb.roundRobin()
    case WeightedRandom:
        return lb.weightedRandom()
    case LeastLatency:
        return lb.leastLatency()
    }
    return lb.providers[0]
}
```

**影响文件**: `internal/provider/loadbalancer.go` (新建)

---

#### 16. 分布式 Agent 协作
**来源**: 通用需求  
**描述**: 多个 Agent 协作完成任务

```go
// agent/distributed.go
type DistributedAgent struct {
    nodeID    string
    peers     map[string]*Peer
    taskQueue chan Task
}

func (a *DistributedAgent) Distribute(task Task) error {
    peer := a.selectPeer(task)
    return a.sendTask(peer, task)
}

func (a *DistributedAgent) Aggregate(results []Result) Result {
    merged := Result{}
    for _, r := range results {
        merged.Content += r.Content + "\n"
    }
    return merged
}
```

**影响文件**: `internal/agent/distributed.go` (新建)

---

## 四、实施路线图

### Phase 1: 核心竞争力（1 周）

| 任务 | 优先级 | 预计时间 |
|------|--------|----------|
| Goal / Stop Condition | P0 | 2 天 |
| Dream & Distill | P0 | 2 天 |
| SEARCH/REPLACE → /apply | P0 | 1 天 |
| Hooks 系统 | P0 | 1 天 |

### Phase 2: 高级功能（2 周）

| 任务 | 优先级 | 预计时间 |
|------|--------|----------|
| Web Dashboard | P1 | 3 天 |
| Configurable Web Search | P1 | 2 天 |
| Semantic Index | P1 | 2 天 |
| /effort knob | P1 | 1 天 |
| Transcript replay | P1 | 1 天 |
| Event log | P1 | 1 天 |

### Phase 3: 扩展功能（2-4 周）

| 任务 | 优先级 | 预计时间 |
|------|--------|----------|
| MCP HTTP SSE transport | P2 | 2 天 |
| LSP 自动发现 | P2 | 2 天 |
| Claude-format skills | P2 | 1 天 |
| 语音硬件集成 | P2 | 3 天 |
| Multi Provider 负载均衡 | P2 | 2 天 |
| 分布式 Agent 协作 | P2 | 5 天 |

### Phase 4: 生态建设（1-2 月）

| 任务 | 优先级 | 预计时间 |
|------|--------|----------|
| Web Dashboard 完善 | P3 | 5 天 |
| VS Code 扩展 | P3 | 2 周 |
| Desktop Client (Tauri) | P3 | 2 周 |

---

## 五、功能优先级矩阵

```
                    高价值
                      │
    Goal/Stop ────────┼──────── Dream/Distill
    Hooks ────────────┼──────── /apply review
                      │
  ────────────────────┼───────────────────── 低实现成本
                      │
    Web Dashboard ────┼──────── Semantic Index
    Distributed ──────┼──────── Load Balancer
                      │
                    低价值
```

**推荐优先级**:
1. **立即**: Goal/Stop, Dream/Distill, /apply review, Hooks
2. **尽快**: Web Dashboard, Web Search, /effort, Event Log
3. **逐步**: Semantic Index, MCP SSE, LSP Discovery
4. **长期**: Distributed, Load Balancer, VS Code, Desktop

---

## 六、技术债务

### 已知问题

1. **reasoning_content 处理不完整**
   - DeepSeek 要求工具调用轮次必须回传 reasoning_content
   - 当前实现合并到 content 前缀，不符合规范

2. **工具调用修复过于简单**
   - Reasonix 有完整的 tool-call repair 流水线
   - LoomCode 仅做基础 JSON 解析修复

3. **缓存命中率未优化**
   - Reasonix 实现 99.82% cache hit
   - LoomCode 仅做基础 TTL 感知

4. **语音输入未实现**
   - MiMo-Code 有完整的 TenVAD + MiMo ASR 集成
   - LoomCode 仅定义接口，无实际实现

---

*文档生成时间: 2026-06-18*
