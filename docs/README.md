# Helix CLI 文档导航

> 版本: v1.0 | 日期: 2026-06-18

---

## 文档索引

### 项目级

| 文档 | 说明 |
|------|------|
| [README.md](../README.md) | 项目首页：设计哲学、技术栈、快速开始、项目状态 |
| [CODEBUDDY.md](../CODEBUDDY.md) | 项目入口指南：概述、构建命令、架构概要、核心原则 |
| [CONTRIBUTING.md](../CONTRIBUTING.md) | 贡献指南：开发流程、代码规范、PR 模板、文档贡献 |

### 规划与设计

| 文档 | 说明 |
|------|------|
| [HELIX_PLAN.md](./HELIX_PLAN.md) | 开发计划：Phase 1-4 分阶段任务、MVP 策略、开发顺序 |
| [HELIX_ARCHITECTURE.md](./HELIX_ARCHITECTURE.md) | 架构设计：分层架构、模块职责、数据流、安全模型、技术选型 |

### 测试

| 文档 | 说明 |
|------|------|
| [HELIX_TEST_GUIDE.md](./HELIX_TEST_GUIDE.md) | 测试指南：规范、数据准备与清理、Mock/Stub 策略与示例 |
| [HELIX_TEST_GENERATORS.md](./HELIX_TEST_GENERATORS.md) | 测试生成器：生成器三种方式、Provider/MemoryStore 集成示例 |

### 其他

| 文档 | 说明 |
|------|------|
| [CHANGELOG.md](./CHANGELOG.md) | 版本更新日志：按 Keep a Changelog 规范记录 |

---

## 阅读路径

### 新加入项目

```
CODEBUDDY.md → HELIX_PLAN.md → HELIX_ARCHITECTURE.md
```

### 开始贡献

```
CONTRIBUTING.md → CODEBUDDY.md（构建命令）→ HELIX_ARCHITECTURE.md（模块职责）
```

### 开始写代码

```
CODEBUDDY.md（构建命令）→ HELIX_ARCHITECTURE.md（模块职责）
```

### 编写测试

```
HELIX_TEST_GUIDE.md → HELIX_TEST_GENERATORS.md
```

### 接入新模型厂商

```
CODEBUDDY.md（Provider 扩展方式）→ HELIX_ARCHITECTURE.md（Provider 层设计）
```
