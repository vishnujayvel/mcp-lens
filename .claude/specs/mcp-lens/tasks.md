# MCP Lens - Implementation Tasks

**Version:** 1.0.0
**Status:** Draft
**Last Updated:** 2026-01-10
**Based On:** [design.md](./design.md)

---

## Overview

This document contains implementation tasks for MCP Lens, organized by phase. Each task is sized for 1-3 hours of implementation work.

**Task Notation:**
- `[ ]` - Pending
- `[x]` - Completed
- `(P)` - Can be executed in parallel with other marked tasks
- `*` - Optional/deferrable

---

## Phase 1: Foundation

### Task 1: Project Scaffolding and Configuration

**Requirements:** FR-1.1, FR-1.5
**Estimated Time:** 2 hours

Set up the Go project structure, dependencies, and configuration management.

- [ ] 1.1: Initialize Go module and create directory structure
  - Create `go.mod` with module `github.com/anthropics/mcp-lens`
  - Create directories: `cmd/mcp-lens/`, `internal/config/`, `internal/hooks/`, `internal/storage/`, `internal/metrics/`, `internal/web/`, `internal/models/`, `web/templates/`, `web/static/`
  - Add dependencies: `modernc.org/sqlite`, `github.com/labstack/echo/v4`
  - Create initial `main.go` with CLI skeleton

- [ ] 1.2: Implement configuration management
  - Create `internal/config/config.go` with Config struct
  - Support TOML config file at `~/.config/mcp-lens/config.toml`
  - Support environment variable overrides (`MCP_LENS_*`)
  - Add default values for all settings
  - Write tests for config loading and defaults

---

### Task 2: SQLite Storage Layer

**Requirements:** FR-1.1
**Estimated Time:** 3 hours

Implement the SQLite storage backend with schema and repository pattern.

- [ ] 2.1: Create database schema and migrations
  - Create `internal/storage/schema.go` with SQL schema constants
  - Implement `InitDB()` function to create tables
  - Create `events`, `sessions`, `mcp_servers`, `daily_stats`, `schema_version` tables
  - Add all indexes defined in design document
  - Write tests verifying schema creation

- [ ] 2.2: Implement Store interface for events
  - Create `internal/storage/store.go` with Store interface
  - Create `internal/storage/sqlite.go` with SQLiteStore implementation
  - Implement `StoreEvent()` with parameterized queries
  - Implement `GetEvents()` with filtering support
  - Write tests for event storage and retrieval

- [ ] 2.3: Implement session and MCP server operations
  - Implement `GetSession()`, `GetSessions()` methods
  - Implement `GetMCPServerStats()`, `GetToolStats()` methods
  - Implement `Cleanup()` for data retention
  - Implement `Close()` for graceful shutdown
  - Write tests for all session/MCP operations

---

### Task 3: Hook Event Receiver

**Requirements:** FR-1.2
**Estimated Time:** 3 hours

Implement the HTTP server that receives Claude Code hook events.

- [ ] 3.1: Define hook event types
  - Create `internal/hooks/events.go` with event structs
  - Define `HookEvent`, `ToolUseEvent`, `SessionEvent` types
  - Match Claude Code hook JSON schema exactly
  - Add JSON tags and validation
  - Write tests for JSON deserialization

- [ ] 3.2: Implement hook receiver HTTP server
  - Create `internal/hooks/receiver.go` with Receiver struct
  - Implement HTTP POST `/hook` endpoint
  - Parse and validate incoming hook payloads
  - Send parsed events to processing channel
  - Implement graceful start/stop
  - Write tests for hook reception

- [ ] 3.3: Implement event processor and storage pipeline
  - Create `internal/hooks/processor.go`
  - Connect receiver events to storage layer
  - Extract MCP server name from tool name patterns
  - Calculate derived fields (duration from Pre/Post correlation)
  - Update session stats on session events
  - Write integration tests for full pipeline

---

### Task 4: Core Domain Models

**Requirements:** FR-1.4, FR-2.1, FR-2.2
**Estimated Time:** 2 hours

Define the domain models and view models for metrics.

- [ ] 4.1: Create domain models
  - Create `internal/models/event.go` with Event model
  - Create `internal/models/session.go` with Session model
  - Create `internal/models/mcp.go` with MCPServer, Tool models
  - Ensure models match database schema
  - Write tests for model validation

