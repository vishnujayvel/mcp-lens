# MCP Lens - Technical Design Document

**Version:** 1.0.0
**Status:** Draft
**Last Updated:** 2026-01-10
**Based On:** [requirements.md](./requirements.md)

---

## 1. Architecture Overview

### 1.1 System Context

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Developer Workstation                        │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌──────────────┐     Hook Events      ┌─────────────────────────┐  │
│  │              │ ─────────────────────▶│                         │  │
│  │  Claude Code │     (HTTP POST)       │       MCP Lens          │  │
│  │              │                       │                         │  │
│  └──────────────┘                       │  ┌───────────────────┐  │  │
│         │                               │  │   Hook Receiver   │  │  │
│         │ MCP                           │  │   (port 9876)     │  │  │
│         ▼                               │  └─────────┬─────────┘  │  │
│  ┌──────────────┐                       │            │            │  │
│  │ MCP Servers  │                       │            ▼            │  │
│  │  - filesystem│                       │  ┌───────────────────┐  │  │
│  │  - github    │                       │  │   Event Store     │  │  │
│  │  - custom    │                       │  │   (SQLite)        │  │  │
│  └──────────────┘                       │  └─────────┬─────────┘  │  │
│                                         │            │            │  │
│                                         │            ▼            │  │
│  ┌──────────────┐     HTTP/HTMX        │  ┌───────────────────┐  │  │
│  │   Browser    │ ◀────────────────────│  │   Web Dashboard   │  │  │
│  │              │                       │  │   (port 9877)     │  │  │
│  └──────────────┘                       │  └───────────────────┘  │  │
│                                         │                         │  │
│                                         └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

### 1.2 Architecture Pattern

**Layered Architecture with Event Sourcing Lite**

The system uses a simplified event sourcing pattern where all Claude Code hook events are stored as immutable records, with derived views (metrics, aggregations) computed on read.

| Layer | Responsibility | Components |
|-------|---------------|------------|
| **Presentation** | Web UI, API responses | Echo handlers, HTMX templates |
| **Application** | Business logic, metrics computation | Services |
| **Domain** | Core entities, events | Models |
| **Infrastructure** | Persistence, external I/O | SQLite repository |

### 1.3 Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| **Language** | Go 1.21+ | Single binary, cross-platform, excellent concurrency |
| **SQLite Driver** | modernc.org/sqlite | Pure Go, no CGO, easy cross-compilation |
| **Web Framework** | Echo v4 | Lightweight, fast, good middleware support |
| **UI Approach** | HTMX + Go templates | No build step, embedded in binary, SSR |
| **Asset Embedding** | Go embed.FS | Single binary distribution |

---

## 2. Component Design

### 2.1 Package Structure

```
mcp-lens/
├── cmd/
│   └── mcp-lens/
│       └── main.go              # Entry point, CLI handling
├── internal/
│   ├── config/
│   │   └── config.go            # Configuration management
│   ├── hooks/
│   │   ├── receiver.go          # HTTP hook receiver
│   │   ├── events.go            # Event type definitions
│   │   └── parser.go            # Event parsing and validation
│   ├── storage/
│   │   ├── store.go             # Storage interface
│   │   ├── sqlite.go            # SQLite implementation
│   │   ├── schema.go            # Database schema
│   │   └── migrations.go        # Schema migrations
│   ├── metrics/
│   │   ├── calculator.go        # Metrics computation
│   │   ├── mcp.go               # MCP-specific metrics
│   │   └── cost.go              # Cost calculations
│   ├── web/
│   │   ├── server.go            # Echo server setup
│   │   ├── handlers.go          # HTTP handlers
│   │   ├── middleware.go        # Custom middleware
│   │   └── routes.go            # Route definitions
│   └── models/
│       ├── session.go           # Session model
│       ├── event.go             # Event model
│       ├── mcp.go               # MCP server/tool models
│       └── metrics.go           # Metrics view models
├── web/
│   ├── templates/
│   │   ├── layout.html          # Base layout
│   │   ├── dashboard.html       # Main dashboard
│   │   ├── mcp.html             # MCP server view
│   │   ├── costs.html           # Cost analytics
│   │   └── partials/            # HTMX partial templates
│   └── static/
│       ├── css/
│       │   └── styles.css       # Custom styles
│       └── js/
│           └── htmx.min.js      # HTMX library
└── go.mod
```

