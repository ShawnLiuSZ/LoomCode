# 配置 Provider

> 设置和管理 AI 模型提供者

## 概述

LoomCode 支持多种 AI 模型 Provider，包括：

- **DeepSeek** - DeepSeek V4 系列模型
- **OpenAI** - GPT-4、GPT-3.5 等
- **MiMo** - Xiaomi MiMo 模型
- **任何 OpenAI 兼容接口**

## 快速配置

### 方式一：环境变量（最简单）

```bash
# DeepSeek
export DEEPSEEK_API_KEY="sk-your-deepseek-key"

# OpenAI
export OPENAI_API_KEY="sk-your-openai-key"

# MiMo
export MIMO_API_KEY="sk-your-mimo-key"

# 启动 LoomCode
loomcode
```

### 方式二：配置文件（推荐）

创建 `loomcode.toml`：

```toml
default_provider = "deepseek"

[[providers]]
name = "deepseek"
display_name = "DeepSeek"
kind = "deepseek"
base_url = "https://api.deepseek.com"
api_key_env = "DEEPSEEK_API_KEY"
default_model = "deepseek-v4-flash"

[[providers.models]]
id = "deepseek-v4-flash"
name = "DeepSeek V4 Flash"
context_window = 131072

[[providers.models]]
id = "deepseek-v4-pro"
name = "DeepSeek V4 Pro"
context_window = 131072
```

## Provider 配置详解

### 基本字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | Provider 唯一标识 |
| `display_name` | string | ❌ | 显示名称 |
| `kind` | string | ✅ | 适配器类型（deepseek/openai/mimo） |
| `base_url` | string | ✅ | API 端点地址 |
| `api_key_env` | string | ❌ | API Key 的环境变量名 |
| `auth_method` | string | ❌ | 认证方式（默认 Bearer） |
| `default_model` | string | ❌ | 默认模型 ID |

### 模型配置

```toml
[[providers.models]]
id = "model-id"
name = "Model Display Name"
context_window = 131072

[providers.models.cost]
input = 0.14          # 每百万 token 输入价格（美元）
cached_input = 0.014  # 缓存输入价格
output = 0.28         # 每百万 token 输出价格

[providers.models.capabilities]
reasoning = false     # 支持推理
tool_call = true      # 支持工具调用
prefix_cache = true   # 支持前缀缓存
vision = false        # 支持图像
voice = false         # 支持语音
```

## 多 Provider 配置

同时配置多个 Provider：

```toml
default_provider = "deepseek"

[[providers]]
name = "deepseek"
kind = "deepseek"
base_url = "https://api.deepseek.com"
api_key_env = "DEEPSEEK_API_KEY"
default_model = "deepseek-v4-flash"

[[providers.models]]
id = "deepseek-v4-flash"
name = "DeepSeek V4 Flash"

[[providers]]
name = "openai"
kind = "openai"
base_url = "https://api.openai.com/v1"
api_key_env = "OPENAI_API_KEY"
default_model = "gpt-4o"

[[providers.models]]
id = "gpt-4o"
name = "GPT-4o"

[[providers]]
name = "mimo"
kind = "mimo"
base_url = "https://api.mimo.ai/v1"
api_key_env = "MIMO_API_KEY"
default_model = "mimo-v2"

[[providers.models]]
id = "mimo-v2"
name = "MiMo V2"
```

## 切换 Provider

### 命令行参数

```bash
# 指定 Provider
loomcode --provider openai

# 指定 Provider 和模型
loomcode --provider deepseek --model deepseek-v4-pro
```

### TUI 中切换

```bash
# 在 TUI 中输入
/model gpt-4o
```

## 配置文件位置

LoomCode 按优先级查找配置文件：

1. `./loomcode.toml` - 项目目录（最高优先级）
2. `~/.loomcode/config.toml` - 全局配置

## 环境变量加载

按优先级加载（后覆盖前）：

1. `~/.loomcode/.env` - 全局环境变量
2. `./.env` - 项目环境变量
3. `./.env.local` - 本地覆盖（不提交 git）
4. `--env-file custom.env` - CLI 指定（最高优先级）

## 故障排除

### API Key 错误

```
Error: provider "deepseek" requires environment variable "DEEPSEEK_API_KEY" to be set
```

**解决方案**：确保环境变量已设置

```bash
export DEEPSEEK_API_KEY="sk-your-key"
```

### 连接超时

```
Error: http request: context deadline exceeded
```

**解决方案**：检查网络连接和 API 端点地址

### 模型不存在

```
Error: model "xxx" not found
```

**解决方案**：检查模型 ID 是否正确，或在配置文件中添加模型

## 下一步

- [自定义 Skills](custom-skills.md) - 扩展 LoomCode 功能
- [会话管理](session-management.md) - 保存对话历史
- [配置文件格式参考](../reference/config-format.md) - 完整语法说明
