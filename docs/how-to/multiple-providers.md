# 配置多 Provider

> 同时使用多个 AI 模型

## 场景

您可能需要：
- 使用不同模型处理不同类型任务
- 在主 Provider 不可用时使用备用
- 比较不同模型的输出质量

## 步骤

### 1. 创建配置文件

创建 `loomcode.toml`：

```toml
default_provider = "deepseek"

# DeepSeek - 主力模型
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

# OpenAI - 备用模型
[[providers]]
name = "openai"
display_name = "OpenAI"
kind = "openai"
base_url = "https://api.openai.com/v1"
api_key_env = "OPENAI_API_KEY"
default_model = "gpt-4o"

[[providers.models]]
id = "gpt-4o"
name = "GPT-4o"
context_window = 128000

[[providers.models]]
id = "gpt-4o-mini"
name = "GPT-4o Mini"
context_window = 128000

# MiMo - 特定任务
[[providers]]
name = "mimo"
display_name = "MiMo"
kind = "mimo"
base_url = "https://api.mimo.ai/v1"
api_key_env = "MIMO_API_KEY"
default_model = "mimo-v2"

[[providers.models]]
id = "mimo-v2"
name = "MiMo V2"
context_window = 32000
```

### 2. 设置环境变量

```bash
# .env 文件
DEEPSEEK_API_KEY=sk-your-deepseek-key
OPENAI_API_KEY=sk-your-openai-key
MIMO_API_KEY=sk-your-mimo-key
```

### 3. 验证配置

```bash
loomcode run "Hello"
```

## 使用方式

### 命令行指定

```bash
# 使用 DeepSeek（默认）
loomcode run "任务描述"

# 使用 OpenAI
loomcode --provider openai run "任务描述"

# 使用 MiMo
loomcode --provider mimo run "任务描述"
```

### TUI 中切换

```bash
# 在 TUI 中输入
/model gpt-4o
/model deepseek-v4-pro
/model mimo-v2
```

### 指定 Provider 和模型

```bash
loomcode --provider openai --model gpt-4o-mini
```

## 最佳实践

### 任务分配建议

| 任务类型 | 推荐 Provider | 原因 |
|----------|---------------|------|
| 代码生成 | DeepSeek | 性价比高 |
| 复杂推理 | GPT-4o | 推理能力强 |
| 快速问答 | GPT-4o-mini | 速度快、成本低 |
| 特定领域 | MiMo | 专业优化 |

### 成本控制

```bash
# 查看当前会话成本
/cost

# 使用低成本模型处理简单任务
/model gpt-4o-mini
```

### 备用策略

当主 Provider 不可用时：

```bash
# 自动切换到备用
loomcode --provider openai run "任务"
```

## 故障排除

### API Key 未设置

```
Error: provider "openai" requires environment variable "OPENAI_API_KEY" to be set
```

**解决**：确保环境变量已设置

### Provider 不可用

```
Error: api error (status 503): Service Unavailable
```

**解决**：切换到其他 Provider

```bash
loomcode --provider deepseek run "任务"
```

## 下一步

- [环境变量管理](environment-variables.md) - 配置和管理密钥
- [配置文件格式参考](../reference/config-format.md) - 完整语法
- [Provider 接口](../reference/provider-interface.md) - 开发自定义适配器
