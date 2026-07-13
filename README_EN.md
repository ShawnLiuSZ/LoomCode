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

**LoomCode** is a pure-CLI, Go-based, extensible multi-model agent programming tool. It combines the best ideas from [DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix) and [MiMo-Code](https://github.com/XiaomiMiMo/MiMo-Code), with deep optimizations for DeepSeek V4 and Xiaomi MiMo, while supporting any OpenAI-compatible provider through a TOML config file.

---

## Features

- **Multi-Model Support** — DeepSeek V4, Xiaomi MiMo, any OpenAI-compatible provider
- **Four Agent Modes** — Build / Plan / Compose / Max
- **Cross-Platform Native** — macOS / Linux / Windows; Grep/Glob implemented natively in Go, no system command dependencies
- **Edit Snapshot Safety Net** — Auto-snapshot before writes; `/rewind` restores prior state
- **Cross-Session Context** — `list_sessions` / `read_session` tools let the agent access historical conversations
- **Tool Call Repair** — RepairPipeline auto-fixes malformed JSON tool calls
- **Setup Wizard** — `loomcode setup` interactively generates `loomcode.toml` + `.env`
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

Five-step wizard: choose provider → enter API key → pick model → generate `loomcode.toml` + `.env` → emit JSON Schema.

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
# Load this file in your editor (VS Code / Vim / etc.) for loomcode.toml autocomplete & validation
```

---

## CLI Commands

| Command | Description |
|---------|-------------|
| `loomcode` | Start interactive TUI (default) |
| `loomcode run <task>` | One-shot task |
| `loomcode setup` | Interactive setup wizard |
| `loomcode schema` | Output JSON Schema for config validation |
| `loomcode dashboard [addr]` | Launch web dashboard (default :8080) |
| `loomcode --provider <name>` | Specify provider (deepseek/mimo/openai) |
| `loomcode --model <id>` | Specify model |
| `loomcode --session <id>` | Resume a session |
| `loomcode --env-file <path>` | Load custom .env |
| `loomcode --version` | Show version |

---

## TUI Interaction

| Key | Action |
|-----|--------|
| Type text | Send task to AI |
| Tab | Switch Agent mode (build/plan/compose/max) |
| `/` then Tab | Command autocomplete |
| ↑↓ / Enter / Esc | Interactive pickers (model selection, etc.) |
| Shift+Enter | Newline |
| Ctrl+C twice | Quit (second confirmation within 3s) |

### TUI Commands

| Command | Description |
|---------|-------------|
| `/help` | Show help |
| `/mode` | Show current mode and model |
| `/build` `/plan` `/compose` `/max` | Switch Agent mode |
| `/model` | Interactive model picker (↑↓ to select, Enter to confirm) |
| `/model <name>` | Switch model directly |
| `/rewind` | List recent edit snapshots |
| `/rewind last` | Restore to the state before the last edit |
| `/rewind <id>` | Restore a snapshot by ID |
| `/skills` | Show built-in tools and external skills |
| `/env` | View environment variables |
| `/env set <KEY> <VALUE>` | Set an environment variable |
| `/env unset <KEY>` | Remove an environment variable |
| `/goal` | Set / view / clear stop condition |
| `/sessions` | List sessions |
| `/sessions new <name>` | Create a new session |
| `/sessions switch <ID>` | Switch session |
| `/remember <text>` | Write to long-term memory (project knowledge / user preferences) |
| `/cost` | Show cost stats |
| `/budget <amount>` | Set session budget cap |
| `/compact` | Compact context |
| `/clear` | Clear chat history |
| `/quit` | Quit |

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

`loomcode.toml` (project root) or `~/.loomcode/loomcode.toml` (global). See [`loomcode.example.toml`](loomcode.example.toml).

```toml
default_provider = "deepseek"

[[providers]]
name          = "deepseek"
display_name  = "DeepSeek"
kind          = "deepseek"
base_url      = "https://api.deepseek.com"
api_key_env   = "DEEPSEEK_API_KEY"
default_model = "deepseek-v4-flash"

  [[providers.models]]
  id             = "deepseek-v4-flash"
  name           = "DeepSeek V4 Flash"
  context_window = 131072

  [providers.models.cost]
  input        = 0.14
  cached_input = 0.014
  output       = 0.28

  [providers.models.capabilities]
  tool_call    = true
  prefix_cache = true
```

### JSON Schema Validation

```bash
loomcode schema > ~/.loomcode/schema.json
```

In VS Code, add this to the top of `loomcode.toml`:

```toml
#:schema ~/.loomcode/schema.json
```

You'll get field autocomplete, type checks, and enum hints.

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

Configure MCP servers in `loomcode.toml` to extend tool capabilities:

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
