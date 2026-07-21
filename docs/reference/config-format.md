# 配置文件格式

> settings.json + models.json 完整语法（JSON）

## 概述

LoomCode 的**活配置格式为 JSON**（不再是 TOML）。配置文件按职责分为两个全局文件（合并加载），并支持项目级覆盖：

- **`~/.loomcode/models.json`**（全局）— 模型配置（providers + default_provider）
- **`~/.loomcode/settings.json`**（全局）— 主配置（env、plugins、permissions、search、agent 等）
- **`<project>/.loomcode/settings.json`**（项目级，可提交共享）— 覆盖全局配置
- **`<project>/.loomcode/settings.local.json`**（项目级，本地覆盖，已 gitignore）— 覆盖 `settings.json`

> 旧版 TOML 配置文件（`loomcode.toml` / `config.toml` / `models.toml`）仅作为**迁移输入**，启动时自动迁移为 JSON，现已废弃；旧路径 `./loomcode.json`、`.claude/loomcode.json` 也已不再使用。

## 加载顺序

来自 `internal/config/loader.go`：

1. 先合并 `~/.loomcode/{models.json, settings.json}`（global）
2. 再叠加 `<project>/.loomcode/{settings.json, settings.local.json}`（project 覆盖 global）
3. `settings.local.json` 覆盖 `settings.json`

优先级：**project > global，local > shared**。

`--config <path>` 仍可作为 CLI 参数指定单个配置文件路径。

## 完整示例

### 全局主配置 `~/.loomcode/settings.json`

```json
{
  "env": {
    "DEEPSEEK_API_KEY": "sk-xxxxxxxxxxxxxxxx"
  },
  "plugins": [
    {
      "name": "my-tool",
      "command": "node",
      "args": ["./mcp-server.js"]
    }
  ],
  "permissions": {
    "shell_allowlist": ["git", "go", "make"]
  },
  "search": {
    "engine": "searxng"
  },
  "agent": {
    "planner_model": "deepseek-v4-pro"
  }
}
```

### 全局模型配置 `~/.loomcode/models.json`

```json
{
  "default_provider": "deepseek",
  "providers": [
    {
      "name": "deepseek",
      "display_name": "DeepSeek",
      "kind": "deepseek",
      "base_url": "https://api.deepseek.com",
      "api_key": "${DEEPSEEK_API_KEY}",
      "default_model": "deepseek-v4-flash",
      "models": [
        {
          "id": "deepseek-v4-flash",
          "name": "DeepSeek V4 Flash",
          "context_window": 131072,
          "cost": {
            "input": 0.14,
            "cached_input": 0.014,
            "output": 0.28
          },
          "capabilities": {
            "reasoning": false,
            "tool_call": true,
            "prefix_cache": true,
            "vision": false,
            "voice": false
          }
        }
      ]
    }
  ]
}
```

完整可复制示例见仓库根目录的 [`settings.example.json`](../../settings.example.json) 与 [`models.example.json`](../../models.example.json)。

## 配置项详解

### 顶层配置

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `default_provider` | string | ✅ | 默认 Provider 名称 |
| `providers` | array | ✅ | Provider 配置列表 |
| `plugins` | array | ❌ | MCP 插件配置（canonical 键） |
| `mcpServers` | object | ❌ | MCP 插件配置（兼容别名，自动并入 `plugins`） |
| `env` | object | ❌ | 环境变量（项目 > 全局 > 系统环境变量） |
| `permissions` | object | ❌ | 权限配置 |
| `search` | object | ❌ | 搜索配置 |
| `experimental` | object | ❌ | 实验性功能 |
| `agent` | object | ❌ | Agent 层配置（如 planner_model） |

### Provider 配置

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | Provider 唯一标识 |
| `display_name` | string | ❌ | 显示名称 |
| `kind` | string | ✅ | 适配器类型（`deepseek` / `mimo` / `openai`） |
| `base_url` | string | ✅ | API 端点地址 |
| `api_key` | string | ❌ | API Key，支持 `${ENV_VAR}` 展开（推荐） |
| `api_key_env` | string | ❌ | 已废弃，仅向后兼容；请改用 `api_key: "${ENV}"` |
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
| `cached_input` | float | 缓存命中输入价格 |
| `output` | float | 每百万 token 输出价格 |

### 能力配置

| 字段 | 类型 | 说明 |
|------|------|------|
| `reasoning` | bool | 支持推理 |
| `tool_call` | bool | 支持工具调用 |
| `prefix_cache` | bool | 支持前缀缓存 |
| `vision` | bool | 支持图像 |
| `voice` | bool | 支持语音 |

