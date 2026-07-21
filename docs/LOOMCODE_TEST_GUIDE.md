# LoomCode 测试指南

> 配套文档: [CODEBUDDY.md](../CODEBUDDY.md) | [架构设计](./LOOMCODE_ARCHITECTURE.md)

---

## 1. 测试规范

- 测试文件与源文件同目录，命名 `*_test.go`
- 使用标准库 `testing` 包，不引入第三方测试框架
- 表驱动测试优先（table-driven tests）
- 需要 mock 外部依赖时使用 `net/http/httptest` 或接口 mock

### 构建与运行

```bash
make test                          # 运行所有测试
go test -run TestName ./internal/...  # 运行单个测试
go test -cover ./...               # 带覆盖率
go test -coverprofile=coverage.out ./...  # 生成覆盖率报告
go tool cover -html=coverage.out   # 浏览器查看覆盖率
```

---

## 2. 各模块测试要点

| 模块 | 测试重点 |
|------|---------|
| `config/` | JSON 解析正确性、配置优先级覆盖、环境变量注入、缺失必填字段报错 |
| `provider/registry.go` | Adapter 注册/去重、按 Kind 查找、未注册 Kind 返回错误 |
| `provider/openai/` | 请求构建（URL/Header/Body）、流式 SSE 解析、错误响应处理、超时重试 |
| `provider/deepseek/` | reasoning_content 提取、前缀缓存命中追踪、工具调用修复流水线 |
| `provider/mimo/` | OAuth 流程模拟、ASR 请求格式、模型列表解析 |
| `tool/registry.go` | 工具注册/去重、Schema 生成、按名称查找 |
| `tool/executor.go` | 工具执行成功/失败、并行分区正确性、守卫链阻断、结果截断 |
| `tool/repair.go` | flatten 扁平化、scavenge 回收、truncation 补全、边界情况 |
| `agent/loop.go` | 单轮对话、多轮工具调用循环、空响应重试、就绪检查 |
| `context/partition.go` | 前缀不变性、日志追加顺序、草稿重置 |
| `memory/sqlite.go` | CRUD 操作、FTS5 搜索、并发读写 |

---

## 3. 测试数据准备与清理

### 目录约定

```
internal/xxx/
├── xxx.go
├── xxx_test.go
└── testdata/           ← 测试数据目录（Go 工具链自动忽略）
    ├── config_valid.json
    ├── config_invalid.json
    ├── sample_response.json
    └── fixtures/        ← 多文件测试夹具
        ├── project/
        │   ├── main.go
        │   └── go.mod
        └── empty_dir/
```

### 文件系统测试

使用 `t.TempDir()` 自动创建和清理临时目录（Go 1.15+）：

```go
func TestReadFile(t *testing.T) {
    dir := t.TempDir()
    content := "package main\n\nfunc main() {}\n"
    filePath := filepath.Join(dir, "main.go")
    os.WriteFile(filePath, []byte(content), 0644)

    result, err := ReadFile(filePath)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result != content {
        t.Errorf("got %q, want %q", result, content)
    }
}
```

对于静态数据，使用 `testdata/` 目录：

```go
func TestConfigLoad(t *testing.T) {
    cfg, err := LoadConfig("testdata/config_valid.json")
    if err != nil {
        t.Fatalf("failed to load valid config: %v", err)
    }
    if cfg.DefaultProvider != "deepseek" {
        t.Errorf("unexpected default provider: %s", cfg.DefaultProvider)
    }
}
```

### 数据库测试（SQLite）

```go
// 优先使用 :memory: 内存模式
func TestMemoryStore(t *testing.T) {
    store, err := NewMemoryStore(":memory:")
    if err != nil {
        t.Fatal(err)
    }
    defer store.Close()
    store.Migrate()
}

// 文件模式使用 TempDir
func TestMemoryStore_File(t *testing.T) {
    dbPath := filepath.Join(t.TempDir(), "test.db")
    store, err := NewMemoryStore(dbPath)
    if err != nil {
        t.Fatal(err)
    }
    defer store.Close()
}
```

### 配置测试数据

`testdata/` 下准备多种配置场景：

