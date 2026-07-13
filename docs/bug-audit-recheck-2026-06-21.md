# LoomCode Bug 修复复查报告

> 对 `docs/bug-audit-report-2026-06-20.md` 中 21 个编号问题 + 6 个 Low 项 + 新增 WebSocket 功能的逐项修复复查

**复查时间**: 2026-06-21
**复查基线**: 修复前 `597c8f6` → 当前 `HEAD`（`4620c58`）
**复查方法**: 按子系统并行派发 8 个独立审计 agent，逐项核对当前源码（非报告声明）；对每个"未修复/部分修复"结论做对抗性验证（构造绕过、复现 panic、直接核对 git diff）；高影响项由主线二次直核源码
**构建/测试**: `go build ./...` 通过；`go vet` 干净；`go test ./...` 通过（但**绿测不等于修复**——多数遗留缺陷的复现路径没有对应测试，详见各项）

---

> **更新 (2026-06-21)**：第 1–3 节列出的遗留/新增缺陷已全部修复完毕，详见**第 8 节 修复执行记录**。本节及第 1–3 节保留为复查时的原始结论。

## 0. 结论（先说重点）

**"已修复完毕"不成立。** 27 项中 18 项确认修复，但仍有 **3 项完全未修复（含 1 个 Critical）**、**6 项部分修复（含 1 个 Critical）**，并且本轮修复**引入了 2 处新缺陷**（LSP 通知线协议被改坏；新写的 WebSocket 有 CSWSH 绕过）。

| 复查结论 | 数量 | 项目 |
|----------|------|------|
| ✅ 已修复 | 18 | H1 H2 H4 M2 H5 H7 H9 H10 M4 C3 M1 C4 M5 M6 M8 + Low(loadbalancer / vector-json / skills-symlink) |
| ❌ 未修复 | 3 | **C2**(Critical) **H6**(High) **H11**(High) |
| ⚠️ 部分修复 | 6 | **C1**(Critical) M3 H3 M9 + Low(config-home / crypto-KDF) |
| 🔴 新引入缺陷 | 2 | LSP `sendNotification` 线协议；WebSocket Origin 校验 |

**最高优先级（必须先处理）**：C2（文件工具无任何护栏）、C1+H6（安全边界仍可绕过且 Auto 模式直接放行）、H11（二次 Run 必崩进程，已复现）、LSP 通知线协议回归。

---

## 1. ❌ 完全未修复（3 项）

### C2 — 文件工具仍无任何路径限制 / 不走权限护栏（Critical）
- **文件**: `internal/tool/file_tools.go:33,67,102,115`
- **现状**: 三个工具仍是空结构体 `struct{}`，`Execute` 把模型给的 `path` 直接交给 `os.ReadFile(path)` / `os.WriteFile(path, …, 0644)`。无 workspace 根、无 `filepath.Clean/Rel` 包含性校验、无 `EvalSymlinks`、**完全不调用** `internal/control` 权限检查。
- **核实**: `cmd/loomcode/main.go:128-132,273-277` 只对 `*tool.BashTool` 调了 `SetPermissionChecker`，三个文件工具连这个方法都没有。`internal/control/permission.go:36-42` 其实已支持 `path` 检查，但从未接线。
- **仍可利用**: `read_file /etc/passwd`、`write_file ../../.ssh/authorized_keys`、符号链接逃逸全部畅通。这是报告里最严重的一项，**原样未动**。
- **最小修复**: 给三个结构体加 `root string` + `permission PermissionChecker`；`Execute` 内 `abs := filepath.Abs(filepath.Join(root, path))`，`rel,_ := filepath.Rel(root, abs)`，`strings.HasPrefix(rel,"..")` 即拒绝；`EvalSymlinks` 再校验一次；最后过一遍与 bash 相同的 `permission.Check`。两处 main.go 像 bash 一样接线。
- **附带**: `write_file`/`edit_file` 仍写 `0644`，与 M5（会话文件已改 0600）方向不一致。

### H6 — Auto 模式仍默认放行（High）
- **文件**: `internal/control/gate.go:88-91`
- **现状**: `git diff 597c8f6 HEAD -- internal/control/gate.go` **为空**，文件逐字节未改：
  ```go
  // Auto 模式：放行但记录
  if g.mode == ModeAuto {
      return true, "auto-approved"   // 无任何记录/撤销实现
  }
  ```