### 2.2 Component Interfaces

#### 2.2.1 Hook Receiver (FR-1.2)

```go
// Package hooks handles Claude Code hook event reception and processing.
package hooks

import (
    "context"
    "time"
)

// HookEvent represents the base structure of all Claude Code hook events.
type HookEvent struct {
    SessionID      string          `json:"session_id"`
    TranscriptPath string          `json:"transcript_path"`
    Cwd            string          `json:"cwd"`
    PermissionMode string          `json:"permission_mode"`
    HookEventName  string          `json:"hook_event_name"`
    Timestamp      time.Time       `json:"timestamp,omitempty"`
}

// ToolUseEvent extends HookEvent for PreToolUse and PostToolUse events.
type ToolUseEvent struct {
    HookEvent
    ToolName     string                 `json:"tool_name"`
    ToolInput    map[string]interface{} `json:"tool_input"`
    ToolResponse map[string]interface{} `json:"tool_response,omitempty"`
}

// Receiver handles incoming hook events via HTTP.
type Receiver interface {
    // Start begins listening for hook events on the configured port.
    Start(ctx context.Context) error

    // Stop gracefully shuts down the receiver.
    Stop(ctx context.Context) error

    // Events returns a channel of parsed events for processing.
    Events() <-chan ParsedEvent
}

// ParsedEvent wraps a hook event with metadata.
type ParsedEvent struct {
    Raw       []byte
    Event     HookEvent
    Tool      *ToolUseEvent // Non-nil for tool events
    ReceivedAt time.Time
}

// ReceiverConfig configures the hook receiver.
type ReceiverConfig struct {
    Port           int           // Default: 9876
    ReadTimeout    time.Duration // Default: 5s
    WriteTimeout   time.Duration // Default: 10s
    MaxBodySize    int64         // Default: 1MB
}
```

#### 2.2.2 Storage Layer (FR-1.1)

```go
// Package storage provides persistence for hook events and metrics.
package storage

import (
    "context"
    "time"
)

// Store defines the persistence interface for MCP Lens.
type Store interface {
    // Event operations
    StoreEvent(ctx context.Context, event *Event) error
    GetEvents(ctx context.Context, filter EventFilter) ([]Event, error)

    // Session operations
    GetSession(ctx context.Context, sessionID string) (*Session, error)
    GetSessions(ctx context.Context, filter SessionFilter) ([]Session, error)

    // MCP metrics operations
    GetMCPServerStats(ctx context.Context, filter TimeFilter) ([]MCPServerStats, error)
    GetToolStats(ctx context.Context, filter TimeFilter) ([]ToolStats, error)

    // Cost operations
    GetCostSummary(ctx context.Context, filter TimeFilter) (*CostSummary, error)
    GetCostByModel(ctx context.Context, filter TimeFilter) ([]ModelCost, error)

    // Maintenance
    Cleanup(ctx context.Context, olderThan time.Time) (int64, error)
    Close() error
}

// Event represents a stored hook event.
type Event struct {
    ID            int64
    SessionID     string
    EventType     string
    ToolName      string
    MCPServer     string // Extracted from tool name pattern
    Success       bool
    DurationMs    int64
    InputTokens   int64
    OutputTokens  int64
    CostUSD       float64
    RawPayload    []byte
    CreatedAt     time.Time
}

// Session represents a Claude Code session.
type Session struct {
    ID           string
    Cwd          string
    StartedAt    time.Time
    EndedAt      *time.Time
    TotalEvents  int
    TotalCostUSD float64
}

// MCPServerStats holds aggregated metrics for an MCP server.
type MCPServerStats struct {
    ServerName    string
    TotalCalls    int64
    SuccessCount  int64
    ErrorCount    int64
    AvgLatencyMs  float64
    P50LatencyMs  float64
    P90LatencyMs  float64
    P99LatencyMs  float64
    LastUsedAt    time.Time
}

// TimeFilter specifies a time range for queries.
type TimeFilter struct {
    From time.Time
    To   time.Time
}

// EventFilter specifies criteria for event queries.
type EventFilter struct {
    TimeFilter
    SessionID  string
    EventTypes []string
    ToolNames  []string
    MCPServers []string
    Limit      int
    Offset     int
}
```

