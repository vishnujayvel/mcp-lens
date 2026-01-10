# MCP Lens

> Lightweight observability dashboard for Claude Code with MCP server intelligence

**Status**: Implementation In Progress

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

# Generate Claude Code hook configuration
./mcp-lens init > ~/.config/claude/hooks.json

# Start MCP Lens
./mcp-lens serve

# Open dashboard
open http://localhost:9877
```

## Features

### Implemented
- [x] Local SQLite storage with WAL mode
- [x] Claude Code hooks integration (all hook events)
- [x] Embedded web dashboard with HTMX
- [x] Basic metrics: sessions, tokens, costs
- [x] MCP server utilization tracking
- [x] Tool call frequency and success rates
- [x] Latency monitoring per MCP server
- [x] Unused MCP server detection
- [x] Cost forecasting (daily/weekly/monthly)
- [x] Dark theme responsive UI

### Planned
- [ ] Budget alerts and thresholds
- [ ] Per-project cost breakdown
- [ ] Session pattern analysis
- [ ] Multi-agent session isolation
- [ ] Export to OTEL backends

## How It Works

MCP Lens uses Claude Code's [hooks system](https://docs.anthropic.com/en/docs/claude-code/hooks) to capture tool invocations:

```
Claude Code ──► PostToolUse Hook ──► MCP Lens ──► SQLite DB ──► Web Dashboard
```

## Commands

```bash
mcp-lens serve      # Start the server (hooks + dashboard)
mcp-lens init       # Generate Claude Code hook configuration
mcp-lens status     # Show server status
mcp-lens purge      # Delete all data
mcp-lens version    # Show version
```

## Configuration

Create `~/.config/mcp-lens/config.toml`:

```toml
[server]
hook_port = 9876
dashboard_port = 9877
bind_address = "127.0.0.1"

[storage]
database_path = "~/.mcp-lens/data.db"
retention_days = 30

[dashboard]
refresh_interval = 30
theme = "auto"

[cost.models]
opus = { input = 15.0, output = 75.0 }
sonnet = { input = 3.0, output = 15.0 }
haiku = { input = 0.25, output = 1.25 }
```

## Development

```bash
# Run tests
make test

# Build for all platforms
make cross-compile

# Format code
make fmt
```

## Architecture

```
cmd/mcp-lens/       # CLI entrypoint
internal/
├── config/         # Configuration management
├── hooks/          # Hook receiver and event processing
├── storage/        # SQLite storage layer
├── metrics/        # Metrics calculation
└── web/            # Dashboard handlers and templates
web/
├── templates/      # HTML templates
└── static/         # CSS, JS assets
```

## Tech Stack

- **Language**: Go 1.21+
- **Database**: SQLite (via modernc.org/sqlite, pure Go)
- **Web**: Echo v4 + HTMX
- **Templates**: Go html/template

## Related Projects

- [ccusage](https://github.com/ryoppippi/ccusage) - CLI usage analysis (9.7k stars)
- [Claude-Code-Usage-Monitor](https://github.com/Maciek-roboblog/Claude-Code-Usage-Monitor) - Terminal UI (6.1k stars)
- [claude-code-otel](https://github.com/ColeMurray/claude-code-otel) - Full OTEL stack

## License

MIT
