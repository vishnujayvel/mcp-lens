# MCP Lens Requirements

## Project Description

MCP Lens is a lightweight observability dashboard for Claude Code with MCP server intelligence. It provides developers with visibility into their Claude Code usage patterns, focusing specifically on MCP (Model Context Protocol) server observability - a gap not addressed by existing tools.

### Key Differentiators
- **MCP-first**: Track MCP server health, tool utilization, latency, and errors
- **Lightweight local stack**: SQLite + embedded web UI (no Prometheus/Grafana required)
- **Privacy-first**: All data stored locally by default
- **Hook + OTEL hybrid**: Combine Claude Code hooks with optional OTEL export

### Target Users
- Individual developers using Claude Code who want visibility into MCP server performance
- Teams seeking lightweight usage analytics without heavy infrastructure
- Developers optimizing their MCP server configurations

---

## Requirements

### 1 Data Collection

**Description**: MCP Lens shall collect telemetry data from Claude Code sessions via hooks integration.

**Acceptance Criteria**:
- When a Claude Code session starts, MCP Lens shall capture and store the session start event with timestamp and session identifier.
- When a Claude Code session ends, MCP Lens shall capture and store the session end event with duration calculated.
- When a tool is invoked, MCP Lens shall capture the PreToolUse event including tool name, MCP server source, and input parameters.
- When a tool completes execution, MCP Lens shall capture the PostToolUse event including execution duration, success/failure status, and output size.
- When a user submits a prompt, MCP Lens shall capture the UserPromptSubmit event with prompt metadata (not content by default).
- When a subagent stops, MCP Lens shall capture the SubagentStop event with agent identifier and execution context.
- When context compaction occurs, MCP Lens shall capture the PreCompact event with compaction metrics.
- When a notification event occurs, MCP Lens shall capture and categorize the notification type.

---

### 2 Local Storage

**Description**: MCP Lens shall persist all collected data locally using SQLite with configurable retention policies.

**Acceptance Criteria**:
- The system shall store all telemetry data in a local SQLite database file.
- The system shall support configurable data retention periods (default: 30 days).
- When retention period expires for records, the system shall automatically purge expired data.
- The system shall maintain database integrity through proper transaction handling.
- The system shall support database file location configuration via settings.
- While the database exceeds configured size limits, the system shall warn users and suggest cleanup actions.

---

### 3 MCP Server Observability

**Description**: MCP Lens shall provide comprehensive observability into MCP server health, utilization, and performance.

**Acceptance Criteria**:
- The system shall track tool call frequency per MCP server over configurable time windows.
- The system shall calculate and display success/failure rates per MCP server.
- The system shall measure and display latency metrics (p50, p90, p99) per MCP server.
- The system shall detect and highlight MCP servers with zero utilization over a configurable period.
- When an MCP server error rate exceeds configurable threshold, the system shall generate an alert.
- The system shall aggregate and categorize errors by MCP server and error type.
- The system shall display tool utilization breakdown per MCP server showing which tools are most/least used.

---

### 4 Web Dashboard

**Description**: MCP Lens shall provide an embedded web dashboard accessible via local HTTP server.

**Acceptance Criteria**:
- The system shall serve a web dashboard on a configurable local port (default: 8420).
- The dashboard shall display an overview page with key metrics: total sessions, total tool calls, active MCP servers.
- The dashboard shall provide an MCP servers page listing all detected servers with health indicators.
- The dashboard shall provide a sessions page showing recent Claude Code sessions with duration and tool usage.
- The dashboard shall provide time-range filtering for all metrics (last hour, day, week, month, custom).
- The dashboard shall auto-refresh metrics at configurable intervals.
- The dashboard shall be fully functional without external JavaScript dependencies (embedded assets).
- The dashboard shall support light and dark color themes.

---

### 5 Basic Usage Metrics

**Description**: MCP Lens shall track and display fundamental Claude Code usage metrics.