- [ ] 4.2: Create metrics view models (P)
  - Create `internal/models/metrics.go`
  - Define `DashboardSummary`, `MCPUtilization`, `ToolSuccessRate`
  - Define `CostSummary`, `CostForecast` structs
  - Add helper methods for formatting
  - Write tests for metric calculations

---

### Task 5: Metrics Calculator

**Requirements:** FR-1.4, FR-2.1, FR-2.2, FR-2.3
**Estimated Time:** 3 hours

Implement metrics computation from stored events.

- [ ] 5.1: Implement basic dashboard metrics
  - Create `internal/metrics/calculator.go` with Calculator struct
  - Implement `GetDashboardSummary()` - total sessions, tokens, costs
  - Query and aggregate from events table
  - Support time range filtering
  - Write tests with sample data

- [ ] 5.2: Implement MCP utilization metrics
  - Create `internal/metrics/mcp.go`
  - Implement `GetMCPUtilization()` - server call counts and percentages
  - Implement `GetToolSuccessRates()` - success/error rates per tool
  - Implement `GetUnusedServers()` - servers with no recent activity
  - Write tests for MCP metrics calculations

- [ ] 5.3: Implement latency percentile calculations (P)
  - Add P50, P90, P99 latency calculation to `GetMCPServerStats()`
  - Use SQLite window functions or in-memory calculation
  - Return `MCPServerStats` with latency histograms
  - Write tests for percentile accuracy

---

### Task 6: Web Dashboard Server

**Requirements:** FR-1.3
**Estimated Time:** 3 hours

Implement the embedded web dashboard using Echo and HTMX.

- [ ] 6.1: Set up Echo server with embedded assets
  - Create `internal/web/server.go` with Server struct
  - Use `//go:embed` to embed templates and static files
  - Configure Echo with timeouts and middleware (logger, recover)
  - Bind to localhost by default
  - Write tests for server startup/shutdown

- [ ] 6.2: Create base HTML templates
  - Create `web/templates/layout.html` with base structure
  - Add HTMX script include
  - Create minimal CSS in `web/static/css/styles.css`
  - Include navigation and layout structure
  - Verify templates parse correctly

- [ ] 6.3: Implement dashboard page handler
  - Create `internal/web/handlers.go` with Handler struct
  - Implement `Dashboard()` handler for main page
  - Create `web/templates/dashboard.html` template
  - Display summary metrics (sessions, tokens, cost, MCP servers)
  - Write tests for handler responses

---

### Task 7: Dashboard Views and HTMX Partials

**Requirements:** FR-1.3, FR-2.1, FR-2.2
**Estimated Time:** 3 hours

Implement additional dashboard pages and HTMX partial updates.

- [ ] 7.1: Implement MCP servers page
  - Create `web/templates/mcp.html` template
  - Implement `MCPServers()` handler
  - Display server utilization table with success rates
  - Add latency columns (avg, P90)
  - Highlight servers with high error rates

- [ ] 7.2: Implement sessions page
  - Create `web/templates/sessions.html` template
  - Implement `Sessions()` handler with pagination
  - Display session list with duration, token count, cost
  - Implement `SessionDetail()` for individual session view
  - Show events within a session

- [ ] 7.3: Implement HTMX partial endpoints
  - Create `web/templates/partials/` directory
  - Implement `/partials/metrics` for dashboard summary refresh
  - Implement `/partials/mcp-table` for MCP table refresh
  - Implement `/partials/recent-events` for event feed
  - Add `hx-get` polling with configurable interval

---

### Task 8: CLI Commands

**Requirements:** FR-1.5
**Estimated Time:** 2 hours

Implement the command-line interface for MCP Lens.

- [ ] 8.1: Implement `serve` command
  - Update `cmd/mcp-lens/main.go` with command parsing
  - Implement `serve` to start both hook receiver and dashboard
  - Support `--hook-port` and `--dashboard-port` flags
  - Support `--bind` flag for network binding
  - Implement graceful shutdown on SIGINT/SIGTERM

- [ ] 8.2: Implement `init` command
  - Generate Claude Code hook configuration JSON
  - Output to stdout or specified file
  - Include all supported hook events
  - Use configured port in curl commands

