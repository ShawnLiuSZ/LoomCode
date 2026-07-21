# LoomCode CLI 基础示例

## 快速开始

### 1. 安装

```bash
# 从源码构建
git clone https://github.com/ShawnLiuSZ/loomcode.git
cd LoomCode
make build

# 或使用 Go install
go install github.com/ShawnLiuSZ/loomcode/cmd/loomcode@latest
```

### 2. 配置

```bash
# 复制配置模板到全局配置目录
cp ../../settings.example.json ~/.loomcode/settings.json
cp ../../models.example.json ~/.loomcode/models.json

# 设置 API Key
export DEEPSEEK_API_KEY="sk-your-key-here"
```

### 3. 使用

```bash
# 启动交互式 TUI
./bin/loomcode

# 执行单次任务
./bin/loomcode run "创建一个 hello.go 文件"

# 启动 Web Dashboard
./bin/loomcode dashboard
```

## 功能演示

### 文件操作

```bash
# 读取文件
./bin/loomcode run "读取 main.go 的内容"

# 创建文件
./bin/loomcode run "创建一个简单的 HTTP 服务器"

# 编辑文件
./bin/loomcode run "在 main.go 中添加日志功能"
```

### 代码搜索

```bash
# 搜索代码
./bin/loomcode run "搜索所有包含 TODO 的文件"

# 查找函数
./bin/loomcode run "找到所有处理 HTTP 请求的函数"
```

### 命令执行

```bash
# 运行测试
./bin/loomcode run "运行所有单元测试"

# 构建项目
./bin/loomcode run "构建并验证项目"
```

## TUI 快捷键

| 快捷键 | 功能 |
|--------|------|
| `Tab` | 切换 Agent 模式 |
| `/` | 显示命令列表 |
| `Ctrl+C` | 退出 |

## Agent 模式

- **Build**: 完整工具权限，可执行任何操作
- **Plan**: 只读分析，不修改文件
- **Compose**: 规格驱动开发
- **Max**: 并行多候选（实验性）

## 更多信息

- [完整文档](../docs/README.md)
- [CLI 命令参考](../docs/reference/cli-commands.md)
- [配置文件格式](../docs/reference/config-format.md)