#### 2.2.3 Metrics Calculator (FR-1.4, FR-2.1, FR-2.2)

```go
// Package metrics computes observability metrics from stored events.
package metrics

import (
    "context"
    "time"
)

// Calculator computes metrics from stored events.
type Calculator interface {
    // Dashboard metrics
    GetDashboardSummary(ctx context.Context, filter TimeFilter) (*DashboardSummary, error)

    // MCP metrics (FR-2.1, FR-2.2)
    GetMCPUtilization(ctx context.Context, filter TimeFilter) ([]MCPUtilization, error)
    GetToolSuccessRates(ctx context.Context, filter TimeFilter) ([]ToolSuccessRate, error)
    GetUnusedServers(ctx context.Context, unusedSince time.Duration) ([]UnusedServer, error)

    // Cost metrics (FR-3.1)
    GetCostForecast(ctx context.Context, lookbackDays int) (*CostForecast, error)
}

// DashboardSummary contains high-level metrics for the main dashboard.
type DashboardSummary struct {
    TotalSessions    int64
    TotalTokens      int64
    TotalCostUSD     float64
    ActiveMCPServers int
    AvgSessionLength time.Duration
    TokensByModel    map[string]int64
    CostByModel      map[string]float64
}

// MCPUtilization tracks how often an MCP server is used.
type MCPUtilization struct {
    ServerName     string
    CallCount      int64
    Percentage     float64 // Percentage of total tool calls
    TrendDirection string  // "up", "down", "stable"
}

// ToolSuccessRate tracks success/failure rates per tool.
type ToolSuccessRate struct {
    ToolName     string
    MCPServer    string
    TotalCalls   int64
    SuccessRate  float64
    ErrorRate    float64
    CommonErrors []string
}

// CostForecast provides cost projections.
type CostForecast struct {
    DailyAverage   float64
    WeeklyEstimate float64
    MonthlyEstimate float64
    Confidence     string // "low", "medium", "high"
}
```

#### 2.2.4 Web Dashboard (FR-1.3)

```go
// Package web provides the HTTP dashboard interface.
package web

import (
    "context"
    "embed"
)

//go:embed templates static
var assets embed.FS

// Server represents the web dashboard server.
type Server interface {
    // Start begins serving the dashboard on the configured port.
    Start(ctx context.Context) error

    // Stop gracefully shuts down the server.
    Stop(ctx context.Context) error
}

// ServerConfig configures the web server.
type ServerConfig struct {
    Port           int    // Default: 9877
    BindAddress    string // Default: "127.0.0.1"
    ReadTimeout    int    // Seconds, default: 30
    WriteTimeout   int    // Seconds, default: 30
    RefreshInterval int   // Seconds, default: 30
}

// Handler defines dashboard HTTP handlers.
type Handler interface {
    // Dashboard renders the main dashboard page.
    Dashboard(c echo.Context) error

    // MCPServers renders MCP server analytics.
    MCPServers(c echo.Context) error

    // Tools renders tool analytics.
    Tools(c echo.Context) error

    // Costs renders cost analytics.
    Costs(c echo.Context) error

    // Sessions renders session history.
    Sessions(c echo.Context) error

    // HTMX partial endpoints
    DashboardMetrics(c echo.Context) error
    MCPServerTable(c echo.Context) error
    CostChart(c echo.Context) error
}
```

---

## 3. Database Design

### 3.1 Schema (FR-1.1)

