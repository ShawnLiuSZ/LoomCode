# Provider 接口参考

> 本文档描述如何开发自定义 Provider 适配器。

## 接口定义

```go
type Provider interface {
    Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    Stream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error)
    Name() string
    Models() []ModelInfo
    Capabilities() Capabilities
    Cost(modelID string, usage Usage) Cost
}

type Adapter interface {
    Kind() string
    Create(cfg Config) (Provider, error)
    ValidateConfig(cfg Config) error
}
```

## 开发步骤

1. 实现 `Adapter` 接口
2. 实现 `Provider` 接口
3. 在 `createProvider` 中注册

## 参考实现

- `internal/provider/openai/` - OpenAI 适配器
- `internal/provider/deepseek/` - DeepSeek 适配器
- `internal/provider/mimo/` - MiMo 适配器

---

*此文档为存根页，完整内容待补充。*
