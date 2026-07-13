# 内置工具列表

> 所有内置工具及其参数

## 概述

LoomCode 提供 6 个内置工具，用于文件操作、命令执行和代码搜索。

## 工具列表

| 工具 | 类型 | 说明 |
|------|------|------|
| `read_file` | 只读 | 读取文件内容 |
| `write_file` | 写入 | 创建或覆盖文件 |
| `edit_file` | 写入 | 精确字符串替换 |
| `bash` | 写入 | 执行 Shell 命令 |
| `grep` | 只读 | 搜索文件内容 |
| `glob` | 只读 | 按模式查找文件 |

---

## `read_file`

读取文件内容。

### 参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `path` | string | ✅ | 文件路径 |

### 示例

```json
{
  "path": "src/main.go"
}
```

### 返回值

```json
{
  "content": "package main\n\nimport \"fmt\"\n\nfunc main() {\n    fmt.Println(\"Hello\")\n}"
}
```

---

## `write_file`

创建或覆盖文件。

### 参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `path` | string | ✅ | 文件路径 |
| `content` | string | ✅ | 文件内容 |

### 示例

```json
{
  "path": "hello.go",
  "content": "package main\n\nimport \"fmt\"\n\nfunc main() {\n    fmt.Println(\"Hello, World!\")\n}"
}
```

### 返回值

```json
{
  "content": "File written: hello.go"
}
```

---

## `edit_file`

精确字符串替换。

### 参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `path` | string | ✅ | 文件路径 |
| `old_text` | string | ✅ | 要替换的文本 |
| `new_text` | string | ✅ | 替换后的文本 |

### 示例

```json
{
  "path": "main.go",
  "old_text": "fmt.Println(\"Hello\")",
  "new_text": "fmt.Println(\"Hello, World!\")"
}
```

### 返回值

```json
{
  "content": "File edited: main.go"
}
```

### 注意事项

- `old_text` 必须在文件中存在
- 支持多行替换
- 替换所有匹配项

---

## `bash`

执行 Shell 命令。

### 参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `command` | string | ✅ | Shell 命令 |

### 示例

```json
{
  "command": "ls -la"
}
```

```json
{
  "command": "go build ./..."
}
```

```json
{
  "command": "git status"
}
```

### 返回值

```json
{
  "content": "total 48\ndrwxr-xr-x  6 user  staff   192 Jun 18 10:00 .\ndrwxr-xr-x  3 user  staff    96 Jun 18 09:00 ..\n..."
}
```

### 注意事项

- 命令在当前工作目录执行
- 支持管道和重定向
- 输出包括 stdout 和 stderr
- 支持环境变量

---

## `grep`

搜索文件内容。

### 参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `pattern` | string | ✅ | 正则表达式 |
| `path` | string | ✅ | 文件或目录路径 |

### 示例

```json
{
  "pattern": "func main",
  "path": "src/"
}
```

```json
{
  "pattern": "TODO|FIXME",
  "path": "."
}
```

### 返回值

```json
{
  "content": "src/main.go:3:func main() {\nsrc/app.go:15:func main() {\n"
}
```

### 注意事项

- 使用 `grep -rn` 递归搜索
- 支持正则表达式
- 返回文件名、行号和匹配内容
- 无匹配时返回提示信息

---

## `glob`

按模式查找文件。

### 参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `pattern` | string | ✅ | Glob 模式 |
| `path` | string | ❌ | 搜索目录（默认当前目录） |

### 示例

```json
{
  "pattern": "**/*.go",
  "path": "src/"
}
```

```json
{
  "pattern": "*.test.js"
}
```

```json
{
  "pattern": "**/*_test.go",
  "path": "."
}
```

### 返回值

```json
{
  "content": "src/main.go\nsrc/app.go\nsrc/config.go\n"
}
```

### Glob 模式

| 模式 | 说明 |
|------|------|
| `*` | 匹配任意字符（不含路径分隔符） |
| `**` | 匹配任意字符（含路径分隔符） |
| `?` | 匹配单个字符 |
| `[abc]` | 匹配括号内任意字符 |
| `[!abc]` | 匹配不在括号内的字符 |

---

## 工具类型

### 只读工具

- `read_file`
- `grep`
- `glob`

只读工具：
- 不修改文件系统
- 可以并行执行
- 在 Plan 模式下可用

### 写入工具

- `write_file`
- `edit_file`
- `bash`

写入工具：
- 修改文件系统
- 串行执行
- 需要 Build/Compose/Max 模式

---

## 工具调用示例

### 读取并编辑文件

```json
// 第一步：读取文件
{
  "tool": "read_file",
  "args": {"path": "main.go"}
}

// 第二步：编辑文件
{
  "tool": "edit_file",
  "args": {
    "path": "main.go",
    "old_text": "fmt.Println(\"Hello\")",
    "new_text": "fmt.Println(\"Hello, World!\")"
  }
}
```

### 搜索并执行

```json
// 第一步：搜索文件
{
  "tool": "grep",
  "args": {"pattern": "TODO", "path": "."}
}

// 第二步：执行命令
{
  "tool": "bash",
  "args": {"command": "go test ./..."}
}
```

---

## 故障排除

### 工具未找到

```
Error: unknown tool: xxx
```

**检查**：
1. 工具名称是否正确
2. 是否已注册工具

### 权限不足

```
Error: permission denied
```

**解决**：
1. 检查文件权限
2. 检查 `permissions.shell_allowlist` 配置

### 命令执行失败

```
Error: exit status 1
```

**检查**：
1. 命令语法是否正确
2. 依赖是否安装
3. 查看错误输出

---

## 下一步

- [MCP 插件协议](mcp-protocol.md) - 扩展更多工具
- [工具执行引擎](../explanation/tool-execution.md) - 理解执行机制
- [安全模型](../explanation/security.md) - 权限控制
