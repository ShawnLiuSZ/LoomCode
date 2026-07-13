# LoomCode Bug 检查报告

**检查时间**: 2026-06-26
**基线**: `go build ./...` 通过 · `go vet` 1 处警告（见 L1） · `go test ./...` 16 个包全绿 · `-race` 在关键包通过
**检查方法**: 基于 `bug-audit-recheck-2026-06-21.md` 第 1–3 节遗留/新引入缺陷做对抗性复验，再并发/资源/错误处理维度做新的一轮排查；按 TDD 写复现测试或代码直核源码

---

## 0. 总览

| 严重度 | 数量 | 项目 |
|--------|------|------|
| 🔴 Critical | 1 | L8 dashboard 优雅关闭死代码 + 资源泄漏 |
| 🟠 High | 4 | L2 Grep/Glob 误杀 loomcode 自身 · L4 SSE Close 双关 panic · L5 SSE reconnect 通知竞态 · L6 truncateMessages orphan tool |
| 🟡 Medium | 8 | L3 / L7 / L9 / L11 / L12 / L13 / L14 / L15 |
| 🟢 Low | 3 | L1 / L10 / L16 / L17 |

第 1–3 节确认的 21 项历史修复**全部保留**，未发现回退。

---

## 1. 🔴 Critical

### L8 — dashboard 优雅关闭是死代码，Ctrl+C 时不 graceful shutdown；MCP 子进程成孤儿
- **文件**:
  - `internal/dashboard/server.go:73-78`（死代码）
  - `cmd/loomcode/main.go:137, 411, 558-570`（资源泄漏 + 不响应信号）
- **现状**:
  ```go
  // dashboard/server.go
  go func() {
      <-context.Background().Done()  // ← context.Background() 永不 Done
      ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
      defer cancel()
      srv.Shutdown(ctx)
  }()
  ```
  上次审计已标记为死代码（`M8 优雅关闭` 旁注），但 `Start()` 仍未接受外部 `ctx`、goroutine 永远等不到 Done；用户 Ctrl+C 时 `srv.Shutdown` 不会被调用，正在处理的 HTTP/WS 连接被 RST，未发送的响应丢失。
- **配套泄漏**: `cmd/loomcode/main.go` 中 `connectPlugins(...)` 返回的 `*mcp.PluginManager` 全部被丢弃（`runCommand` line 137、`chatCommand` line 411）。stdio MCP 插件对应的子进程既没注册 cleanup、也没 `DisconnectAll` 调用，TUI 退出时这些子进程**变孤儿进程**继续运行。
- **最小修复**:
  1. `dashboard.NewServer(addr)` / `Start(ctx context.Context)`：用传入 ctx 替换 `context.Background()`，并在 main.go 里把 `signal.NotifyContext` 的 ctx 传过去
  2. `main.go` 把 `connectPlugins` 返回的 `pm` 缓存到局部变量，在 `chatCommand` / `runCommand` / `dashboardCommand` 退出前 `defer pm.DisconnectAll()`（**前提：DisconnectAll 当前无超时控制，长时间 hang 的 server 仍会卡住**；可加 context + 短超时保护）

---

## 2. 🟠 High

### L2 — GrepTool / GlobTool 用 `-cmd.Process.Pid` 杀进程组但未 `Setpgid`，可能杀死 loomcode 自身
- **文件**: `internal/tool/command_tools.go:155-159, 222-226`
- **现状**:
  ```go
  // GrepTool.Execute / GlobTool.Execute：都没设 cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
  cmd := exec.CommandContext(ctx, "grep", "-rn", pattern, path)
  // ... 无 Setpgid
  if truncated && cmd.Process != nil {
      syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)  // ← 负 PID = 进程组 ID
  }
  ```
  BashTool 已经修复（line 67-68 显式 `Setpgid: true`）。但 Grep/Glob 没继承此修复。子进程没被放进独立进程组时，负 PID 命中的是**父进程（loomcode）所在的进程组**。`grep -r` 截断 512KB 时，会把 loomcode 自己一起杀掉。
- **触发条件**: 用户用大文件 grep / glob 到大量文件导致输出 > 512KB（很常见）。
- **最小修复**: 与 BashTool 对齐，加 `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}`；或退一步用 `cmd.Process.Kill()` 杀单个 PID（与"孤儿子进程"取舍，看产品语义）。

