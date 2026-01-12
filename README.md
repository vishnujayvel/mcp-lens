# MCP Lens

> Lightweight observability dashboard for Claude Code with MCP server intelligence

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![SQLite](https://img.shields.io/badge/SQLite-WAL-003B57?style=flat&logo=sqlite)](https://www.sqlite.org/)

## Why MCP Lens?

Existing Claude Code analytics tools focus on token usage and costs. MCP Lens fills a gap by providing:

- **MCP Server Observability**: Track which MCP servers you actually use, their latency, and error rates
- **Unused Server Detection**: Identify MCP servers you've configured but never call
- **Lightweight Local Dashboard**: SQLite + TUI (no Prometheus/Grafana stack required)
- **Privacy-First**: All data stored locally by default

## Quick Start

```bash
# Build
make build

# Initialize data directory and see hook configuration
./mcp-lens init

# Add the hook to your Claude Code settings (~/.claude/settings.json)
# In the "hooks.PostToolUse" section, add:
{
  "type": "command",
  "command": "/path/to/mcp-lens-hook.sh"
}

# After using Claude Code, sync events and view stats
./mcp-lens sync
./mcp-lens stats

# Or launch the interactive TUI dashboard
./mcp-lens
```

## Features

- Local SQLite storage with WAL mode for concurrent access
- Claude Code hooks integration (all hook events)
- File-based JSONL event collection
- Interactive TUI dashboard
- MCP server utilization tracking with health indicators
- Error severity analysis (low/medium/high/critical)
- Real-time event streaming (`tail` command)

## Architecture

### Design Principles

**Zero-Daemon Architecture**: MCP Lens doesn't run as a background process. Instead:

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│   Claude Code   │────►│  Hook Script     │────►│  events.jsonl   │
│   (your work)   │     │  (append-only)   │     │  (local file)   │
└─────────────────┘     └──────────────────┘     └────────┬────────┘
                                                          │
                        ┌──────────────────┐              │ on-demand
                        │   mcp-lens sync  │◄─────────────┘
                        │   (batch ETL)    │
                        └────────┬─────────┘
                                 │
                        ┌────────▼─────────┐
                        │   SQLite + WAL   │
                        │   (local DB)     │
                        └────────┬─────────┘
                                 │
              ┌──────────────────┼──────────────────┐
              │                  │                  │
     ┌────────▼───────┐ ┌───────▼────────┐ ┌──────▼───────┐
     │  mcp-lens tui  │ │ mcp-lens stats │ │ mcp-lens tail│
     │  (dashboard)   │ │ (one-shot)     │ │ (streaming)  │
     └────────────────┘ └────────────────┘ └──────────────┘
```

**Why This Matters**:
- **No CPU overhead**: Nothing runs until you explicitly invoke it
- **No memory footprint**: Single binary, starts fast, exits when done
- **Crash-proof**: Append-only JSONL survives any failure
- **Portable**: Copy `~/.mcp-lens/` to any machine

### Concurrency Model

Uses SQLite's **Write-Ahead Logging (WAL)** for safe concurrent access:

```go
db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)")
```

- Multiple readers can query while sync writes
- No lock contention between TUI refresh and data ingestion
- Automatic crash recovery via WAL replay

### Data Flow

1. **Capture**: Claude Code hooks append JSON events to `events.jsonl`
2. **Sync**: `mcp-lens sync` reads new events, deduplicates, stores to SQLite
3. **Query**: Dashboard/stats read from SQLite (no file parsing)

## Commands

```bash
mcp-lens            # Launch interactive TUI dashboard
mcp-lens init       # Initialize data directory and show hook config
mcp-lens sync       # Sync events from JSONL to SQLite
mcp-lens stats      # Show MCP server statistics (one-shot)
mcp-lens tail       # Stream events in real-time
mcp-lens purge      # Delete all data
mcp-lens version    # Show version
```

## Configuration

Configuration is stored in `~/.mcp-lens/config.toml`:

```toml
[storage]
data_dir = "~/.mcp-lens"
events_file = "events.jsonl"
database = "data.db"
retention_days = 30

[dashboard]
refresh_interval = 5
```

## Project Structure

```
cmd/mcp-lens/       # CLI entrypoint
internal/
├── analytics/      # MCP utilization and error analysis
├── cli/            # Command implementations
├── collector/      # JSONL parsing and sync engine
├── config/         # Configuration management
├── hooks/          # Hook event payload handling
├── storage/        # SQLite storage layer (WAL mode)
└── tui/            # Terminal UI dashboard
```

## Development

```bash
# Run tests
make test

# Run tests with race detector
make test-race

# Build
make build

# Format code
make fmt
```

## Tech Stack

- **Language**: Go 1.21+ (single binary, no runtime dependencies)
- **Database**: SQLite with WAL (via modernc.org/sqlite, pure Go)
- **TUI**: termui/v3

## Related Projects

- [ccusage](https://github.com/ryoppippi/ccusage) - CLI usage analysis
- [Claude-Code-Usage-Monitor](https://github.com/Maciek-roboblog/Claude-Code-Usage-Monitor) - Terminal UI
- [claude-code-otel](https://github.com/ColeMurray/claude-code-otel) - Full OTEL stack

## License

MIT
