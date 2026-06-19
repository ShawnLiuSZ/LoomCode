# Helix 优化建议 · 复核报告（验收 Round 3）

> 对照 Round 1/2 复核意见的代码级复核
> 复核日期：2026-06-19 · 复核基准：`HEAD (f29e339)` + 未推送的 CI workflow commit

---

## 0. 总体结论

**P0-3 截断已真接入，但 partition 前缀缓存仍为死代码；CI 版本矩阵需对齐；其余项均已落实。**

- 构建状态：`go build ./...` ✅ / `go vet ./...` ✅ / `go test ./... -count=1` ✅（全绿）
- 改动规模：Round 2 改动 7 文件，+107 / −980

---

## 1. 验收结果总表

| 编号 | 项 | Round 1 | Round 3 | 关键证据 |
|---|---|---|---|---|
| P0-3 | ContextWindow 截断 | ❌ | ✅ **通过** | `loop.go:158,285` 两处 `a.truncateMessages(a.getContextWindow())` 在每次 API 调用前执行；`:72-79` `getContextWindow()` 从 provider Models() 获取窗口大小；`:96-129` 截断逻辑：80% 预算、优先删 tool 结果、保留 system+最近 4 条 |
| P0-3 | `BuildMessages()` 接线 | ❌ | ❌ **仍未通过** | `BuildMessages()` 全程零调用；`:162,289` 仍发 `a.messages`；partition 被填充但对请求零影响 |
| P0-4 | Skill 注入策略 | 🟡 | ✅ **通过** | `loop.go:391-398` 只注入 name + description（一行），不塞全文 SKILL.md |
| P0-5 | CI/go.mod 版本 | 🟡 | 🟡 **部分** | `go.mod:3` 为 `go 1.23`；`ci.yml:14` 矩阵仍含 `1.21`,`1.22`——与 go.mod 最低要求矛盾 |
| 清理 | `internal/cache` | 🔲 | ✅ **通过** | 目录已删除，零文件 |
| 清理 | `internal/errors` | 🔲 | ✅ **通过** | 目录已删除，零文件 |
| P1 | Dashboard mockup | 🔲 | ✅ **通过** | `handlers.go:2-4` package doc 标注 mockup；`:26,40,53` 所有响应含 `_mockup: true` |

---

## 2. ❌ 仍未通过：P0-3 `BuildMessages()` 仍是死代码

**现状**：`truncateMessages()` 已正确接入（✅），但 partition 的 `BuildMessages()` 仍从未被调用。

**问题**：
- `a.messages` 直接发给 provider（`loop.go:162,289`），partition 的不可变前缀对请求零影响
- Provider 端的 prompt cache 永远不会命中（前缀不固定）

**返工方向**（二选一）：
1. **方案 A（推荐）**：承认 partition 设计与 tool-call 协议不兼容（`BuildMessages()` 丢失 `ToolCalls`/`ToolCallID`），删除 partition 相关代码止损
2. **方案 B**：扩展 `LogEntry` 支持 `ToolCalls`/`ToolCallID`，让 `BuildMessages()` 输出完整消息——工作量较大

**建议**：选方案 A。partition 是为纯文本对话设计的，与 tool-call 协议结构性不兼容。当前 `truncateMessages()` 已解决上下文溢出问题，partition 的 prefix cache 收益有限（DeepSeek/MiMo 的 prefix cache 需要连续相同的 prefix，而 tool results 会打断前缀）。

---

## 3. ⚠️ CI 矩阵需对齐

| 位置 | 当前值 | 建议值 |
|---|---|---|
| `go.mod:3` | `go 1.23` | `go 1.23`（不变） |
| `ci.yml:14` 矩阵 | `['1.21', '1.22', '1.23']` | `['1.23']` 或 `['1.23', '1.24']` |
| `ci.yml:45,62` 其他 job | `'1.23'` | `'1.23'`（不变） |

**原因**：`go.mod` 声明 `go 1.23` 为最低要求，Go 1.21/1.22 编译会失败或触发 toolchain 自动下载，使矩阵失去意义。

---

## 4. ✅ 已通过项确认

- **截断**：`truncateMessages()` 在 `RunStream()` 和 `Run()` 两处均正确调用，逻辑合理（80% 预算、优先删 tool、保底 4 条）
- **Skill 注入**：改为 name+description 概览，prompt 不再膨胀
- **死代码清理**：`cache/`、`errors/` 目录确认删除
- **Dashboard**：mockup 标识清晰，package doc 诚实标注

---

## 5. 建议下一步

1. **CI 矩阵对齐**（§3）—— 5 分钟修复
2. **删除 partition 死代码**（§2 方案 A）—— 止损，避免误导
3. 其余 🔲 项（MCP 接线、LSP、Goal cadence）按优先级延后

---

*本报告基于 Round 3 复核时代码。CI workflow 文件尚未推送（需手动 push with workflow scope）。*