### L4 — MCP SSE `Close()` 无锁 + `readSSEStream` send-on-closed 双重 panic 风险
- **文件**: `internal/mcp/sse_client.go:277-280, 288-291`
- **现状**:
  ```go
  // readSSEStream（line 277-280）
  select {
  case c.eventCh <- event:
  default:
  }
  
  // Close（line 288-291）
  func (c *SSEClient) Close() error {
      close(c.eventCh)   // ← 无 sync.Once / 标志位
      return nil
  }
  ```
  任何并发 Close → panic `close of closed channel`；`Close` 之后 `readSSEStream` 再向已关闭的 `eventCh` 发送 → panic `send on closed channel`。
- **影响**: `PluginManager.DisconnectAll` 多次调用或与 listenSSE 退出时序竞态，会让进程崩溃。
- **最小修复**: 用 `sync.Once` + closeOnce flag 保护 `close(c.eventCh)`，并在 `readSSEStream` 的 send 处 `defer recover()` 或检查 closed 标志。

### L5 — SSE 客户端 reconnect 通知与等待者之间存在窗口期丢信号
- **文件**: `internal/mcp/sse_client.go:198-201`
- **现状**:
  ```go
  // listenSSE 内
  c.reconnecting.Store(1)
  c.notifyMu.Lock()
  close(c.notifyCh)              // ← 先 close
  c.notifyCh = make(chan struct{}, 1)   // ← 再赋值
  c.notifyMu.Unlock()
  ```
  与 `waitForResponseWithRetry`（line 382-427）存在竞态：等待者可能在 `close` 之后、`make` 之前读取 `c.notifyCh`（旧 channel 的 close 信号），但 Go 的 channel close 是即时广播，所有等待者都能收到；但**在 `c.notifyCh = make(...)` 之后到 `c.notifyCh` 字段对其它 goroutine 可见**之间，新的 reconnect 信号塞到新 channel 也不会被等待者读到（它已经返回）。
- **影响**: 重连期间正在 wait 的请求可能误判为超时（`canRetry=false` 分支直接 `connection lost waiting for response`），不会用新 channel 收。
- **最小修复**: 用 `sync.Cond` 或单独的 "重连信号" channel + 重试循环（重试时重新读 `c.notifyCh`）；最简单是 `waitForResponseWithRetry` 在 `case <-notifyCh:` 内**不**消费（只是 signal），让等待者主动重读 `c.notifyCh`。

### L6 — `truncateMessages` 仍可能产生 orphan tool 消息（API 400 风险）
- **文件**: `internal/agent/loop.go:179-227`（尤其 line 199-225）
- **现状**:
  ```go
  for i := len(a.messages) - keepRecent - 1; i >= 1; i-- {
      msg := a.messages[i]
      if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
          roundStart = i
          roundEnd = i
          for j := i + 1; j < len(a.messages)-keepRecent; j++ {  // ← 范围被 keepRecent 截断
              if a.messages[j].Role == "tool" {
                  roundEnd = j
              } else {
                  break
              }
          }
          break
      }
  }
  ```
  当 `len(a.messages) == keepRecent + 2`（典型 6 条：system + assistant+tool_calls + tool + 3 条最近），外层 for 进入第二轮时：
  - 找到 assistant tool_call 在 i=1
  - 内层 for `j < len - keepRecent = 2` 不会进入 → `roundEnd = 1`
  - **删除 messages[1]，但 messages[2] (tool) 留下 → orphan tool 消息**
- **影响**: LLM API（OpenAI/DeepSeek）要求 tool 消息必须紧跟其 assistant tool_call，orphan tool 会触发 400 错误。
- **状态**: 上一轮 `compactMessages`（`context.go:103-145`）已加保护（line 121-127 `for cut < len && a.messages[cut].Role == "tool" { cut++ }`），但 `truncateMessages` 是它的**回退路径**（line 125, 131），是失败兜底；一旦摘要 API 失败就会触发这个 bug。
- **最小修复**: `truncateMessages` 同样在 roundEnd 之后循环 `for roundEnd+1 < len && a.messages[roundEnd+1].Role == "tool" { roundEnd++ }`，把对应的 tool 结果一起带上。

---

## 3. 🟡 Medium

### L3 — MCP SSE 事件解析 TrimPrefix 缺空格兼容
- **文件**: `internal/mcp/sse_client.go:267-272`
- **现状**:
  ```go
  } else if strings.HasPrefix(line, "data:") {
      event.Data = strings.TrimPrefix(line, "data: ")  // ← 硬编码一个空格
  }
  ```
  若服务器发送 `data:{"jsonrpc":"2.0",...}`（无空格，符合 SSE 规范）→ `TrimPrefix("data: ")` 不匹配 → `event.Data = "data:{...}"`，下游 `json.Unmarshal` 失败，事件被静默丢弃。
  `event:` / `id:` 字段同样问题。
