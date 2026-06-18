# CLI 命令参考

> 所有命令行选项和参数

## 语法

```bash
helix [选项] [命令] [参数]
```

## 命令

### `helix`（无参数）

启动交互式 TUI。

```bash
helix
helix --provider deepseek
helix --model deepseek-v4-pro
```

### `helix run <task>`

执行单次任务。

```bash
# 直接指定任务
helix run "创建一个 hello.go 文件"

# 从管道读取
echo "解释这段代码" | helix run

# 指定 Provider
helix --provider openai run "优化这段代码"
```

### `helix setup`

运行配置向导。

```bash
helix setup
```

### `helix chat`

启动交互式 TUI（与无参数相同）。

```bash
helix chat
helix tui
```

## 选项

### `--provider <name>`

指定 Provider。

```bash
helix --provider deepseek
helix --provider openai
helix --provider mimo
```

**可用值**：
- `deepseek` - DeepSeek V4 系列
- `openai` - GPT-4、GPT-3.5 等
- `mimo` - Xiaomi MiMo
- 任何在 `helix.toml` 中配置的 Provider

### `--model <id>`

指定模型 ID。

```bash
helix --model deepseek-v4-flash
helix --model gpt-4o
helix --model deepseek-v4-pro
```

### `--config <path>`

指定配置文件路径。

```bash
helix --config ./my-config.toml
helix --config ~/.helix/config.toml
```

### `--env-file <path>`

指定环境变量文件。

```bash
helix --env-file ./production.env
helix --env-file ~/.helix/secrets.env
```

### `--session <id>`

恢复历史会话。

```bash
helix --session session_1234567890
```

### `--version`

显示版本信息。

```bash
helix --version
```

输出示例：

```
Helix CLI 0.1.0 (commit: abc1234, built: 2026-06-18)
```

### `--help`

显示帮助信息。

```bash
helix --help
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

### Helix 配置

| 变量名 | 说明 |
|--------|------|
| `HELIX_PROVIDER` | 默认 Provider |
| `HELIX_MODEL` | 默认模型 |

## 配置文件

Helix 按优先级查找配置文件：

1. `--config` 指定的路径
2. `./helix.toml` - 项目目录
3. `~/.helix/config.toml` - 全局配置

## 示例

### 基本使用

```bash
# 启动 TUI
helix

# 单次任务
helix run "创建一个 Python 脚本"

# 从管道读取
cat code.py | helix run "解释这段代码"
```

### 指定 Provider

```bash
# 使用 DeepSeek
helix --provider deepseek run "任务"

# 使用 OpenAI
helix --provider openai run "任务"
```

### 指定模型

```bash
# 使用特定模型
helix --model gpt-4o run "复杂任务"

# 使用低成本模型
helix --model gpt-4o-mini run "简单任务"
```

### 恢复会话

```bash
# 列出会话
helix run "列出所有会话"

# 恢复指定会话
helix --session session_1234567890
```

### 使用自定义配置

```bash
# 使用指定配置文件
helix --config ./project.toml run "任务"

# 使用指定环境变量
helix --env-file ./production.env run "任务"
```

### 组合使用

```bash
# 多个选项组合
helix --provider openai --model gpt-4o --session session_123 --env-file ./prod.env
```

## 故障排除

### 命令未找到

```bash
# 检查安装
which helix

# 重新安装
go install github.com/ShawnLiuSZ/Helix/cmd/helix@latest
```

### 配置文件未找到

```bash
# 检查配置文件路径
ls -la helix.toml
ls -la ~/.helix/config.toml
```

### 权限问题

```bash
# 检查文件权限
ls -la ~/.helix/
chmod -R 755 ~/.helix/
```

## 下一步

- [配置文件格式](config-format.md) - helix.toml 完整语法
- [内置工具列表](built-in-tools.md) - 所有内置工具
- [快速入门](../tutorials/getting-started.md) - 5 分钟上手