```sql
-- Core events table (append-only, event sourcing)
CREATE TABLE events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    tool_name TEXT,
    mcp_server TEXT,
    success INTEGER,
    duration_ms INTEGER,
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    cost_usd REAL DEFAULT 0.0,
    raw_payload BLOB,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    -- Indexes for common queries
    CONSTRAINT chk_event_type CHECK (event_type IN (
        'PreToolUse', 'PostToolUse', 'UserPromptSubmit',
        'Stop', 'SubagentStop', 'Notification',
        'SessionStart', 'SessionEnd', 'PreCompact'
    ))
);

CREATE INDEX idx_events_session ON events(session_id);
CREATE INDEX idx_events_created ON events(created_at);
CREATE INDEX idx_events_type ON events(event_type);
CREATE INDEX idx_events_mcp_server ON events(mcp_server);
CREATE INDEX idx_events_tool ON events(tool_name);

-- Sessions table (derived from events, updated on session lifecycle)
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    cwd TEXT,
    started_at DATETIME NOT NULL,
    ended_at DATETIME,
    total_events INTEGER DEFAULT 0,
    total_tokens INTEGER DEFAULT 0,
    total_cost_usd REAL DEFAULT 0.0
);

CREATE INDEX idx_sessions_started ON sessions(started_at);

-- MCP servers table (configuration tracking)
CREATE TABLE mcp_servers (
    name TEXT PRIMARY KEY,
    first_seen_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_seen_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    total_calls INTEGER DEFAULT 0,
    total_errors INTEGER DEFAULT 0
);

-- Pre-aggregated daily stats (for performance)
CREATE TABLE daily_stats (
    date DATE NOT NULL,
    mcp_server TEXT,
    tool_name TEXT,
    call_count INTEGER DEFAULT 0,
    success_count INTEGER DEFAULT 0,
    error_count INTEGER DEFAULT 0,
    total_latency_ms INTEGER DEFAULT 0,
    total_tokens INTEGER DEFAULT 0,
    total_cost_usd REAL DEFAULT 0.0,

    PRIMARY KEY (date, mcp_server, tool_name)
);

CREATE INDEX idx_daily_stats_date ON daily_stats(date);

-- Schema version for migrations
CREATE TABLE schema_version (
    version INTEGER PRIMARY KEY,
    applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### 3.2 MCP Server Identification Strategy

Since Claude Code doesn't explicitly tag which MCP server a tool belongs to, we use the following heuristics:

1. **Tool Name Prefix Pattern**: Many MCP servers use a naming convention like `mcp_filesystem_read` or `github_create_issue`. Extract the prefix before the first underscore.

2. **Configuration File Parsing**: Read `~/.config/claude/mcp.json` (if it exists) to get the list of configured MCP servers and their tool mappings.

3. **Learning Mode**: Build a mapping from tool names to MCP servers based on observed patterns.

```go
// MCPIdentifier extracts MCP server name from tool information.
type MCPIdentifier interface {
    // Identify returns the MCP server name for a given tool.
    Identify(toolName string, toolInput map[string]interface{}) string
}

// Rule-based identification
type RuleBasedIdentifier struct {
    prefixRules map[string]string // prefix -> server name
    exactRules  map[string]string // exact tool name -> server name
}
```

---

## 4. API Design

### 4.1 Hook Receiver API

**Endpoint**: `POST /hook`

Receives Claude Code hook events. No authentication required (localhost only).

```
POST http://localhost:9876/hook
Content-Type: application/json

