# Helix CLI 版本更新日志

> 遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 规范  
> 版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)

---

## [Unreleased] (开发中)

> 以下内容为自 v0.1.0 计划以来的变更，尚未发布。

### 新增
- TUI 中文输入支持（IME 多字节字符）
- 光标按 rune 宽度渲染，消除 CJK 乱码
- `/model` 交互式模型选择器（↑↓ 移动、Enter 确认、Esc 取消）
- `/model <name>` 直接切换指定模型
- 模型列表从 Provider 动态获取，不再硬编码未接入厂商
- Skills 自动加载（`~/.agents/skills` + `~/.helix/skills`）
- `/skills` 显示内置工具 + 外部 skills（含来源标记）
- `--session <id>` 恢复历史会话
- `./bin/helix` 无参数直接启动 TUI
- `build.sh` 构建脚本（dev/release/tui/test）
- `.gitignore` 覆盖所有构建产物

### 变更
- 移除硬编码模型列表（不再显示 gpt-4o、claude 等未接入厂商）
- Provider 切换通过 CLI `--provider` 参数，非 TUI 命令

---

## [0.1.0] - 2026-06-18

### 新增

#### 核心框架
- Go Module 初始化，目录结构搭建
- TOML 配置系统（多 Provider 注册、环境变量注入）
- Provider 适配器模式（OpenAI、DeepSeek、MiMo）
- Agent 推理循环（多轮工具调用、流式输出）
- 工具系统（read_file/write_file/edit_file/bash/grep/glob）
- CLI 入口（run/setup/chat/pipe 模式）
- Bubble Tea 交互式 TUI

#### 安全与审批
- 编辑门控三模式（review/auto/yolo）
- Shell 命令白名单 + 危险命令拦截
- 敏感文件保护（.env、credentials 等）
- 成本控制器（flash-first 策略、绿/黄/红分级）

#### 缓存与成本
- 三层上下文分区（不可变前缀/追加日志/易变草稿）
- 前缀缓存管理（SHA256、TTL、命中追踪）
- 工具调用修复流水线（flatten/scavenge/truncation）
- 并行工具调度（只读并行、写串行、信号量控制）

#### 多 Agent 系统
- 四模式 Agent（Build/Plan/Compose/Max）
- 子 Agent 系统（spawn/run/cancel/parallel）
- 会话管理器（JSONL 持久化、列表/切换/恢复）
- Goal 停止条件 + Judge 模型评估

#### 记忆系统
- SQLite FTS5 全文搜索存储
- 四层记忆体系（checkpoint/project/global/history）
- Dream 知识提取 + Distill 模式识别
- 上下文提示构建（注入 LLM 前缀）

#### 插件与集成
- MCP 客户端（JSON-RPC 2.0 over stdio）
- LSP 客户端（completion/hover/definition/symbols）
- 语音输入框架（ASR 接口 + MiMo 适配器）
- 环境变量管理（.env 加载 + `/env` 命令）

#### 工程化
- Makefile + build.sh 构建脚本
- GoReleaser 多平台发布配置
- install.sh 一键安装脚本
- `.gitignore` 构建产物排除
- 235 个单元测试（15 个模块）

### 变更
- 确定项目名称为 Helix
- 确定形态为纯 CLI 工具
- Provider 层采用 Adapter 工厂模式
- 测试辅助代码统一到 `internal/testutil/`

---

## 版本号说明

| 阶段 | 版本 | 说明 |
|------|------|------|
| 规划与设计 | 0.x | 文档编写、技术验证 |
| Phase 1-4 完成 | 0.1.0 | 全功能 MVP |
| 正式发布 | 1.0.0 | 稳定可用版本 |

---

[Unreleased]: https://github.com/ShawnLiuSZ/Helix/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/ShawnLiuSZ/Helix/releases/tag/v0.1.0
