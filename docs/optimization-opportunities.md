# LoomCode CLI — 优化机会清单

> 基于 2026-07-15 全量代码审查，覆盖构建状态、测试结果、架构、代码质量、安全性五个维度。

---

## 当前状态快照

| 维度 | 状态 |
|------|------|
| `go build ./...` | ✅ 通过 |
| `go vet ./...` | ✅ 通过 |
| `go test ./...` | ❌ `internal/mcp` 测试超时（TestSSEClientCallToolEmpty 挂起 60s） |
| 生产代码 | ~15,900 行 / 77 文件 |
| 测试代码 | ~12,800 行 / 69 文件（测试/生产比 81%） |
| 依赖数 | 8 个直接依赖，零 LLM SDK 依赖 |

---

## P0 — 必须修复

### 1. MCP SSE 客户端测试超时（功能性 Bug）

**文件:** `internal/mcp/sse_client.go:367-410` + `internal/mcp/sse_client_test.go:116-123`

**问题:** `TestSSEClientCallToolEmpty` 创建一个指向 `localhost:8080` 的客户端（无实际服务器），期望未连接时返回错误。但 `CallTool()` → `call()` **不检查 `initialized` 标志**，直接发起 HTTP POST 请求。由于 `httpClient.Timeout = 30s`，该 POST 会挂起 30 秒，随后 `waitForResponseWithRetry` 又等待 30 秒，最终触发测试超时。

**根因:** `call()` 方法缺少前置状态检查。

**修复方案:**
```go
// 在 call() 方法开头添加
func (c *SSEClient) call(ctx context.Context, method string, params any) (*Response, error) {
    if !c.initialized {
        return nil, fmt.Errorf("client not connected")
    }
    // ... 原有逻辑
}
```

同理，`ListTools()` 也应在未连接时提前返回错误，避免 `TestSSEClientListToolsEmpty` 潜在的 30 秒挂起（当前该测试碰巧因为 `c.call()` 的 HTTP 请求立即失败而通过，但这是巧合而非设计）。

---

## P1 — 应尽快修复

### 2. `ui/app.go` 单文件 1968 行（架构债务）

**文件:** `internal/ui/app.go`

**问题:** TUI 应用的所有逻辑集中在一个文件：结构体定义、消息渲染、输入处理、斜杠命令、工具审批、模型切换、会话管理、成本显示、Markdown 渲染、编辑快照。

**影响:** 难以维护、难以并行开发、测试覆盖率仅 17%（334 行测试 / 1968 行代码）。

**建议拆分方案:**
| 新文件 | 内容 | 估计行数 |
|--------|------|----------|
| `app.go` | App 结构体、NewApp、Init/Update/View 核心接口 | ~400 |
| `app_render.go` | 消息渲染、Markdown 渲染、状态栏、成本显示 | ~400 |
| `app_commands.go` | 斜杠命令处理（/mode, /model, /cost 等） | ~350 |
| `app_tools.go` | 工具审批流程、pendingWrite 处理 | ~200 |
| `app_session.go` | 会话保存/恢复、模型切换 | ~250 |
| `app_input.go` | 输入处理、命令联想、任务队列 | ~300 |

### 3. `main.go` 职责过多 + 初始化流程重复（637 行）

**文件:** `cmd/loomcode/main.go`

**问题 A:** `chatCommand()`（110 行）和 `runCommand()`（73 行）共享完全相同的初始化链路：
```
loadConfig → selectProvider → createProvider → NewRegistry → RegisterDefaults 
→ NewPermission → configureToolPermissions → connectPlugins
```
但各自独立实现，导致维护时容易遗漏同步。

**修复:** 提取公共初始化函数：
```go
type runtime struct {
    cfg      *config.Config
    prov     provider.Provider
    allProvs []provider.Provider
    tools    *tool.Registry
    perm     *control.Permission
    plugins  *mcp.PluginManager
    cpMgr    *tool.CheckpointManager
}

func initRuntime(chatMode bool) (*runtime, error) { ... }
```

**问题 B:** `registerAutoFormatHook` 中 `auto-gofmt` 和 `auto-gofmt-edit` 两个 Hook 的 Handler 函数体完全相同（仅 ToolName 不同），应合并为工厂函数：
```go
func makeGofmtHook(toolName string) tool.Hook {
    return tool.Hook{
        Name:     "auto-gofmt-" + toolName,
        Type:     tool.HookPostExecute,
        ToolName: toolName,
        Handler:  gofmtHandler,
    }
}
hm.Add(makeGofmtHook("write_file"))
hm.Add(makeGofmtHook("edit_file"))
```

### 4. SSE 解析逻辑重复

**文件:** `internal/provider/sse.go` vs `internal/mcp/sse_client.go`

