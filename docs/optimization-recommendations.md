# LoomCode 优化建议文档

> 基于对 ~21k 行代码、110 个 `.go` 文件、17 个 `internal/` 包的实地审查（2026-06-19）
> 审查范围：终端用户体验 · 性能与成本 · 架构与代码质量 · 生态分发与集成

---

## 0. 一句话结论

**LoomCode 当前的瓶颈不是"还缺什么功能"，而是"已经写好的功能没接进真实执行路径"。**

`cmd/loomcode/main.go` 只 import 了 8 个 `internal/` 包；路线图标为 `✅ 完成` 的特性中，约有 7 项是**编译通过、测试通过、但 `main` 永远不会调用**的死代码或 stub。与此同时，唯一真正跑在主路径上的 TUI，存在一个让"AI 回复永不显示"的 P0 断裂。

所以本文档的核心建议是：**把投入从"继续造新功能"转向"接线 + 硬化已有功能"**——这是当前性价比最高的方向。下面所有结论都带 `文件:行号`，可直接定位。

---

## 1. 根因：建好了，但没接线

### 1.1 证据

- `go build ./...`、`go vet ./...`、`go test ./...` **全部通过**（实测）。问题不在编译正确性。
- `cmd/loomcode/main.go` 实际 import 的 `internal/` 包只有：`agent`、`config`、`dashboard`、`provider`(+deepseek/mimo/openai)、`session`、`tool`、`ui`。
- 以下包**未被任何非测试文件引用**（验证：跨包 grep 构造函数无命中）：
  `internal/cache`、`internal/control`、`internal/voice`、`internal/lsp`、`internal/mcp`、`internal/memory`、`internal/errors`，以及 `agent/dream.go`、`agent/hooks.go`。

### 1.2 现状矫正表（路线图 `✅ 完成` vs 真实状态）

| 路线图特性 | 标注 | **真实状态** | 证据 |
|---|---|---|---|
| 命令执行沙箱（权限） | ✅ P0 | 🔴 **确认未生效（安全）**——`BashTool` 的检查被 `if t.permission != nil` 包裹，而 `SetPermissionChecker` 从不被调用 → 主路径上 `permission` 恒为 nil，bash 任意命令无任何拦截执行；`internal/control` 整包未被 import | `tool/command_tools.go:46-47`；`SetPermissionChecker` 全代码零调用；`main.go:120,257` `RegisterDefaults()` 不注入 checker |
| Goal / Stop Condition | ✅ | 🟢 **真接入**（但有冗余调用，见 §3.2） | `agent/loop.go:223,250` |
| Dream & Distill | ✅ | 🔴 死代码——`DreamScheduler` 只在测试里实例化，依赖的 `memory.Dream()` 是"模拟实现" | `memory/layers.go:118` |
| Hooks 系统 | ✅ | 🔴 未接线——`HookManager` 存在，`loop.go` 的工具执行从不调用 | `agent/hooks.go` 无引用 |
| Web Dashboard | ✅ | 🟡 接了，但**全是硬编码假数据** | `dashboard/handlers.go:22,33,45` |
| Semantic Index | ✅ | 🟡 余弦计算是真的，但唯一 embedding 实现是 `MockEmbeddings`(hash 取 sin) | `memory/semantic.go:213-220` |
| MCP HTTP SSE / 插件系统 | ✅ | 🔴 `sse_client.go`/`plugin.go` 是真协议客户端，但 `PluginManager` 从不被 `Connect`；且 `plugin.go` 尚未提交 git | `mcp/manager.go` 无引用 |
| LSP 自动发现 | ✅ | 🔴 客户端真实，但无任何代码路径使用 | `lsp/*` 无引用 |
| Multi Provider 负载均衡 | ✅ | 🟡 `costOptimized()` 是 stub（直接返回 `providers[0]`）；是否真正用于选路需核实 | `provider/loadbalancer.go:206` |
| 分布式 Agent 协作 | ✅ | 🟡 名不副实——`distributed.go` 无任何 net/rpc/grpc，实为本地 goroutine 池 | `agent/distributed.go` |
| 性能优化（缓存） | ✅ | 🔴 `cache` 包完整但零处使用 | `cache/cache.go` 无引用 |
| CI/CD 流水线 | ✅ | 🔴 workflow 文件未提交（`git status` 显示 `?? .github/`），GitHub 上不运行 | `git ls-files .github/` 为空 |
| 错误处理统一 | ✅ | 🔴 `internal/errors` 整包零使用，代码里 160 处裸 `fmt.Errorf` | `errors/errors.go` 无引用 |
| voice（语音） | — | 🔴 整包死代码，`MiMoASR.Recognize` 是 stub | `voice/voice.go:71-77` |

