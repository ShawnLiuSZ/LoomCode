# CLI 命令参考

> 所有命令行选项和参数

## 语法

```bash
loomcode [选项] [命令] [参数]
```

## 命令

### `loomcode`（无参数）

启动交互式 TUI。

```bash
loomcode
loomcode --provider deepseek
loomcode --model deepseek-v4-pro
```

### `loomcode run <task>`

执行单次任务。

```bash
# 直接指定任务
loomcode run "创建一个 hello.go 文件"

# 从管道读取
echo "解释这段代码" | loomcode run

# 指定 Provider
loomcode --provider openai run "优化这段代码"
```

### `loomcode setup`

运行配置向导。

```bash
loomcode setup
```

### `loomcode chat`

启动交互式 TUI（与无参数相同）。

```bash
loomcode chat
loomcode tui
```

## 选项

### `--provider <name>`

指定 Provider。

```bash
loomcode --provider deepseek
loomcode --provider openai
loomcode --provider mimo
```

**可用值**：
- `deepseek` - DeepSeek V4 系列
- `openai` - GPT-4、GPT-3.5 等
- `mimo` - Xiaomi MiMo
- 任何在 `settings.json` / `models.json` 中配置的 Provider

### `--model <id>`

指定模型 ID。

```bash
loomcode --model deepseek-v4-flash
loomcode --model gpt-4o
loomcode --model deepseek-v4-pro
```

### `--config <path>`

指定配置文件路径。

```bash
loomcode --config ./my-config.json
loomcode --config ~/.loomcode/settings.json
```

### `--env-file <path>`

指定环境变量文件。

```bash
loomcode --env-file ./production.env
loomcode --env-file ~/.loomcode/secrets.env
```

### `--session <id>`

恢复历史会话。

```bash
loomcode --session session_1234567890
```

### `--version`

显示版本信息。

```bash
loomcode --version
```

输出示例：

```
LoomCode CLI 0.1.0 (commit: abc1234, built: 2026-06-18)
```

### `--help`

显示帮助信息。

```bash
loomcode --help
```

## 退出码

| 退出码 | 说明 |
|--------|------|
| `0` | 正常退出 |
| `1` | 错误退出 |

## 环境变量

### Provider 相关

| 变量名 | 说明 |
|--------|------|
| `DEEPSEEK_API_KEY` | DeepSeek API 密钥 |
| `OPENAI_API_KEY` | OpenAI API 密钥 |
| `MIMO_API_KEY` | MiMo API 密钥 |

### LoomCode 配置

| 变量名 | 说明 |
|--------|------|
| `LOOMCODE_PROVIDER` | 默认 Provider |
| `LOOMCODE_MODEL` | 默认模型 |

## 配置文件

LoomCode 使用 JSON 配置，按以下顺序合并加载（来自 `internal/config/loader.go`）：

1. `--config` 指定的路径
2. 先合并 `~/.loomcode/{models.json, settings.json}`（global）
3. 再叠加 `<project>/.loomcode/{settings.json, settings.local.json}`（project 覆盖 global；`settings.local.json` 覆盖 `settings.json`）

优先级：**project > global，local > shared**。旧版 TOML 配置（`loomcode.toml` / `config.toml` / `models.toml`）仅作迁移输入，已废弃。

## 示例

### 基本使用

```bash
# 启动 TUI
loomcode

# 单次任务
loomcode run "创建一个 Python 脚本"

# 从管道读取
cat code.py | loomcode run "解释这段代码"
```

### 指定 Provider

```bash
# 使用 DeepSeek
loomcode --provider deepseek run "任务"

# 使用 OpenAI
loomcode --provider openai run "任务"
```

### 指定模型

```bash
# 使用特定模型
loomcode --model gpt-4o run "复杂任务"

# 使用低成本模型
loomcode --model gpt-4o-mini run "简单任务"
```

### 恢复会话

```bash
# 列出会话
loomcode run "列出所有会话"

# 恢复指定会话
loomcode --session session_1234567890
```

### 使用自定义配置

```bash
# 使用指定配置文件
loomcode --config ./project.json run "任务"

# 使用指定环境变量
loomcode --env-file ./production.env run "任务"
```

### 组合使用

```bash
# 多个选项组合
loomcode --provider openai --model gpt-4o --session session_123 --env-file ./prod.env
```

## 故障排除

### 命令未找到

```bash
# 检查安装
which loomcode

# 重新安装
go install github.com/ShawnLiuSZ/loomcode/cmd/loomcode@latest
```

### 配置文件未找到

```bash
# 检查配置文件路径
ls -la ~/.loomcode/settings.json
ls -la ~/.loomcode/models.json
```

### 权限问题

```bash
# 检查文件权限
ls -la ~/.loomcode/
chmod -R 755 ~/.loomcode/
```

## 下一步

- [配置文件格式](config-format.md) - settings.json / models.json 完整语法（JSON）
- [内置工具列表](built-in-tools.md) - 所有内置工具
- [快速入门](../tutorials/getting-started.md) - 5 分钟上手
