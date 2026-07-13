# 开发路线图

> LoomCode CLI 功能演进计划

**最后更新**: 2026-06-18
**版本**: v0.1.0 → v0.2.0 规划

---

## 总览

基于 [功能完整性审查](feature-completeness-review.md) 和 [审查建议](review-suggestions.md)，制定以下开发路线图：

```
v0.1.0 (当前) ──→ v0.1.1 ──→ v0.1.2 ──→ v0.2.0 ──→ v0.3.0
     │              │           │           │           │
     └── 核心框架    └── 安全修复  └── 高级功能  └── 生态扩展  └── 平台化
```

---

## v0.1.1 - 安全与稳定性（1 周）

### 目标
修复安全漏洞，提升系统稳定性

### 任务清单

| 任务 | 优先级 | 预计时间 | 状态 |
|------|--------|----------|------|
| API Key 安全修复 | P0 | 2h | ✅ 完成 |
| 命令执行沙箱 | P0 | 4h | ✅ 完成 |
| 并发安全修复 | P0 | 2h | ✅ 完成 |
| 错误处理统一 | P1 | 3h | ✅ 完成 |
| 资源泄漏修复 | P1 | 2h | ✅ 完成 |

### 详细说明

#### API Key 安全修复
- 轮换已泄露的 API Key
- 创建 `.env.example` 模板
- 清理 Git 历史中的密钥

#### 命令执行沙箱
```go
// 在执行前调用权限检查
func (t *BashTool) Execute(ctx context.Context, args map[string]any) (*Result, error) {
    command := args["command"].(string)
    
    // 检查权限
    if allowed, reason := t.permission.Check("bash", args); !allowed {
        return nil, fmt.Errorf("blocked: %s", reason)
    }
    
    // 执行命令
    // ...
}
```

#### 并发安全修复
- 使用 `sync.Mutex` 保护 `streamBuf`
- 使用 `atomic` 操作计数器

---

## v0.1.2 - 核心功能补全（2 周）

### 目标
实现 MiMo-Code 和 Reasonix 的核心差异化功能

### 任务清单

| 任务 | 来源 | 预计时间 | 状态 |
|------|------|----------|------|
| Goal / Stop Condition | MiMo-Code | 2d | ✅ 完成 |
| Dream & Distill | MiMo-Code | 2d | ✅ 完成 |
| SEARCH/REPLACE → /apply | Reasonix | 1d | ✅ 完成 |
| Hooks 系统 | Reasonix | 1d | ✅ 完成 |

### 详细说明

#### Goal / Stop Condition
```
/goal "实现用户认证模块"

# Agent 执行任务，直到 judge 模型判定目标达成
# 防止 "乐观停止"
```

**实现方案**:
```go
type Goal struct {
    Condition string
    Judge     provider.Provider
}

func (g *Goal) Evaluate(messages []Message) bool {
    prompt := fmt.Sprintf(`
    Goal: %s
    
    Conversation: %s
    
    Has the goal been achieved? Answer ACHIEVED or NOT_ACHIEVED.
    `, g.Condition, formatMessages(messages))
    
    resp, _ := g.Judge.Chat(context.Background(), &provider.ChatRequest{
        Messages: []Message{{Role: "user", Content: prompt}},
    })
    
    return strings.Contains(resp.Content, "ACHIEVED")
}
```

#### Dream & Distill
```
/dream    # 扫描最近会话，提取知识
/distill  # 发现重复工作流，打包为 skills
```

**实现方案**:
```go
func (d *DreamScheduler) Run() error {
    // 1. 收集最近会话
    sessions := d.sessionMgr.Recent(7 * 24 * time.Hour)
    
    // 2. 分析模式
    patterns := d.analyzePatterns(sessions)
    
    // 3. 提取知识
    knowledge := d.extractKnowledge(patterns)
    
    // 4. 保存到 MEMORY.md
    return d.saveToMemory(knowledge)
}
```

#### SEARCH/REPLACE → /apply
```
# Agent 提议编辑
SEARCH: func oldFunc() {
REPLACE: func newFunc() {
    // 新代码
}

# 用户确认
/apply  # 应用所有更改
/reject # 拒绝所有更改
```

#### Hooks 系统
```toml
# loomcode.toml
[hooks]
pre_tool_use = "echo 'Executing: {{.ToolName}}'"
post_tool_use = "echo 'Completed: {{.ToolName}}'"
stop = "echo 'Session ended'"
```