> 图例：🟢 真接入 · 🟡 接了但残缺/假数据 · 🔴 死代码/stub · ⚠️ 安全相关，需立即核实

**这张表是本文档的主线。** 后续 P0/P1 清单基本都是在把 🔴/🟡/⚠️ 推回 🟢。

---

## 2. P0 清单：必须最先处理

> 排序即建议执行顺序。每条可直接转为开发任务。

| # | 问题 | 证据 | 修复方向 | 量级 |
|---|---|---|---|---|
| P0-1 | **TUI 永不显示 AI 回复**：流式文本被读出后 `_ = text` 丢弃 | `ui/app.go:752-773`（注释"实际流式需要改造 TUI 架构"），`streamChunkMsg` 通道已存在但从不喂数据 `app.go:166` | 用 `program.Send`/`tea.Cmd` 把每个 chunk 经 `streamChunkMsg` 回传，`streamDoneMsg` 时把 `streamBuf` 落为一条消息 | M |
| P0-2 | **命令沙箱确认未生效**（安全）：`BashTool` 的 `permission.Check` 被 `if t.permission != nil` 守卫，但 `SetPermissionChecker` 从不被调用 → 主路径上 agent 可无拦截执行任意 shell 命令 | `tool/command_tools.go:46-47`；`SetPermissionChecker` 零调用；`main.go:120,257` 未注入；`internal/control` 整包未 import | `RegisterDefaults()`/`main.go` 构造 `BashTool` 后注入 `control` 的 permission/allowlist；guard 改为"无 checker 即拒绝危险命令"而非放行；加测试 | S→M |
| P0-3 | **成本三件套全未接线**：`cache`、`context/partition`（前缀缓存）、`ContextWindow` 截断都没在主循环调用 → 每轮按原价重发全量历史，成本随步数二次增长 | `agent/loop.go:85-89,195-199,127-242`；`partition.BuildMessages()` 从不调用 | 主循环改走 `partition.BuildMessages()`（稳定 system 前缀命中 provider 缓存）+ 发请求前按 `ContextWindow` 截断/压缩历史 | M→L |
| P0-4 | **Skills 是空操作**：`/skills` 只列描述，从不注入 prompt 或注册为 tool | `skills/manager.go` 只读首行；`agent`/`tool` 无 `skills.` 引用 | 加载的 skill 注入 system prompt 或注册进 tool registry；否则把文档降级为"仅元数据" | M |
| P0-5 | **安装路径全部不可用**：CI 未提交 + `go.mod` 要求 `go 1.26.3` + 无 `LICENSE` + 无 release tag | `go.mod:3`；`?? .github/`；`ls LICENSE*` 无文件 | `go.mod` 降到 `go 1.23`、提交 `.github/workflows/`、补 `LICENSE`、打首个 tag 触发 goreleaser | S |

---

## 3. 分维度深挖

### 3.1 终端用户体验

主路径上唯一真实的入口是 TUI，但它的关键交互大量缺失或断裂。

