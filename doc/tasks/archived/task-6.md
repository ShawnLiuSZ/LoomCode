# Task-6: feat: 补充 workflows — task-init 和 task-archive

## 原始需求（只读 - 请勿修改）

> 以下内容由 GitHub Action 从 Issue #6 自动提取

## 概述

补充两个 workflow 脚本：`task-init` 和 `task-archive`，用于任务的初始化和归档流程。

## 需要补充的内容

### 1. task-init workflow
- 用途：任务初始化
- 功能点：（待补充）

### 2. task-archive workflow  
- 用途：任务归档
- 功能点：（待补充）

## 备注

事例和详细需求将在评论中补充。

---

## 进度核对表

- [x] 需求解析完成
- [x] 技术方案确认
- [x] 核心功能开发
- [ ] 单元测试
- [ ] 自测通过
- [ ] PR 已创建
- [ ] Code Review 通过

## 变更日志与决策记录 (ADR)

### 2026-07-16: 补充 workflows
- **决策：** 添加 task-init.yml 和 task-archive.yml 两个 GitHub Actions workflow
- **原因：** 支持任务驱动型开发流程，自动化 task.md 的生成和归档
- **影响：** 创建分支时自动生成 task.md，PR 合并时自动归档

## AI 开发过程（过程文档）

### 需求解析

从 issue 评论中获取了三个示例文件：
1. `task-init.yml` - 初始化任务文档的 workflow
2. `task-archive.yml` - 归档任务的 workflow  
3. `copilot-instructions.md` - 项目规范文档

### 开发记录

**改动：** 添加两个 workflow 文件到 `.github/workflows/` 目录

**文件：**
- `.github/workflows/task-init.yml` - 分支创建时自动生成 task.md
- `.github/workflows/task-archive.yml` - PR 合并时自动归档 task.md

## 知识沉淀（知识文档）

- workflow 使用 `actions/github-script` 执行 Node.js 脚本
- task.md 路径：`doc/version/current/tasks/task-{issue号}.md`
- 归档路径：`doc/tasks/archived/task-{issue号}.md`

## 测试验收（验收文档）

| 验收项 | 预期结果 | 实际结果 | 状态 |
|--------|---------|---------|------|
| 创建分支生成 task.md | 自动创建 doc/version/current/tasks/task-{N}.md | 待验证 | ⏳ |
| PR 合并后归档 | 自动移动到 doc/tasks/archived/ | 待验证 | ⏳ |

---

## 归档信息

- **PR:** #8
- **合并时间:** 2026-07-17
- **合并人:** ShawnLiuSZ
- **分支:** hotfix/lsz/6-supplement-workflows@main260716 → main