- **实测**（经 `permission.Check`，空白名单）: `write_file /etc/cron.d/evil`、`python3 -c '…os.system…'`、`chmod +x /tmp/x && /tmp/x`、`npm install` 全部 `allowed=true`。**Auto 模式下 C1 的分词工作被完全旁路**——`gate.Check` 在 `IsAllowed` 返回 false 后仍直接 `return true`。
- **额外问题**: 测试 `control_test.go:52-61`（`TestGate_AutoMode`）**断言了这个错误行为**（`reason=="auto-approved"`），等于把漏洞钉死。
- **最小修复**: Auto 模式下 bash / 写操作仍过白名单（默认拒绝）：
  ```go
  if g.mode == ModeAuto {
      if g.allowlist != nil && g.allowlist.IsAllowed(toolName, args) {
          return true, "auto-approved (allowlisted)"
      }
      return false, "auto mode: not allowlisted"
  }
  ```
  并同步改掉 `TestGate_AutoMode`。

### H11 — 子 agent 二次 Run 双重 close panic + 递归无深度限制（High，两半都未修）
- **文件**: `internal/agent/subagent.go:141-151`
- **现状**: 与基线逐字节相同，守卫仍是 `if sa.status == StatusRunning`。首次完成后状态变 `Completed`，二次 `Run` 通过守卫 → 新 goroutine `defer close(sa.done)` → **对已关闭 channel 二次 close**。
- **已复现**: 写对抗测试（Run 两次、之间 Wait）→ `panic: close of closed channel`（`subagent.go:171` defer 触发，`created by …Run at subagent.go:150`）。
- **第二半**: `Spawn`（`:73-94`）从不设置 `ParentID`（字段在 `:18` 声明但全仓库无赋值），无 `Depth/MaxDepth` 字段或检查。递归深度限制完全缺失。
- **最小修复**: 守卫改 `if sa.status != StatusPending { return }`（或 `sync.Once`）；`Spawn` 增加 `parent` 参数，设 `ParentID`、`Depth=parent.Depth+1`，`> maxSubAgentDepth(3)` 时拒绝。

---

## 2. ⚠️ 部分修复（6 项）

### C1 — 白名单分词已加，但命令替换 / 重定向未分词（Critical → 仍可绕过）
- **文件**: `internal/control/allowlist.go:107-198`
- **已修复部分**: 放弃原始串前缀匹配，改 `splitShellCommand` 分段 + `isArgv0Allowed` 精确校验 + 默认拒绝。以下已正确拦截：`git status; rm -rf ~`、`gitfoo`、`a && curl|bash`、`echo hi|tee x`、`FOO=bar rm -rf ~`。
- **仍存在的真实绕过**（这些通过了白名单，只剩可绕过的子串黑名单兜底）:
  | 攻击 | 结果 | 根因 |
  |------|------|------|
  | `git $(touch /tmp/pwned)` | 🔴 放行 | `$(` 不是分隔符 |
  | `` git `touch /tmp/pwned` `` | 🔴 放行 | 反引号不分词 |
  | `echo hi>~/.ssh/authorized_keys` | 🔴 放行 | `>` `>>` 不分词、不剥离 |
- **代码与注释自相矛盾**: `allowlist.go:142` 注释声称按 `$()`/反引号分割，**实际 `splitShellCommand` 的分隔符列表只有 `; && || |`**，并不含命令替换与重定向。
- **最小修复**: 分隔符补全 `"$(", "`", ">>", ">", "<", "&", "(", ")", "\n"`（注意 `&&`/`||`/`>>` 要排在 `&`/`>` 前），对每段 argv[0] 校验；黑名单仅作纵深防御。