### 插件配置（MCP）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | 插件名称（唯一） |
| `type` | string | ❌ | 传输类型：`stdio` / `sse` |
| `command` | string | ❌ | stdio 命令路径（与 `url` 二选一） |
| `args` | array | ❌ | 命令参数 |
| `env` | object | ❌ | 子进程环境变量 |
| `url` | string | ❌ | sse 服务地址（与 `command` 二选一） |

### 权限配置

| 字段 | 类型 | 说明 |
|------|------|------|
| `shell_allowlist` | array | 允许的 Shell 命令白名单 |

### 搜索配置

| 字段 | 类型 | 说明 |
|------|------|------|
| `engine` | string | 搜索引擎（grep/ripgrep/searxng） |

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

## API Key 与环境变量

`api_key` 字段支持 `${ENV_VAR}` 语法，加载时自动从下列位置解析（优先级从高到低）：

1. `settings.json` 的 `env` 字段
2. 系统环境变量

```json
{
  "providers": [
    {
      "name": "deepseek",
      "api_key": "${DEEPSEEK_API_KEY}"
    }
  ]
}
```

> 旧版 `api_key_env: "DEEPSEEK_API_KEY"` 仍可使用，但会打印弃用警告，请迁移至 `api_key: "${DEEPSEEK_API_KEY}"`。

## 配置验证

LoomCode 会验证配置文件：

1. 必填字段检查
2. 格式验证（仅支持 `.json`）
3. 环境变量存在性检查（使用 `${ENV_VAR}` 时）

### 验证错误示例

```
Error: provider "deepseek" requires environment variable "DEEPSEEK_API_KEY" to be set
```

## 示例配置

### 基本配置

```json
{
  "default_provider": "deepseek",
  "providers": [
    {
      "name": "deepseek",
      "kind": "deepseek",
      "base_url": "https://api.deepseek.com",
      "api_key": "${DEEPSEEK_API_KEY}",
      "default_model": "deepseek-v4-flash",
      "models": [
        {
          "id": "deepseek-v4-flash",
          "name": "DeepSeek V4 Flash"
        }
      ]
    }
  ]
}
```

### 多 Provider 配置

```json
{
  "default_provider": "deepseek",
  "providers": [
    {
      "name": "deepseek",
      "kind": "deepseek",
      "base_url": "https://api.deepseek.com",
      "api_key": "${DEEPSEEK_API_KEY}",
      "default_model": "deepseek-v4-flash",
      "models": [
        { "id": "deepseek-v4-flash", "name": "DeepSeek V4 Flash" }
      ]
    },
    {
      "name": "openai",
      "kind": "openai",
      "base_url": "https://api.openai.com/v1",
      "api_key": "${OPENAI_API_KEY}",
      "default_model": "gpt-4o",
      "models": [
        { "id": "gpt-4o", "name": "GPT-4o" }
      ]
    }
  ]
}
```

### 带插件配置

```json
{
  "default_provider": "deepseek",
  "providers": [
    {
      "name": "deepseek",
      "kind": "deepseek",
      "base_url": "https://api.deepseek.com",
      "api_key": "${DEEPSEEK_API_KEY}",
      "default_model": "deepseek-v4-flash",
      "models": [
        { "id": "deepseek-v4-flash", "name": "DeepSeek V4 Flash" }
      ]
    }
  ],
  "plugins": [
    {
      "name": "github",
      "command": "mcp-server-github",
      "args": ["--token", "${GITHUB_TOKEN}"]
    }
  ],
  "permissions": {
    "shell_allowlist": ["ls", "pwd", "git"]
  }
}
```

## 故障排除

### JSON 解析错误

```
Error: parse json config: invalid character
```

**检查**：
1. JSON 语法是否正确（引号、逗号、括号匹配）
2. 字符串是否使用双引号
3. 数组/对象格式是否正确

### 缺少必填字段

```
Error: base_url is required
```

**解决**：添加缺失的必填字段

### 环境变量未设置

```
Error: provider "deepseek" requires environment variable "DEEPSEEK_API_KEY" to be set
```

**解决**：设置对应的环境变量，或在 `env` 字段中声明

## 下一步

- [CLI 命令参考](cli-commands.md) - 命令行选项
- [内置工具列表](built-in-tools.md) - 工具配置
- [Provider 接口](provider-interface.md) - 自定义适配器
- [MCP 插件协议](mcp-protocol.md) - MCP 插件集成说明
