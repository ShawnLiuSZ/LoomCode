# TUI 交互指南

> 学习 LoomCode 交互式界面的操作

## 界面概览

```
┌─────────────────────────────────────────────────────────────┐
│ 🛠 LoomCode CLI | build | DeepSeek | deepseek-v4-flash        │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ▸ 请帮我创建一个简单的 HTTP 服务器                          │
│    我来帮你创建一个简单的 HTTP 服务器。首先让我检查当前...    │
│                                                             │
│ 🛠 build > _                                                │
│ DeepSeek | deepseek-v4-flash | Tab:模式 | /:命令 | $0.0000  │
└─────────────────────────────────────────────────────────────┘
```

### 界面区域

| 区域 | 说明 |
|------|------|
| **标题栏** | 当前模式、Provider、模型 |
| **消息区** | 对话历史 |
| **输入区** | 用户输入 |
| **状态栏** | Provider、模型、成本 |

## 基本操作

### 输入消息

直接在输入框输入文本，按 `Enter` 发送：

```
🛠 build > 创建一个 Python 脚本
```

### 快捷键

| 按键 | 功能 |
|------|------|
| `Enter` | 发送消息 |
| `Tab` | 切换 Agent 模式 |
| `Esc` | 清空当前输入 |
| `Ctrl+C` | 退出 |
| `↑` / `↓` | 在命令列表中选择 |

## 命令系统

### 输入 `/` 后按 `Tab`

显示可用命令列表：

```
/help      显示帮助
/mode      显示当前模式
/build     切换到 build 模式
/plan      切换到 plan 模式
/compose   切换到 compose 模式
/max       切换到 max 模式
/model     显示/切换模型
/skills    显示可用工具列表
/clear     清空聊天
/cost      显示成本
/env       环境变量管理
/sessions  会话列表
/quit      退出
```

### 常用命令详解

#### `/help` - 显示帮助

```
/help
```

输出完整的命令列表和使用提示。

#### `/mode` - 查看当前模式

```
/mode
```

显示当前使用的 Agent 模式和模型。

#### `/model` - 切换模型

```bash
# 显示可用模型列表
/model

# 直接切换到指定模型
/model gpt-4o
/model deepseek-v4-pro
```

#### `/skills` - 查看工具

```
/skills
```

显示所有内置工具和外部 Skills。

#### `/cost` - 查看成本

```
/cost
```

显示当前会话的 API 调用成本。

#### `/clear` - 清空聊天

```
/clear
```

清空当前对话历史。

#### `/env` - 环境变量管理

```bash
# 查看所有环境变量
/env

# 设置环境变量
/env set MY_KEY my_value

# 移除环境变量
/env unset MY_KEY

# 重新加载环境变量
/env reload
```

## Agent 模式

### 模式切换

按 `Tab` 键在模式间循环切换：

```
build → plan → compose → build → ...
```

### 模式说明

| 模式 | 说明 | 工具权限 |
|------|------|----------|
| **build** | 默认模式，完整工具权限 | 读写文件、执行命令 |
| **plan** | 只读分析，不执行写操作 | 仅读取文件 |
| **compose** | 规格驱动开发 | 完整权限 |
| **max** | 并行 N 候选，选最优 | 完整权限（实验性） |

### 使用场景

#### Build 模式（默认）

适用于大多数任务：

```
🛠 build > 创建一个 REST API
```

#### Plan 模式

用于分析和规划，不修改文件：

```
📋 plan > 分析这个项目的架构
```

#### Compose 模式

规格驱动开发：

```
📦 compose > 按照 PRD 实现用户认证模块
```

#### Max 模式

并行生成多个候选，选择最优：

```
⚡ max > 实现一个排序算法
```

## 模型选择

### 交互式选择

```
/model
```

显示模型列表，使用 `↑` / `↓` 选择，按 `Enter` 确认：

```
选择模型 (↑↓ 移动, Enter 确认, Esc 取消):

当前: deepseek-v4-flash

可用模型:
▶ deepseek-v4-flash
  deepseek-v4-pro
  gpt-4o
  gpt-4o-mini
  mimo-v2
```

### 直接指定

```
/model gpt-4o
```

## 会话管理

### 恢复历史会话

```bash
# 启动时恢复指定会话
loomcode --session session_1234567890
```

### 会话列表

```
/sessions
```

显示所有历史会话。

## 高级技巧

### 多行输入

输入长文本时，可以使用 `\` 继续下一行：

```
🛠 build > 这是一个很长的 \
... 任务描述 \
... 需要多行显示
```

### 命令补全

输入 `/h` 后按 `Tab`，自动补全为 `/help`。

### 快速清空

按 `Esc` 快速清空当前输入。

## 下一步

- [内置工具列表](../reference/built-in-tools.md) - 了解所有可用工具
- [Agent 模式详解](../explanation/agent-modes.md) - 深入理解各模式
- [配置 Provider](configuring-providers.md) - 设置多个 AI 模型
