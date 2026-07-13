# CODEBUDDY.md

This file provides guidance to CodeBuddy Code when working with code in this repository.

---

## 项目概述

**LoomCode CLI** 是一个纯 CLI 形态、基于 Go 语言的可扩展多模型 Agent 编程工具。融合 DeepSeek-Reasonix 和 MiMo-Code 的优点，为 DeepSeek V4 和 Xiaomi MiMo 大模型提供深度优化，同时支持任意 OpenAI 兼容厂商通过配置接入。

- 语言: Go（CGO_ENABLED=0 单二进制分发）
- 配置: TOML
- 插件协议: MCP（stdio + HTTP）
- 记忆存储: SQLite FTS5
- TUI: Bubble Tea + Lip Gloss

---

## 设计文档索引

所有文档统一导航入口: [`docs/README.md`](docs/README.md)

| 文档 | 内容 |
|------|------|
| `docs/LOOMCODE_PLAN.md` | 分 Phase 开发计划、MVP 策略 |
| `docs/LOOMCODE_ARCHITECTURE.md` | 架构设计、模块职责、数据流、安全模型 |
| `docs/LOOMCODE_TEST_GUIDE.md` | 测试规范、数据准备、Mock/Stub 策略 |
| `docs/LOOMCODE_TEST_GENERATORS.md` | Mock/Stub 生成器及接口集成示例 |

---

## 构建命令

```bash
make build          # go build -o bin/loomcode cmd/loomcode/main.go
make test           # go test ./...
make lint           # golangci-lint run
make install        # go install ./cmd/loomcode
make release        # goreleaser build --snapshot --clean
```

---

## 架构概要

```
cmd/loomcode/main.go          ← CLI 入口
internal/
  config/                  ← TOML 配置加载、环境变量注入
  provider/                ← 多厂商模型接入（核心可扩展层）
    ├── registry.go        ← Adapter 注册中心
    ├── adapter.go         ← Adapter 工厂接口
    ├── provider.go        ← Provider 实例接口
    ├── capabilities.go    ← Capabilities 能力声明
    ├── deepseek/          ← DeepSeek V4 适配器
    ├── mimo/              ← MiMo V2.5 适配器
    └── openai/            ← 通用 OpenAI 兼容适配器
  agent/                   ← Agent 引擎
    ├── loop.go            ← 推理循环
    ├── modes.go           ← Build/Plan/Compose/Max 模式
    ├── subagent.go        ← 子 Agent 管理
    └── judge.go           ← Goal 停止条件评估
  tool/                    ← 工具系统
    ├── registry.go        ← 工具注册
    ├── executor.go        ← 执行引擎（分区+守卫链+并行）
    ├── repair.go          ← 工具调用修复流水线
    └── tools/             ← 具体工具实现
  context/                 ← 上下文管理
    ├── partition.go       ← 三层分区（不可变前缀/追加日志/易变草稿）
    ├── cache.go           ← 前缀缓存管理
    ├── checkpoint.go      ← 检查点/重建
    └── compress.go        ← 压缩/剪枝
  memory/                  ← 记忆系统（SQLite FTS5）
  control/                 ← 控制层（成本/权限/门控）
  session/                 ← 会话管理 + JSONL 持久化
  mcp/                     ← MCP 客户端
  ui/                      ← Bubble Tea TUI
```

---

## 核心设计原则

1. **Provider 插件化** — Adapter 工厂模式，任何 OpenAI 兼容厂商通过 TOML 配置接入，无需修改代码
2. **Capability-Driven** — Agent 行为由 Provider 的 `Capabilities` 声明驱动，不做 if-else 厂商判断
3. **Config Over Code** — 模型、工具、插件全部 TOML 声明
4. **Compose Over Inherit** — 接口组合构建复杂行为
5. **Single Binary** — CGO_ENABLED=0，零依赖部署

---

## Phase 1 开发顺序（当前阶段）

```
1. go mod init + 目录结构
2. TOML 配置加载
3. OpenAI 通用 Provider
4. DeepSeek Provider
5. 工具注册 + 执行
6. Agent 推理循环
7. CLI 入口 (loomcode run)
8. Bubble Tea TUI
9. install.sh + .goreleaser.yaml
```

策略是"可交互优先"——尽快做出最小可用版本，能跑通 `loomcode run "任务"` 的端到端闭环。

---

## Provider 扩展方式

接入新厂商只需编辑 `loomcode.toml`，使用 `kind = "openai"` 即可，无需写代码：

```toml
[[providers]]
name         = "my-provider"
display_name = "我的厂商"
kind         = "openai"
base_url     = "https://api.example.com/v1"
api_key_env  = "MY_API_KEY"

  [[providers.models]]
  id   = "my-model"
  name = "My Model"
  context_window = 32768
```