| 严重度 | 问题 | 证据 | 修复 |
|---|---|---|---|
| P0 | AI 回复永不显示（见 P0-1） | `app.go:752-773` | 见 P0-1 |
| P1 | 成本显示恒为 `$0.0000`：`costSession/Total/Last` 声明并渲染，但全代码无赋值 | `app.go:70-72,414,742-750` | 从响应 usage 累加，或删掉这块 UI 以免误导 |
| P1 | `/sessions` 在命令表和 `/help` 里宣传，但 `handleCommand` 无对应 case → "未知命令" | `app.go:89,358`，switch `337-427` | 加 `/sessions` 处理 + 仿 `showModelPicker` 的交互式选择器 |
| P1 | 首次无 API key：TUI 正常启动，但首条消息在请求时才失败，且该错误可能因 P0-1 根本不渲染 | `config.go:115`，`main.go:230`，`provider.go:120` | 启动时检测缺 key，在 TUI 顶部显示可操作提示（"运行 loomcode setup 或设置 .env"） |
| P1 | `loomcode setup` 不是向导，只是打印说明并生成 `.env` 模板 | `main.go:141-169` | 改为交互式：选 provider → 粘贴 key → 写配置 → 跑一次测试请求 |
| P1 | 错误信息泄漏裸 Go error（HTTP/JSON/transport 直接 `%v` 给用户） | `main.go:100,114,239,252`，`app.go:182,766` | 对常见失败类（缺 key/401/网络/模型不存在）映射为短提示，原始错误藏到 `--verbose` |
| P1 | 无中断：运行中用 `context.Background()`，Esc 不能取消当轮，Ctrl+C 直接退整个程序 | `app.go:754` | 存 `CancelFunc`，运行时 Esc 取消当轮，Ctrl+C 保留为退出 |
| P1 | 无滚动回看：超过一屏的历史永久不可达，无 PgUp/PgDn | `app.go:636-645` | 引入 Bubbles `viewport` 或加滚动 offset |
| P2 | 长消息不按终端宽度换行，会溢出/截断；可见行数计算也错 | `app.go:634-680,639` | 用 `lipgloss.Width`/`wordwrap` 换行，按换行后行数算窗口 |
| P2 | 行内编辑失效：方向键/Home/End/中间删除都不工作，但 `renderInput` 却为任意光标位写了渲染代码（永不触发） | `app.go:194,262-266,281,682-724` | 实现光标移动，或直接换 Bubbles `textinput/textarea`（顺带获得历史/粘贴/多行） |
| P2 | Tab 循环只含 build/plan/compose，遗漏 `/max`；进入 Max 后 Tab 卡死 | `app.go:552-562`（`cycleMode`） | 把所有模式纳入循环，或明确文档说明 Max 不进 Tab；并在 `/help` 加各模式一句话说明 |
| P2 | voice 整包死代码 + stub，但被当作能力存在 | `voice/voice.go:71-77` | 接一个 push-to-talk 键喂 `voice.Input`，否则移出构建 |

**最大三个 UX 收益**：让回复真的显示出来并可取消（P0-1）→ 修掉"宣传了但不存在"的功能（成本/`/sessions`/voice）→ 让首次运行可生还（真向导 + 可操作的缺 key 提示）。

### 3.2 性能与成本

成本侧的杠杆几乎全部来自"把已建好的机制接上"。

