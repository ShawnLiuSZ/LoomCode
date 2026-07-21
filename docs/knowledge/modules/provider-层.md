---
tags:
  - 核心模块
  - Provider
  - 模型接入
created: 2026-07-14
updated: 2026-07-14
aliases:
  - Provider层
  - 模型适配
---

# Provider 层

> 🔌 多厂商模型接入、能力声明、成本计算
> 📅 最后更新：2026-07-14

---

## 概述

Provider 层采用 **Adapter 工厂模式**，将不同 LLM 厂商的 API 差异封装在适配器中。任何 OpenAI 兼容厂商通过 JSON 配置即可接入，无需修改代码。

**代码路径**：`internal/provider/`

## 关键文件

| 文件 | 职责 |
|------|------|
| `provider.go` | `Provider` 接口 + `Adapter` 接口定义 |
| `registry.go` | 适配器注册中心 |
| `capabilities.go` | `Capabilities` 能力声明 |
| `config.go` | Provider 配置结构 |
| `model.go` | 模型信息 |
| `message.go` | 消息结构（ChatRequest/ChatResponse/StreamEvent） |
| `loadbalancer.go` | 负载均衡 |
| `retry.go` | 重试机制 |
| `oauth.go` | OAuth 认证 |
| `sse.go` | SSE 流式解析 |
| `deepseek/provider.go` | DeepSeek V4 适配器 |
| `mimo/provider.go` | MiMo V2.5 适配器 |
| `openai/provider.go` | 通用 OpenAI 兼容适配器 |

## Provider 接口

```go
type Provider interface {
    Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    Stream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error)
    Name() string
    Models() []ModelInfo
    Capabilities() Capabilities
    Cost(modelID string, usage Usage) Cost
}
```

详见：[[../interfaces/provider-接口|Provider 接口]]

## Capabilities 能力声明

```go
type Capabilities struct {
    SupportsReasoning    bool          // 支持 reasoning_content
    SupportsToolCall     bool          // 支持原生工具调用
    SupportsPrefixCache  bool          // 支持前缀缓存
    SupportsStreaming    bool          // 支持流式输出
    SupportsVision       bool          // 支持图片输入
    SupportsVoice        bool          // 支持语音输入
    SupportsOAuth        bool          // 支持 OAuth 认证
    NeedsToolRepair      bool          // 需要工具调用修复
    CacheTTL             time.Duration // 缓存有效期
    MaxToolCallsPerRound int           // 单轮最大工具调用数
}
```

**核心设计**：Agent 行为由 `Capabilities` 驱动，而非 if-else 判断厂商。

## 内置适配器

| 适配器 | Kind | 特性 |
|--------|------|------|
| `deepseek.Adapter` | `"deepseek"` | Prefix Cache、工具修复、reasoning_content |
| `mimo.Adapter` | `"mimo"` | OAuth、语音 ASR、Prefix Cache |
| `openai.Adapter` | `"openai"` | 通用 OpenAI 兼容协议 |

## 新增 Provider（配置驱动，无需写代码）

```json
// settings.json 或 models.json 的 providers 数组
{
  "providers": [
    {
      "name": "qwen",
      "display_name": "通义千问",
      "kind": "openai",
      "base_url": "https://dashscope.aliyuncs.com/compatible-mode/v1",
      "api_key": "${DASHSCOPE_API_KEY}",
      "default_model": "qwen-max",
      "models": [
        {
          "id": "qwen-max",
          "name": "Qwen Max",
          "context_window": 32768
        }
      ]
    }
  ]
}
```

## 配置结构

| 字段 | 说明 |
|------|------|
| `name` | Provider 唯一标识 |
| `display_name` | 显示名称 |
| `kind` | 适配器类型（deepseek/mimo/openai） |
| `base_url` | API 基础 URL |
| `api_key_env` | API Key 环境变量名 |
| `default_model` | 默认模型 ID |
| `auth_method` | 认证方式 |
| `models[]` | 模型列表（含成本、能力、上下文窗口） |

## 相关文档

- [[../interfaces/provider-接口|Provider 接口]] — 完整接口定义
- [[agent-引擎|Agent 引擎]] — 消费 Provider 能力
- [[config-系统|配置系统]] — Provider 配置加载
- [[../architecture/架构总览|架构总览]]