{
  "session_id": "abc123",
  "hook_event_name": "PostToolUse",
  "tool_name": "Read",
  "tool_input": {...},
  "tool_response": {...}
}
```

**Response**: `200 OK` with empty body (fast acknowledgment)

### 4.2 Dashboard Routes

| Route | Method | Description | Template |
|-------|--------|-------------|----------|
| `/` | GET | Main dashboard | dashboard.html |
| `/mcp` | GET | MCP server analytics | mcp.html |
| `/tools` | GET | Tool analytics | tools.html |
| `/costs` | GET | Cost analytics | costs.html |
| `/sessions` | GET | Session history | sessions.html |
| `/sessions/:id` | GET | Session detail | session-detail.html |

### 4.3 HTMX Partial Endpoints

For dynamic updates without full page reload:

| Route | Method | Description | Trigger |
|-------|--------|-------------|---------|
| `/partials/metrics` | GET | Dashboard metrics cards | hx-get, 30s poll |
| `/partials/mcp-table` | GET | MCP server table | hx-get |
| `/partials/cost-chart` | GET | Cost chart SVG | hx-get |
| `/partials/recent-events` | GET | Recent events list | hx-get, 10s poll |

### 4.4 API Endpoints (Future)

For programmatic access (Phase 4):

| Route | Method | Description |
|-------|--------|-------------|
| `/api/v1/metrics` | GET | JSON metrics export |
| `/api/v1/events` | GET | Event query API |
| `/api/v1/sessions` | GET | Session list API |

---

## 5. Technology Stack

### 5.1 Core Dependencies

| Dependency | Version | Purpose | License |
|------------|---------|---------|---------|
| Go | 1.21+ | Runtime | BSD-3 |
| modernc.org/sqlite | v1.39+ | SQLite driver (pure Go) | BSD-3 |
| github.com/labstack/echo/v4 | v4.12+ | Web framework | MIT |
| html/template | stdlib | HTML templating | BSD-3 |
| embed | stdlib | Asset embedding | BSD-3 |

### 5.2 Development Dependencies

| Dependency | Purpose |
|------------|---------|
| github.com/stretchr/testify | Test assertions |
| golang.org/x/tools/cmd/goimports | Code formatting |

### 5.3 Frontend Stack

| Technology | Version | Purpose |
|------------|---------|---------|
| HTMX | 2.0+ | Dynamic updates without JS |
| CSS (custom) | - | Minimal styling |

No build step required. All assets embedded in binary.

---

## 6. Security Design

### 6.1 Network Security (NFR-4)

```
┌─────────────────────────────────────────────────────────────┐
│                    Localhost Only (default)                  │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────┐        ┌─────────────────────────────┐    │
│  │ Claude Code  │───────▶│  Hook Receiver (127.0.0.1)  │    │
│  └──────────────┘        │  Port 9876                   │    │
│                          └─────────────────────────────┘    │
│                                                              │
│  ┌──────────────┐        ┌─────────────────────────────┐    │
│  │   Browser    │◀──────▶│  Dashboard (127.0.0.1)      │    │
│  │  localhost   │        │  Port 9877                   │    │
│  └──────────────┘        └─────────────────────────────┘    │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**Security measures:**

1. **Localhost binding by default**: Both servers bind to `127.0.0.1`
2. **No authentication needed for localhost**: Simplifies setup
3. **Optional network access**: Requires explicit `--bind 0.0.0.0` flag
4. **Basic auth for network access**: Required when binding to non-localhost
5. **Input validation**: All hook payloads validated before processing
6. **SQL injection prevention**: Parameterized queries only
7. **XSS prevention**: Template auto-escaping enabled

### 6.2 Data Privacy (NFR-1)

1. **No prompt/response content stored**: Only metadata (tool names, tokens, costs)
2. **Local storage only**: No network calls by default
3. **Configurable retention**: Default 30 days, user-configurable
4. **Purge command**: `mcp-lens purge` to delete all data

---

## 7. Configuration Design

### 7.1 Configuration File

Location: `~/.config/mcp-lens/config.toml` or `$MCP_LENS_CONFIG`

```toml
# MCP Lens Configuration

[server]
hook_port = 9876
dashboard_port = 9877
bind_address = "127.0.0.1"

[storage]
database_path = "~/.mcp-lens/data.db"
retention_days = 30

[dashboard]
refresh_interval = 30  # seconds
theme = "auto"  # auto, light, dark

[cost]
# Per 1M tokens pricing (update as needed)
[cost.models]
opus = { input = 15.00, output = 75.00 }
sonnet = { input = 3.00, output = 15.00 }
haiku = { input = 0.25, output = 1.25 }

[alerts]
enabled = false
budget_daily = 0.0
budget_weekly = 0.0
budget_monthly = 0.0
webhook_url = ""
```

### 7.2 CLI Interface

```bash
# Start MCP Lens (hook receiver + dashboard)
mcp-lens serve

# Start with custom ports
mcp-lens serve --hook-port 9876 --dashboard-port 9877

# Generate Claude Code hook configuration
mcp-lens init

# View current status
mcp-lens status

# Purge all data
mcp-lens purge --confirm

# Export data
mcp-lens export --format json --output metrics.json

# Version info
mcp-lens version
```

### 7.3 Generated Hook Configuration

