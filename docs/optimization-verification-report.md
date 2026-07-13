# LoomCode 优化建议 · 复核报告（验收 Round 1）

> 对照 [`optimization-recommendations.md`](optimization-recommendations.md) 的**代码级**复核（不看声明，看代码）
> 复核日期：2026-06-19 · 复核基准：当前工作区（未提交）相对 `HEAD (7d998de)` 的全部改动

---

## 0. 总体结论

**大部分 P0/P1 是真改了，质量不错；但有 1 个 P0 看着改了其实没改到点上，且新引入了一个会让刚加的 CI 直接挂掉的版本矛盾，另有几项仍未动。**

- 构建状态：`go build ./...` ✅ / `go vet ./...` ✅ / `go test ./... -count=1` ✅（全绿）
- 改动规模：41 文件，+1494 / −2709（净删 1215 行，主要来自删除死代码）
- 验收口径：✅ 通过（真接入、有证据）· 🟡 部分（接了一半/有新问题）· ❌ 未通过（看着改了实则无效）· 🔲 未动

---

## 1. 验收结果总表

| 编号 | 项 | 验收 | 关键证据 |
|---|---|---|---|
| P0-1 | TUI 流式显示 | ✅ 通过 | `ui/app.go:924` `program.Send(streamChunkMsg)` → `:184` 累加 → `:773` 实时渲染；`_ = text` 已删 |
| P0-2 | 命令执行沙箱 | ✅ 通过 | `main.go:130,275` `SetPermissionChecker(control.NewPermission)`；`control/permission.go:36` `Check` 有真实拦截 |
| P0-3 | 上下文成本（前缀缓存+截断） | ❌ **未通过** | 请求仍发 `a.messages`（`loop.go:100,224`），`BuildMessages()` 从不调用；无 `ContextWindow` 截断 |
| P0-4 | Skills 注入 | ✅ 通过（含小瑕疵） | `loop.go:328-338` 注入 skill 全文；`app.go:125` 主路径注入。瑕疵：无条件塞全文 |
| P0-5 | 安装链（CI/版本/LICENSE） | 🟡 部分 | LICENSE/CI/降版本均做；但 go.mod 与 CI 矩阵版本矛盾（见 §3） |
| P1 | cached token 计费 | ✅ 通过 | `deepseek/provider.go:94-99,204,217` 拆 cached/uncached 计价 |
| P1 | 连接池 | ✅ 通过 | `retry.go:47-49` 共享 `defaultTransport`，`MaxIdleConnsPerHost:10`，两构造函数共用 |
| P1 | SQLite WAL | ✅ 通过 | `store.go:62,66` `journal_mode=WAL` + `busy_timeout=5000` |
| P1 | embedding 批量+缓存+预归一化 | ✅ 通过 | `semantic.go:18,41,117` `EmbedBatch` + `embedCache` + `normalizeVector` |
| P1 | SSE 解析去重 | ✅ 通过 | 新增 `provider/sse.go`（209 行），删 `openai/sse.go`，三 provider 复用 |
| 清理 | 死代码删除 | ✅ 通过 | `distributed/dream/hooks/voice` 及测试整删（−1255 行） |
| P1 | docs 死链 | ✅ 通过 | 补 `explanation/*`、`reference/{provider-interface,mcp-protocol}.md` stub |
| P1 | Goal 冗余调用 | 🟡 待确认 | `loop.go` 仍 3 处 `Evaluate`（260/291/300），cadence 是否可配未确认 |
| 生态 | MCP 插件接线 | 🔲 未动 | `cmd/agent/ui` 无 `mcp.` 引用，插件仍无法被运行的二进制加载 |
| P0-3 | `internal/cache` | 🔲 未动 | 仍零使用（接 cache 或删，两头未落） |
| P1 | `internal/errors` | 🔲 未动 | 仍零使用，代码内 160 处裸 `fmt.Errorf` 未变 |
| P1 | Dashboard 真实数据 | 🔲 未动 | `handlers.go` 假数据、`/ws` stub 未改 |
| P1 | LSP 接线 | 🔲 未动 | 仍无代码路径使用 |

---

## 2. ❌ 必须返工：P0-3 看着改了，实则无效

这是本轮**唯一一个"动了但没解决"**的 P0，也是最大成本/稳定性杠杆。

**现状**：循环里加了 `partition.SetPrefix()` / `partition.AppendLog()`，看起来接上了前缀缓存。