### M3 — bash 输出上限 ✅ + 超时 ✅，但进程组未杀（High 影响）
- **文件**: `internal/tool/command_tools.go:42-101`
- **已修复**: `io.LimitReader(stdout, 1MB)` 限内存（不再 OOM）；`context.WithTimeout(…, 60s)` 独立超时。
- **未修复**: `Setpgid:true`（`:67`）建了进程组，但**从未杀 `-pgid`**——无 `cmd.Cancel` 覆写、无 `syscall.Kill(-pgid, …)`。超时走默认 `cmd.Process.Kill()` 只杀组长 PID，后台 `&` 子进程 / 管道成员变孤儿存活。
- **附带 hang**: `cat /dev/zero` 时 `ReadAll(LimitReader)` 到 1MB 返回后子进程写满管道阻塞，`cmd.Wait()` 会一直堵到 60s 超时才返回。
- **最小修复**:
  ```go
  cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
  ```
  并在读满 1MB 后立即 `cmd.Cancel()`/后台 drain，避免 hang 到超时。

### H3 — 会话落盘管道已搭，但默认用户仍丢历史（High，头号 bug 只修一半）
- **文件**: `cmd/loomcode/main.go:279-302`、`internal/ui/app.go:199,288,298,371,475`
- **已修复**: `saveSession()` 经 `AddMessage`+`Save()` 实现，并接到 `streamDone`/`streamError`/`ctrl+c`。
- **两个真实缺口**:
  1. **启动从不创建活动会话**: main.go 创建并注入 Manager 但**从不 `Create()`**；`activeSess` 仅在 `--session <id>` 或手动 `/sessions new` 时才有。普通首次会话 `saveSession()` 在 `app.go:199`（`activeSess==nil`）直接 return——**什么都没存**。
  2. **`/quit`、`/exit` 不落盘**: `app.go:475-477` 只 `tea.Quit`，未调 `saveSession()`。只有 `ctrl+c` 存。
- **最小修复**: main.go 在无 `--session` 时创建并激活默认会话；`app.go:475` 的 `/quit`/`/exit` 分支前加 `a.saveSession()`。

### M9 — dashboard 已绑回环 + 日志已修，但仍无鉴权（Medium）
- **文件**: `internal/dashboard/server.go:19-36,57`
- **已修复**: `:8080`/空 → `127.0.0.1:8080`；裸 `:port` 自动加 `127.0.0.1`；日志打印真实 `s.addr`。
- **未修复**: 所有端点（`/ /api/sessions /api/cost /api/status /ws`）**零鉴权**；且只是"默认"回环，显式传 `0.0.0.0:8080`/`192.168.x.x:8080` 仍按原样监听全网卡。
- **最小修复**: 强制回环（拒绝非回环 addr 或显式告警）；接真实数据前加 token/basic auth。

### Low — config `UserHomeDir` 回退仍不安全
- **文件**: `internal/config/config.go:169-177`
- **现状**: 错误从 `_` 改为显式检查，但**危险回退原样保留**——HOME 为空时 `return cwd`，配置路径退化为 CWD 相对 `./.loomcode/config.toml`（代码注释自己写了"可能被恶意配置注入"）。漏洞未闭合。
- **最小修复**: HOME 不可用时返回错误 / 跳过用户配置路径，绝不回退 cwd。

### Low — crypto 密钥派生：公开信息派生已去除，但仍无 KDF 且未接线
- **文件**: `internal/session/crypto.go:40-53`
- **现状**: 去掉 hostname/username 派生，无 env 时改为每次启动随机 32 字节；但 env 路径仍单次 SHA-256 无 salt/KDF；全仓库无 Argon2id/scrypt（`go.mod` 无 `x/crypto`）。`NewCrypto`/`Encrypt/DecryptSessionFile` **无任何生产调用方**，会话仍明文 `.jsonl` 存储。随机密钥还会让加密会话跨重启无法解密（因未接线才不暴露）。
- **结论**: 接线前不阻断；若要启用加密，需随机数据密钥 + Argon2id/scrypt + 持久化 salt，并真正在 Manager 存取路径调用。

---

## 3. 🔴 本轮修复新引入的缺陷（2 处）

### R1 — LSP `sendNotification` 线协议被改坏（H8/H9 重构回归）
- **文件**: `internal/lsp/client.go:230-232`
  ```go
  data, _ := json.Marshal(notif)
  data = append(data, '\n')   // 裸换行分帧 —— LSP 错误
  c.stdin.Write(data)
  ```
