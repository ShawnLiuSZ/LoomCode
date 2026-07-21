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

创建 `models.json`（或 `settings.json`）：

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
          "context_window": 131072
        },
        {
          "id": "deepseek-v4-pro",
          "name": "DeepSeek V4 Pro",
          "context_window": 131072
        }
      ]
    }
  ]
}
```

## Provider 配置详解

### 基本字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | Provider 唯一标识 |
| `display_name` | string | ❌ | 显示名称 |
| `kind` | string | ✅ | 适配器类型（deepseek/openai/mimo） |
| `base_url` | string | ✅ | API 端点地址 |
| `api_key` | string | ❌ | API Key，支持 `${ENV_VAR}` 展开（推荐）；旧 `api_key_env` 已废弃 |
| `auth_method` | string | ❌ | 认证方式（默认 Bearer） |
| `default_model` | string | ❌ | 默认模型 ID |

### 模型配置

```json
{
  "id": "model-id",
  "name": "Model Display Name",
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
```

## 多 Provider 配置

同时配置多个 Provider：

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
    },
    {
      "name": "mimo",
      "kind": "mimo",
      "base_url": "https://api.mimo.ai/v1",
      "api_key": "${MIMO_API_KEY}",
      "default_model": "mimo-v2",
      "models": [
        { "id": "mimo-v2", "name": "MiMo V2" }
      ]
    }
  ]
}
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

LoomCode 使用 JSON 配置，按以下顺序合并加载（来自 `internal/config/loader.go`）：

1. `--config` 指定的路径
2. 先合并 `~/.loomcode/{models.json, settings.json}`（global）
3. 再叠加 `<project>/.loomcode/{settings.json, settings.local.json}`（project 覆盖 global；`settings.local.json` 覆盖 `settings.json`）

优先级：**project > global，local > shared**。旧版 TOML 配置（`loomcode.toml` / `config.toml` / `models.toml`）仅作迁移输入，已废弃。

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
