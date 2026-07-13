# 架构概览

> LoomCode 的系统整体设计。

## 分层架构

```
┌─────────────────────────────────┐
│         TUI (Bubble Tea)        │  用户交互层
├─────────────────────────────────┤
│       Agent (MultiAgent)        │  任务执行层
├─────────────────────────────────┤
│    Provider (OpenAI/DeepSeek)   │  模型接入层
├─────────────────────────────────┤
│      Tool (Executor/Registry)   │  工具执行层
├─────────────────────────────────┤
│  Memory (Store/SemanticIndex)   │  记忆存储层
└─────────────────────────────────┘
```

## 核心组件

| 组件 | 包 | 职责 |
|------|-----|------|
| TUI | `internal/ui` | 用户交互、消息渲染 |
| Agent | `internal/agent` | 任务编排、多模式支持 |
| Provider | `internal/provider` | 模型 API 接入 |
| Tool | `internal/tool` | 工具注册与执行 |
| Session | `internal/session` | 会话持久化 |
| Skills | `internal/skills` | 扩展技能管理 |
| Control | `internal/control` | 权限与门控 |

---

*此文档为存根页，完整内容待补充。*
