# LoomCode 优化建议 · 复核报告（验收 Round 3）

> 确认 Round 2 遗留两项（R-1 CI 矩阵收敛、R-2 成对删除）的修复
> 复核日期：2026-06-19 · 基准：commit `9395e3e`（已提交）

---

## 0. 总体结论

**R-2 早已闭环；R-1 一度回退为构建失败，已按方案 A 修复并验证通过。**

| 项 | Round 2 | Round 3（修复后） | 说明 |
|---|---|---|---|
| R-2 成对删除截断 | 🟡 待硬化 | ✅ **闭环** | 按完整轮次删除，有测试覆盖；仅剩 1 个窄边界情形 |
| R-1 CI/版本 | 🟡 矩阵需收敛 | ✅ **已修复（方案 A）** | `go.mod` 设 `go 1.25.0` + CI 全部 `1.25`，`go mod tidy` 后模块图一致 |

> ✅ 修复后构建状态：`go build ./...` ✅ / `go vet ./...` ✅ / `go test ./... -count=1` ✅（全绿，0 FAIL）。
>
> ⚠️ 修复前曾失败：`go.mod 1.23` 与 sqlite 依赖要求的 `go 1.25` 冲突，`go build ./...`/`go test ./...` 报 `updates to go.mod needed`。`go test ./internal/agent/` 单独能过（其闭包不含 sqlite）掩盖了问题——这正是"单包测试不能代替全量构建"的教训。

---

## 1. R-1 —— ✅ 已修复（方案 A：接受 Go 1.25）

> **修复内容**：`go.mod` 的 `go` 指令设为 `1.25.0`（sqlite 依赖所要求的最低版本），CI 三处 Go 版本统一为 `1.25`，`go mod tidy` 后 go.sum 干净。修复后 `go build ./...` / `go test ./...` 全绿。
> **代价**：`go install` 现需 Go ≥ 1.25。若日后想降低门槛，需改走方案 B（降 sqlite）。
>
> 以下为问题根因留档：

### 🔴 回退：go 1.23 与依赖冲突（已解决）

### 现象
- `go build ./...` 失败：`go: updates to go.mod needed; to update it: go mod tidy`
- `go mod tidy -diff` 显示唯一改动是把 `go 1.23` 改回 **`go 1.25.0`**

### 根因（确凿）
| 依赖 | 声明的 go 版本 | 用途 |
|---|---|---|
| `modernc.org/sqlite v1.52.0` | `go 1.25.0` | `internal/memory/store.go` 的纯 Go SQLite 驱动 |
| `modernc.org/libc`（其依赖） | `go 1.25.0` | 同上传递依赖 |

主模块必须 `go ≥ 1.25`。手动降到 `1.23` 使模块图不一致 → 构建/测试/CI 全部失败。

**"降低安装门槛到 1.23" 与 "用 sqlite v1.52.0" 不可兼得。**

### 修复（需拍板，二选一）

| 方案 | 操作 | 代价 |
|---|---|---|
| **A. 接受 Go 1.25 门槛** | `go.mod` 设 `go 1.25`（去掉 `.0`）+ CI 矩阵设 `['1.25']` | `go install` 需 Go 1.25（较新），安装面收窄；但最省事、最稳 |
| **B. 降 sqlite 保住低门槛** | 把 `modernc.org/sqlite` 降到支持 go 1.23 的旧版本（如 v1.3x），`go mod tidy` 后验证编译+测试 | 旧驱动可能缺修复；需回归验证；可能连带降 libc 等 |

> 建议：若在意 `go install` 覆盖面 → A 之外优先试 B（验证旧版 sqlite 可用）；若只在意尽快绿 → 直接 A。无论哪种，**go.mod 与 CI 矩阵必须同源一致**，且改完要 `go mod tidy` 确保模块图干净。

---

## 2. R-2 —— ✅ 闭环

### 验收
| 检查点 | 结果 | 证据 |
|---|---|---|
| 是否成对删除（避免孤儿 tool 消息） | ✅ | `loop.go` `truncateMessages`：定位 `assistant(tool_calls)` 后向后并入其所有 `tool` 结果，整轮 `[roundStart, roundEnd]` 一起删 |
| 是否有测试 | ✅ | `goal_test.go:187 TestTruncate` 通过 |

### 遗留窄边界（P2，非阻塞）
当某 `assistant(tool_calls)` 恰好位于"保留最近 N 条"窗口外缘、而其 `tool` 结果落在保留窗口内时，向后扫描受 `j < len-keepRecent` 限制不会并入窗口内的 tool 结果 → 仅删 assistant，窗口内的 tool 结果可能成孤儿。触发条件苛刻，建议删除时把"跨越保留边界的整轮"一并纳入，或保留窗口按轮次对齐。

---

## 3. 下一步

1. **先恢复可构建**：按 §1 选 A 或 B，`go mod tidy` 后确认 `go build ./...` 与 `go test ./...` 双绿。
2. R-2 窄边界（§2）可顺手硬化，非阻塞。
3. 回 [修订版路线图](development-roadmap-revised.md) P1：MCP/cache/errors/Dashboard。

---

*本轮暴露的教训：改 `go` 指令后必须 `go build ./...` 全量验证 + `go mod tidy`，单包测试会掩盖模块图不一致。*
