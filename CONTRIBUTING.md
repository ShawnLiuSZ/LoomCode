# 贡献指南

感谢你对 LoomCode CLI 的关注！本文档将帮助你了解如何参与项目贡献。

---

## 行为准则

- 尊重所有贡献者，保持建设性讨论
- 聚焦技术问题，就事论事
- 帮助新人融入项目

---

## 如何贡献

### 报告 Bug

1. 在 GitHub Issues 中搜索是否已有相同问题
2. 使用 Bug 报告模板，提供：
   - 环境信息（OS、Go 版本、LoomCode 版本）
   - 复现步骤
   - 期望行为 vs 实际行为
   - 相关日志或截图

### 提交功能请求

1. 在 Issues 中说明功能的使用场景和价值
2. 标注 `enhancement` 标签
3. 等待社区讨论和核心团队确认方向

### 贡献代码

#### 1. 环境准备

```bash
# 安装 Go 1.25+
brew install go

# 克隆仓库
git clone https://github.com/ShawnLiuSZ/loomcode.git
cd loomcode

# 安装开发工具
make dev-setup
```

#### 2. 选择任务

- 查看 Issues 中 `good first issue` 标签的任务
- 查看 [开发计划](./docs/LOOMCODE_PLAN.md) 中未完成的 Phase 任务
- 较大的改动请先创建 Issue 讨论方案

#### 3. 创建分支

```bash
git checkout -b feature/your-feature-name
# 或
git checkout -b fix/your-bug-fix
```

#### 4. 开发

- 遵循 [架构设计](./docs/LOOMCODE_ARCHITECTURE.md) 中的设计原则
- 阅读 [CODEBUDDY.md](./CODEBUDDY.md) 了解构建命令和架构概要

**代码风格：**
- 遵循 Go 官方代码风格（`gofmt`、`go vet`）
- 运行 `make lint` 确保通过 golangci-lint 检查
- 公开函数和类型添加注释

**配置格式：**
- 配置使用 JSON 格式（`settings.json` + `models.json`）
- 参考 [`settings.example.json`](settings.example.json) 和 [`models.example.json`](models.example.json)

**Provider 扩展：**
- 新增 Provider 适配器放在 `internal/provider/<name>/` 下
- 实现 `Adapter` 和 `Provider` 接口
- 在 `registry.go` 中注册

**工具扩展：**
- 新增工具放在 `internal/tool/tools/` 下
- 实现 `Tool` 接口
- 标注 `IsReadOnly()` 以支持并行调度

#### 5. 测试

- 阅读 [测试指南](./docs/LOOMCODE_TEST_GUIDE.md)
- 新功能必须包含测试
- Bug 修复必须包含回归测试
- 运行 `make test` 确保全部通过

```bash
make test
go test -cover ./...
```

#### 6. 提交

```bash
# 提交信息格式
# <type>: <简短描述>

# type 可选值：
# feat      — 新功能
# fix       — Bug 修复
# docs      — 文档更新
# refactor  — 重构
# test      — 测试
# chore     — 构建/工具

# 示例：
git commit -m "feat: add Qwen provider adapter"
git commit -m "fix: handle SSE connection timeout"
git commit -m "docs: update test generator examples"
```

#### 7. 发起 Pull Request

1. 推送到你的分支：`git push origin feature/your-feature`
2. 在 GitHub 创建 Pull Request
3. 填写 PR 描述模板：
   - 变更内容
   - 关联 Issue
   - 测试说明
   - 截图（如涉及 UI 变更）
4. 等待 CI 通过和代码审查

---

## 开发约定

### 目录结构

```
internal/
├── config/       # 配置系统
├── provider/     # 模型 Provider（按厂商分子目录）
├── agent/        # Agent 引擎
├── tool/         # 工具系统
├── context/      # 上下文管理
├── memory/       # 记忆系统
├── control/      # 控制层
├── session/      # 会话管理
├── mcp/          # MCP 客户端
├── ui/           # TUI 界面
└── testutil/     # 测试工具（仅测试使用）
```

### 接口设计原则

1. **面向接口编程** — 核心组件通过接口交互，便于测试和扩展
2. **Capability-Driven** — 行为由 Provider 能力声明驱动，不做厂商硬编码
3. **小接口** — 每个接口 3-5 个方法，通过组合构建复杂行为
4. **Context 传播** — 所有 IO 操作接受 `context.Context` 参数

### 错误处理

- 使用 `fmt.Errorf` 包装错误，保留上下文
- 不要在库代码中使用 `log.Fatal` 或 `panic`
- 错误信息小写开头，不以标点结尾

---

## 文档贡献

- 新增文档放在 `docs/` 目录下
- 更新 `docs/README.md` 导航索引
- 重大变更同步更新 `docs/CHANGELOG.md`
- 文档使用中文编写

---

## 版本管理

- 遵循 [语义化版本](https://semver.org/lang/zh-CN/)（MAJOR.MINOR.PATCH）
- 变更日志遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 规范
- 当前版本号参考 [CHANGELOG.md](./docs/CHANGELOG.md)

---

## 获取帮助

- 查看 [文档导航](./docs/README.md)
- 在 GitHub Issues 中提问
- 阅读 [CODEBUDDY.md](./CODEBUDDY.md) 了解项目架构