The `mcp-lens init` command generates:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "curl -s -X POST -H 'Content-Type: application/json' -d @- http://localhost:9876/hook"
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "curl -s -X POST -H 'Content-Type: application/json' -d @- http://localhost:9876/hook"
          }
        ]
      }
    ],
    "SessionStart": [...],
    "SessionEnd": [...],
    "Stop": [...],
    "SubagentStop": [...]
  }
}
```

---

## 8. Deployment Architecture

### 8.1 Single Binary Distribution (FR-1.5)

```
mcp-lens (single binary, ~15MB)
├── Hook receiver server
├── Web dashboard server
├── SQLite engine (embedded)
├── HTML templates (embedded)
├── Static assets (embedded)
└── CLI interface
```

### 8.2 Build Matrix

| OS | Arch | Binary Name |
|----|------|-------------|
| macOS | arm64 | mcp-lens-darwin-arm64 |
| macOS | amd64 | mcp-lens-darwin-amd64 |
| Linux | amd64 | mcp-lens-linux-amd64 |
| Linux | arm64 | mcp-lens-linux-arm64 |
| Windows | amd64 | mcp-lens-windows-amd64.exe |

### 8.3 Installation Methods

```bash
# macOS/Linux - curl
curl -fsSL https://github.com/owner/mcp-lens/releases/latest/download/install.sh | sh

# macOS - Homebrew (future)
brew install mcp-lens

# Go install
go install github.com/owner/mcp-lens/cmd/mcp-lens@latest
```

---

## 9. Requirements Traceability

| Requirement | Design Component | Section |
|-------------|-----------------|---------|
| FR-1.1 Local SQLite Storage | storage package, SQLite schema | 2.2.2, 3.1 |
| FR-1.2 Hooks Integration | hooks package, Receiver interface | 2.2.1 |
| FR-1.3 Web Dashboard | web package, HTMX templates | 2.2.4, 4.2 |
| FR-1.4 Basic Metrics | metrics package, DashboardSummary | 2.2.3 |
| FR-1.5 Single Binary | embed.FS, Go build | 8.1 |
| FR-2.1 MCP Utilization | MCPUtilization, mcp_servers table | 2.2.3, 3.1 |
| FR-2.2 Tool Success Rates | ToolSuccessRate, daily_stats | 2.2.3, 3.1 |
| FR-2.3 Latency Monitoring | MCPServerStats, P50/P90/P99 | 2.2.2 |
| FR-2.4 Unused Detection | GetUnusedServers | 2.2.3 |
| FR-2.5 Error Aggregation | ToolSuccessRate.CommonErrors | 2.2.3 |
| FR-3.1 Cost Forecasting | CostForecast | 2.2.3 |
| FR-3.2 Budget Alerts | config.toml alerts section | 7.1 |
| NFR-1 Privacy | Localhost default, no content storage | 6.2 |
| NFR-2 Performance | Indexed queries, pre-aggregation | 3.1 |
| NFR-4 Security | Localhost binding, input validation | 6.1 |

---

## 10. Open Questions and Future Considerations

### 10.1 Deferred to Phase 3+

1. **Multi-agent tracking** (FR-4.2): Requires additional schema for agent hierarchy
2. **OTEL export** (FR-4.3): Separate package for OpenTelemetry integration
3. **Team aggregation** (FR-4.4): Requires central server component

### 10.2 Technical Debt Considerations

1. **Pre-aggregation strategy**: daily_stats table may need hourly granularity
2. **MCP identification**: Heuristic approach may need refinement
3. **Template organization**: May need nested partials as UI grows

---

## 11. References

### Research Sources

- [modernc.org/sqlite Documentation](https://pkg.go.dev/modernc.org/sqlite)
- [Echo Framework Guide](https://echo.labstack.com/guide/)
- [Claude Code Hooks Reference](https://code.claude.com/docs/en/hooks)
- [HTMX Documentation](https://htmx.org/docs/)
- [Go embed Package](https://pkg.go.dev/embed)

### Related Documents

- [requirements.md](./requirements.md) - Functional and non-functional requirements
- [research.md](./research.md) - Competitive analysis and market research

---

## Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-01-10 | MCP Lens Team | Initial design based on requirements |
