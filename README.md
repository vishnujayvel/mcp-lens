# MCP Lens

> Lightweight observability dashboard for Claude Code with MCP server intelligence

**Status**: Beta

## Why MCP Lens?

Existing Claude Code analytics tools focus on token usage and costs. MCP Lens fills a gap by providing:

- **MCP Server Observability**: Track which MCP servers you actually use, their latency, and error rates
- **Unused Server Detection**: Identify MCP servers you've configured but never call
- **Lightweight Local Dashboard**: SQLite + embedded web UI (no Prometheus/Grafana stack required)
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

### Implemented
- [x] Local SQLite storage with WAL mode
- [x] Claude Code hooks integration (all hook events)
- [x] File-based JSONL event collection
- [x] Interactive TUI dashboard
- [x] Basic metrics: sessions, tool calls, error rates
- [x] MCP server utilization tracking
- [x] Tool call frequency and success rates
- [x] Server health indicators (good/warning/critical/unused)
- [x] Error severity analysis
- [x] Real-time event streaming (`tail` command)

### Planned
- [ ] Web dashboard with HTMX
- [ ] Budget alerts and thresholds
- [ ] Per-project cost breakdown
- [ ] Token and cost tracking
- [ ] Export to OTEL backends

## How It Works

MCP Lens uses Claude Code's [hooks system](https://docs.anthropic.com/en/docs/claude-code/hooks) to capture tool invocations:

```
Claude Code ──► PostToolUse Hook ──► JSONL File ──► Sync ──► SQLite DB ──► Dashboard
```

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

## Architecture

```
cmd/mcp-lens/       # CLI entrypoint
internal/
├── analytics/      # MCP utilization and error analysis
├── cli/            # Command implementations
├── collector/      # JSONL parsing and sync engine
├── config/         # Configuration management
├── hooks/          # Hook event payload handling
├── storage/        # SQLite storage layer
└── tui/            # Terminal UI dashboard
web/
├── templates/      # HTML templates (future)
└── static/         # CSS assets (future)
```

## Tech Stack

- **Language**: Go 1.21+
- **Database**: SQLite (via modernc.org/sqlite, pure Go)
- **TUI**: termui/v3

## Related Projects

- [ccusage](https://github.com/ryoppippi/ccusage) - CLI usage analysis
- [Claude-Code-Usage-Monitor](https://github.com/Maciek-roboblog/Claude-Code-Usage-Monitor) - Terminal UI
- [claude-code-otel](https://github.com/ColeMurray/claude-code-otel) - Full OTEL stack

## License

MIT
