# 环境变量管理

> 配置和管理 API 密钥

## 概述

LoomCode 使用环境变量管理 API 密钥和其他敏感配置。

## 环境变量列表

### Provider API Keys

| 变量名 | 说明 | 必填 |
|--------|------|------|
| `DEEPSEEK_API_KEY` | DeepSeek API 密钥 | 使用 DeepSeek 时 |
| `OPENAI_API_KEY` | OpenAI API 密钥 | 使用 OpenAI 时 |
| `MIMO_API_KEY` | MiMo API 密钥 | 使用 MiMo 时 |

### LoomCode 配置

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `LOOMCODE_PROVIDER` | 默认 Provider | 从配置文件读取 |
| `LOOMCODE_MODEL` | 默认模型 | 从配置文件读取 |

### 系统变量

| 变量名 | 说明 |
|--------|------|
| `PATH` | 系统路径 |
| `HOME` | 用户主目录 |
| `USER` | 当前用户名 |

## 设置方式

### 方式一：Shell 导出（临时）

```bash
# 当前终端会话有效
export DEEPSEEK_API_KEY="sk-your-key-here"
export OPENAI_API_KEY="sk-your-key-here"

# 启动 LoomCode
loomcode
```

### 方式二：.env 文件（推荐）

创建 `.env` 文件：

```bash
# .env
DEEPSEEK_API_KEY=sk-your-deepseek-key
OPENAI_API_KEY=sk-your-openai-key
MIMO_API_KEY=sk-your-mimo-key
```

### 方式三：Shell 配置文件（永久）

添加到 `~/.bashrc` 或 `~/.zshrc`：

```bash
# ~/.zshrc
export DEEPSEEK_API_KEY="sk-your-deepseek-key"
export OPENAI_API_KEY="sk-your-openai-key"
```

然后重新加载：

```bash
source ~/.zshrc
```

## .env 文件加载顺序

LoomCode 按优先级加载 .env 文件（后覆盖前）：

1. **全局配置**
   ```
   ~/.loomcode/.env
   ```

2. **项目配置**
   ```
   ./.env
   ```

3. **本地覆盖**（不提交 git）
   ```
   ./.env.local
   ```

4. **CLI 指定**（最高优先级）
   ```bash
   loomcode --env-file custom.env
   ```

## TUI 中管理环境变量

### 查看所有环境变量

```
/env
```

输出示例：

```
环境变量:

  DEEPSEEK_API_KEY = sk-37e1********9425
  OPENAI_API_KEY = sk-your********key-here
```

### 设置环境变量

```
/env set MY_API_KEY my-value
```

### 移除环境变量

```
/env unset MY_API_KEY
```

### 重新加载

```
/env reload
```

## 安全最佳实践

### 1. 不要提交 .env 文件

确认 `.gitignore` 包含：

```
.env
.env.local
```

### 2. 使用 .env.example

创建 `.env.example` 模板：

```bash
# .env.example
DEEPSEEK_API_KEY=sk-your-deepseek-key
OPENAI_API_KEY=sk-your-openai-key
MIMO_API_KEY=sk-your-mimo-key
```

### 3. 定期轮换密钥

```bash
# 生成新密钥后更新 .env
vim .env
```

### 4. 使用密钥管理工具

考虑使用专业密钥管理工具：

- **1Password**
- **LastPass**
- **HashiCorp Vault**
- **AWS Secrets Manager**

### 5. 最小权限原则

只授予必要的权限：

```bash
# 只导出必要的变量
export DEEPSEEK_API_KEY="sk-xxx"
# 不要导出整个 .env
```

## 故障排除

### API Key 未设置

```
Error: provider "deepseek" requires environment variable "DEEPSEEK_API_KEY" to be set
```

**解决**：

```bash
# 检查变量是否设置
echo $DEEPSEEK_API_KEY

# 设置变量
export DEEPSEEK_API_KEY="sk-your-key"
```

### .env 文件未加载

**检查**：
1. 文件路径正确
2. 文件格式正确（无 BOM、无空格）
3. 重启 LoomCode

### 环境变量冲突

```bash
# 检查变量来源
env | grep DEEPSEEK

# 临时覆盖
DEEPSEEK_API_KEY="sk-new-key" loomcode
```

### 密钥泄露检测

```bash
# 检查是否意外提交了密钥
git log --all --full-history -- .env
```

## 开发者指南

### 环境变量提供者接口

```go
// tool/env.go
type EnvProvider interface {
    EnvForSubprocess() []string
}

// 设置全局提供者
tool.SetEnvProvider(&envProvider{})
```

### 子进程环境

工具执行时会自动继承环境变量：

```go
// 执行 bash 命令时
cmd.Env = tool.EnvForSubprocess()
```

## 下一步

- [配置 Provider](../tutorials/configuring-providers.md) - 设置 AI 模型
- [会话管理](session-management.md) - 保存对话历史
- [安全模型](../explanation/security.md) - 理解安全机制