**问题**：
1. **真正发请求的还是 `a.messages`**——`loop.go:100`（流式）和 `:224`（goal 路径）都是 `Messages: a.messages`，`partition.BuildMessages()` 全程没被调用。partition 被填充但对请求零影响，等于"换层皮的建好没接线"。
2. **完全没有上下文截断**——`a.messages` 只 append 不裁剪（`loop.go:150,164,250,278`），`ContextWindow` 仍未被消费。

**后果**：
- 成本：长历史每轮重发（前缀缓存 + 已修的 cached 计费能省一部分，但 partition 机制仍是死的）。
- **稳定性（更严重）**：长会话迟早超出模型上下文窗口 → 请求**直接失败**，目前没有任何保护。

**返工方向**：
- 请求改走 `partition.BuildMessages()`，让稳定 system 前缀真正驱动请求；
- 发请求前按 `caps`/`ContextWindow` 对 `a.messages` 做截断或压缩最旧的 tool 结果（可复用 `control` 的 `CompressResult`）。

---

## 3. ⚠️ 新引入的矛盾：CI 与 go.mod 版本打架

P0-5 本意是让 CI 能跑，但这步把它跑挂了。

| 位置 | 值 |
|---|---|
| `go.mod:3` | `go 1.25.0` |
| `.github/workflows/ci.yml:14` | 矩阵 `['1.21','1.22','1.23']` |
| `ci.yml:45,62` | 其他 job `'1.23'` |

Go 1.21/1.22/1.23 跑 `go build` 会撞 `go.mod requires go >= 1.25.0`：要么触发 toolchain 自动下载使矩阵失去意义，要么直接失败。

**修复（二选一）**：
- **推荐**：`go.mod` 降到 `go 1.23` 并对齐矩阵——`go install` 门槛最低、用户面最广；
- 或把 CI 矩阵升到 `1.25`。

另：`1.25.0` 的 `.0` patch 级 pin 建议去掉（`go` 指令惯例只写 `major.minor`）。

---

## 4. ✅ 已通过项的亮点

- **流式显示**用 `program.Send` + `streamChunkMsg` 正确实现了 Bubble Tea 流式回传，并在 View 里实时渲染缓冲区——核心交互恢复可用。
- **沙箱**不是空壳：`Check` 对敏感路径与危险 shell 模式有硬拦截，且在主路径用真实 `control.Permission` 注入。
- **cached 计费**同步修复了"成本统计虚高"和"负载均衡 CostOptimized 失效"两个连带问题。
- **删代码**比"为接而接"更果断正确：`distributed/dream/hooks/voice` 名不副实或长期不用，直接删 1255 行，是健康的止损。

---

## 5. 🔲 仍未处理（请确认是否有意延后）

| 项 | 影响 | 备注 |
|---|---|---|
| MCP 插件接线 | 头号扩展机制之一仍不可用 | 需加 `[[mcp_servers]]` 配置 + 启动时 `Connect` |
| `internal/cache` | 死代码维护成本 | P0-3 的"接或删"两头未落 |
| `internal/errors` | 死代码 + 160 处裸 error 不一致 | 采用 typed error 或删包 |
| Dashboard 假数据 / `/ws` stub | "Web Dashboard 完成"仍名不副实 | 注入真实 session/cost 或标为 mockup |
| LSP 接线 | 无 IDE/编辑器集成面 | 接入工具或上下文，或标实验 |
| Goal cadence | 可能仍有冗余 judge 调用 | 确认 3 处 `Evaluate` 是否可配/去冗余 |
| P0-4 skill 注入策略 | skill 多时撑大 prompt/成本 | 改按需注入，而非无条件塞全文 |

---

## 6. 建议下一步（按优先级）

1. **CI/go.mod 版本对齐**（§3）—— 最快，且正堵着刚加的 CI。
2. **P0-3 真正接 `BuildMessages()` + 加 `ContextWindow` 截断**（§2）—— 最大成本/稳定性杠杆，且修掉长会话必崩的隐患。
3. **诚实度收尾**：MCP 接线或文档降级；`cache`/`errors` 接或删；Dashboard 接真实数据或标 mockup。
4. 重写路线图，按本报告把每项标成 ✅/🟡/❌/实验，停止"未接入即完成"。

---

*本报告基于复核时工作区代码，行号可能随后续提交变动。下一轮返工后建议再出一份 Round 2 复核确认 P0-3 与 CI 闭环。*
