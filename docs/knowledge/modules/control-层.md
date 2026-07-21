---
tags:
  - 核心模块
  - 控制层
  - 权限
  - 成本
created: 2026-07-14
updated: 2026-07-14
aliases:
  - Control
  - 权限控制
  - 成本控制
---

# 控制层

> 🛡️ 权限沙箱、成本控制、编辑门控、命令白名单
> 📅 最后更新：2026-07-14

---

## 概述

控制层是 LoomCode 的安全守卫，负责权限管理、成本控制、编辑门控和命令白名单。确保 Agent 的行为在可控范围内。

**代码路径**：`internal/control/`

## 关键文件

| 文件 | 职责 |
|------|------|
| `permission.go` | 权限管理（review/auto/yolo 模式） |
| `allowlist.go` | 命令白名单 + 路径白名单 |
| `cost.go` | 成本控制（Token 计费、预算上限） |
| `gate.go` | 编辑门控 |

## 权限模式

| 模式 | 行为 | 适用场景 |
|------|------|----------|
| **review** | 每个编辑块弹出确认 | 谨慎模式 |
| **auto** | 自动应用，工作区内放行 | 信任模式（默认） |
| **yolo** | 跳过所有确认 | 完全自主（需显式启用） |

## 命令白名单

Auto 模式下默认放行的安全命令：

```go
[]string{
    "ls", "cat", "head", "tail", "grep", "find",
    "pwd", "echo", "wc", "git", "go", "which",
}
```

可通过 `settings.json` 的 `permissions` 扩展：

```json
{
  "permissions": {
    "shell_allowlist": ["git", "npm", "go", "ls", "cat"]
  }
}
```

## 路径白名单

文件读写限制在工作区（cwd）内：
- Auto 模式：工作区内放行，区外拒绝
- 工作区根通过 `Allowlist.SetAllowedPaths()` 设置

## 成本控制

| 功能 | 说明 |
|------|------|
| Token 计费 | 按 Provider 的 Cost 配置实时计算 |
| 预算上限 | `SetCostBudget()` 设置会话预算 |
| 超预算回调 | `onBudgetExceeded` 触发时通知 |
| 成本统计 | `/cost` 命令查看累计成本 |

## 多层守卫链

工具调用请求经过的守卫序列：

```
工具调用请求
  │
  ├── 守卫1: 工具存在检查         → 未知工具直接拒绝
  ├── 守卫2: 重复成功阻断         → 同一写操作 ≥2 次成功 → 阻止
  ├── 守卫3: Plan 模式阻断        → Plan Agent 禁止写工具
  ├── 守卫4: 权限门控             → 敏感操作需用户确认
  ├── 守卫5: 风暴检测             → 连续3次相同失败 → 注入反思提示
  └── 守卫6: 沙箱执行             → 隔离执行环境
```

## 关键方法

| 方法 | 说明 |
|------|------|
| `NewPermission(mode)` | 创建权限管理器 |
| `Allowlist().SetShellCommands()` | 设置命令白名单 |
| `Allowlist().SetAllowedPaths()` | 设置路径白名单 |
| `SetCostBudget(amount)` | 设置成本预算 |
| `OnBudgetExceeded(fn)` | 注册超预算回调 |

## 相关文档

- [[tool-系统|工具系统]] — 权限注入到工具执行
- [[agent-引擎|Agent 引擎]] — 成本回调
- [[../architecture/架构总览|架构总览]]
