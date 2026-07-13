# LoomCode CLI 审查建议

> 本文档记录代码审查中发现的问题和改进建议，供后续迭代参考。

---

## API 集成

### ToolCall JSON 格式

**问题**: ToolCall 序列化格式与 OpenAI/DeepSeek 标准不兼容

- 旧格式使用 `Name`/`ID`/`Args` 字段名
- DeepSeek API 要求标准 OpenAI 格式：`id`/`type`/`function`/`arguments`

**修复**: 已修复为 `ToolCallFunc{Name, Arguments}` 结构，Args 序列化为 JSON 字符串放入 `Function.Arguments`

**参考**: [DeepSeek Function Calling 文档](https://api-docs.deepseek.com/zh-cn/guides/function_calling)

### 错误信息显示

**问题**: TUI 中错误消息被单行截断，无法查看完整 API 错误

**修复**: 错误消息改为多行展开显示，每行独立渲染

---

## TUI 交互

### 中文输入

**问题**: 中文输入时出现乱码、光标错位

**修复**:
- 添加 `tea.KeyRunes` 处理多字节字符
- 光标位置改用 `len([]rune())` 计算字符数
- `renderInput` 按 rune 宽度渲染光标

### 模型列表

**问题**: 硬编码模型列表显示了未接入的厂商（gpt-4o、claude）

**修复**: 改为从 `Provider.Models()` 动态获取，只显示配置中注册的模型

---

## Provider 适配器

### 已实现

| Provider | Kind | 特性 |
|----------|------|------|
| DeepSeek | deepseek | reasoning_content, 前缀缓存, 工具修复 |
| MiMo | mimo | OAuth, 语音 ASR, CNY 成本 |
| OpenAI 兼容 | openai | 通用适配, 任意厂商接入 |

### 待优化

- [x] DeepSeek: 前缀缓存 TTL 动态感知
- [x] MiMo: OAuth token 自动刷新
- [x] 通用: HTTP 重试 + 指数退避

---

## 安全性

### 已实现

- [x] 编辑门控三模式（review/auto/yolo）
- [x] Shell 命令白名单 + 危险命令拦截
- [x] 敏感文件保护（.env、credentials、.pem）
- [x] API Key 环境变量注入（不写入配置文件）

### 待优化

- [x] 支持 macOS Keychain / Windows Credential Manager（通过 LOOMCODE_ENCRYPTION_KEY 环境变量）
- [x] 会话文件加密存储（AES-256-GCM）

---

## 性能

### 已实现

- [x] 只读工具并行执行
- [x] 工具结果自动压缩（>3000 token）
- [x] CGO_ENABLED=0 静态编译

### 待优化

- [x] Provider 连接池复用
- [x] 消息上下文增量更新（避免全量序列化）
