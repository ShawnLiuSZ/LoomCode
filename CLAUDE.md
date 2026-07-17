# CLAUDE.md

## Project

LoomCode CLI — multi-model AI agent tool (Go). Binary names: `loomcode` and `loom` (same source, dual command). Module: `github.com/ShawnLiuSZ/loomcode`. Requires Go 1.25+.

## Quick Commands

```bash
make build          # build both binaries to bin/
make test           # go test ./... -count=1
make lint           # golangci-lint run ./... (install: make dev-setup)
make test-qa        # build + vet + test (CI-equivalent)
```

Single package test:
```bash
go test ./internal/agent/ -count=1 -v
go test ./internal/tool/ -run TestPrune -count=1 -v
```

Cross-compile check (no CGO):
```bash
CGO_ENABLED=0 GOOS=windows go build ./...
CGO_ENABLED=0 GOOS=linux   go build ./...
```

## Architecture

Entry: `cmd/loomcode/main.go` — initializes provider, tools, permissions, plugins, then launches TUI or single-run.

```
internal/
  agent/     — Agent loop, modes (build/plan/compose/max), coordinator, subagent
  provider/  — Provider interface + adapters: deepseek/, mimo/, openai/
  tool/      — Tool interface, registry, executor, all built-in tools
  config/    — TOML/JSON config loading, validation, schema generation
  control/   — Permissions, allowlist, cost budget, workspace trust
  session/   — Session persistence (JSONL), crypto, manager
  mcp/       — MCP client (stdio + SSE), plugin manager
  ui/        — Bubble Tea TUI (app.go is the main model)
  dashboard/ — Web dashboard (WebSocket)
  memory/    — SQLite FTS5 long-term memory
  skills/    — Skill loader from ~/.loomcode/skills/ and ~/.agents/skills/
```

## Key Interfaces

- `provider.Provider` — Chat, Stream, Models, Capabilities, Cost
- `tool.Tool` — Name, Execute, Schema, IsReadOnly
- `tool.Registry` — tool registration and lookup

Adding a new Provider: implement `Adapter` + `Provider` in `internal/provider/<name>/`, register in `cmd/loomcode/main.go` `createProvider()`.

Adding a new Tool: implement `Tool` interface in `internal/tool/`, register in `RegisterDefaults()`.

## Conventions

- **Error handling**: `fmt.Errorf("context: %w", err)`. No `log.Fatal` or `panic` in library code.
- **Context propagation**: all IO operations accept `context.Context`.
- **Concurrency**: use `sync.Mutex`/`sync.RWMutex` for shared state. Goroutines sending to channels must have `select { case <-ctx.Done() }` fallback to avoid leaks.
- **Provider capabilities**: behavior driven by `Capabilities` struct, not if-else on provider name.
- **Commit messages**: conventional format — `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`.

## Testing

- Tests use `go test ./... -count=1` (no caching in CI).
- CI runs with `-race` flag — code must pass race detector.
- `.golangci.yml` excludes `errcheck` in test files and for common no-op Close/Remove calls.
- Snapshot/golden tests: compare output against expected strings, no external fixtures.

## Gotchas

- `internal/tool/executor.go:pruneResult` — tool output pruning logic. `maxLines <= 0` uses default; head/tail split is `maxLines/2`.
- `internal/mcp/sse_client.go` — uses two HTTP clients: `httpClient` (30s timeout for POST) and `sseClient` (no timeout, only `ResponseHeaderTimeout` for SSE streams).
- `internal/provider/deepseek/provider.go` — tokenizer loaded via `sync.Once` pattern with `atomic.Bool` + mutex (double-check locking). Not `sync.Once` to allow test reset.
- `internal/agent/coordinator.go` — planner/executor separation. `RunStream` uses goroutine fan-in with `ctx.Done()` on both text and error channels.
- Session data stored in `~/.loomcode/sessions/` as JSONL (first line = meta, rest = messages).
- Config search order: `--config` > `./loomcode.json` > `~/.loomcode/loomcode.json` > `./models.json` > `~/.loomcode/models.json` > defaults.

## Bug Audit

42 bugs found and fixed across 5 rounds (2026-07-16). Reports in `mdocs/bug-audit/`. Main categories: concurrency (15), resource leaks (8), logic errors (10), error handling (6), security (3). Current state: no known unfixed bugs.

## Document Management Rules

### 1. 绑定式文档（与 Issue 一一对应）

- `task.md`: `doc/version/current/tasks/task-{issue号}.md`（只读需求区，开发记录 + ADR + 进度与 commit 同步）
- `api_report.md`: `doc/api_report.md`（累积式，按 Issue 按 API 组织，不覆盖历史）

### 2. 落盘位置铁律

- **交付物**（仅上述绑定式文档 `task.md` / `api_report.md`——它们是仓库唯一入库的交付文档）必须直接写入规定的仓库路径；**严禁**先写到系统临时目录 / 暂存区（`/tmp`、`/private/tmp/...`、各平台 working / temp）再搬运或事后复制。
- **会话级过程文件**（子代理编排 `brief` / `report`、中间计算、调试脚本、草稿）写入**项目内** `.superpowers/`（superpowers SDD / `brief` / `report` / 中间缓存）或 `.claude-tmp/`（临时脚本、测试输出、沙箱）；二者均已 git 忽略、用完即删，**不得**写入 OS 系统临时目录（`/private/tmp/...` 已被 `~/.claude/settings.json` 的 `permissions.deny` 对 Read/Edit 硬拦，技术上也写不进去）。
- **需跨会话复盘的过程产物 / 一次性计划**写入工作区根目录 `foods_tech/scratchpad`（即项目上级目录 `../scratchpad`；工作区共享，跨会话 / 跨项目持久，且位于所有 git 仓之外、天然不入库）；含可长期复用知识的须先晋升为交付物（`task.md` / 知识库）再删。
- 设计 spec / 实现计划由 superpowers 原生流程自理，本规范不规定其落点、不对其加限制。

### 3. 禁止范围

- 交付文档落系统临时目录 / OS scratchpad / 任何本地不入库目录
- 仓库根目录直接新建 Markdown
- 在仓库内 commit 计划 / 设计类文档（这类应放本地不入库目录，如 `docs/superpowers/`、`foods_tech/scratchpad`）

### 4. 例外审批

其他文档类型需事先征得用户同意。
