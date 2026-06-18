# Helix CLI 版本更新日志

> 遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 规范  
> 版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)

---

## [Unreleased]

### 新增
- 项目初始化，确定技术选型与架构方向
- 完成 DeepSeek-Reasonix 与 MiMo-Code 项目调研
- 编写开发计划文档 (`HELIX_PLAN.md`)
- 编写架构设计文档 (`HELIX_ARCHITECTURE.md`)
- 编写测试指南 (`HELIX_TEST_GUIDE.md`)
- 编写测试生成器指南 (`HELIX_TEST_GENERATORS.md`)
- 建立项目文档导航索引 (`docs/README.md`)

### 变更
- 确定项目名称为 Helix
- 确定形态为纯 CLI 工具，暂不做桌面应用
- 确定 Provider 层采用 Adapter 工厂模式实现可扩展

---

## [0.1.0] - 2026-06-18

### 新增
- 创建项目仓库，初始化 Go Module
- 安装 Go 1.26.3 开发环境
- 创建基础目录结构
- 编写 `CODEBUDDY.md` 项目入口指南

---

## 版本号说明

| 阶段 | 版本 | 说明 |
|------|------|------|
| 规划与设计 | 0.x | 文档编写、技术验证 |
| Phase 1 MVP | 0.1+ | 核心框架、基础 Agent |
| Phase 2 缓存优化 | 0.2+ | 前缀缓存、成本控制 |
| Phase 3 多 Agent | 0.3+ | 多模式、记忆系统 |
| Phase 4 生态 | 0.4+ | Dream/Distill、MCP、LSP |
| 正式发布 | 1.0.0 | 稳定可用版本 |

---

[Unreleased]: https://github.com/ShawnLiuSZ/Helix/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/ShawnLiuSZ/Helix/releases/tag/v0.1.0
