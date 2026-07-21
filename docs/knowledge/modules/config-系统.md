---
tags:
  - 核心模块
  - 配置系统
  - TOML
created: 2026-07-14
updated: 2026-07-14
aliases:
  - Config
  - 配置加载
---

# 配置系统

> ⚙️ TOML 配置加载、环境变量注入、JSON Schema、交互式向导
> 📅 最后更新：2026-07-14

---

## 概述

配置系统负责 TOML 配置文件加载、环境变量注入、JSON Schema 生成和交互式配置向导。采用四级配置优先级，支持项目级和用户级配置。

**代码路径**：`internal/config/`

## 关键文件

| 文件 | 职责 |
|------|------|
| `config.go` | `Config` 结构体、配置加载/解析 |
| `env.go` | 环境变量文件加载（.env） |
| `schema.go` | JSON Schema Draft 7 生成 |
| `wizard.go` | 交互式配置向导 |

## 配置优先级

```
--config <path>                            ← CLI 参数（最高优先级）
    │
<project>/.loomcode/settings.local.json   ← 项目本地覆盖（gitignore）
<project>/.loomcode/settings.json         ← 项目级共享配置
    │
~/.loomcode/{settings.json, models.json}  ← 全局配置（合并加载）
    │
内置默认值                                ← 最低优先级（旧版 TOML 仅迁移输入，已废弃）
```

## 环境变量加载顺序

```
1. ~/.loomcode/.env        ← 全局配置
2. ./.env                  ← 项目配置
3. ./.env.local            ← 本地覆盖（不提交 git）
4. --env-file custom.env   ← CLI 参数（最高优先级）
```

后加载覆盖前加载。

## 配置文件结构

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
            "tool_call": true,
            "prefix_cache": true
          }
        }
      ]
    }
  ],
  "plugins": [
    {
      "name": "my-tool",
      "command": "node",
      "args": ["./mcp-server.js"]
    }
  ],
  "permissions": {
    "shell_allowlist": ["git", "npm", "go", "ls"]
  },
  "search": {
    "engine": "bing"
  }
}
```

## 交互式配置向导

```bash
loomcode setup
```

五步引导：
1. 选择 Provider
2. 输入 API Key
3. 选择模型
4. 生成 `settings.json` + `.env`
5. 输出 JSON Schema

## JSON Schema 校验

```bash
loomcode schema > ~/.loomcode/schema.json
```

在编辑器中关联 `~/.loomcode/schema.json` 到 `settings.json` / `models.json`，即可获得字段补全、类型校验、枚举提示。

## 关键方法

| 方法 | 说明 |
|------|------|
| `Load(path)` | 加载指定路径配置 |
| `LoadDefault()` | 按优先级加载默认配置 |
| `LoadEnvFiles(dir)` | 加载 .env 文件链 |
| `GenerateJSONSchema()` | 生成 JSON Schema |
| `WriteConfig(cfg, path)` | 写入配置文件 |
| `WriteEnvFile(vars, path)` | 写入 .env 文件 |
| `WriteSchemaFile()` | 写入 Schema 文件 |

## 相关文档

- [[provider-层|Provider 层]] — Provider 配置消费方
- [[mcp-插件|MCP 插件]] — 插件配置
- [[../architecture/架构总览|架构总览]]