- [ ] 8.3: Implement utility commands (P)
  - Implement `version` command showing build info
  - Implement `status` command showing server status
  - Implement `purge --confirm` for data deletion
  - Write tests for CLI parsing

---

## Phase 2: MCP Intelligence

### Task 9: MCP Server Identification

**Requirements:** FR-2.1, FR-2.4
**Estimated Time:** 2 hours

Implement intelligent MCP server identification from tool calls.

- [ ] 9.1: Implement rule-based MCP identifier
  - Create `internal/hooks/identifier.go` with MCPIdentifier interface
  - Implement prefix extraction (e.g., `mcp_filesystem_read` → `filesystem`)
  - Add known tool mappings for common MCP servers
  - Handle built-in Claude Code tools (Read, Write, Bash, etc.)
  - Write tests for identification patterns

- [ ] 9.2: Implement MCP config file parsing (P)
  - Read Claude Code MCP config from `~/.config/claude/mcp.json`
  - Extract configured server names and their tools
  - Build tool-to-server mapping from config
  - Fall back to prefix rules if config unavailable
  - Write tests with sample config files

---

### Task 10: Unused Server Detection

**Requirements:** FR-2.4
**Estimated Time:** 1 hour

Implement detection and alerting for unused MCP servers.

- [ ] 10.1: Implement unused server detection
  - Add `GetUnusedServers()` to metrics calculator
  - Compare configured servers against usage data
  - Return servers with no calls in configurable period (default 7 days)
  - Distinguish "never used" vs "stopped being used"
  - Write tests for detection logic

---

### Task 11: Error Aggregation

**Requirements:** FR-2.5
**Estimated Time:** 2 hours

Implement MCP error tracking and aggregation.

- [ ] 11.1: Capture and store error information
  - Extract error details from PostToolUse tool_response
  - Store error type and message in events table
  - Update mcp_servers table with error counts
  - Track error frequency over time

- [ ] 11.2: Implement error aggregation queries
  - Add `GetErrorSummary()` to storage layer
  - Group errors by MCP server and error type
  - Return sample error messages (up to 5 per type)
  - Support time-range filtering
  - Write tests for error aggregation

---

## Phase 3: Cost Intelligence

### Task 12: Cost Calculation

**Requirements:** FR-1.4, FR-3.1
**Estimated Time:** 2 hours

Implement token-based cost calculation.

- [ ] 12.1: Implement cost calculator
  - Create `internal/metrics/cost.go` with CostCalculator
  - Define model pricing constants (Opus, Sonnet, Haiku)
  - Support configurable pricing from config file
  - Calculate cost from input/output token counts
  - Write tests for cost calculations

- [ ] 12.2: Implement token extraction from transcripts
  - Parse Claude Code transcript JSONL files
  - Extract token usage from conversation data
  - Associate tokens with sessions on SessionEnd
  - Handle missing or corrupted transcript files
  - Write tests for transcript parsing

---

### Task 13: Cost Analytics Dashboard

**Requirements:** FR-3.1, FR-3.2
**Estimated Time:** 2 hours

Implement cost visualization and forecasting.

- [ ] 13.1: Implement costs page
  - Create `web/templates/costs.html` template
  - Implement `Costs()` handler
  - Display daily/weekly/monthly cost summaries
  - Show cost breakdown by model
  - Add time-series chart (simple SVG or table)

- [ ] 13.2: Implement cost forecasting
  - Add `GetCostForecast()` to metrics calculator
  - Calculate daily average from recent data
  - Project weekly and monthly estimates
  - Indicate confidence level based on data availability
  - Write tests for forecast calculations

---

### Task 14: Budget Alerts

**Requirements:** FR-3.2
**Estimated Time:** 1 hour

Implement budget threshold alerts.

- [ ] 14.1: Implement budget checking
  - Add budget thresholds to config
  - Check current spend against daily/weekly/monthly budgets
  - Return alert status (ok, warning, critical)
  - Display alert banner on dashboard when threshold exceeded
  - Write tests for budget checking logic

---

## Phase 4: Testing and Polish

### Task 15: Integration Testing

**Requirements:** NFR-2, NFR-3
**Estimated Time:** 2 hours

Create comprehensive integration tests.