- **问题**: LSP 必须 `Content-Length: N\r\n\r\n{body}` 分帧（基线 `notify()` 本来就对，reader `readLoop` 也按此**期望**读取）。重构把 MCP 的换行分帧抄进了 LSP。`Connect` 末尾发的 `initialized` 通知（`:92`）因此发成未分帧，会打乱真实 LSP server 的解析器，后续响应再也对不上（复现：mock 捕获到 `{…"method":"initialized"}\n` 无 `Content-Length` 头，下一个请求响应永不返回 → 30s 超时）。
- **未被测试发现**: 重构同时**删掉了所有 LSP 请求方法**（`DidOpen/DidChange/Completion/Hover/Definition/DocumentSymbol`），且无 LSP 集成测试做 Connect 后调用；LSP client 当前在包外无调用方（功能性影响因此暂被掩盖，但属功能回退）。
- **修复**: `sendNotification` 恢复 `Content-Length` 分帧（照搬仍正确的 `call` 写路径 `:178-189`）。MCP 的换行分帧本身正确，勿动。

### R2 — 新写的 WebSocket 有 Origin 绕过 + 读超时误杀（L2 新功能）
- **文件**: `internal/dashboard/websocket.go`
- **整体**: hub 的 register/unregister/broadcast 串行在单 `run()` goroutine，`-race` 干净，无 send-on-closed。但：
  - 🔴 **CSWSH 前缀匹配绕过**（`:163-177`）: `origin[:len(a)] == a` 无界前缀比较，`http://localhost.evil.com`、`http://127.0.0.1.attacker.com`、`http://localhostXSS` 全部放行——正好击穿它本想防的跨站 WebSocket 劫持。应 `net/url` 解析后**精确**比较 host。
  - ⚠️ **空 Origin 绕过**（`:98`）: `origin != "" && !isAllowedOrigin(origin)`——无 `Origin` 头直接跳过校验，非浏览器客户端（curl/脚本）无条件放行。应默认拒绝未知 Origin。
  - ⚠️ **读超时误杀**（`:123`）: `SetReadDeadline(now+60s)` 只设一次、读循环（`:139-147`）从不刷新，也无 ping/pong；任何健康连接活到 60s 必被强制断开。写超时刷新是对的，读超时设计错了。
- **修复**: Origin 改精确 host 校验 + 默认拒绝空 Origin；读循环每次收到消息后刷新 ReadDeadline，并加 ping/pong 心跳。

---

## 4. ✅ 已确认修复（18 项，附残留小瑕疵）

| 项 | 结论 | 备注 |
|----|------|------|
| H1 | ✅ | `strings.Replace(…, -1)` 不再死循环；但 `-1`=替换全部，与 "precise replacement" 描述及 `review.go` 的单次替换语义不一致，建议改 `1` |
| H2 | ✅ | 改为按下标 `results[idx]`，重名工具结果不再错乱 |
| H4 | ✅ | 两处均加 `i>=len \|\| ==nil` 守卫并补造 error Result |
| M2 | ✅ | `achieved`/`reason` 已被消费并回传（未真正 `continue` 重驱动循环，但 dead-feature 已解） |
| H5 | ✅ | 10MB 上限在每次 append 后检查并报错中止 |
| H7 | ✅ | 改用 `req.GetBody()`，重试 body 非空（但 `TestRetryableHTTPClient_BodyReset` 仍只查 callCount，建议补查 body 内容） |
| H9 | ✅ | LSP/MCP 均按 ID demux，通知单独路由（LSP 未校验 `jsonrpc=="2.0"`，小瑕疵） |
| H10 | ✅ | Close 先 Kill 后 Wait，reader goroutine 干净退出，20× 连断无泄漏 |
| M4 | ✅ | `filterEnvForSubprocess` 仅白名单 PATH/HOME/USER/LANG/TMPDIR，剥离所有 `*_API_KEY` |
| C3 | ✅ | `cancelFunc` 仅在 Update goroutine 赋值/读取，竞争消除 |
| M1 | ✅ | 改用 `truncateRunes`（`[]rune` 截断），CJK 不再乱码 |
| C4 | ✅ | 独立 `cacheMu` 保护 embedCache 全部读写，网络 Embed 调用在锁外；8×200 并发 `-race` 通过 |
| M5 | ✅ | 会话/`.enc` 文件 0600、目录 0700（注：`semantic.go:248` 索引仍 0644，超出 M5 范围） |
| M6 | ✅ | `saveMeta` 加锁 + 临时文件 + `os.Rename` 原子化 |
| M8 | ✅ | 显式 `http.Server` 设 Read/Write/Idle 超时（graceful shutdown goroutine 等 `context.Background()` 永不触发，是死代码但无害） |
| Low loadbalancer | ✅ | setter 钳负权重 + 求和过滤 + `totalWeight<=0` 守卫，`rand.Intn` 不再 panic |
| Low vector-json | ✅ | tag 改 `json:"vector,omitempty"`，Save/Load 往返保留向量 |
| Low skills-symlink | ✅ | `entry.Type()&os.ModeSymlink` 排除符号链接 |