- **对照**: `internal/provider/sse.go:84-92` `ExtractSSEData` 是正确的（先 `data: ` 再 `data:` fallback）。
- **最小修复**: 抽出一个 `parseSSEField(line, prefix) string` 工具函数，先试 `prefix+" "`，再试 `prefix`，否则用。

### L7 — EditFileTool 错误消息误导
- **文件**: `internal/tool/file_tools.go:221-223`
- **现状**:
  ```go
  if oldText == "" {
      return nil, fmt.Errorf("path and old_text are required")
  }
  ```
  只检查了 `oldText`，没检查 `path`；若两者都为空，错误消息说"path 和 old_text 都需要"，但实际只检查了 oldText。另一面 `path==""` 在 `resolveWithinRoot` 会单独报 "path is required"。两个错误信息不一致。
- **最小修复**: 分别检查或统一为"path 和 old_text 至少有一个为空"。

### L9 — `saveSession` 触发 welcome 消息后即生成空会话文件，与"懒创建"语义矛盾
- **文件**: `internal/ui/app.go:305-327`
- **现状**:
  ```go
  func (a *App) saveSession() {
      if a.sessionMgr == nil { return }
      if a.activeSess == nil {
          if len(a.messages) == 0 { return }
          a.activeSess = a.sessionMgr.Create("default", a.model, a.provider.Name())
      }
      // ...
  }
  ```
  构造函数（`app.go:234-236`）已经把 welcome 消息塞进 `a.messages`，所以 `len(a.messages)` 永远 ≥ 1。`saveSession` 在 `streamError`（`app.go:413`）/`streamDone`（`app.go:423`）/`ctrl+c`（`app.go:537`）/`/quit`（`app.go:655`）都会被调用。**只要走 chat 一次，磁盘就会出现一个仅含 1 条 welcome 消息的会话文件**——注释"且不会在每次启动时产生空会话文件"未生效。
- **最小修复**: 改成检查"用户实际输入过的消息数"（`savedMsgCount > 0` 或维护 `a.userSentCount`），而非 `len(a.messages)`。

### L11 — `sendNotification` 写错误被吞
- **文件**: `internal/mcp/client.go:381-404`, `internal/lsp/client.go:217-235`
- **现状**: `c.stdin.Write(data)` 错误被忽略。server 中途断开时（典型场景：`initialized` 通知在 server 已关闭 stdin 之后到达），发送失败但无任何日志。
- **影响**: 调试"为什么 MCP/LSP 协议握手没完成"时无任何线索。
- **最小修复**: 把 `c.stdin.Write(data)` 改成 `if _, err := c.stdin.Write(data); err != nil { log.Printf(...) }`；LSP 端同样处理（顺便把 `json.Marshal` 错误也升级为日志）。

### L12 — `addJitter` 注释与实现不一致
- **文件**: `internal/provider/retry.go:145-152`
- **现状**: 注释说"±25%"，实际是 `delay - rand.Intn(delay/2)` 即 `[-50%, 0%]` 单边抖动。功能可用（不会让重试间隔变得异常），但与设计意图不符。
- **最小修复**: 改为 `delay - delay/2 + rand.Intn(delay)`（即 `[-50%, +50%]`）；或 `delay + (rand-0.5)*delay/2` 实现 ±25%。

### L13 — `isDisconnectedErr` 字符串精确匹配脆弱
- **文件**: `internal/mcp/client.go:319-324`
- **现状**:
  ```go
  msg := err.Error()
  return msg == "server disconnected" ||
      msg == "write request: io: read/write on closed pipe" ||
      msg == "write request: write |1: broken pipe"
  ```
  Linux/macOS/Windows 错误消息字面值不同（如 macOS 上 broken pipe 是 `write |1: broken pipe`，但跨版本 go 可能变），且 `c.cmd.Process.Kill()` 后 readLoop 报的 `read |0: file already closed` 不在列表里。reconnect 不触发，callRaw 直接返回 error 给上层。
- **最小修复**: 用 `errors.Is(err, io.ErrClosedPipe) || errors.Is(err, fs.ErrClosed) || ...`，或维护一个 `disconnected atomic.Bool` 由 cleanup / readLoop 主动设。

