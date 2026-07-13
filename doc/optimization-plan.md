# LoomCode 项目优化方案

> 审计时间：2026-07-11  
> 审计范围：全项目代码（internal/、cmd/、TUI 交互、架构设计）  
> 审计人：TRAE AI Agent  
> 客观信号：`go build ./...` ✅ `go vet ./...` ✅

---

## 目录

- [一、TUI 交互体验优化](#一tui-交互体验优化)
  - [P0 — 严重问题（影响可用性）](#p0--严重影响可用性)
  - [P1 — 重要改进（影响核心体验）](#p1--重要改进影响核心体验)
  - [P2 — 体验改进](#p2--体验改进)
  - [P3 — 细节打磨](#p3--细节打磨)
- [二、项目架构优化](#二项目架构优化)
  - [高优先级](#高优先级)
  - [中优先级](#中优先级)
  - [低优先级](#低优先级)
- [三、错误处理与日志](#三错误处理与日志)
- [四、配置与硬编码](#四配置与硬编码)
- [五、安全审计](#五安全审计)
- [六、建议修复路线图](#六建议修复路线图)

---

## 一、TUI 交互体验优化

基于 `internal/ui/app.go`（1859 行）的全面审查，共发现 50 个优化点，以下按优先级列出。

### P0 — 严重问题（影响可用性）

#### 1.1 欢迎界面宣传不存在的 @ mention 功能

- **文件**：`internal/ui/app.go` L1429
- **问题**：欢迎界面显示 `"Press / to use commands, @ to mention files"`，但全代码库中没有任何 `@` 文件提及功能的实现。用户尝试输入 `@` 会发现没有任何效果，造成困惑。
- **修复方案**：删除 `@ to mention files` 提示，或实现文件提及功能。

#### 1.2 强制深色背景，亮色终端不可用

- **文件**：`internal/ui/app.go` L440
- **问题**：`lipgloss.SetHasDarkBackground(true)` 硬编码强制深色背景。在亮色背景终端（如 macOS Terminal 默认白底）上，白色/浅色文字不可见。
- **修复方案**：改为检测失败时回退到深色，而非强制覆盖：
  ```go
  // 检测终端背景色，检测失败时回退到深色
  hasDark := lipgloss.HasDarkBackground()
  if !hasDark {
      // 尝试查询，失败则默认深色
      hasDark = true
  }
  lipgloss.SetHasDarkBackground(hasDark)
  ```

#### 1.3 glamour 渲染器硬编码 DarkStyle

- **文件**：`internal/ui/app.go` L226-228, L471-474
- **问题**：Markdown 渲染器始终使用 `glamourstyles.DarkStyle`，无法跟随终端实际背景色。在亮色终端上 Markdown 渲染效果很差。
- **修复方案**：根据 `lipgloss.HasDarkBackground()` 选择 `DarkStyle` 或 `LightStyle`。

#### 1.4 无最小终端尺寸检查

- **文件**：`internal/ui/app.go` L451-474
- **问题**：`WindowSizeMsg` 处理中 `a.height - 8` 在终端高度 < 8 时会变成负数，viewport 高度为负会导致 panic 或渲染异常。
- **修复方案**：
  ```go
  if a.height < 10 || a.width < 40 {
      // 显示"终端窗口太小"提示
      return a, nil
  }
  ```

---

### P1 — 重要改进（影响核心体验）

#### 1.5 缺少输入历史回溯功能

- **文件**：`internal/ui/app.go` 全文
- **问题**：代码中没有任何输入历史记录机制。用户无法用上箭头快速调出上次输入，这是 TUI 交互的重大缺失。
- **修复方案**：
  1. 在 App 结构体中添加 `inputHistory []string` 和 `historyIndex int`
  2. 发送消息时追加到历史
  3. 上/下箭头在输入为空时回溯历史
  4. 可选：持久化到 `~/.loomcode/history`

#### 1.6 textarea 高度固定为 3 行

- **文件**：`internal/ui/app.go` L214
- **问题**：`ta.SetHeight(3)` 硬编码高度，多行输入时用户无法看到全部内容，超出部分需要内部滚动。
- **修复方案**：在 `WindowSizeMsg` 或输入变化时动态调整高度，例如根据内容行数 + 1（最大不超过终端高度的 1/3）。

#### 1.7 加载状态无 spinner 动画

- **文件**：`internal/ui/app.go` L1251-1258
- **问题**：加载时只显示静态文本"思考中..."，没有 spinner 动画。长时间等待时用户无法确认程序是否卡死。
- **修复方案**：引入 `bubbles/spinner`，在 `loading` 状态下显示旋转动画：
  ```go
  type spinnerMsg struct{}
  // 在 init() 中启动 spinner
  // loading 时渲染 spinner.View() + "思考中..."
  ```

#### 1.8 viewport 高度计算不准确

- **文件**：`internal/ui/app.go` L461 等 18 处使用 `a.height - 8`
- **问题**：`height - 8` 假设标题栏(1) + 分隔线(1) + 空行(1) + 活动状态(1) + 输入区(3) + 状态栏(1) = 8 行。但当 `pendingApproval` 弹出或 `showSuggestions` 时，额外行数未计入偏移，导致布局错乱。
- **修复方案**：提取 `calcViewportHeight()` 函数，动态计算偏移：
  ```go
  offset := 8 // 基础偏移
  if a.pendingApproval != nil {
      offset += approvalHeight
  }
  if a.showSuggestions {
      offset += len(a.suggestions)
  }
  ```

#### 1.9 窗口缩放后渲染缓存未失效

- **文件**：`internal/ui/app.go` L471-474, L1473-1482
- **问题**：窗口大小变化时重建了 `glamourRenderer`（新的 WordWrap 宽度），但 `renderCache` 中缓存的旧渲染结果（按旧宽度渲染）不会被清除。缩放窗口后，已缓存的消息仍以旧宽度显示。
- **修复方案**：在 `WindowSizeMsg` 中清空 `renderCache`：
  ```go
  a.renderCache = make(map[string]string)
  ```

#### 1.10 审批对话框无"全部允许"选项

- **文件**：`internal/ui/app.go` L538-557
- **问题**：每次文件写入/编辑都需要单独审批，批量操作时体验极差。只有 `Enter`(确认) 和 `Esc`(拒绝)。
- **修复方案**：增加 `A` 键——"全部允许"，设置 `autoApprove = true`，后续操作不再弹出审批。

#### 1.11 审批 diff 截断后无提示

- **文件**：`internal/ui/app.go` L1620-1622, L1652-1653
- **问题**：大文件 diff 被截断到 30 行(write)或 20 行(edit)后，没有显示"... 还有 N 行未显示"的提示，用户可能误以为看到了完整 diff 就批准了。
- **修复方案**：截断时追加提示行：
  ```go
  if len(lines) > maxLines {
      diff = strings.Join(lines[:maxLines], "\n")
      diff += fmt.Sprintf("\n... 还有 %d 行未显示（共 %d 行）", len(lines)-maxLines, len(lines))
  }
  ```

#### 1.12 Ctrl+C 直接退出无二次确认

- **文件**：`internal/ui/app.go` L633-636
- **问题**：`Ctrl+C` 立即保存并退出，没有"再按一次确认退出"的机制。用户误按会丢失上下文。
- **修复方案**：
  ```go
  case "ctrl+c":
      if a.confirmQuit {
          return a, tea.Quit
      }
      a.confirmQuit = true
      a.addSystem("再按一次 Ctrl+C 确认退出")
      // 3 秒后重置 confirmQuit
  ```

#### 1.13 无 SIGTERM/SIGHUP 信号处理

- **文件**：`internal/ui/app.go` 全文
- **问题**：只处理 `Ctrl+C`（tea.KeyMsg），未处理 `SIGTERM`/`SIGHUP` 等系统信号。终端关闭或 kill 进程时不会执行 `saveSession`，对话丢失。
- **修复方案**：在 `main.go` 中捕获信号并通知 TUI：
  ```go
  sigCh := make(chan os.Signal, 1)
  signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGHUP)
  go func() {
      <-sigCh
      p.Send(tea.Quit())
  }()
  ```

---

### P2 — 体验改进

#### 1.14 /help 未列出键盘快捷键

- **文件**：`internal/ui/app.go` L757-792
- **问题**：帮助文本只列出斜杠命令和少量提示，完全缺少 `Ctrl+A/E/K/U/W`、`PgUp/PgDn`、`Esc`、`Ctrl+C` 等快捷键说明。
- **修复方案**：在帮助文本中增加"键盘快捷键"章节。

#### 1.15 大段粘贴时 Enter 可能被解释为发送信号

- **文件**：`internal/ui/app.go` L651
- **问题**：没有针对粘贴的特殊处理。大段多行文本粘贴时，`Enter` 字符可能被解释为发送信号。
- **修复方案**：检测 bracketed paste mode，粘贴期间临时禁用 Enter 发送。

#### 1.16 friendlyError 覆盖面有限

- **文件**：`internal/ui/app.go` L1721-1749
- **问题**：未覆盖 DNS 解析失败(`no such host`)、代理错误(`proxy`)、配额耗尽(`quota`/`insufficient_quota`)、计费相关(`billing`)、内容过滤(`content filter`/`safety`)等常见错误。
- **修复方案**：扩展错误匹配规则，增加上述场景的友好提示。

#### 1.17 模型选择器无搜索过滤

- **文件**：`internal/ui/app.go` L983-1043
- **问题**：模型列表只能用上下箭头逐个浏览，当 provider 有几十个模型时体验很差。
- **修复方案**：在模型选择器中支持输入过滤，类似 fuzzy finder。

#### 1.18 命令联想仅前缀匹配

- **文件**：`internal/ui/app.go` L692-698
- **问题**：只用 `strings.HasPrefix` 匹配，输入 `/h` 只匹配 `/help`。不支持模糊匹配。
- **修复方案**：支持子串匹配或 fuzzy matching。

#### 1.19 流式取消后已接收内容丢失

- **文件**：`internal/ui/app.go` L638-646
- **问题**：取消请求时只追加"请求已取消"系统消息，`streamBuf` 中的部分内容被丢弃。
- **修复方案**：取消前检查 `streamBuf` 是否有内容，如有则保存为部分响应。

#### 1.20 欢迎界面 tips 为英文

- **文件**：`internal/ui/app.go` L1426-1432
- **问题**：tips 内容为英文，而 `/help` 和所有系统消息均为中文，语言风格不统一。
- **修复方案**：统一为中文。

#### 1.21 无 NO_COLOR 环境变量支持

- **文件**：`internal/ui/app.go` 全文
- **问题**：业界惯例 `NO_COLOR` 环境变量禁用所有颜色输出，当前代码未检查此变量。
- **修复方案**：在初始化时检查 `os.Getenv("NO_COLOR")`，禁用颜色输出。

#### 1.22 预算耗尽提示不醒目

- **文件**：`internal/ui/app.go` L266-272
- **问题**：预算用尽时只追加一条 `system` 消息，使用灰色斜体样式，与普通系统消息无异。
- **修复方案**：使用 `errorStyle` 或更醒目的样式（红色加粗）。

---

### P3 — 细节打磨

| # | 问题 | 文件 | 行号 |
|---|------|------|------|
| 1.23 | 缺少 Ctrl+L 清屏快捷键 | app.go | L632-671 |
| 1.24 | 缺少 Ctrl+D 快捷键 | app.go | L632-671 |
| 1.25 | Tab 键功能过载（切换模式 + 命令联想） | app.go | L654-662 |
| 1.26 | 命令联想缺少 Ctrl+N/Ctrl+P 导航 | app.go | L606-630 |
| 1.27 | Approval 模式下所有非 Enter/Esc 按键被吞 | app.go | L538-557 |
| 1.28 | textarea 宽度初始化为 80（硬编码） | app.go | L215 |
| 1.29 | 分隔线在极窄终端下溢出 | app.go | L1243 |
| 1.30 | 状态栏窄终端下内容被压缩 | app.go | L1506-1509 |
| 1.31 | renderMessages 的 visibleLines 参数未使用 | app.go | L1299 |
| 1.32 | /clear 使用 `[:0]` 未释放底层数组 | app.go | L837 |
| 1.33 | 任务队列无删除/重排功能 | app.go | L850-861 |
| 1.34 | /compact 压缩阈值和保留条数硬编码 | app.go | L1174-1213 |
| 1.35 | 会话切换无未保存提醒 | app.go | L1128-1146 |
| 1.36 | renderCache 无 LRU 策略 | app.go | L1474-1481 |
| 1.37 | 未知命令不列出最接近的命令建议 | app.go | L887-891 |
| 1.38 | 流式输出无 token 生成速率指示 | app.go | L1532-1550 |
| 1.39 | 错误提示无重试操作 | app.go | L503-514 |
| 1.40 | 欢迎界面 Logo 256 色码兼容性存疑 | app.go | L1407-1409 |
| 1.41 | 无首次使用引导 | app.go | L1402-1454 |
| 1.42 | 模型选择器窄终端下不换行 | app.go | L1343-1383 |
| 1.43 | 窗口缩放时 viewport 滚动位置可能错乱 | app.go | L460-468 |

---

## 二、项目架构优化

### 高优先级

#### 2.1 main.go 装配逻辑过重

- **文件**：`cmd/loomcode/main.go`（~600 行）
- **问题**：`createProvider`、`configureToolPermissions`、`connectPlugins`、`registerAutoFormatHook` 等装配函数全部塞在 main.go，`runCommand` 与 `chatCommand` 装配代码大量重复。
- **修复方案**：抽取到 `internal/bootstrap` 包，提取公共 `buildAgent` 工厂，main.go 仅做 flag 解析与分发。

#### 2.2 Provider 注册硬编码

- **文件**：`cmd/loomcode/main.go`
- **问题**：`reg.Register(&openai.Adapter{})` 等三处硬编码，新增 Provider 必须改 main.go。
- **修复方案**：用 `init()` 自注册或显式注册表，符合开闭原则。

#### 2.3 记忆系统未实现

- **文件**：`internal/tool/memory_tool.go`
- **问题**：README 宣传"SQLite FTS5 记忆存储"，但无 `internal/memory/` 包。`RecallMemoryTool` 默认返回 "No memory configured."
- **修复方案**：实现基于 SQLite FTS5 的记忆持久化，或修正 README 描述。

#### 2.4 会话加密未接入生产

- **文件**：`internal/session/crypto.go` L36-37
- **问题**：AES-256-GCM 加密器实现完整，但 `deriveKey` 用随机密钥（每次启动不同，无法跨重启解密）。会话实际明文存储。
- **修复方案**：改为基于用户密码 + Argon2id/scrypt + 随机 salt 的密钥派生方案。

#### 2.5 Dashboard 全部返回 Mockup 数据

- **文件**：`internal/dashboard/handlers.go`
- **问题**：`handleSessions`/`handleCost`/`handleStatus` 返回写死的模拟值，Dashboard 无法反映真实会话/成本/状态。
- **修复方案**：向 Server 注入 `*session.Manager` 与 `*agent.EventLog`，返回真实数据。

### 中优先级

#### 2.6 deepseek 与 openai provider 代码大量重复

- **文件**：`internal/provider/deepseek/provider.go`、`internal/provider/openai/provider.go`
- **问题**：`Chat`、`Stream`、`parseChatResponse`、`truncateSensitive`、`Cost` 几乎逐行相同。
- **修复方案**：提取 `BaseOpenAICompatibleProvider` 基类，子类只覆盖差异（reasoning_content 解析、usage 字段名、capabilities）。

#### 2.7 LoadBalancer 的 Models()/Capabilities() 只返回 providers[0]

- **文件**：`internal/provider/loadbalancer.go` L381-402
- **问题**：多 provider 负载均衡下，首个 provider 的能力声明会误导上层（如 CacheTTL、MaxToolCallsPerRound）。
- **修复方案**：按当前选中 provider 返回，或聚合声明。

#### 2.8 Stream 未记录指标

- **文件**：`internal/provider/loadbalancer.go` L366-373
- **问题**：`Chat` 记录了延迟与失败，`Stream` 直接透传未调 `RecordRequest`，导致 LeastLatency/CostOptimized 策略在流式场景下无数据支撑。
- **修复方案**：在 `Stream` 中也调用 `RecordRequest`。

#### 2.9 Run 与 RunStream 严重重复

- **文件**：`internal/agent/loop.go` L497-627 vs L281-458
- **问题**：消息初始化、压缩、指纹记录、成本累计、工具执行回填逻辑几乎一致，仅流式/非流式与 channel 处理不同，约 130 行重复。
- **修复方案**：抽取 `runOnce(stepCtx)` 共享核心逻辑。

#### 2.10 estimateTokens 过于粗糙

- **文件**：`internal/agent/context.go`
- **问题**：`len(content)/3` 对 CJK 与代码 token 估算偏差大，可能导致压缩触发时机不准。
- **修复方案**：接入 provider 的 tokenizer 或用更精确的启发式（按 BPE 分词）。

#### 2.11 AddMessage 每条消息一次 fsync

- **文件**：`internal/session/session.go` L321-324
- **问题**：高频对话时 IO 开销大。
- **修复方案**：批量/延迟 fsync 或可配置 fsync 策略。

#### 2.12 会话无上限/清理机制

- **文件**：`internal/session/session.go`
- **问题**：`~/.loomcode/sessions/` 无限增长。
- **修复方案**：加 TTL 或数量上限自动归档。

#### 2.13 MCP 工具 IsReadOnly() 恒返回 false

- **文件**：`internal/mcp/manager.go` L48
- **问题**：所有 MCP 工具都被当写入工具串行执行，即使只读的 MCP 工具也无法并行，性能损失。
- **修复方案**：从 MCP 工具元数据推断或允许配置。

### 低优先级

| # | 问题 | 文件 |
|---|------|------|
| 2.14 | `truncateSensitive` 在三个 provider 中重复实现 | openai L249, deepseek L315, mimo L321 |
| 2.15 | `costOptimized` 用 `models[0]` 估算成本，多模型 provider 下失真 | loadbalancer.go L248-252 |
| 2.16 | `runMax` 评分回退粗糙（纯靠长度启发式） | modes.go L427-445 |
| 2.17 | `runPlan`/`runCompose` 未走 Agent 抽象 | modes.go L330-383 |
| 2.18 | `archiveDroppedMessages` 用时间戳生成假 sessionID，无清理机制 | context.go L120 |
| 2.19 | SubAgent `RunParallel` 消息聚合用字符串拼接 | subagent.go L326-334 |
| 2.20 | `partition` 函数为死代码 | executor.go L149-159 |
| 2.21 | `pruneResult` 剪枝对 JSON/代码等结构化内容可能破坏语义 | executor.go L221-236 |
| 2.22 | `loadSessions` 全量加载到内存 | session.go L342-365 |
| 2.23 | skills `Load()` 吞掉所有错误 | manager.go L48-52 |
| 2.24 | skills 无热加载 | manager.go |
| 2.25 | skills 无项目级目录支持 | manager.go |

---

## 三、错误处理与日志

### 3.1 34 处 log.Printf 无日志级别区分

- **范围**：全项目（config/agent/tool/dashboard/mcp/lsp/ui）
- **问题**：全部使用标准库 `log` 包，无任何日志级别区分。生产环境无法按级别过滤日志，排障困难。部分代码用字符串 `"Warning:"`/`"WARN:"` 手动标注级别。
- **修复方案**：引入 `log/slog`（Go 1.21+ 内置），统一日志级别为 Debug/Info/Warn/Error。

**受影响文件清单**：

| 文件 | 行号 | 当前写法 | 建议级别 |
|------|------|----------|----------|
| config/env.go | 21, 27, 32, 56 | `"Warning: ..."` | WARN |
| agent/fingerprint.go | 75 | `"[fingerprint] WARN: ..."` | WARN |
| agent/context.go | 105, 117, 123, 129, 133 | 各种错误 | ERROR |
| agent/loop.go | 489 | unmarshal 错误 | WARN |
| tool/command_tools.go | 98, 164, 234, 238 | kill/wait 错误 | WARN |
| tool/registry.go | 80 | register 错误 | ERROR |
| dashboard/websocket.go | 59, 68 | 连接/断开 | INFO |
| dashboard/server.go | 119 | shutdown 错误 | ERROR |
| mcp/sse_client.go | 166, 471 | 重连/通知错误 | ERROR |
| mcp/client.go | 438, 446, 451 | 通知错误 | ERROR/WARN |
| mcp/plugin.go | 177 | lifecycle hook 错误 | ERROR |
| lsp/client.go | 246, 254, 262, 266 | 通知错误 | ERROR |
| lsp/discovery.go | 285 | walk 错误 | WARN |
| ui/app.go | 196, 394, 1143 | 加载/保存/激活错误 | ERROR |

### 3.2 错误信息过于简略

| 文件 | 行号 | 当前错误信息 | 问题 |
|------|------|-------------|------|
| session/crypto.go | 93 | `"ciphertext too short"` | 未说明最小长度要求 |
| session/replay.go | 43 | `"no session to replay"` | 未说明如何创建 session |
| session/session.go | 222 | `"no active session"` | 缺乏操作指引 |
| provider/loadbalancer.go | 353, 369 | `"no available provider"` | 未列出尝试过哪些 provider 及失败原因 |
| provider/oauth.go | 61 | `"no token set"` | 未说明如何设置 token |
| lsp/client.go | 216 | `"server returned error"` | 无任何错误详情 |
| mcp/sse_client.go | 419 | `"timeout waiting for response"` | 未包含请求 ID 或超时时长 |

### 3.3 错误直接透传未包装上下文

多处 `return nil, err` / `return err` 直接透传底层错误，未添加当前操作上下文：

- `internal/session/session.go`：L241, 250, 254, 259, 274, 284, 289, 295, 300, 371
- `internal/tool/websearch.go`：L107, 113, 125, 139, 185, 192, 204, 216, 253, 258, 270, 282
- `internal/tool/git_tools.go`：L32, 37, 179, 190, 199, 288, 304, 317, 400, 409

---

## 四、配置与硬编码

### 4.1 超时参数硬编码（应归集到 consts 或配置）

| 文件 | 行号 | 硬编码值 | 说明 |
|------|------|----------|------|
| tool/command_tools.go | 48 | `bashTimeout = 60s` | bash 命令超时 |
| tool/websearch.go | 91, 164, 241 | `Timeout: 10s` | 搜索引擎 HTTP 超时（3 处重复） |
| mcp/sse_client.go | 48 | `Timeout: 30s` | SSE 客户端 HTTP 超时 |
| mcp/sse_client.go | 240 | `heartbeatTimeout = 60s` | 心跳超时 |
| mcp/sse_client.go | 407 | `30s` | 等待响应超时 |
| mcp/client.go | 42 | `rpcTimeout = 30s` | RPC 超时 |
| dashboard/server.go | 106-108 | `10s, 30s, 60s` | Dashboard 服务器超时 |
| dashboard/websocket.go | 136, 153, 169 | `10s, 120s, 10s` | WebSocket 读写超时 |
| lsp/client.go | 221 | `30s` | LSP 请求超时 |
| provider/oauth.go | 64, 161 | `60s` | OAuth token 提前刷新 |

### 4.2 重试/退避参数硬编码

| 文件 | 行号 | 硬编码值 |
|------|------|----------|
| provider/retry.go | 29-32 | `MaxRetries: 3, BaseDelay: 1s, MaxDelay: 30s` |
| provider/retry.go | 55-57 | `MaxIdleConns: 20, MaxIdleConnsPerHost: 10, IdleConnTimeout: 90s` |
| mcp/sse_client.go | 148-152 | `initialDelay: 1s, maxDelay: 30s, multiplier: 2.0, jitterPct: 0.25, maxAttempts: 10` |
| mcp/client.go | 39-41 | `defaultMaxRetries: 3, baseRetryDelay: 1s, maxRetryDelay: 30s` |

### 4.3 端口/地址硬编码

| 文件 | 行号 | 硬编码值 |
|------|------|----------|
| dashboard/server.go | 53, 57 | `"127.0.0.1:8080"` |
| tool/websearch.go | 329 | `"http://localhost:8888"` |

### 4.4 缓冲区/阈值/限制硬编码

| 文件 | 行号 | 硬编码值 |
|------|------|----------|
| tool/executor.go | 14 | `maxToolResultLines = 200` |
| tool/executor.go | 42, 54 | `maxParallel: 3`, 上限 `16` |
| control/cost.go | 66 | `compressThreshold: 3000` |
| agent/effort.go | 62-66 | `EffortLow: 20, EffortMedium: 50, EffortHigh: 100` |
| agent/loop.go | 282-283 | `make(chan string, 100)` |
| agent/message_bus.go | 31 | `make(chan BusMessage, 64)` |
| provider/deepseek/provider.go | 50 | `MaxToolCallsPerRound: 16` |
| provider/deepseek/provider.go | 49 | `CacheTTL: 5 * time.Minute` |

---

## 五、安全审计

### 5.1 Dashboard auth token 用 == 比较（时序攻击）

- **文件**：`internal/dashboard/server.go` L91, L95
- **问题**：`t == s.authToken` 用普通字符串比较，存在时序攻击风险。
- **修复方案**：使用 `subtle.ConstantTimeCompare`。

### 5.2 WebSocket /ws 未走 requireAuth 中间件

- **文件**：`internal/dashboard/server.go` L73
- **问题**：`handleWebSocket` 直接注册未套鉴权中间件。
- **修复方案**：在 WebSocket 握手时校验 token。

### 5.3 auth token 通过 URL query 传递

- **文件**：`internal/dashboard/server.go`
- **问题**：`?token=xxx` 会进入浏览器历史、Referer 头、服务器日志，存在泄露风险。
- **修复方案**：改用短期一次性 token 或 cookie。

### 5.4 openai Stream 错误路径未截断敏感内容

- **文件**：`internal/provider/openai/provider.go` L176-178
- **问题**：`Chat` 用了 `truncateSensitive`，`Stream` 错误分支直接 `string(body)`，不一致且有泄露风险。
- **修复方案**：在 Stream 错误分支也使用 `truncateSensitive`。

### 5.5 LoadEnvFiles 始终返回 nil

- **文件**：`internal/config/env.go` L16-36
- **问题**：即使加载失败也只记日志返回 nil，调用方无法感知配置加载失败。
- **修复方案**：返回 error 或至少在日志中标记为 ERROR 级别。

---

## 六、建议修复路线图

### 第一批 — 立即修复（P0 + 安全）

1. 删除 `@ mention` 虚假提示
2. 修复亮色终端兼容（`SetHasDarkBackground` + glamour style）
3. 增加最小终端尺寸检查
4. SIGTERM/SIGHUP 信号处理
5. Ctrl+C 二次确认
6. Dashboard auth token 常量时间比较
7. WebSocket 鉴权

### 第二批 — 核心体验（P1）

8. 输入历史回溯（上箭头调出上次输入）
9. textarea 自适应高度
10. spinner 加载动画
11. viewport 高度动态计算
12. 窗口缩放缓存失效
13. 审批"全部允许"选项
14. 审批 diff 截断提示

### 第三批 — 架构优化

15. 引入 `log/slog` 替换标准 log 包
16. 硬编码参数归集到 consts / 配置
17. main.go 装配逻辑解耦
18. Provider 适配器去重（提取基类）
19. Run/RunStream 去重
20. LoadBalancer 多 provider 语义修正

### 第四批 — 功能补全

21. Dashboard 真实数据接入
22. 记忆系统实现（SQLite FTS5）
23. 会话加密接入生产
24. 会话清理机制
25. MCP 工具只读推断

---

## 附录：项目概况

| 项 | 值 |
|---|---|
| 模块名 | `github.com/ShawnLiuSZ/loomcode` |
| Go 版本 | 1.25.0 |
| 定位 | 纯 CLI 形态、多模型、可扩展的 Agent 编程工具 |
| 核心依赖 | Bubble Tea (TUI)、Lip Gloss/Glamour (渲染)、modernc.org/sqlite、BurntSushi/toml |
| CGO | `CGO_ENABLED=0`（纯静态） |
| 测试规模 | README 声称 343 个测试通过 |
