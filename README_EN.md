# LoomCode CLI

> Double LoomCode · Multi-Model · Extensible

<p align="center">
  <img src="https://img.shields.io/badge/status-active-success?style=flat-square" alt="Status: Active">
  <img src="https://img.shields.io/badge/version-dev-informational?style=flat-square" alt="Version: dev">
  <img src="https://img.shields.io/badge/language-Go-00ADD8?style=flat-square&logo=go" alt="Language: Go">
  <img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="License: MIT">
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey?style=flat-square" alt="Platform: macOS | Linux | Windows">
  <img src="https://img.shields.io/badge/tests-343%20passing-brightgreen?style=flat-square" alt="Tests: 343 passing">
</p>

English | [简体中文](README.md)

> **Rename Notice (2026-07-13)**: This project was formerly named **Helix**. It has been renamed to **LoomCode** as of today. The CLI command changed from `helix` to `loom` / `loomcode` (both are equivalent). The config directory changed from `~/.helix/` to `~/.loomcode/`. The repository URL will be migrated to `ShawnLiuSZ/loomcode` in the near future.

**LoomCode** is a pure-CLI, Go-based, extensible multi-model agent programming tool. It combines the best ideas from [DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix) and [MiMo-Code](https://github.com/XiaomiMiMo/MiMo-Code), with deep optimizations for DeepSeek V4 and Xiaomi MiMo, while supporting any OpenAI-compatible provider through a TOML config file.

---

## Features

- **Multi-Model Support** — DeepSeek V4, Xiaomi MiMo, any OpenAI-compatible provider
- **Four Agent Modes** — Build / Plan / Compose / Max
- **Cross-Platform Native** — macOS / Linux / Windows; Grep/Glob implemented natively in Go, no system command dependencies
- **Edit Snapshot Safety Net** — Auto-snapshot before writes; `/rewind` restores prior state
- **Cross-Session Context** — `list_sessions` / `read_session` tools let the agent access historical conversations
- **Tool Call Repair** — RepairPipeline auto-fixes malformed JSON tool calls
- **Setup Wizard** — `loomcode setup` interactively generates config + `.env`
- **Config Schema** — `loomcode schema` outputs JSON Schema Draft 7 for editor autocomplete
- **MCP Plugin Protocol** — stdio + HTTP, plug in external tools
- **Long-Term Memory** — SQLite FTS5; `/remember` writes project knowledge and user preferences
- **Cost Control** — Real-time token cost tracking with budget caps
- **Auto-Loading Skills** — `~/.loomcode/skills/` directory auto-discovery
- **Prefix Cache** — Leverages DeepSeek/MiMo prefix caching to reduce cost

---

## Installation

**Option 1: Install Script (recommended for end users)**

```bash
curl -fsSL https://raw.githubusercontent.com/ShawnLiuSZ/LoomCode/main/scripts/install.sh | bash
```

The script detects OS and architecture (macOS/Linux/Windows × amd64/arm64), downloads the prebuilt binary, and configures PATH.

**Option 2: Go Source Install (for Go developers)**

```bash
go install github.com/ShawnLiuSZ/loomcode/cmd/loomcode@latest
```

Installs to `$GOPATH/bin/loomcode`. Make sure `$GOPATH/bin` is on your PATH.

**Option 3: Build From Source (for contributors)**

```bash
git clone https://github.com/ShawnLiuSZ/loomcode.git
cd LoomCode
make build
./bin/loomcode --version
```

---

## Quick Start

### First Run: Interactive Setup Wizard

```bash
loomcode setup
```

Five-step wizard: choose provider → enter API key → pick model → generate `~/.loomcode/models.toml` (TOML) + `.env` → emit JSON Schema. (For manual setup, `~/.loomcode/models.json` is recommended.)

### Launch TUI

```bash
loomcode                       # Start interactive TUI (default)
loomcode --provider deepseek   # Specify provider
loomcode --model deepseek-v4-pro  # Specify default model
loomcode --session <id>        # Resume a previous session
```

### One-Shot Task

```bash
loomcode run "Create a hello.go file"
echo "Explain this code" | ./bin/loomcode run
```

### Emit Config Schema

```bash
loomcode schema > ~/.loomcode/schema.json
# Load this file in your editor (VS Code / Vim / etc.) for config autocomplete & validation
```

---

## CLI Commands

### Subcommands

| Command | Description |
|---------|-------------|
| `loomcode` / `loomcode chat` / `loomcode tui` | Start interactive TUI (default) |
| `loomcode run <task>` | One-shot task; also accepts task content from stdin |
| `loomcode setup` | Interactive setup wizard; generates `~/.loomcode/models.toml` + `.env` |
| `loomcode schema` | Output JSON Schema for config validation |
| `loomcode dashboard [addr]` | Launch web dashboard (default `:8080`) |

### Global Options

| Option | Description |
|--------|-------------|
| `--provider <name>` | Specify provider (`deepseek` / `mimo` / `openai`) |
| `--model <id>` | Specify model ID |
| `--config <path>` | Specify config file path |
| `--env-file <path>` | Load custom `.env` |
| `--session <id>` | Resume a session |
| `--version` | Show version |

### Examples

```bash
loomcode                                  # Start TUI
loomcode run "explain this code"          # One-shot task
loomcode --provider deepseek --model deepseek-v4-pro
loomcode --session session_xxxxxxxx       # Resume a session
loomcode dashboard :9090                  # Launch dashboard on port 9090
```

---

## TUI Interaction

| Key | Action |
|-----|--------|
| Type text | Send task to AI |
| Tab | Switch Agent mode (build / plan / compose / max) |
| `/` then Tab | Command autocomplete |
| ↑↓ / Enter / Esc | Interactive pickers (model selection, session resume, etc.) |
| Shift+Enter | Newline |
| Ctrl+C twice | Quit (second confirmation within 3s) |

### TUI Commands

| Command | Description |
|---------|-------------|
| `/help` | Show help |
| `/mode` | Show current mode and model |
| `/build` / `/plan` / `/compose` / `/max` | Switch Agent mode |
| `/goal` | View current stop condition |
| `/goal "<condition>"` | Set stop condition |
| `/goal clear` | Clear stop condition |
| `/steps` | View current max steps |
| `/steps <n>` | Set max steps |
| `/model` | Interactive model picker (↑↓ to select, Enter to confirm) |
| `/model <name>` | Switch model directly |
| `/skills` | Show built-in tools and external skills |
| `/clear` | Clear chat history |
| `/cost` | Show cost stats |
| `/budget` | View current budget |
| `/budget <amount>` | Set session budget cap |
| `/budget clear` | Clear budget |
| `/env` | View environment variables |
| `/env set <KEY> <VALUE>` | Set an environment variable |
| `/env unset <KEY>` | Remove an environment variable |
| `/env reload` | Reload environment variables |
| `/sessions` | List sessions |
| `/sessions new <name>` | Create a new session |
| `/sessions switch <ID>` | Switch to a session |
| `/sessions rename <ID> <new-name>` | Rename a session |
| `/resume` | List and resume historical sessions |
| `/resume <ID>` | Resume a specific session |
| `/compact` | Compact context |
| `/queue` | View task queue |
| `/remember <text>` | Write to long-term memory (project knowledge / user preferences) |
| `/rewind` | List recent edit snapshots |
| `/rewind last` | Restore to the state before the last edit |
| `/rewind <id>` | Restore a snapshot by ID |
| `/quit` / `/exit` | Quit |

---

## Edit Snapshot Safety Net (/rewind)

LoomCode automatically creates snapshot copies before every write_file / edit_file operation. Snapshots live in `~/.loomcode/checkpoints/`, capped at 100 entries. Overwritten or deleted files can be restored via `/rewind`.

```bash
# List the 20 most recent snapshots
/rewind

# Restore the most recent edit
/rewind last

# Restore a specific snapshot
/rewind 1700000000000_001
```

Snapshot metadata (`meta.json`) records: original path, timestamp, triggering tool (`write_file` / `edit_file`), whether the file existed, and file size.

---

## Cross-Session Context

The agent can invoke these tools during task execution to access historical sessions:

| Tool | Description |
|------|-------------|
| `list_sessions` | List recent sessions (filterable by limit/role) |
| `read_session` | Read the full message history of a session |

This lets the agent "remember" prior discussions, conventions, and decisions, avoiding repetitive questions.

---

## Configuration

### Config File

Provider configs support **TOML** and **JSON**. Recommended locations:

- `~/.loomcode/models.json` (global, JSON, recommended)
- `~/.loomcode/models.toml` (global, TOML)
- `./models.json` / `./models.toml` (project-level)

Legacy paths (`loomcode.toml`, `~/.loomcode/loomcode.toml`, `~/.loomcode/config.toml`) remain compatible but are deprecated.

See [`models.example.json`](models.example.json) for a JSON example and [`loomcode.example.toml`](loomcode.example.toml) for a TOML example.

```json
{
  "default_provider": "deepseek",
  "providers": [
    {
      "name": "deepseek",
      "display_name": "DeepSeek",
      "kind": "deepseek",
      "base_url": "https://api.deepseek.com",
      "api_key_env": "DEEPSEEK_API_KEY",
      "default_model": "deepseek-v4-flash",
      "models": [
        {
          "id": "deepseek-v4-flash",
          "name": "DeepSeek V4 Flash",
          "context_window": 131072,
          "cost": {
            "input": 0.14,
            "cached_input": 0.014,
            "output": 0.28
          },
          "capabilities": {
            "tool_call": true,
            "prefix_cache": true
          }
        }
      ]
    }
  ]
}
```

### Supported Provider Types

| `kind` | Provider | Authentication | Notes |
|--------|----------|----------------|-------|
| `deepseek` | DeepSeek | API Key (`Authorization: Bearer`) | Supports reasoning_content, prefix cache, tool-call repair |
| `mimo` | Xiaomi MiMo | API Key (`Authorization: Bearer`) | Supports voice, reasoning, prefix cache; offers both Pay-As-You-Go API and Token Plan |
| `openai` | OpenAI and any OpenAI-compatible provider | API Key | Generic adapter for GPT series, local compatible services, etc. |

For custom OpenAI-compatible providers, set `kind` to `openai` and provide the correct `base_url`.

### Xiaomi MiMo Connection Modes

MiMo currently only supports API Key authentication (OAuth is not yet implemented). It provides two connection modes; use the corresponding `base_url` and `api_key_env`:

| Mode | Description | `base_url` | API Key format |
|------|-------------|------------|----------------|
| Pay-As-You-Go API | Billed by actual usage, suitable for light use | `https://api.xiaomimimo.com/v1` | `sk-xxxxx` |
| Token Plan | Fixed subscription with limited calls in the package | `https://token-plan-cn.xiaomimimo.com/v1` | `tp-xxxxx` |

Pay-As-You-Go example:

```json
{
  "providers": [
    {
      "name": "mimo",
      "kind": "mimo",
      "base_url": "https://api.xiaomimimo.com/v1",
      "api_key_env": "MIMO_API_KEY",
      "default_model": "mimo-v2.5-pro"
    }
  ]
}
```

Token Plan example:

```json
{
  "providers": [
    {
      "name": "mimo-token-plan",
      "kind": "mimo",
      "base_url": "https://token-plan-cn.xiaomimimo.com/v1",
      "api_key_env": "MIMO_TOKEN_PLAN_KEY",
      "default_model": "mimo-v2.5-pro"
    }
  ]
}
```

> Tip: You can declare both modes as separate providers in the config and switch between them with `/model`.

### Offline Token Counting

The DeepSeek provider includes a pure-Go DeepSeek V3 tokenizer. It counts prompt tokens locally for accurate context-window estimation and compaction, with no Python or `transformers` dependency.

```bash
# Use the built-in tokenizer
loomcode --provider deepseek

# Use a custom tokenizer.json (optional)
LOOMCODE_TOKENIZER_PATH=/path/to/tokenizer.json loomcode --provider deepseek
```

Other providers (MiMo / OpenAI) currently fall back to a rough character-based estimate and can be extended later.

### Model Configuration Fields

| Field | Required | Description |
|-------|----------|-------------|
| `id` | Yes | Model ID used in API requests, e.g. `deepseek-v4-pro` |
| `name` | Yes | Display name |
| `context_window` | Yes | Context window size in tokens |
| `cost.input` | No | Input price per million tokens |
| `cost.cached_input` | No | Cache-hit input price per million tokens |
| `cost.output` | No | Output price per million tokens |
| `capabilities.reasoning` | No | Whether reasoning/thinking mode is supported |
| `capabilities.tool_call` | No | Whether function/tool calls are supported |
| `capabilities.prefix_cache` | No | Whether prefix caching is supported |
| `capabilities.vision` | No | Whether vision input is supported |
| `capabilities.voice` | No | Whether voice input is supported |

### JSON Schema Validation

```bash
loomcode schema > ~/.loomcode/schema.json
```

In VS Code, add this to the top of your TOML config:

```toml
#:schema ~/.loomcode/schema.json
```

You'll get field autocomplete, type checks, and enum hints. JSON configs can also be associated with `schema.json` in your editor.

### API Key

The config file uses `api_key_env` to reference an environment variable. You can also set `api_key` directly, which takes precedence over `api_key_env`:

```json
{
  "providers": [
    {
      "name": "deepseek",
      "api_key": "sk-xxxxxxxxxxxxxxxx"
    }
  ]
}
```

> Security recommendation: prefer `api_key_env` + `.env` to avoid committing plaintext secrets.

### Switching Models

In the TUI:

```bash
/model           # interactive picker (only configured models are listed)
/model <id>      # switch directly to a configured model
```

When launching from CLI:

```bash
loomcode --model deepseek-v4-pro
```

### Environment Variables

Four-level `.env` loading (later ones override earlier):

```
1. ~/.loomcode/.env        ← global config
2. ./.env                ← project config
3. ./.env.local          ← local override (not committed)
4. --env-file custom.env ← CLI flag (highest priority)
```

```bash
# .env example
DEEPSEEK_API_KEY=sk-xxxxxxxxxxxxxxxx
MIMO_API_KEY=
OPENAI_API_KEY=
```

---

## MCP Plugins

Configure MCP servers in your config file (`~/.loomcode/models.json` recommended, or `~/.loomcode/models.toml` / project-level equivalents) to extend tool capabilities:

```toml
# stdio mode
[[plugins]]
name    = "my-tool"
command = "node"
args    = ["./mcp-server.js"]

# HTTP/SSE mode
[[plugins]]
name = "remote-tool"
url  = "https://mcp.example.com/sse"
```

---

## Auto-Loading Skills

On startup, skills are auto-loaded from:

```
~/.agents/skills/*/SKILL.md   ← lower priority
~/.loomcode/skills/*/SKILL.md    ← higher priority (same name overrides)
```

Each skill is a directory containing a `SKILL.md` file. Use `/skills` to list all loaded skills.

---

## Tech Stack

| Layer | Technology |
|-------|------------|
| Language | Go (CGO_ENABLED=0) |
| Config | TOML + JSON Schema |
| TUI | Bubble Tea + Lip Gloss |
| Memory Storage | SQLite FTS5 |
| Plugin Protocol | MCP (stdio + HTTP) |
| LSP | JSON-RPC 2.0 |
| API Protocol | OpenAI-compatible |
| Cross-Platform | Unix + Windows (build-tag dispatch) |

---

## Project Structure

```
cmd/loomcode/main.go          ← CLI entry
internal/
  config/                  ← config loading, wizard, schema generation
  provider/                ← multi-vendor model access
    ├── deepseek/          ← DeepSeek V4 adapter
    ├── mimo/              ← MiMo V2.5 adapter
    └── openai/            ← generic OpenAI-compatible adapter
  agent/                   ← agent engine (loop/modes/subagent/judge)
  tool/                    ← tool system
    ├── platform_*.go      ← cross-platform process management
    ├── checkpoint.go      ← edit snapshot manager
    ├── command_tools.go   ← Bash/Grep/Glob tools
    ├── file_tools.go     ← Read/Write/Edit tools
    ├── git_tools.go       ← Git tools
    ├── session_tools.go   ← cross-session context tools
    └── registry.go        ← tool registry
  context/                 ← three-tier partitioned context
  memory/                  ← long-term memory (SQLite FTS5)
  control/                 ← permissions / cost / gating
  session/                 ← session management + JSONL persistence
  mcp/                     ← MCP client
  ui/                      ← Bubble Tea TUI
```

---

## Development

```bash
make build          # Build
make test           # Run tests
make lint           # Lint
make install        # Install to $GOPATH/bin
make release        # Release build
```

Cross-compilation verification:

```bash
GOOS=windows go build ./...    # Verify Windows build
GOOS=linux   go build ./...    # Verify Linux build
```

See [CONTRIBUTING.md](CONTRIBUTING.md).

---

## Project Status

Currently in **active development**.

| Phase | Status |
|-------|--------|
| Phase 1 Core Framework MVP | Done |
| Phase 2 Caching & Cost Optimization | Done |
| Phase 3 Multi-Agent & Memory | Done |
| Phase 4 Ecosystem & Evolution | Done |
| Phase 5 Cross-Platform & Safety Net | Done |

**343 tests passing**, covering 18 modules.

---

## Documentation

- [CODEBUDDY.md](CODEBUDDY.md) — Project entry guide
- [CONTRIBUTING.md](CONTRIBUTING.md) — Contribution guide
- [Docs Index](docs/README.md) — Full documentation index

---

## Acknowledgements

This project incorporates core design ideas from:

- [DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix) — Planner/Executor split-session architecture
- [MiMo-Code](https://github.com/XiaomiMiMo/MiMo-Code) — Prefix cache scheduling and capability-driven tool declaration

---

## License

MIT
