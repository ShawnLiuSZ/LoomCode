# AGENTS.md

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
- Config search order: `--config` > `./loomcode.toml` > `~/.loomcode/models.{json,toml}` > `./models.{json,toml}` > `~/.loomcode/loomcode.toml` (deprecated) > defaults.

## Bug Audit

42 bugs found and fixed across 5 rounds (2026-07-16). Reports in `mdocs/bug-audit/`. Main categories: concurrency (15), resource leaks (8), logic errors (10), error handling (6), security (3). Current state: no known unfixed bugs.
