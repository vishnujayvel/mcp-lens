# MCP Lens

Lightweight observability dashboard for Claude Code with MCP server intelligence.

## Project Context

### Paths
- Specs: `.claude/specs/mcp-lens/`
- Steering: `.claude/steering/`

## What is MCP Lens?

MCP Lens is an open-source tool that helps developers understand their Claude Code usage patterns, with a focus on MCP (Model Context Protocol) server observability.

### Key Differentiators
1. **MCP-first**: Track MCP server health, tool utilization, latency, errors
2. **Lightweight local stack**: SQLite + embedded web UI (no Prometheus/Grafana required)
3. **Privacy-first**: All data stored locally by default
4. **Hook + OTEL hybrid**: Combine Claude Code hooks with optional OTEL export

## Development Workflow

### Specification-Driven Development
1. `/kiro:spec-requirements mcp-lens` - Generate requirements
2. `/kiro:spec-design mcp-lens` - Create technical design
3. `/kiro:spec-tasks mcp-lens` - Generate implementation tasks
4. `/kiro:spec-impl mcp-lens` - Execute tasks with TDD

### Check Progress
- `/kiro:spec-status mcp-lens` - View current phase and progress

## Tech Stack (Planned)
- **Language**: Go (single binary, no runtime deps)
- **Storage**: SQLite (embedded, portable)
- **Web UI**: HTMX or embedded SPA
- **Data Collection**: Claude Code hooks integration