---

## v0.2.0 - 高级功能（1 月）

### 目标
实现 Web 监控、搜索集成、语义索引

### 任务清单

| 任务 | 来源 | 预计时间 | 状态 |
|------|------|----------|------|
| Web Dashboard | Reasonix | 5d | ✅ 完成 |
| Configurable Web Search | Reasonix | 2d | ✅ 完成 |
| Semantic Index | Reasonix | 2d | ✅ 完成 |
| /effort knob | Reasonix | 1d | ✅ 完成 |
| Transcript replay | Reasonix | 1d | ✅ 完成 |
| Event log | Reasonix | 1d | ✅ 完成 |
| MCP HTTP SSE | MiMo-Code | 2d | ✅ 完成 |
| LSP 自动发现 | MiMo-Code | 2d | ✅ 完成 |

### 详细说明

#### Web Dashboard
```
┌─────────────────────────────────────────────────────────────┐
│ LoomCode Dashboard                                    [Settings]│
├─────────────────────────────────────────────────────────────┤
│ Sessions          │ Cost Analysis           │ Provider Status│
│ ┌───────────────┐ │ ┌─────────────────────┐ │ ┌────────────┐│
│ │ Session 1     │ │ │ $0.12 ▲ +$0.03      │ │ │ DeepSeek ✅ ││
│ │ 10 min ago    │ │ │ ████████████░░░░░░  │ │ │ MiMo ✅    ││
│ │ 15 messages   │ │ │ ████████████████░░  │ │ │ OpenAI ✅  ││
│ └───────────────┘ │ └─────────────────────┘ │ └────────────┘│
└─────────────────────────────────────────────────────────────┘
```

**技术栈**:
- 后端: Go + Fiber
- 前端: React + TailwindCSS
- 实时: WebSocket

#### Configurable Web Search
```
/search-engine bing      # 切换到 Bing
/search-engine tavily    # 切换到 Tavily
/search-engine searxng   # 切换到 SearXNG
```

**支持的搜索引擎**:
| 引擎 | 配置 |
|------|------|
| Bing | `BING_API_KEY` |
| Baidu | `BAIDU_API_KEY` |
| SearXNG | `SEARXNG_ENDPOINT` |
| Tavily | `TAVILY_API_KEY` |
| Perplexity | `PERPLEXITY_API_KEY` |
| Exa | `EXA_API_KEY` |
| Brave | `BRAVE_API_KEY` |

#### Semantic Index
```go
type SemanticIndex struct {
    embeddings EmbeddingProvider
    store      vectorstore.VectorStore
}

func (s *SemanticIndex) Search(query string, topK int) ([]Document, error) {
    // 1. 生成查询向量
    queryVec, _ := s.embeddings.Embed(query)
    
    // 2. 向量搜索
    results, _ := s.store.Search(queryVec, topK)
    
    return results, nil
}
```

#### /effort knob
```
/effort low    # 快速响应，适合简单任务
/effort medium # 平衡模式（默认）
/effort high   # 深度思考，适合复杂推理
```

**实现**:
```go
func (a *Agent) SetEffort(level EffortLevel) {
    switch level {
    case EffortLow:
        a.maxSteps = 5
        a.reasoningEffort = "low"
    case EffortMedium:
        a.maxSteps = 10
        a.reasoningEffort = "medium"
    case EffortHigh:
        a.maxSteps = 20
        a.reasoningEffort = "high"
    }
}
```

---

## v0.3.0 - 平台化（2-3 月）

### 目标
实现分布式协作、桌面客户端、IDE 集成

### 任务清单

| 任务 | 预计时间 | 状态 |
|------|----------|------|
| Multi Provider 负载均衡 | 3d | ✅ 完成 |
| 分布式 Agent 协作 | 1w | ✅ 完成 |
| Desktop Client (Tauri) | 2w | ⬜ |
| VS Code 扩展 | 2w | ⬜ |

### 详细说明

#### Multi Provider 负载均衡
```go
type LoadBalancer struct {
    providers []Provider
    strategy  Strategy
    metrics   *Metrics
}

func (lb *LoadBalancer) Select() Provider {
    switch lb.strategy {
    case RoundRobin:
        return lb.roundRobin()
    case WeightedRandom:
        return lb.weightedRandom()
    case LeastLatency:
        return lb.leastLatency()
    case CostOptimized:
        return lb.costOptimized()
    }
    return lb.providers[0]
}
```