### L14 — `Client.Connect()` 缺少并发锁，多次调用会创建多个子进程
- **文件**: `internal/mcp/client.go:81-143`
- **现状**: `Connect()` / `connect()` / `reconnect()` 内部无锁，多个 goroutine 并发调用会产生多个 `exec.Cmd` 写同一个 `c.stdin`。
- **影响**: 在 `PluginManager.Disconnect` / `Connect` 紧接调用、加上并发工具调用时的重连，可能 race。
- **最小修复**: 加 `sync.Mutex`，`Connect` / `Close` / `reconnect` 串行化。

### L15 — LSP `readLoop` 跳空行错误被吞
- **文件**: `internal/lsp/client.go:113`
- **现状**: `c.stdout.ReadString('\n')` 读空行时错误被忽略。若 server 发的 header 与 LSP 规范不符（缺空行），`io.ReadFull` 会等不到完整 body → goroutine 永久 hang 在读。
- **最小修复**: 检查错误并 return。

---

## 4. 🟢 Low

### L1 — 测试代码 self-assignment
- **文件**: `internal/ui/app_test.go:239`
- **现状**: `app2.modelList = app2.modelList` 显然是 copy-paste 残留。`go vet` 已警告。
- **最小修复**: 删除该行。

### L10 — OAuth token 刷新未实现
- **文件**: `internal/provider/oauth.go:58-65`
- **现状**: `GetToken` 检测到过期（`time.Now().Add(60s).After(ExpiresAt)`）后只写 TODO 然后返回旧 token。代码注释自己写了"由上层处理 401 错误"。
- **影响**: OAuth 模式下会话超过 1 小时后会 401，但上层没有"刷新 token"路径；目前 `Provider` 接口没有 token 刷新回调。
- **最小修复**: 实现 refresh_token 流程（用 `http.PostForm(tokenURL, ...)`），更新 `m.token` 和 `m.onRefresh` 回调；或在 Provider 接口上加 `RefreshToken(ctx) error`。

### L16 — MCP callRaw 成功响应被遗弃
- **文件**: `internal/mcp/client.go:295-300`
- **现状**: `c.stdin.Write` 失败 → `cancelPending` 从 map 删除 → 但 readLoop 可能已经把 response 推到 `respCh`（buffered 1）。该 channel 永远不被读取，response 留在 channel 里直到 GC。
- **影响**: 极小泄漏，每个失败请求 ~100 字节。仅在反复断连时累计。
- **最小修复**: 在 `cancelPending` 里也 `close(respCh)` 或 `select { case <-respCh: default: }` drain。

### L17 — 已修（信息项）
- 上一轮修复的 `Executor.SetMaxParallel` 并发安全、`OAuthManager` 的 `GetToken`/`IsExpired`/`NeedsRefresh` 锁保护均保留。
- `Lsp` sendNotification 改回 Content-Length 分帧（`lsp/client.go:228-235`）✓
- `WebSocket` Origin 精确 host + 默认拒绝空 Origin（`dashboard/websocket.go:165-178`）✓
- `SubAgent.Run` 一次性 + 深度限制（`agent/subagent.go:82-134, 233-266`）✓
- `Bash` 进程组清理（`tool/command_tools.go:67-75`）✓
- `Auto` 模式白名单守门（`control/gate.go:88-92`）✓
- `homeDir` 不回退 cwd（`config/config.go` 第 9 节"未做"项已确认修）✓

---

## 5. 建议处置（按优先级）

1. **L8**（Critical，资源/可用性）：dashboard 收 ctx + main.go `defer pm.DisconnectAll()`。一次性改完。
2. **L2**（High，误杀 loomcode）：Grep/Glob 加 `Setpgid`，与 Bash 对齐。10 行。
3. **L4 + L5**（SSE 客户端并发）：合并修复——`sync.Once` 保护 close + `sync.Cond` 通知。30 行。
4. **L6**（API 400 风险）：`truncateMessages` 复用 `compactMessages` 的"跳孤立 tool"逻辑。5 行。
5. **L3 / L11 / L12 / L13 / L14 / L15**（Medium，6 项）：按 TDD 写复现测试 → 最小修复。可一次性提交。
6. **L1 / L7 / L9 / L10 / L16**（Low）：收口轮清理。

每项建议 systematic-debugging 风格：先写红测 → 最小改 → 转绿 → 回归。

---

*检查生成: 2026-06-26 ｜ 方法: 基线 build/vet/test + 历史 21 项修复直核 + 并发/资源/错误处理维度新排查 ｜ 状态: 未改动代码*