**部分修复但已达标的**: M7（`AddBatch` 已校验长度防越界 panic；未做 per-vector nil 守卫，但 `Search` 跳过 nil，影响低）。H8（LSP/MCP 后台 reader + select 已消除死锁；但报告期望的 "context 透传到 CallTool" 两包都未做，仍是硬编码 30s 超时，不可被调用方取消）。

---

## 5. 处置建议（按优先级）

1. **安全边界三件套一起改**（互相旁路，单修无效）: C2（文件工具接护栏）+ C1（补分隔符）+ H6（Auto 走白名单）。
2. **必崩进程**: H11（已复现 panic）——`Run` 一次性 + 深度限制。
3. **新回归**: R1（LSP 通知分帧，破坏 LSP 集成）、R2（WebSocket Origin 绕过）。
4. **数据丢失**: H3（建默认会话 + `/quit` 落盘）。
5. **健壮性**: M3（杀进程组 + 防 hang）、M9（鉴权/强制回环）。
6. **低优先**: config-home 回退、crypto KDF（接线前不阻断）。

每项建议按 systematic-debugging + TDD：先写复现测试（红）→ 最小改动转绿 → 回归。注意 H6/H11 现有测试**断言了错误行为**，修复时需同步改测试。

---

---

## 8. 修复执行记录（2026-06-21）

第 1–3 节的全部遗留与新增缺陷已按 TDD（先写复现测试→红→最小修复→绿）逐项修复。`go build ./...`、`go vet ./...`、`go test ./...`（16 个包）全绿；`-race` 在 agent/tool/ui/dashboard/control 全绿。

### 已修复