**Acceptance Criteria**:
- The system shall track total token usage per session (input and output tokens separately).
- The system shall calculate and display estimated costs based on model pricing.
- The system shall track session duration and active time.
- The system shall count tool invocations per session.
- The system shall display daily/weekly/monthly usage summaries.
- The system shall track which Claude models are used and their relative usage.

---

### 6 Configuration

**Description**: MCP Lens shall be configurable via file and environment variables.

**Acceptance Criteria**:
- The system shall read configuration from a TOML file in `.claude/mcp-lens.toml` if present.
- The system shall support configuration via environment variables with MCP_LENS_ prefix.
- Environment variables shall override file-based configuration.
- The system shall use sensible defaults when no configuration is provided.
- The system shall validate configuration on startup and report errors clearly.
- Where privacy mode is enabled, the system shall not capture prompt content or tool input/output values.

---

### 7 CLI Interface

**Description**: MCP Lens shall provide a command-line interface for management and quick queries.

**Acceptance Criteria**:
- The CLI shall support a `start` command to launch the dashboard server.
- The CLI shall support a `stop` command to gracefully shutdown the server.
- The CLI shall support a `status` command showing server state and key metrics summary.
- The CLI shall support a `mcp` command listing MCP server statistics.
- The CLI shall support a `export` command to export data in JSON or CSV format.
- The CLI shall support a `config` command to view/validate current configuration.
- The CLI shall provide `--help` documentation for all commands.

---

### 8 Privacy and Security

**Description**: MCP Lens shall implement privacy-first design principles protecting user data.

**Acceptance Criteria**:
- The system shall store all data locally by default with no external transmission.
- The system shall not capture prompt content or tool outputs unless explicitly enabled.
- The system shall bind the web dashboard to localhost only by default.
- Where network binding is configured for non-localhost, the system shall require explicit user confirmation.
- The system shall not log sensitive data (API keys, credentials) under any configuration.
- The system shall provide a data purge command to completely remove all stored data.

---

### 9 Single Binary Distribution

**Description**: MCP Lens shall be distributed as a single binary with no runtime dependencies.

**Acceptance Criteria**:
- The system shall compile to a single executable binary for each supported platform.
- The system shall embed all web assets (HTML, CSS, JS) within the binary.
- The system shall not require external databases, runtimes, or services to function.
- The system shall support macOS (arm64, amd64), Linux (arm64, amd64), and Windows (amd64) platforms.
- The binary size shall not exceed 50MB.

---

### 10 OTEL Export (Optional)

**Description**: MCP Lens shall optionally export telemetry data to OpenTelemetry-compatible backends.

**Acceptance Criteria**:
- Where OTEL export is enabled, the system shall export metrics in OTLP format.
- The system shall support configurable OTEL endpoint URLs.
- The system shall batch exports to minimize network overhead.
- When OTEL export fails, the system shall retry with exponential backoff.
- The system shall continue local operation regardless of OTEL export status.
- The system shall support OTEL authentication via headers configuration.

---

## Non-Functional Requirements

### Performance
- Dashboard page load time shall be under 500ms for up to 100,000 stored events.
- Hook event processing shall add less than 5ms latency to Claude Code operations.
- SQLite queries for dashboard metrics shall complete within 100ms.

### Reliability
- The system shall gracefully handle Claude Code crashes without data loss.
- The system shall recover from unexpected shutdowns on next startup.
- The system shall not interfere with Claude Code functionality under any failure condition.

### Usability
- Installation shall require no more than 3 steps (download, configure hook, start).
- The dashboard shall be usable without reading documentation for basic metrics.
- Error messages shall be actionable and include suggested fixes.

---

## Out of Scope (v1.0)

The following features are explicitly out of scope for the initial release:
- Team/organization aggregation and multi-user support
- Cost forecasting and budget alerts
- Session pattern analysis and ML-based predictions
- Multi-agent session isolation tracking
- Real-time WebSocket streaming
- Cloud sync or remote storage options