- [ ] 15.1: Create end-to-end test suite
  - Test full hook reception → storage → metrics pipeline
  - Test dashboard renders with sample data
  - Test CLI commands
  - Verify graceful shutdown preserves data

- [ ] 15.2: Create performance benchmarks (P)
  - Benchmark event storage throughput
  - Benchmark metrics query performance
  - Verify <100ms hook processing latency
  - Document benchmark results

---

### Task 16: Documentation and Build

**Requirements:** FR-1.5, NFR-6
**Estimated Time:** 2 hours

Create documentation and build automation.

- [ ] 16.1: Create Makefile and build scripts
  - Add targets: build, test, lint, clean
  - Add cross-compilation targets for all platforms
  - Create install.sh for curl-based installation
  - Verify binary size under 20MB

- [ ] 16.2: Create user documentation
  - Update README.md with installation instructions
  - Document configuration options
  - Add quick start guide
  - Include troubleshooting section

---

## Requirements Coverage Matrix

| Requirement | Tasks |
|-------------|-------|
| FR-1.1 Local SQLite Storage | 1.1, 2.1, 2.2, 2.3 |
| FR-1.2 Hooks Integration | 3.1, 3.2, 3.3 |
| FR-1.3 Web Dashboard | 6.1, 6.2, 6.3, 7.1, 7.2, 7.3 |
| FR-1.4 Basic Metrics | 4.2, 5.1, 12.1 |
| FR-1.5 Single Binary | 1.1, 8.1, 8.2, 8.3, 16.1 |
| FR-2.1 MCP Utilization | 5.2, 9.1, 9.2 |
| FR-2.2 Tool Success Rates | 5.2 |
| FR-2.3 Latency Monitoring | 5.3 |
| FR-2.4 Unused Detection | 10.1 |
| FR-2.5 Error Aggregation | 11.1, 11.2 |
| FR-3.1 Cost Forecasting | 12.1, 12.2, 13.1, 13.2 |
| FR-3.2 Budget Alerts | 14.1 |
| NFR-1 Privacy | 2.1, 6.1 |
| NFR-2 Performance | 15.1, 15.2 |
| NFR-3 Reliability | 15.1 |
| NFR-4 Security | 6.1 |
| NFR-6 Maintainability | 16.1, 16.2 |

---

## Task Dependencies

```
Task 1 (Scaffolding)
    └── Task 2 (Storage) ──────────────────┐
    └── Task 3 (Hooks) ────────────────────┤
    └── Task 4 (Models) ───────────────────┤
                                           ▼
                                    Task 5 (Metrics)
                                           │
    ┌──────────────────────────────────────┤
    ▼                                      ▼
Task 6 (Web Server)                 Task 9 (MCP ID)
    │                                      │
    ▼                                      ▼
Task 7 (Dashboard Views)           Task 10 (Unused)
    │                               Task 11 (Errors)
    │                                      │
    └──────────────────┬───────────────────┘
                       ▼
                Task 8 (CLI)
                       │
    ┌──────────────────┼──────────────────┐
    ▼                  ▼                  ▼
Task 12 (Cost)   Task 13 (Cost UI)  Task 14 (Budget)
                       │
                       ▼
                Task 15 (Testing)
                       │
                       ▼
                Task 16 (Docs)
```

---

## Execution Order (Recommended)

**Sprint 1: Foundation (Tasks 1-4)**
1. Task 1: Project Scaffolding
2. Task 2: SQLite Storage
3. Task 3: Hook Receiver
4. Task 4: Domain Models

**Sprint 2: Core Features (Tasks 5-8)**
5. Task 5: Metrics Calculator
6. Task 6: Web Dashboard Server
7. Task 7: Dashboard Views
8. Task 8: CLI Commands

**Sprint 3: MCP Intelligence (Tasks 9-11)**
9. Task 9: MCP Server Identification
10. Task 10: Unused Server Detection
11. Task 11: Error Aggregation

**Sprint 4: Cost & Polish (Tasks 12-16)**
12. Task 12: Cost Calculation
13. Task 13: Cost Analytics Dashboard
14. Task 14: Budget Alerts
15. Task 15: Integration Testing
16. Task 16: Documentation and Build

---

## Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-01-10 | MCP Lens Team | Initial task breakdown from design |
