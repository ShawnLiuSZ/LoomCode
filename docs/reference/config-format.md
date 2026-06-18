# 配置文件格式

> helix.toml 完整语法

## 概述

Helix 使用 TOML 格式的配置文件。

## 文件位置

按优先级查找：

1. `--config` 指定的路径
2. `./helix.toml` - 项目目录
3. `~/.helix/config.toml` - 全局配置

## 完整示例

```toml
# 默认 Provider
default_provider = "deepseek"

# Provider 配置
[[providers]]
name = "deepseek"
display_name = "DeepSeek"
kind = "deepseek"
base_url = "https://api.deepseek.com"
api_key_env = "DEEPSEEK_API_KEY"
auth_method = "bearer"
default_model = "deepseek-v4-flash"

# 模型配置
[[providers.models]]
id = "deepseek-v4-flash"
name = "DeepSeek V4 Flash"
context_window = 131072

[providers.models.cost]
input = 0.14
cached_input = 0.014
output = 0.28

[providers.models.capabilities]
reasoning = false
tool_call = true
prefix_cache = true
vision = false
voice = false

# MCP 插件配置
[[plugins]]
name = "github"
command = "mcp-server-github"
args = ["--token", "${GITHUB_TOKEN}"]

# 权限配置
[permissions]
shell_allowlist = ["ls", "pwd", "cat", "grep", "find"]

# 搜索配置
[search]
engine = "grep"

# 实验性功能
[experimental]
maxMode = true
batchTool = false
```

## 配置项详解

### 顶层配置

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `default_provider` | string | ✅ | 默认 Provider 名称 |
| `providers` | array | ✅ | Provider 配置列表 |
| `plugins` | array | ❌ | MCP 插件配置 |
| `permissions` | object | ❌ | 权限配置 |
| `search` | object | ❌ | 搜索配置 |
| `experimental` | object | ❌ | 实验性功能 |

### Provider 配置

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | Provider 唯一标识 |
| `display_name` | string | ❌ | 显示名称 |
| `kind` | string | ✅ | 适配器类型 |
| `base_url` | string | ✅ | API 端点地址 |
| `api_key_env` | string | ❌ | API Key 的环境变量名 |
| `auth_method` | string | ❌ | 认证方式（默认 bearer） |
| `default_model` | string | ❌ | 默认模型 ID |
| `models` | array | ✅ | 模型配置列表 |

### 模型配置

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | string | ✅ | 模型 ID |
| `name` | string | ❌ | 模型显示名称 |
| `context_window` | int | ❌ | 上下文窗口大小（token） |
| `cost` | object | ❌ | 成本配置 |
| `capabilities` | object | ❌ | 能力配置 |

### 成本配置

| 字段 | 类型 | 说明 |
|------|------|------|
| `input` | float | 每百万 token 输入价格（美元） |
| `cached_input` | float | 缓存输入价格 |
| `output` | float | 每百万 token 输出价格 |

### 能力配置

| 字段 | 类型 | 说明 |
|------|------|------|
| `reasoning` | bool | 支持推理 |
| `tool_call` | bool | 支持工具调用 |
| `prefix_cache` | bool | 支持前缀缓存 |
| `vision` | bool | 支持图像 |
| `voice` | bool | 支持语音 |

### 插件配置

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | 插件名称 |
| `command` | string | ✅ | 命令路径 |
| `args` | array | ❌ | 命令参数 |
| `env` | array | ❌ | 环境变量 |

### 权限配置

| 字段 | 类型 | 说明 |
|------|------|------|
| `shell_allowlist` | array | 允许的 Shell 命令白名单 |

### 搜索配置

| 字段 | 类型 | 说明 |
|------|------|------|
| `engine` | string | 搜索引擎（grep/ripgrep） |

### 实验性功能

| 字段 | 类型 | 说明 |
|------|------|------|
| `maxMode` | bool | 启用 Max 模式 |
| `batchTool` | bool | 启用批量工具 |

## 适配器类型

| kind | 说明 |
|------|------|
| `deepseek` | DeepSeek V4 系列 |
| `openai` | OpenAI 兼容接口 |
| `mimo` | Xiaomi MiMo |

## 环境变量替换

配置文件中可以使用环境变量：

```toml
api_key_env = "DEEPSEEK_API_KEY"
```

Helix 会从环境变量中读取实际的 API Key。

## 配置验证

Helix 会验证配置文件：

1. 必填字段检查
2. 格式验证
3. 环境变量存在性检查

### 验证错误示例

```
Error: provider "deepseek" requires environment variable "DEEPSEEK_API_KEY" to be set
```

## 示例配置

### 基本配置

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
```

### 多 Provider 配置

```toml
default_provider = "deepseek"

[[providers]]
name = "deepseek"
kind = "deepseek"
base_url = "https://api.deepseek.com"
api_key_env = "DEEPSEEK_API_KEY"

[[providers.models]]
id = "deepseek-v4-flash"

[[providers]]
name = "openai"
kind = "openai"
base_url = "https://api.openai.com/v1"
api_key_env = "OPENAI_API_KEY"

[[providers.models]]
id = "gpt-4o"
```

### 带插件配置

```toml
default_provider = "deepseek"

[[providers]]
name = "deepseek"
kind = "deepseek"
base_url = "https://api.deepseek.com"
api_key_env = "DEEPSEEK_API_KEY"

[[plugins]]
name = "github"
command = "mcp-server-github"
args = ["--token", "${GITHUB_TOKEN}"]

[permissions]
shell_allowlist = ["ls", "pwd", "git"]
```

## 故障排除

### TOML 解析错误

```
Error: parse config: toml: invalid character
```

**检查**：
1. TOML 语法是否正确
2. 字符串引号是否匹配
3. 数组格式是否正确

### 缺少必填字段

```
Error: base_url is required
```

**解决**：添加缺失的必填字段

### 环境变量未设置

```
Error: provider "deepseek" requires environment variable "DEEPSEEK_API_KEY" to be set
```

**解决**：设置对应的环境变量

## 下一步

- [CLI 命令参考](cli-commands.md) - 命令行选项
- [内置工具列表](built-in-tools.md) - 工具配置
- [Provider 接口](provider-interface.md) - 自定义适配器