| 严重度 | 问题 | 证据 | 修复 |
|---|---|---|---|
| P0 | 上下文无截断/无预算：每步把不断增长的全量历史按原价重发，成本二次增长；`ContextWindow` 解析了却从不用 | `agent/loop.go:127,141,218,242`；`control` 的 `CompressResult`/`ShouldCompress` 从不被循环调用 | 发请求前按模型 `ContextWindow` 截断/压缩最旧的 tool 结果 |
| P0 | 前缀缓存机制（`partition`）建好但循环不走它 → provider 端 prompt cache 永不命中，每轮付全价 | `loop.go:23,39` 构造了 Partition 却用 `a.messages` 直接发；`SetPrefix/AppendLog/BuildMessages` 从不调用 | 请求统一走 `partition.BuildMessages()`，记录命中 |
| P0 | `cache` 包零使用 | 跨包无 `NewMemoryCache/NewLRUCache` 引用 | 至少接入 embedding 缓存（按内容哈希）；否则删除以止损 |
| P1 | cached 定价从不生效：`CachedInput` 一路配置到 `ModelInfo`，DeepSeek 也解析了 `prompt_cache_hit_tokens`，但 `Cost()` 一律按全价算输入 | `deepseek/provider.go:94,199,287`，`openai:92`，`mimo:92` | 把 usage 拆成 cached/uncached，命中部分按 `CachedInput` 计；这同时修复负载均衡 `CostOptimized` 失效 |
| P1 | Goal judge 冗余调用：每 3 步评一次，最后一步又评一次，而最后这次结果被丢弃（两个分支返回同一值） | `loop.go:223-228,250-255`；`goal.go:75` | 删掉无意义的末步 `Evaluate`，judge 频率做成可配置 |
| P1 | 连接池被丢：`NewRetryableClientWithConfig` 不设 `Transport`，退回默认 `MaxIdleConnsPerHost=2` | `retry.go:54-58`（对比 `:37` 的池化版本） | 两个构造函数共用同一 tuned `Transport`，单 host LLM 端点把上限提到 10-20 |
| P1 | embedding N+1 + 无缓存：`AddBatch` 每文档一次 HTTP，`Search` 每次都重新 embed query | `memory/semantic.go:60,75-82,108` | 加 `EmbedBatch`，按内容哈希缓存（复用 cache 包） |
| P1 | 语义检索每次全量 O(N·D) 线性扫，且全程持 `RLock`，向量范数每次重算 | `semantic.go:118-137,188-190` | 插入时预归一化（检索变点积）；规模大再上 ANN(HNSW) |
| P1 | SQLite 无 WAL/busy_timeout/连接池调优，叠加包级 `RWMutex` 串行化，读阻塞在写后 | `memory/store.go:56,117,136` | 开 `journal_mode=WAL` + `busy_timeout`，写连接 `SetMaxOpenConns(1)` |
| P2 | DeepSeek 自写 `lineScanner` 每次 Scan 新分配 4096 字节、EOF 处理可能空转 | `deepseek/provider.go:349-360` | 换 `bufio.Scanner`/`ReadString('\n')`，复用缓冲 |
| P2 | 流读取无 `ctx.Done()` 分支，客户端取消后 goroutine+连接滞留 | `openai/provider.go:235-237` | SSE 读循环加 `ctx.Done()` case，区分 `io.EOF` 与真错误 |
| P2 | 多处按字节而非 rune 截断（代码含中文）；goal 把每条消息截到 200 字符再判，易误判 | `goal.go:103,107,174-179`，`cost.go:172` | 按 rune 边界截断；重估 judge 输入的截断长度 |

**最大三个性能/成本收益**：①接通 partition 前缀缓存 + `ContextWindow` 截断 + cache 包(P0-3)——单项最大成本杠杆；②修 cached token 计费(P1)——成本统计与 CostOptimized 同时恢复正确；③砍冗余 judge 调用 + 批量/缓存 embedding——直接减少整次 API 往返。

### 3.3 架构与代码质量

底盘是健康的（分层清晰、provider 抽象优秀、343 个测试全绿），债务集中在"未接线的存量"和少数热点文件。

