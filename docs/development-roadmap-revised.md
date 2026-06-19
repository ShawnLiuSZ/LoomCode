# 开发路线图（修订版）

> **本版取代 [`development-roadmap.md`](development-roadmap.md)。** 旧版把大量"已建好但未接入主路径"的死代码标为 `✅ 完成`，与真实状态严重不符。本版基于 2026-06-19 [代码级复核](optimization-verification-report.md) 重写，建议把旧版归档。
>
> **修订日期**：2026-06-19 · **基准**：复核后工作区

---

## 0. 修订原则

1. **状态以"是否在主路径真实生效"为准**，不以"代码是否存在 / 测试是否通过"为准。代码在、测试绿，但 `main` 不调用，一律不算完成。
2. **图例**：🟢 真生效 · 🟡 部分/有缺陷 · 🔴 未接线或未通过 · 🗑️ 已删（止损） · ⬜ 未做 · ❓ 待复核（本轮未单独验证）

---

## 1. 现状快照：矫正后的"已完成"

原路线图 v0.1.1 → v0.4.0 共 27 处 `✅`。按真实状态重新分类如下。

### 1.1 🟢 真生效（其中多数是本轮才补上的）

| 特性 | 来源 | 证据 |
|---|---|---|
| API Key 安全（.env.example + gitignore） | v0.1.1 | `.env` 未入库 |
| 命令执行沙箱（权限拦截） | v0.1.1 | `main.go:130,275` 注入 `control.Permission`；`permission.go:36` 有真实拦截 ✅ 本轮修复 |
| Goal / Stop Condition | v0.1.2 | `loop.go` 真调用 `goal.Evaluate`（cadence 待优化，见 🟡） |
| TUI 流式显示 | — | `app.go:924,184,773` ✅ 本轮修复（此前回复永不显示） |
| Skills 注入 system prompt | — | `loop.go:328-338` + `app.go:125` ✅ 本轮修复（此前是空操作） |
| cached token 计费 | — | `deepseek/provider.go:94-99` ✅ 本轮修复 |
| 连接池 / SQLite WAL / embedding 批量+缓存+预归一 | v0.4.0 性能 | `retry.go:47`、`store.go:62`、`semantic.go:18,41` ✅ 本轮修复 |
| SSE 解析去重 | — | 新增 `provider/sse.go`，三 provider 复用 ✅ 本轮 |

### 1.2 🟡 部分 / 有缺陷

| 特性 | 真实状态 | 缺口 |
|---|---|---|
| 性能优化（上下文成本） | partition 被填充但请求仍发 `a.messages`，无截断 | **P0 返工**：`loop.go:100,224` 未走 `BuildMessages()`；无 `ContextWindow` 截断 → 长会话必崩 |
| Semantic Index | 余弦/批量/缓存已改善 | embedding 实现仍是 `MockEmbeddings`（真 embedding provider 未接） |
| Multi Provider 负载均衡 | 框架在 | `loadbalancer.go:206` `costOptimized()` 仍是 stub；是否真用于选路 ❓ |
| CI/CD | workflow 已提交 | **版本矛盾**：`go.mod 1.25.0` vs CI 矩阵 `1.21–1.23`，会把 CI 跑挂 |
| Goal cadence | 接入了 | `loop.go` 仍 3 处 `Evaluate`，是否可配/去冗余 ❓ |

### 1.3 🔴 未接线 / 未通过（仍标"完成"但名不副实）

| 特性 | 真实状态 |
|---|---|
| 上下文截断/前缀缓存（P0-3） | ❌ 未通过，见 1.2 + 复核报告 §2 |
| MCP HTTP SSE / 插件系统 | `sse_client.go`/`plugin.go` 真实但 `PluginManager` 从不 `Connect`；`plugin.go` 仍未提交 git |
| LSP 自动发现 | 客户端真实但无任何代码路径使用 |
| Web Dashboard | `handlers.go` 全假数据，`/ws` 是 stub |
| `internal/cache` | 整包零使用（接 cache 或删，两头未落） |
| `internal/errors` 统一错误处理 | 整包零使用，代码内 160 处裸 `fmt.Errorf` |

### 1.4 🗑️ 已删（健康止损）

| 特性 | 删除原因 |
|---|---|
| 分布式 Agent 协作 | 无任何网络，实为本地 goroutine 池，名不副实 → 删 |
| Dream & Distill | `DreamScheduler` 仅测试实例化，依赖的 `memory.Dream()` 是模拟实现 → 删 |
| Hooks 系统 | `HookManager` 从不被工具执行调用 → 删 |
| voice（语音） | 整包死代码 + `MiMoASR` 是 stub → 删 |

