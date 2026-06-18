# 快速入门

> 5 分钟上手 Helix CLI

## 前置条件

- Go 1.21 或更高版本
- 一个 AI 模型 API Key（DeepSeek、OpenAI 或其他兼容接口）

## 安装

### 方式一：从源码构建

```bash
# 克隆仓库
git clone https://github.com/ShawnLiuSZ/Helix.git
cd Helix

# 构建
make build

# 验证安装
./bin/helix --version
```

### 方式二：使用 Go install

```bash
go install github.com/ShawnLiuSZ/Helix/cmd/helix@latest
```

### 方式三：Homebrew（macOS/Linux）

```bash
brew tap ShawnLiuSZ/helix
brew install helix
```

## 配置

### 1. 创建配置文件

```bash
# 交互式配置向导
helix setup

# 或手动创建
cp helix.example.toml helix.toml
```

### 2. 设置 API Key

```bash
# 方式一：环境变量（推荐）
export DEEPSEEK_API_KEY="sk-your-key-here"

# 方式二：.env 文件
echo "DEEPSEEK_API_KEY=sk-your-key-here" > .env
```

### 3. 验证配置

```bash
helix run "Hello, what model are you?"
```

## 基本使用

### 启动交互式 TUI

```bash
helix
```

### 单次任务模式

```bash
# 直接执行任务
helix run "创建一个 hello.go 文件"

# 从管道读取
echo "解释这段代码" | helix run
```

### 常用命令

| 命令 | 说明 |
|------|------|
| `Tab` | 切换 Agent 模式 |
| `/help` | 显示帮助 |
| `/model` | 选择模型 |
| `/clear` | 清空聊天 |
| `/cost` | 查看成本 |
| `Ctrl+C` | 退出 |

## 下一步

- [配置 Provider](configuring-providers.md) - 设置多个 AI 模型
- [TUI 交互指南](tui-guide.md) - 学习更多界面操作
- [内置工具](../reference/built-in-tools.md) - 了解可用工具
