# 自定义 Skills

> 创建和管理自定义技能

## 概述

Skills 是 LoomCode 的扩展机制，允许您添加自定义工具和能力。

## Skills 目录结构

```
~/.loomcode/skills/          # 高优先级
~/.agents/skills/         # 低优先级
```

## 创建自定义 Skill

### 基本结构

```
~/.loomcode/skills/
└── my-custom-skill/
    └── SKILL.md
```

### SKILL.md 模板

```markdown
# My Custom Skill

> 简短描述这个 skill 的用途

## 功能说明

详细说明这个 skill 提供什么功能。

## 使用方法

如何使用这个 skill。

## 示例

具体的使用示例。
```

## Skill 加载优先级

1. `~/.loomcode/skills/` - 高优先级（同名覆盖）
2. `~/.agents/skills/` - 低优先级

### 示例

```bash
# 两个目录都有同名 skill
~/.agents/skills/code-review/SKILL.md
~/.loomcode/skills/code-review/SKILL.md

# LoomCode 会使用 ~/.loomcode/skills/code-review/SKILL.md
```

## 查看已加载的 Skills

在 TUI 中输入：

```
/skills
```

输出示例：

```
📦 内置工具 (6):

  ✏️ read_file - Read the contents of a file
  ✏️ write_file - Create or overwrite a file with content
  ✏️ edit_file - Make a precise string replacement in a file
  ✏️ bash - Execute a shell command
  📖 grep - Search for a pattern in files
  📖 glob - Find files matching a glob pattern

🧩 外部 Skills (3):

  📄 code-review [loomcode] - Code review guidelines
  📄 architecture - Architecture documentation
  📄 testing - Testing best practices
```

## Skill 类型

### 文档型 Skills

仅包含 `SKILL.md`，用于提供上下文信息：

```
my-skill/
└── SKILL.md
```

### 工具型 Skills

包含可执行工具的 Skills（高级用法）：

```
my-tool-skill/
├── SKILL.md
├── tool.go
└── tool_test.go
```

## 使用场景

### 1. 项目特定指南

为特定项目创建开发指南：

```bash
mkdir -p ~/.loomcode/skills/my-project
cat > ~/.loomcode/skills/my-project/SKILL.md << 'EOF'
# My Project Guidelines

## 代码规范
- 使用 TypeScript strict 模式
- 所有函数必须有 JSDoc 注释
- 测试覆盖率要求 80%+

## 架构原则
- 使用 Clean Architecture
- 依赖注入
- 单一职责原则
EOF
```

### 2. 团队共享

将 Skills 放入项目仓库：

```bash
mkdir -p .loomcode/skills/team-standards
cat > .loomcode/skills/team-standards/SKILL.md << 'EOF'
# Team Standards

## Git 提交规范
- 使用 Conventional Commits
- 每个 PR 必须有测试
- 代码审查至少 2 人批准
EOF
```

### 3. 个人偏好

创建个人工作流 Skills：

```bash
mkdir -p ~/.loomcode/skills/my-workflow
cat > ~/.loomcode/skills/my-workflow/SKILL.md << 'EOF'
# My Workflow

## 每日流程
1. 先查看待办事项
2. 处理高优先级任务
3. 代码审查
4. 更新文档

## 工具偏好
- 编辑器: VS Code
- 终端: iTerm2
- Git 客户端: lazygit
EOF
```

## 管理 Skills

### 查看所有 Skills

```bash
/skills
```

### 删除 Skill

```bash
rm -rf ~/.loomcode/skills/old-skill
```

### 更新 Skill

```bash
# 编辑 SKILL.md
vim ~/.loomcode/skills/my-skill/SKILL.md
```

## 最佳实践

### SKILL.md 编写指南

1. **标题清晰** - 第一行应简洁描述 skill 用途
2. **结构化内容** - 使用标题和列表组织信息
3. **提供示例** - 包含具体的使用示例
4. **保持简洁** - 避免冗长，聚焦核心信息

### 示例

```markdown
# Code Review Checklist

> 代码审查检查清单

## 必须项
- [ ] 代码能编译/运行
- [ ] 有测试覆盖
- [ ] 符合项目代码规范
- [ ] 无安全漏洞

## 建议项
- [ ] 代码可读性好
- [ ] 无重复代码
- [ ] 有适当的注释
- [ ] 文档已更新
```

## 故障排除

### Skill 未显示

```
/skills 没有显示我的 skill
```

**检查**：
1. 确认目录结构正确
2. 确认 `SKILL.md` 文件存在
3. 重启 LoomCode

### 权限问题

```bash
chmod -R 755 ~/.loomcode/skills/my-skill
```

## 下一步

- [内置工具列表](../reference/built-in-tools.md) - 了解内置工具
- [MCP 插件协议](../reference/mcp-protocol.md) - 更强大的扩展机制
- [架构概览](../explanation/architecture.md) - 理解 LoomCode 设计