```
testdata/
├── config_full.json         # 完整多 Provider 配置
├── config_minimal.json      # 最小必填配置
├── config_missing_key.json  # 缺少 api_key
├── config_invalid_url.json  # 非法 base_url
└── config_custom.json       # 自定义 Provider
```

### 清理原则

1. **临时数据** — 一律使用 `t.TempDir()`，测试结束自动清理
2. **SQLite** — 优先使用 `:memory:` 内存模式；文件模式用 `t.TempDir()`
3. **静态测试数据** — 放入 `testdata/` 目录，由 git 版本管理
4. **环境变量** — 使用 `t.Setenv()` 自动恢复：

```go
func TestAPIKeyFromEnv(t *testing.T) {
    t.Setenv("DEEPSEEK_API_KEY", "test-key-123")
    // 测试结束后环境变量自动恢复
}
```

5. **全局状态** — 避免修改全局变量。如必须修改，在测试结束后恢复：

```go
func TestRegistryOverride(t *testing.T) {
    original := registry.Default()
    defer registry.SetDefault(original)
    registry.SetDefault(newTestRegistry())
}
```

---

## 4. Mock 与 Stub 策略

### 概念区分

| 类型 | 用途 | 示例 |
|------|------|------|
| **Stub** | 返回预设数据，不验证调用方式 | Provider 返回固定 ChatResponse |
| **Mock** | 验证调用次数、参数、顺序 | 验证 API 被调用 3 次 |
| **Fake** | 轻量级可工作实现 | 内存 SQLite 替代文件数据库 |
| **Spy** | 记录调用信息供后续断言 | 记录工具执行日志 |

不引入 testify、gomock 等第三方库。通过 helper 函数和接口实现所有 mock/stub 需求。

### 基础 Mock 策略

对外部依赖的 mock 通过接口实现：

```go
type LLMClient interface {
    Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
}

type mockLLMClient struct {
    response *ChatResponse
    err      error
}
func (m *mockLLMClient) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
    return m.response, m.err
}
```

对于 HTTP 调用，使用 `httptest.NewServer` 模拟 API 端点：

```go
func TestOpenAIProvider_Chat(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("Authorization") != "Bearer test-key" {
            t.Error("missing auth header")
        }
        w.Write([]byte(`{"choices":[{"message":{"content":"hello"}}]}`))
    }))
    defer server.Close()
    p := NewProvider(ProviderConfig{BaseURL: server.URL})
}
```

### Stub：固定响应

```go
type stubProvider struct {
    name     string
    models   []ModelInfo
    response *ChatResponse
}

func (s *stubProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
    return s.response, nil
}
func (s *stubProvider) Stream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
    ch := make(chan StreamEvent, 1)
    ch <- StreamEvent{Type: EventText, Content: s.response.Content}
    close(ch)
    return ch, nil
}
func (s *stubProvider) Models() []ModelInfo        { return s.models }
func (s *stubProvider) Capabilities() Capabilities  { return Capabilities{} }
func (s *stubProvider) Name() string                { return s.name }
```

### Stub：预设工具响应序列

```go
type stubExecutor struct {
    results []*ToolResult
    calls   []ToolCall  // Spy 功能
}

func (s *stubExecutor) Execute(ctx context.Context, calls []ToolCall) ([]*ToolResult, error) {
    s.calls = append(s.calls, calls...)
    var results []*ToolResult
    for i := range calls {
        if i < len(s.results) {
            results = append(results, s.results[i])
        }
    }
    return results, nil
}
```

### Mock：验证 HTTP 请求

```go
func TestDeepSeekProvider_ChatRequest(t *testing.T) {
    var receivedBody []byte
    var receivedAuth string

    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        receivedAuth = r.Header.Get("Authorization")
        receivedBody, _ = io.ReadAll(r.Body)
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte(`{"choices":[{"message":{"content":"pong"}}]}`))
    }))
    defer server.Close()

    p := NewDeepSeekProvider(ProviderConfig{BaseURL: server.URL, APIKey: "sk-test-key"})
    resp, _ := p.Chat(context.Background(), &ChatRequest{
        Messages: []Message{{Role: "user", Content: "ping"}},
    })

    if receivedAuth != "Bearer sk-test-key" {
        t.Errorf("auth = %q, want %q", receivedAuth, "Bearer sk-test-key")
    }
    if resp.Content != "pong" {
        t.Errorf("content = %q, want %q", resp.Content, "pong")
    }
}
```