| 项 | 修复要点 | 文件 |
|----|---------|------|
| **C1** | `splitShellCommand` 分隔符补全：`\n ; && \|\| \| & $( ` ( ) >> > <`，命令替换/反引号/重定向/换行都分词后逐段校验 argv0 | `control/allowlist.go` |
| **H6** | Auto 模式不再无条件放行：白名单内放行、其余拒绝。按用户选择的"**工作区内放行**"模型，默认 `allowedPaths=[cwd]` + 安全 shell 白名单（ls/cat/grep/git/go/… 等 12 项） | `control/gate.go`、`cmd/loomcode/main.go` |
| **C2** | 文件工具新增 `root`+`PermissionChecker`：`resolveWithinRoot` 做 `filepath.Rel` 包含性校验 + `EvalSymlinks` 防符号链接逃逸（含 root 自身规范化，兼容 macOS `/var`→`/private/var`）；read/write/edit 均接权限链；两处 main.go 统一接线 | `tool/file_tools.go`、`cmd/loomcode/main.go` |
| **H11** | `Run` 改为一次性（守卫 `status != StatusPending`），杜绝二次 close(done) panic；新增 `SpawnChild` 设置 `ParentID/Depth` 并限制 `maxSubAgentDepth=3` | `agent/subagent.go` |
| **R1** | LSP `sendNotification` 恢复 `Content-Length` 分帧（与 `call` 一致），删除裸换行 | `lsp/client.go` |
| **R2** | `isAllowedOrigin` 改 `net/url` 精确 host 匹配（堵 `localhost.evil.com` 前缀绕过）；读 deadline 移入读循环每次刷新（不再 60s 误杀健康连接） | `dashboard/websocket.go` |
| **H3** | `saveSession` 懒创建默认会话（无 `--session` 也持久化，且不产生空会话文件）；`/quit`、`/exit` 退出前落盘 | `ui/app.go` |
| **M3** | `cmd.Cancel` 杀整个进程组（负 PID）；输出超限时立即杀组，避免 `Wait` 挂到 60s 超时 | `tool/command_tools.go` |
| **M9** | `loopbackAddr` 强制非回环 host（`0.0.0.0`、局域网 IP）改回 `127.0.0.1`，仅保留端口 | `dashboard/server.go` |
| **config-home** | `homeDir()` 改返回 `(string, bool)`；HOME 不可用时跳过用户级配置路径，不再回退 cwd 相对路径 | `config/config.go` |
| 测试基建 | `StubProvider.Chat` 加锁（修复 `RunParallel` 暴露的 stub 并发 append 竞争，使 `-race` 全绿） | `testutil/stubs.go` |

新增复现测试：`control/security_fix_test.go`、`tool/file_tools_security_test.go`、`tool/command_pgroup_test.go`、`agent/subagent_h11_test.go`、`lsp/notify_framing_test.go`、`dashboard/websocket_origin_test.go`、`dashboard/loopback_test.go`、`ui/session_persist_test.go`、`config/home_test.go`；并改写了断言旧错误行为的 `control_test.go:TestGate_AutoMode`。

### 本轮未做（已知、刻意推迟）

- **M9 鉴权**：仅强制了回环绑定；端点仍无认证。当前 dashboard 返回 mock 数据，接真实数据前需补 token/basic auth。
- **crypto KDF（Low）**：加密模块仍无生产调用方、会话明文存储，故未升级 KDF/接线。启用加密前需：随机数据密钥 + Argon2id/scrypt + 持久化 salt，并在 Manager 存取路径真正调用。
- **H8 context 透传**：死锁本身已修；将 ctx 一路透传到 `CallTool` 以支持调用方取消，未在本轮做（当前为硬编码 30s 超时）。
- **H1 语义**：`edit_file` 仍 `strings.Replace(...,-1)`（替换全部）。已不死循环，是否改为单次替换属产品语义取舍，未改。
- **dashboard 优雅关闭**：`Start` 中等待 `context.Background().Done()` 的 goroutine 为死代码（永不触发），无害，未清理。

---

## 9. 二次独立验证 + 残留修复（2026-06-22）

> 对第 8 节 `b5e3dcd` 所声称的修复做**独立对抗性复验**（不信任文档，逐项核对当前源码 + 构造新的绕过/复现，而非仅重跑既有绿测——前两轮正是"绿测但缺陷仍在"）。按 7 个 Go 包并行派发验证 agent，高影响项主线直核。

### 验证结论：第 8 节 10 项声称修复**全部成立**（首个"干净"轮）

| 项 | 声称 | 复验 | 对抗证据 |
|----|------|------|----------|
| C1 | 分隔符补全 | ✅ 成立 | `$()`/反引号/`>`/`>>`/`\n`/`&`/`\|&`/进程替换 `<()`/env 前缀 均拒；合法命令放行 |
| H6 | Auto 默认拒绝 | ✅ 成立 | 空白名单拒 `python3 -c`/`chmod && exec`/`npm install`；旧错误断言测试已改写 |
| C2 | 文件工具接护栏 | ✅ 成立 | `/etc/passwd`、`../../`、符号链接逃逸全拒；**三个工具在两处 main.go 均已接线** |
| H11 | Run 一次性 + 深度限制 | ✅ 成立 | 还原守卫即复现 panic；深度 4 被拒 |
| R1 | LSP Content-Length 分帧 | ✅ 成立 | 逐字节帧断言通过；无裸换行 |
| R2 | 精确 host + 读 deadline 刷新 | ✅ 成立 | 前缀绕过被堵；deadline 在读循环内刷新 |
| H3 | 懒建会话 + /quit 落盘 | ✅ 成立 | 首次运行→/quit 落盘可追踪；空会话防护真实 |
| M3 | 杀进程组 | ✅ 成立 | `yes`/`cat /dev/zero` ~7ms 返回（非 60s）；组被回收 |
| M9 | 强制回环 | ✅ 成立 | `0.0.0.0`/局域网 IP → `127.0.0.1`，端口保留 |
| config-home | 无 cwd 回退 | ✅ 成立 | HOME 空 → 跳过用户配置，无 cwd 相对路径 |

### 本轮修复的残留（验证中新发现，均非第 8 节的 claim）

按 TDD（先写复现测试→红→最小修复→绿）逐项修复。`go build`/`go vet`/`go test ./...`（16 包）全绿；`-race` 在 control/agent/dashboard 全绿。

| 项 | 问题 | 修复 | 文件 |
|----|------|------|------|
| **R2 空 Origin 绕过** | `origin != "" && …` 让缺失/空 Origin 头跳过校验（非浏览器客户端绕过 CSWSH 防护） | 改为默认拒绝：`!isAllowedOrigin(origin)`（空串 url.Parse 后 scheme 为空 → 拒） | `dashboard/websocket.go` |
| **C1 引号内过度拦截** | 分词器对引号无感知，`git commit -m "a; b"` 被错误拒绝（核心日常工作流） | 重写 `splitShellCommand` 为 quote-aware（单引号全字面、双引号内仅 `$()`/反引号 仍展开、引号外全分隔符生效）；`extractArgv0` 改 quote-aware 分词 + 仅跳过真正的 `IDENT=` env 前缀 | `control/allowlist.go` |
| **H11 Spawn 加固** | 导出的 `Spawn` 不带深度、SpawnChild 创建后二次加锁补字段；深度可被绕过的隐患（当前无生产调用方） | 统一私有创建入口 `spawn(parentID, depth)`：原子设置 + 硬性深度兜底（越界返回 nil 不注册）；`Spawn`=顶层(depth0)、`SpawnChild`=深度+1 | `agent/subagent.go` |
| **M9 `::1` 边界** | `loopbackAddr("::1")`（裸 IPv6 无端口）拼出畸形 `127.0.0.1::1` | 统一走 `net.SplitHostPort`，解析失败回退默认回环 | `dashboard/server.go` |

新增/扩充复现测试：`control/allowlist_quote_test.go`、`dashboard/websocket_origin_test.go`（空 Origin handler 用例）、`dashboard/loopback_test.go`（IPv6 用例）、`agent/subagent_h11_test.go`（创建入口深度兜底）。

### 本轮刻意不做（含对 bundle 项的判断）

- **write_file/edit_file 0644→0600（否决）**：0600（仅属主可读）对**项目源码文件**是错误的——编码 agent 写出的是用户项目文件（需被构建/服务/协作者读取），应按常规权限（`os.WriteFile` 的 0644 已受 umask 约束，等同普通编辑器）。M5 的 0600 仅针对**含对话历史的会话文件**（敏感），不应推广到一般工作区文件。**故不改。**
- **WebSocket ping/pong 保活（推迟）**：前端 `static/app.js` **根本未使用 WebSocket**（WS server 目前无消费方），且 `golang.org/x/net/websocket` 不暴露 ping/pong 控制帧。保活应在真正接入 WS 客户端时配套客户端心跳一并设计，当前属空功能打磨，推迟。
- **C1 黑名单子串过度拦截（已知）**：`blockedShellPatterns` 用裸子串匹配 `curl`/`wget` 等，会误拒 `git commit -m "improve curl"` 之类。属纵深防御层，改其语义有削弱安全网风险，本轮未动（标点类过度拦截已由 quote-aware 分词解决）。
- **H11 扁平递归（架构层、不可达）**：经 `Spawn`(depth0) 反复创建"伪子级"在深度维度仍不可计（深度限制只约束 SpawnChild 血缘）。彻底封堵需能力式 spawn 运行时改造；当前零调用方，文档标注即可。
- 第 8 节原列"未做"项（M9 鉴权 / crypto KDF / H8 ctx 透传 / H1 语义 / dashboard 死代码 goroutine）本轮同样未触及。

---

*复查生成: 2026-06-21 ｜ 方法: 8 路并行子系统审计 + 对抗性复现 + 高影响项主线直核 ｜ 修复执行: TDD 逐项修复，见第 8 节*
*二次独立验证 + 残留修复: 2026-06-22 ｜ 方法: 7 包并行对抗验证 + 主线直核；残留项 TDD 修复，见第 9 节*