### 1.5 ❓ 待复核（本轮未单独验证，原标"完成"）

并发安全修复 · 错误处理统一(v0.1.1) · `/apply` SEARCH/REPLACE · Configurable Web Search · `/effort` knob · Transcript replay · Event log

---

## 2. 真实待办

> 全部是**接线 + 硬化 + 诚实化**，不是造新功能。这是当前性价比最高的方向。

### P0 — 返工（堵当前最大风险）

| # | 任务 | 为什么 |
|---|---|---|
| R-1 | CI / `go.mod` 版本对齐（go.mod 降 `1.23` + 矩阵对齐，去掉 `.0`） | 正堵着刚加的 CI；同时降低 `go install` 门槛 |
| R-2 | P0-3 真正接 `partition.BuildMessages()` + 加 `ContextWindow` 截断/压缩 | 最大成本杠杆；且修掉"长会话超窗必崩"的稳定性隐患 |

### P1 — 接线或止损（消除"宣称 vs 真实"落差）

| # | 任务 | 二选一 |
|---|---|---|
| W-1 | MCP 插件 | 加 `[[mcp_servers]]` 配置 + 启动 `Connect`，**或**文档降级为未支持 |
| W-2 | `internal/cache` | 接入 embedding/响应缓存，**或**删包止损 |
| W-3 | `internal/errors` | 在 provider/tool/agent 边界采用 typed error，**或**删包 |
| W-4 | Web Dashboard | 注入真实 session/cost + 实现 WS，**或**明确标 mockup |
| W-5 | LSP | 接入工具/上下文，**或**标实验 |

### P2 — 硬化

| # | 任务 |
|---|---|
| H-1 | Skill 注入改按需（现无条件塞全文，skill 多时撑大 prompt/成本） |
| H-2 | Goal cadence 可配 + 去冗余 judge 调用 |
| H-3 | 拆 `ui/app.go`（god-file）并补 UI 测试（命令解析抽成可测 dispatcher） |
| H-4 | semantic 接真 embedding provider，替换 `MockEmbeddings` |
| H-5 | `loadbalancer.costOptimized()` 用真实成本指标，替换 stub |

---

## 3. 版本重排（诚实版）

原计划把"平台化"（桌面端/VSCode）放 v0.3.0 且标完成，但主路径当时是断的。重排为：

```
v0.5.0 修复与诚实化 ──→ v0.6.0 扩展真正可用 ──→ v0.7.0 平台化
   (主路径打通+装得上)      (MCP/Skills/缓存)        (desktop/vscode)
```

### v0.5.0 — 修复与诚实化（约 1–1.5 周）
- R-1 CI/版本对齐 · R-2 P0-3 上下文截断
- 把 1.3/1.5 的特性逐项接线或降级，路线图状态全部对齐真实
- 交付标准：CI 真绿 · 长会话不崩 · 文档不再宣称未生效的功能

### v0.6.0 — 扩展真正可用（约 2 周）
- W-1 MCP 接线（让插件能被加载）· W-2 cache 落地 · H-1 skill 按需注入 · H-4 真 embedding
- 交付标准：第三方能用 MCP 加一个工具并被模型调用

### v0.7.0 — 平台化（按需，2 月+）
- 桌面客户端 / VSCode 扩展 / Dashboard 真实数据
- **前置条件**：v0.5/v0.6 完成，主路径稳固、安装链通畅

---

## 4. 里程碑（现实版）

| 里程碑 | 目标 | 交付物 |
|---|---|---|
| M1 修复闭环 | CI 真绿 + P0-3 闭环 | R-1、R-2 完成；Round 2 复核通过 |
| M2 诚实路线图 | 状态全对齐真实 | 1.3/1.5 项全部接线或降级 |
| M3 扩展可用 | MCP/Skills/cache 真生效 | 第三方插件 demo 跑通 |
| M4 平台化 | 桌面端/IDE | 仅在 M1–M3 达成后启动 |

---

## 5. 明确不做 / 推迟

- **桌面客户端、VSCode 扩展**：推迟到 v0.7.0。主路径未稳、安装未通之前开新端，只会拉大落差。
- **真分布式 Agent**：已删，需求真出现再做，别用本地 goroutine 池冒充。
- **插件"市场"**：当前无注册/发现/安装，先把单个 MCP 插件能加载做实，再谈市场。

---

*本修订版基于复核时代码。每次返工后请更新本表对应状态，并以 git tag 作为版本唯一真相源。*