| 严重度 | 问题 | 证据 | 修复 |
|---|---|---|---|
| P1 | `internal/errors`（197 行）整包零使用，全代码 160 处裸 `fmt.Errorf` | `errors/errors.go` 无引用 | 在 provider/tool/agent 边界采用 typed error，否则删包止损 |
| P1 | `ui/app.go` 是 849 行 god-file（TUI 循环+按键+命令解析+渲染+流式+env 脱敏），且 `internal/ui` **零测试** | `app.go:23-863` | 拆 `commands.go`/`render.go`/`update.go`，命令解析抽成可测 dispatcher 加表驱动测试 |
| P1 | 三个 provider 各自复制一份 SSE 解析（`lineScanner`/`lineReader`/`readSSEStream` 命名都不一致 = copy-paste 漂移） | `deepseek:239,349`，`mimo:239,322`，`openai:228` | 抽 `internal/provider` 共享 `sseScanner` + `readSSE(resp, parseFn)`，各 provider 只给 per-chunk parser |
| P1 | "分布式"名不副实（无网络）；"语义记忆"用 hash 假 embedding | `agent/distributed.go`；`semantic.go:213-220` | 要么实现真能力，要么改名/标实验，别在路线图标"完成" |
| P2 | `cmd/loomcode/main.go`(389 行) 的 `createProvider`/选路/REPL 无测试，仅 4 个 env helper 测试 | `main.go` | 抽 `createProvider`/flag 解析为可测函数，用 mock provider 跑 REPL 集成测试 |
| P2 | `runMax` 的"评委"其实是"返回最长非空串" | `agent/modes.go:238-244` | 用 LLM-as-judge 或带分启发式替代 `len()` |
| P2 | `agent/dream.go`+`hooks.go` 投机式存在、从不调用；hooks 的天然接入点是工具执行生命周期 | 两文件无引用 | 把 hooks 接进 `executeTools`，dream 移到 `experimental` build tag |
| P2 | `go.mod` 锁到 patch 级 `go 1.26.3`（应为 `1.26`），逼所有人用精确工具链 | `go.mod:3` | 改 `go 1.26`（或更低，见 P0-5），跑 `go mod tidy`（当前依赖全标 `// indirect` 不实） |
| P2 | `build.sh` 与 `Makefile` 重复同一套 build/test/release 逻辑，会漂移 | 两文件 | 一方 delegate 另一方，构建参数只留一处 |
| P2 | 本地工作区 `.env` 含真实 key（已 gitignore、未入库，风险在本地/备份泄漏） | `git check-ignore .env` 命中 | 文档提示禁止 `git add -f .env`；若曾外传则轮换 |

**最大三个架构收益**：①保住优秀的 provider 抽象，把三处 SSE 解析合一；②拆 `app.go` god-file 并补 UI 测试（它是状态最重却零测试的地方）；③对所有 🔴/🟡 特性二选一——接线或诚实标注实验，停止"未接入即完成"。

### 3.4 生态分发与集成

当前没有任何一条对外路径是通的：装不上、扩展机制是空的、文档大量 404。

| 严重度 | 问题 | 证据 | 修复 |
|---|---|---|---|
| P0 | 没有可用安装路径：`go install` 被 `go 1.26.3` 挡死；brew/install.sh/预编译都依赖不存在的 release；CI 未提交 | `go.mod:3`；无 release tag；`?? .github/` | 见 P0-5：降 Go 版本 + 提交 CI + 补 LICENSE + 打 tag，一条链同时修好 install/brew/binary/CI |
| P0 | Skills 扩展机制是空操作（见 P0-4），但 `docs/how-to/custom-skills.md:7` 宣称"添加自定义工具和能力" | `skills/manager.go` | 接线，或把文档降级为"当前仅描述性元数据" |
| P0 | MCP 插件无法被运行的二进制加载：`PluginManager` 从不构造，无配置项声明 MCP server | `mcp/manager.go` 无引用 | 加 `[[mcp_servers]]` 配置，启动时 `PluginManager.Connect` |
| P1 | 无 `LICENSE` 文件，但 README/goreleaser/docs 到处声明 MIT 并链接 `../LICENSE` | `ls LICENSE*` 无 | 加顶层 MIT `LICENSE` |
| P1 | Dashboard 是假数据 + WebSocket 是 stub（`/ws` 只回 `{"status":"ready"}`，`Broadcast` 空循环），但 examples 当真实端点宣传 | `dashboard/handlers.go`，`websocket.go` | 注入真实 session/cost，或明确标为 UI mockup |
| P1 | `docs/README.md` 索引 6+ 死链（`reference/provider-interface.md`、`reference/mcp-protocol.md`、整个 `explanation/*` 空目录） | `docs/explanation/` 为空 | 补 stub 页或删链；CONTRIBUTING 的"新增 Provider"尤其依赖缺失的 `provider-interface.md` |
| P1 | `loomcode dashboard` 未写进 CLI 参考，也不在 `usage()` | `docs/reference/cli-commands.md`；`main.go:312-326` | 文档补 `loomcode dashboard [addr]`，usage 补该命令 |
| P1 | 插件"市场"是孤儿死代码，`plugin.go` 还未提交；`Start()` 从不调 `Init()`，与 example 矛盾 | `mcp/plugin.go`（`?? internal/mcp/plugin.go`） | 提交文件、接线或停止宣称完成、修 `Init()→Start()` 生命周期 |
| P2 | 版本三方不一致：README/cli-commands 写 `0.1.0`、CHANGELOG `[0.1.0]`、路线图到 `v0.4.0`、二进制默认 `dev` | 多处 | 以 git tag 为唯一真相源对齐 |
| P2 | `examples/{basic,dashboard,plugin}` 各只有一个 README，无可跑代码 | `examples/` | 放最小可运行示例，或并入 docs |
| P2 | README "235 tests" 实为 343；TUI 命令表漏 `/goal`、`/sessions`、`/max` | README:11；`app.go:88-89` | 从 `commands` 切片生成命令列表，测试数由 CI 注入 |