**问题:** 两个包各自实现了 SSE 流解析：
- `provider/sse.go`: 自定义 `SSEReader`，逐字节解析
- `mcp/sse_client.go`: `bufio.Scanner` 逐行解析

两套实现的行为相同但代码不同，维护成本翻倍。

**建议:** 提取通用 SSE 解析到独立的 `internal/sse` 包，两个消费者共用。

---

## P2 — 建议优化

### 5. 测试覆盖缺口

| 缺失测试的文件 | 行数 | 风险 |
|----------------|------|------|
| `internal/memory/layers.go` | 171 | 记忆层管理逻辑无测试 |
| `internal/provider/sse.go` | 238 | SSE 流解析器无测试 |
| `internal/provider/message.go` | 83 | 消息类型转换无测试 |
| `internal/mcp/protocol.go` | 202 | MCP 协议类型无测试 |
| `internal/lsp/protocol.go` | 199 | LSP 协议类型无测试 |
| `internal/dashboard/handlers.go` | 58 | HTTP 处理器无测试 |

**优先级:** `sse.go` 和 `layers.go` 包含核心业务逻辑，应优先补测试。

### 6. 错误静默吞没

**文件:** `internal/dashboard/handlers.go:20,31,44,57`

```go
_, _ = w.Write(data)           // line 20
_ = json.NewEncoder(w).Encode(sessions)  // line 31
_ = json.NewEncoder(w).Encode(cost)      // line 44
_ = json.NewEncoder(w).Encode(status)    // line 57
```

HTTP 响应写入失败被完全忽略。虽然 HTTP handler 中无法向客户端报告写入失败，但应至少记录日志以便排查问题。

### 7. `SSEClient.waitForResponseWithRetry` 硬编码 30 秒超时

**文件:** `internal/mcp/sse_client.go:420`

```go
timeout := time.After(30 * time.Second)
```

应改为可配置或从 context deadline 推导，当前无法通过 ctx 控制总超时（ctx 的 deadline 和 30s 取较早者，但调用方无法设置超过 30s 的等待）。

### 8. `ExportEnvToSubprocess` 中 API Key 列表硬编码

**文件:** `cmd/loomcode/main.go:569-575`

```go
apiKeys := map[string]bool{
    "DEEPSEEK_API_KEY":  true,
    "MIMO_API_KEY":      true,
    "OPENAI_API_KEY":    true,
    "ANTHROPIC_API_KEY": true,
    "TAVILY_API_KEY":    true,
}
```

新增 Provider 时必须手动更新此列表，否则 API Key 可能泄露到子进程环境变量。应从配置文件中所有 Provider 的 `api_key_env` 字段动态收集。

### 9. `agent/loop.go` 831 行偏大

**文件:** `internal/agent/loop.go`

虽然结构尚可（结构体定义 + 构造函数 + Run 循环），但 Run 循环内包含了工具执行、消息构建、缓存调度、修复流水线等多个关注点。可考虑将 Run 循环中的工具执行和结果处理提取为独立方法，降低单函数复杂度。

### 10. `tool` 包 19 个文件，缺乏子包组织

**文件:** `internal/tool/`

15+ 个内置工具（bash, read_file, write_file, edit_file, git, websearch, task, session, memory, skill 等）全部平铺在 `tool` 包下。建议按功能域分组为子包：
- `tool/builtin/` — 核心内置工具（file, edit, bash）
- `tool/vcs/` — Git 工具
- `tool/web/` — Web 搜索
- `tool/meta/` — Session, memory, skill, task

### 11. `dashboard/server.go` 使用 panic 处理 crypto/rand 错误

**文件:** `internal/dashboard/server.go:45`

```go
panic("dashboard: crypto/rand unavailable: " + err.Error())
```

crypto/rand 不可用是极端边界情况，但 panic 会导致进程崩溃。应改为返回 error 让调用方决定处理方式。

---

## 正面评价（做得好的地方）

- **Provider 插件化设计** — Adapter 工厂模式 + Capability 声明，新厂商接入零代码改动
- **测试/生产代码比 81%** — 远超行业平均，agent 和 tool 包覆盖尤其好
- **零外部 LLM SDK 依赖** — Provider 全部基于 net/http 直接实现，避免了 SDK 锁定
- **CGO_ENABLED=0 单二进制** — modernc.org/sqlite 纯 Go 实现，部署零依赖
- **代码无 TODO/FIXME/HACK 标记** — 说明已知债务已被清理或显式记录
- **安全控制完善** — 权限白名单、成本门控、编辑快照、环境变量过滤四层防护
- **错误处理整体规范** — 生产代码中仅 1 处 panic（dashboard），无 os.Exit 泄漏到 internal 包
