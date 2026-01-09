# MCP Lens

> Lightweight observability dashboard for Claude Code with MCP server intelligence

**Status**: Planning Phase

## Why MCP Lens?

Existing Claude Code analytics tools focus on token usage and costs. MCP Lens fills a gap by providing:

- **MCP Server Observability**: Track which MCP servers you actually use, their latency, and error rates
- **Unused Server Detection**: Identify MCP servers you've configured but never call
- **Lightweight Local Dashboard**: SQLite + embedded web UI (no Prometheus/Grafana stack required)
- **Privacy-First**: All data stored locally by default

## Planned Features

### Phase 1: Foundation
- [ ] Local SQLite storage with configurable retention
- [ ] Claude Code hooks integration (all 8 events)
- [ ] Embedded web dashboard (single binary)
- [ ] Basic metrics: tokens, costs, session duration

### Phase 2: MCP Intelligence
- [ ] MCP server utilization tracking
- [ ] Tool call frequency and success rates
- [ ] Latency histograms per MCP server
- [ ] Unused MCP server detection
- [ ] MCP error aggregation

### Phase 3: Cost Intelligence
- [ ] Cost forecasting (daily/weekly/monthly projections)
- [ ] Budget alerts and thresholds
- [ ] Per-project cost breakdown

### Phase 4: Advanced Analytics
- [ ] Session pattern analysis
- [ ] Multi-agent session isolation
- [ ] Export to OTEL backends (optional)

## How It Works

MCP Lens uses Claude Code's [hooks system](https://docs.anthropic.com/en/docs/claude-code/hooks) to capture tool invocations:

```
Claude Code ──► PostToolUse Hook ──► MCP Lens ──► SQLite DB ──► Web Dashboard
```

## Installation

Coming soon.

## Related Projects

- [ccusage](https://github.com/ryoppippi/ccusage) - CLI usage analysis (9.7k stars)
- [Claude-Code-Usage-Monitor](https://github.com/Maciek-roboblog/Claude-Code-Usage-Monitor) - Terminal UI (6.1k stars)
- [claude-code-otel](https://github.com/ColeMurray/claude-code-otel) - Full OTEL stack

## License

MIT
