# LoomCode CLI 文档

> 双螺旋 · 多模型 · 可扩展

## 文档导航

### 📚 Tutorials（学习教程）

面向新手的实践步骤，帮助您从零开始使用 LoomCode。

- [快速入门](tutorials/getting-started.md) - 5 分钟上手 LoomCode
- [配置 Provider](tutorials/configuring-providers.md) - 设置 DeepSeek、OpenAI 等模型
- [TUI 交互指南](tutorials/tui-guide.md) - 学习交互式界面操作

### 📖 How-to Guides（使用指南）

解决特定问题的实用配方。

- [配置多 Provider](how-to/multiple-providers.md) - 同时使用多个 AI 模型
- [自定义 Skills](how-to/custom-skills.md) - 创建和管理自定义技能
- [会话管理](how-to/session-management.md) - 保存、恢复、管理对话历史
- [环境变量管理](how-to/environment-variables.md) - 配置和管理 API 密钥

### 📋 Reference（参考手册）

技术细节的完整描述。

- [CLI 命令参考](reference/cli-commands.md) - 所有命令行选项和参数
- [配置文件格式](reference/config-format.md) - settings.json / models.json 完整语法（JSON）
- [内置工具列表](reference/built-in-tools.md) - 所有内置工具及其参数
- [Provider 接口](reference/provider-interface.md) - Provider 适配器开发指南
- [MCP 插件协议](reference/mcp-protocol.md) - MCP 插件集成说明

### 💡 Explanation（概念解释）

帮助理解系统设计和架构。

- [架构概览](explanation/architecture.md) - 系统整体设计
- [Agent 模式](explanation/agent-modes.md) - Build/Plan/Compose/Max 模式详解
- [工具执行引擎](explanation/tool-execution.md) - 工具调用的并行与串行机制
- [安全模型](explanation/security.md) - 权限控制和沙箱机制

### 📝 项目审查

代码质量与架构改进建议。

- [审查建议文档](review-suggestions.md) - 10 个主要问题及修复建议
- [功能完整性审查](feature-completeness-review.md) - 结合 MiMo/DeepSeek 官方文档的功能对比
- [开发路线图](development-roadmap.md) - v0.1.1 → v0.3.0 功能演进计划

### 🤖 知识库

面向智能体快速参考的项目知识库（Obsidian 规范，含双向链接）。

- [智能体索引 (AGENTS.md)](knowledge/AGENTS.md) - 🤖 智能体优先阅读
- [知识库首页](knowledge/00-Index.md) - 全部文档导航
- [架构总览](knowledge/architecture/架构总览.md) - 分层架构与模块依赖
- [核心模块文档](knowledge/MOC/MOC-核心模块.md) - 11 个核心模块详解
- [接口协议文档](knowledge/MOC/MOC-接口协议.md) - Provider/Tool/MCP 接口

---

## 快速链接

| 资源 | 链接 |
|------|------|
| GitHub | [ShawnLiuSZ/LoomCode](https://github.com/ShawnLiuSZ/loomcode) |
| 问题反馈 | [GitHub Issues](https://github.com/ShawnLiuSZ/loomcode/issues) |
| 贡献指南 | [CONTRIBUTING.md](../CONTRIBUTING.md) |
| 许可证 | [MIT License](../LICENSE) |

---

*最后更新: 2026-06-18*
