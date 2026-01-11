# MCP Lens

Lightweight observability dashboard for Claude Code with MCP server intelligence.

## What is MCP Lens?

MCP Lens is an open-source tool that helps developers understand their Claude Code usage patterns, with a focus on MCP (Model Context Protocol) server observability.

### Key Differentiators
1. **MCP-first**: Track MCP server health, tool utilization, latency, errors
2. **Lightweight local stack**: SQLite + embedded web UI (no Prometheus/Grafana required)
3. **Privacy-first**: All data stored locally by default
4. **Hook-based collection**: Integrates with Claude Code hooks for automatic data capture

## Tech Stack
- **Language**: Go (single binary, no runtime deps)
- **Storage**: SQLite (embedded, portable)
- **Web UI**: HTMX (embedded, no build step)
- **Data Collection**: Claude Code hooks integration

## Development

### Prerequisites
- Go 1.21+
- Make

### Build & Test
```bash
# Build
make build

# Run tests
make test

# Run with race detector
make test-race
```

### Project Structure
```
cmd/mcp-lens/     - CLI entry point
internal/
  analytics/      - MCP utilization and error analysis
  cli/            - Command implementations
  collector/      - JSONL parsing and sync engine
  config/         - Configuration management
  hooks/          - Claude Code hook payload handling
  storage/        - SQLite storage layer
  tui/            - Terminal UI dashboard
web/
  static/         - CSS and assets
  templates/      - HTML templates
```

### Key Files
- `internal/collector/event.go` - Event parsing (minimal and full formats)
- `internal/collector/sync.go` - JSONL to SQLite sync engine
- `internal/storage/sqlite.go` - Database schema and queries
- `internal/analytics/utilization.go` - MCP server utilization analysis