**最大三个生态收益**：①让安装变成真的（P0-5 一条链）；②停止分发未接线的"完成"特性（至少把 Skills+MCP 接进 agent，否则诚实降级文档）；③修复文档信任问题（死链/版本/假端点）——新用户前 5 分钟全是 404 和"做不到说明所说"的功能。

---

## 4. 建议执行顺序

按"解锁价值 / 成本"排序，分三波。前两波几乎都是**接线与硬化，不是造新功能**。

### 第 1 波：让它"真的能用 + 装得上"（约 1 周）
1. **P0-1** 修 TUI 流式显示 + Esc 取消 —— 没有这条，整个工具对用户是黑屏。
2. **P0-5** Go 版本降级 + 提交 CI + LICENSE + 首个 release tag —— 一条链打通所有安装路径。
3. **P0-2** 核实并接通命令沙箱 —— 安全底线，不能停留在"标了完成"。
4. 修"宣传了但不存在"：成本显示、`/sessions`、缺 key 提示、`setup` 真向导。

### 第 2 波：成本与扩展接线（约 1-2 周）
5. **P0-3** 接通 partition 前缀缓存 + `ContextWindow` 截断 + cache 包 —— 最大成本杠杆。
6. **P1** 修 cached token 计费 + 砍冗余 judge 调用 + 批量/缓存 embedding。
7. **P0-4** Skills 真正注入 prompt/注册 tool；**MCP** 加配置并在启动时 `Connect`。

### 第 3 波：质量与诚实度（持续）
8. 拆 `ui/app.go` god-file 并补测试；三处 SSE 解析合一；采用或删除 `internal/errors`。
9. **重写路线图**：按 §1.2 矫正表把每项标成 🟢/🟡/🔴/实验；Dashboard 接真实数据或标 mockup；修文档死链与版本。

### 明确不建议现在做
- 桌面客户端、VS Code 扩展（路线图 v0.3.0 未完成项）—— 在主路径都没修好、安装都不通之前投入新端，会进一步拉大"宣称 vs 真实"的落差。
- "分布式 Agent"做真网络化 —— 当前本地 goroutine 池已够用，先改名止损，需求真出现再做。

---

## 附录：数字与事实矫正

| 项 | 文档宣称 | 实测 |
|---|---|---|
| 版本 | README `0.1.0` / 路线图 `v0.4.0` 完成 / 二进制 `dev` | 无 release tag，三方不一致 |
| 测试数 | README "235 passing" | 343 个 `func Test*`（+156 子测试），全绿 |
| `go build` / `go vet` / `go test` | — | **全部通过** |
| 主路径 import 的 internal 包 | 路线图暗示"全部完成" | 仅 8 个；7+ 个"完成"特性在主路径不可达 |
| CI | 路线图 ✅ | workflow 未提交，GitHub 上不运行 |
| MCP 测试耗时 | — | 单 MCP 套件 ~65s，疑似 sleep-based，值得排查 |

---

*生成于 2026-06-19。所有结论基于当时工作区代码，行号可能随后续提交变动；落地前请以最新代码为准。*