#### 分布式 Agent 协作
```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Agent 1    │────▶│   Agent 2    │────▶│   Agent 3    │
│  (Research)  │     │  (Implement) │     │   (Review)   │
└──────────────┘     └──────────────┘     └──────────────┘
        │                   │                   │
        └───────────────────┼───────────────────┘
                            ▼
                    ┌──────────────┐
                    │   Aggregator │
                    └──────────────┘
```

#### Desktop Client (Tauri)
- 多标签页支持
- 文件树侧边栏
- 实时成本/缓存监控
- 与 CLI 共享配置

#### VS Code 扩展
- 代码补全
- 内联建议
- 快捷命令面板
- 错误诊断

---

## 里程碑

### M1: 安全修复完成
**日期**: 2026-06-25
**交付物**:
- [ ] 所有 P0 安全问题修复
- [ ] 测试覆盖率 > 80%

### M2: 核心功能补全
**日期**: 2026-07-09
**交付物**:
- [ ] Goal/Stop Condition
- [ ] Dream & Distill
- [ ] /apply review
- [ ] Hooks 系统

### M3: 高级功能完成
**日期**: 2026-08-06
**交付物**:
- [ ] Web Dashboard
- [ ] Web Search 集成
- [ ] Semantic Index
- [ ] /effort knob

### M4: 平台化完成
**日期**: 2026-09-03
**交付物**:
- [ ] Desktop Client
- [ ] VS Code 扩展
- [ ] 分布式 Agent

---

## 资源需求

### 人力

| 阶段 | 开发 | 测试 | 设计 |
|------|------|------|------|
| v0.1.1 | 1 人 | 0.5 人 | 0 |
| v0.1.2 | 1 人 | 0.5 人 | 0 |
| v0.2.0 | 2 人 | 1 人 | 0.5 人 |
| v0.3.0 | 3 人 | 1 人 | 1 人 |

### 基础设施

| 资源 | 用途 | 成本 |
|------|------|------|
| GitHub Actions | CI/CD | 免费 |
| Vercel | Dashboard 托管 | $20/月 |
| OpenAI API | 测试 | $50/月 |
| DeepSeek API | 测试 | $20/月 |

---

## 风险管理

| 风险 | 影响 | 概率 | 缓解措施 |
|------|------|------|----------|
| API 变更 | 高 | 中 | 监控官方文档，及时适配 |
| 安全漏洞 | 高 | 中 | 定期安全审计 |
| 性能瓶颈 | 中 | 低 | 压力测试，性能优化 |
| 人力不足 | 中 | 中 | 外包非核心模块 |

---

## 成功指标

### v0.1.1
- [ ] 0 个 P0 安全漏洞
- [ ] 测试覆盖率 > 80%
- [ ] 并发安全问题清零

### v0.1.2
- [ ] Goal/Stop Condition 可用
- [ ] Dream & Distill 可用
- [ ] /apply review 可用

### v0.2.0
- [ ] Web Dashboard 可用
- [ ] 3+ 搜索引擎集成
- [ ] 语义搜索可用

### v0.3.0
- [ ] Desktop Client 发布
- [ ] VS Code 扩展发布
- [ ] 分布式 Agent 原型

---

*文档生成时间: 2026-06-18*

---

## v0.4.0 - 生态与优化（1 月）

### 目标
性能优化、插件系统完善、CI/CD 完善、文档示例

### 任务清单

| 任务 | 预计时间 | 状态 |
|------|----------|------|
| 性能优化（缓存、并发） | 3d | ✅ 完成 |
| 插件系统完善 | 2d | ✅ 完成 |
| CI/CD 流水线 | 2d | ✅ 完成 |
| 示例项目和文档 | 2d | ✅ 完成 |
| 错误处理统一 | 1d | ✅ 完成 |

### 详细说明

#### 性能优化
- 响应缓存（Redis/内存）
- 连接池优化
- 并发请求优化
- 内存使用优化

#### 插件系统完善
- 插件生命周期管理
- 插件依赖解析
- 插件版本管理
- 插件市场接口

#### CI/CD 流水线
- GitHub Actions 配置
- 自动化测试
- 自动化发布
- 代码质量检查

#### 示例项目和文档
- 快速入门示例
- API 使用示例
- 插件开发指南
- 架构设计文档
