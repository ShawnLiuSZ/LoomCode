# TUI Overhaul 复查报告

> 对照 [ux-and-feature-improvements.md](ux-and-feature-improvements.md) 落地的 TUI 改进（commit `1bd79df`：textarea / markdown / viewport / compact）的代码级复查
> 复查日期：2026-06-19 · 基准：`internal/ui/app.go`（单文件，29KB）

---

## 0. 总体结论

**方向对、大部分接得不错，但有一个"打不了字"的 showstopper 必须先修。**

| 项 | 状态 |
|---|---|
| textarea 输入框 | 🔴 **P0：键盘事件没接到 textarea，用户无法输入** |
| markdown 渲染 | 🟡 已接，但每 token 全量重渲染（长会话卡） |
| viewport 滚动 | ✅ 可用 |
| glamour 渲染 / friendlyError / Esc 中断 / `/compact` / 命令联想 | ✅ 在 |
| 构建 | ✅ `go build`/`go vet` 绿（但 `internal/ui` 无测试，没抓到 P0） |

---

## 1. 🔴 P0：打不了字（showstopper）

把输入框换成了 Bubbles `textarea`，但**键盘事件根本没转发给 textarea**——用户无法输入任何文字。

### 根因（代码路径 + 实证双重确认）

1. `Update()` 在 `case tea.KeyMsg: return a.handleKey(msg)`（`app.go:246`）**提前 return**。
2. `handleKey` 只显式拦截 `ctrl+c / esc / enter / tab / pgup / pgdown`，其余所有键（字母、退格、方向键、Shift+Enter）落到结尾 `return a, nil`（`app.go:383`），**从不调用 `a.textArea.Update()`**。
3. 唯一的 `a.textArea.Update(msg)` 在 `app.go:280`，对按键事件**永远不可达**。
4. Bubble Tea 子组件只有在其 `Update` 被调用时才更新 → textarea 只闪光标，收不到任何输入。

> 讽刺点：被替换掉的手写输入虽简陋但**能打字**；换成 textarea 后反而完全打不了。`internal/ui` 无任何测试，所以 `go build`/`go test` 全绿也没抓到——"编译通过 ≠ TUI 能用"。

### 实证

临时探针测试（已删除，可复现）：

```go
app := NewApp(testutil.NewStubProvider(nil), tool.NewRegistry())
app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
var m tea.Model = app
m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
// 期望 "hi"，实际：
//   textArea.Value() = ""
//   FAIL: typing did NOT reach textarea
```

### 修复（约 5 行，外科手术式）

把 `handleKey` 结尾的 `return a, nil`（`app.go:383`）改为转发给 textarea：

```go
var cmd tea.Cmd
a.textArea, cmd = a.textArea.Update(msg)
if strings.HasPrefix(a.textArea.Value(), "/") {
    a.updateSuggestions()
}
return a, cmd
```

这样字母 / 退格 / 方向键 / Home/End / Shift+Enter（换行）都会正确进 textarea，而 `enter`/`tab`/`esc`/`ctrl+c`/`pgup`/`pgdown` 仍被前面的 case 显式拦截。

### 防回归

`internal/ui` 必须补一个最小测试：构造 `App` → 发 `KeyRunes` → 断言 `textArea.Value()`。否则同类问题还会再犯。

---

## 2. 🟡 markdown 每 token 全量重渲染

`renderMessages` 在每次 `streamChunkMsg`（每个 token）都对**全部历史消息**跑一遍 glamour 渲染（`app.go:851` 循环内 `renderMarkdown(msg.Content)`）。会话越长越卡。

**修复**：按消息缓存渲染结果（消息不变就不重渲染），流式时只对 `streamBuf` 实时渲染。非阻塞。

---

## 3. ✅ 确认做对的部分

- **viewport 滚动**真能用：`pgup`/`pgdown` 显式处理（`app.go:374-380`）+ 鼠标走 `app.go:285` 的 `viewport.Update`。
- **glamour markdown 渲染**已接，`renderMarkdown` 带纯文本回退（`app.go:883`）。
- **`friendlyError`** 已用上，解决了之前"裸 Go 错误泄漏给用户"。
- **Esc 中断** loading、**`/compact`** 命令、**`/` 命令联想**（Tab 循环）都在。
- 手写输入的死代码（`cursorPos`/`renderInput`/`wrapLine`）已清掉，无残留。
- `go.mod 1.25.0` + 新依赖（bubbles/glamour/chroma/clipboard）一致，构建绿。

---

## 4. 下一步

1. **先修 P0**（§1）：5 行转发 + 补一个 `internal/ui` 输入测试。没有这步，TUI 不可用。
2. 顺手修 markdown 重渲染（§2）。
3. 修完建议手动跑一次 `./bin/loomcode` 真机确认能打字、能滚动、能看渲染——TUI 这类必须人眼过一遍。

---

*教训：改 TUI 必须有"发按键 → 断言状态"的单测，并真机跑一次；`internal/ui` 长期零测试是这次 P0 漏网的直接原因。*