### Mock：调用计数和参数验证

```go
type callRecorder struct {
    mu    sync.Mutex
    calls [][]any
}

func (r *callRecorder) Record(args ...any) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.calls = append(r.calls, args)
}

func (r *callRecorder) Count() int {
    r.mu.Lock()
    defer r.mu.Unlock()
    return len(r.calls)
}
```

### Fake：模拟流式 SSE

```go
func TestStreamParsing(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/event-stream")
        flusher, _ := w.(http.Flusher)
        events := []string{
            `data: {"choices":[{"delta":{"content":"Hello"}}]}`,
            `data: {"choices":[{"delta":{"content":" World"}}]}`,
            `data: [DONE]`,
        }
        for _, e := range events {
            fmt.Fprintf(w, "%s\n\n", e)
            flusher.Flush()
        }
    }))
    defer server.Close()
    // ...
}
```

### 选择策略

```
需要验证请求内容（URL/Header/Body）？
  Yes → httptest.NewServer + 断言 recorded values
  No  → stubProvider / stubExecutor

需要验证调用次数或顺序？
  Yes → callRecorder + 断言 Count()
  No  → 简单 stub

需要模拟多步骤交互（流式、多次工具调用）？
  Yes → stubExecutor（预设结果序列）+ callRecorder
  No  → 简单 stub

需要模拟错误路径？
  使用 stub 返回 error，或 httptest 返回 4xx/5xx
```

---

## 5. Stub 与 Mock 生成器

详细内容参见: [测试生成器指南](./LOOMCODE_TEST_GENERATORS.md)

### 辅助构造器

```go
func NewTestProvider(opts ...TestProviderOption) *StubProvider {
    p := &StubProvider{
        name: "test-provider",
        models: []ModelInfo{{ID: "test-model", ContextWindow: 131072}},
        caps: Capabilities{SupportsToolCall: true, SupportsStreaming: true},
        chatResponse: &ChatResponse{Content: "default response"},
    }
    for _, opt := range opts {
        opt(p)
    }
    return p
}

type TestProviderOption func(*StubProvider)
func WithModels(models []ModelInfo) TestProviderOption {
    return func(p *StubProvider) { p.models = models }
}
func WithChatResponse(content string) TestProviderOption {
    return func(p *StubProvider) { p.chatResponse = &ChatResponse{Content: content} }
}
func WithCapabilities(caps Capabilities) TestProviderOption {
    return func(p *StubProvider) { p.caps = caps }
}
```

### 表驱动 Stub 工厂

```go
// StubToolRegistry 预注册常用工具的 stub 实现
func NewStubToolRegistry() *StubToolRegistry {
    r := &StubToolRegistry{tools: make(map[string]*StubTool)}
    r.Register("read_file", &StubTool{
        NameVal: "read_file", IsReadOnlyVal: true,
        ExecuteFunc: func(ctx context.Context, args map[string]any) (*ToolResult, error) {
            return &ToolResult{Content: fmt.Sprintf("content of %s", args["path"])}, nil
        },
    })
    r.Register("bash", &StubTool{
        NameVal: "bash", IsReadOnlyVal: false,
        ExecuteFunc: func(ctx context.Context, args map[string]any) (*ToolResult, error) {
            cmd := args["command"].(string)
            if cmd == "invalid" {
                return nil, errors.New("command not found")
            }
            return &ToolResult{Content: fmt.Sprintf("executed: %s", cmd)}, nil
        },
    })
    return r
}
```

### 生成器使用时机

| 阶段 | 做法 |
|------|------|
| **Phase 1 (MVP)** | 手写 stub/mock，接口频繁变动时生成器维护成本高 |
| **Phase 2+** | 接口稳定后，对 `Provider`、`Tool`、`MemoryStore` 等核心接口引入 `go:generate` |
| **持续维护** | 接口变更时运行 `go generate ./...` 更新所有 mock |
