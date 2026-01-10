# MCP Lens - Requirements Specification

**Version:** 1.0.0
**Status:** Draft
**Last Updated:** 2026-01-10
**Based On:** [research.md](./research.md)

---

## 1. Executive Summary

MCP Lens is a lightweight observability dashboard for Claude Code with a focus on MCP (Model Context Protocol) server intelligence. The project fills a clear market gap: no existing tool provides MCP server health monitoring, utilization tracking, or latency analysis.

### Key Differentiators

1. **MCP-first**: Primary focus on MCP server health, tool utilization, latency, and errors
2. **Lightweight local stack**: SQLite + embedded web UI (no Prometheus/Grafana required)
3. **Privacy-first**: All data stored locally by default with optional export
4. **Hook + OTEL hybrid**: Combine Claude Code hooks with optional OpenTelemetry export

### Target Users

- Individual developers using Claude Code with MCP servers
- Teams wanting visibility into Claude Code usage patterns
- Developers optimizing MCP server configurations

---

## 2. User Personas

### 2.1 Solo Developer (Primary)

**Name:** Alex
**Role:** Full-stack developer using Claude Code daily
**Goals:**
- Understand which MCP servers are actually being used vs. configured
- Track costs and usage to stay within budget limits
- Identify slow or error-prone MCP servers
- Quick setup with minimal dependencies

**Pain Points:**
- No visibility into MCP server utilization
- Unclear cost tracking with 5-hour billing windows
- Heavy observability stacks require too much infrastructure

### 2.2 Team Lead (Secondary)

**Name:** Jordan
**Role:** Engineering team lead managing multiple developers
**Goals:**
- Aggregate usage metrics across team members
- Identify cost outliers and optimization opportunities
- Understand tool adoption patterns

**Pain Points:**
- No team-level analytics in existing tools
- Privacy concerns with centralized monitoring

### 2.3 DevOps Engineer (Tertiary)

**Name:** Sam
**Role:** Platform engineer maintaining developer tooling
**Goals:**
- Integrate Claude Code metrics with existing observability stack
- Export data to Prometheus/Grafana when needed
- Monitor MCP server health across the organization

**Pain Points:**
- Current OTEL integration in Claude Code has bugs
- No standardized metrics for MCP servers

---

## 3. Functional Requirements

### Phase 1: Foundation

#### FR-1.1: Local SQLite Storage

**Priority:** P0 (Must Have)

**User Story:**
As a developer, I want my observability data stored locally in SQLite so that I don't need external infrastructure and my data remains private.

**Acceptance Criteria:**
- [ ] SQLite database created automatically on first run
- [ ] Database file stored in configurable location (default: `~/.mcp-lens/data.db`)
- [ ] Configurable retention period (default: 30 days)
- [ ] Automatic cleanup of data older than retention period
- [ ] Database schema supports all required metrics (sessions, events, MCP calls)
- [ ] Database can be backed up by simply copying the file

---

#### FR-1.2: Claude Code Hooks Integration

**Priority:** P0 (Must Have)

**User Story:**
As a developer, I want MCP Lens to capture all Claude Code hook events so that I have complete visibility into my Claude Code sessions.

**Acceptance Criteria:**
- [ ] Support all 8 Claude Code hook events:
  - `PreToolUse` - Before tool execution
  - `PostToolUse` - After tool execution
  - `UserPromptSubmit` - When user submits a prompt
  - `Stop` - Session/turn completion
  - `SubagentStop` - Subagent completion
  - `Notification` - User prompts/completions
  - `SessionStart` - Session lifecycle start
  - `SessionEnd` - Session lifecycle end
- [ ] Hook receiver accepts data via HTTP POST (default: `localhost:9876`)
- [ ] Hook configuration file generated for user's Claude Code settings
- [ ] Events persisted to SQLite within 100ms of receipt
- [ ] Graceful handling of malformed hook data

---

#### FR-1.3: Embedded Web Dashboard

**Priority:** P0 (Must Have)

**User Story:**
As a developer, I want a web dashboard accessible via browser so that I can visualize my Claude Code usage without installing additional software.

**Acceptance Criteria:**
- [ ] Single binary includes web server and static assets
- [ ] Dashboard accessible at `http://localhost:9877` by default
- [ ] No external JavaScript frameworks required at runtime
- [ ] Responsive design works on desktop and tablet
- [ ] Dashboard loads within 2 seconds on localhost
- [ ] Auto-refresh option with configurable interval (default: 30s)

---

#### FR-1.4: Basic Metrics Display

**Priority:** P0 (Must Have)

**User Story:**
As a developer, I want to see basic usage metrics so that I understand my Claude Code consumption patterns.

**Acceptance Criteria:**
- [ ] Token usage displayed (input tokens, output tokens, total)
- [ ] Cost tracking with per-model breakdown (Opus, Sonnet, Haiku)
- [ ] Session duration and count
- [ ] Daily/weekly/monthly aggregation views
- [ ] Time-series charts for usage over time
- [ ] Current billing window indicator (5-hour window)

---

#### FR-1.5: Single Binary Distribution

**Priority:** P0 (Must Have)

**User Story:**
As a developer, I want to download a single binary so that I can start using MCP Lens immediately without dependency management.

**Acceptance Criteria:**
- [ ] Available for macOS (arm64, amd64), Linux (arm64, amd64), Windows (amd64)
- [ ] No runtime dependencies (no Python, Node.js, etc.)
- [ ] Binary size under 20MB
- [ ] Installation via curl/wget one-liner
- [ ] Version command shows build info (`mcp-lens --version`)

---

### Phase 2: MCP Intelligence

#### FR-2.1: MCP Server Utilization Tracking

**Priority:** P0 (Must Have)

**User Story:**
As a developer, I want to see which MCP servers are being used so that I can optimize my MCP configuration.

**Acceptance Criteria:**
- [ ] List all configured MCP servers from Claude Code settings
- [ ] Track invocation count per MCP server
- [ ] Display utilization percentage relative to total tool calls
- [ ] Identify MCP servers with zero usage (unused)
- [ ] Time-series view of MCP server usage patterns
- [ ] Filter by time range (today, week, month, custom)

---

#### FR-2.2: Tool Call Frequency and Success Rates

**Priority:** P0 (Must Have)

**User Story:**
As a developer, I want to see tool call statistics so that I can identify which tools are most valuable and which are failing.

**Acceptance Criteria:**
- [ ] List all tools with call count and percentage
- [ ] Success rate per tool (successful / total calls)
- [ ] Error rate and common error types per tool
- [ ] Drill-down to see individual tool call details
- [ ] Sort by frequency, success rate, or error rate
- [ ] Highlight tools with >10% error rate

---

#### FR-2.3: MCP Server Latency Monitoring

**Priority:** P1 (Should Have)

**User Story:**
As a developer, I want to see latency metrics for MCP servers so that I can identify performance bottlenecks.

**Acceptance Criteria:**
- [ ] Track response time for each MCP tool call
- [ ] Display p50, p90, p99 latency per MCP server
- [ ] Latency histogram visualization
- [ ] Alert threshold configuration (e.g., warn if p90 > 500ms)
- [ ] Comparison view across MCP servers
- [ ] Trend analysis showing latency changes over time

---

#### FR-2.4: Unused MCP Server Detection

**Priority:** P1 (Should Have)

**User Story:**
As a developer, I want to be notified about unused MCP servers so that I can remove unnecessary configurations.

**Acceptance Criteria:**
- [ ] Automatically detect MCP servers with no calls in configurable period (default: 7 days)
- [ ] Display unused servers in dedicated dashboard section
- [ ] Optional notification when new unused server detected
- [ ] Distinguish between "never used" and "stopped being used"
- [ ] Provide configuration snippet for removing unused servers

---

#### FR-2.5: MCP Error Aggregation

**Priority:** P1 (Should Have)

**User Story:**
As a developer, I want to see aggregated MCP errors so that I can diagnose issues with my MCP servers.

**Acceptance Criteria:**
- [ ] Aggregate errors by MCP server and error type
- [ ] Display error message samples (up to 5 per type)
- [ ] Error frequency over time chart
- [ ] Link errors to specific sessions for context
- [ ] Export error logs for debugging
- [ ] Filter by severity (warning, error, fatal)

---

### Phase 3: Cost Intelligence

#### FR-3.1: Cost Forecasting

**Priority:** P1 (Should Have)

**User Story:**
As a developer, I want to see cost forecasts so that I can plan my budget and avoid unexpected charges.

**Acceptance Criteria:**
- [ ] Daily, weekly, monthly cost projections based on recent usage
- [ ] Projection confidence intervals (low, expected, high)
- [ ] Comparison to previous period
- [ ] Adjustable projection window (7, 14, 30 days lookback)
- [ ] Per-model cost projection breakdown

---

#### FR-3.2: Budget Alerts and Thresholds

**Priority:** P1 (Should Have)

**User Story:**
As a developer, I want to set budget thresholds so that I receive warnings before exceeding my planned spend.

**Acceptance Criteria:**
- [ ] Configure daily, weekly, monthly budget limits
- [ ] Warning threshold (e.g., 80% of budget)
- [ ] Critical threshold (e.g., 100% of budget)
- [ ] In-dashboard alert banner when threshold exceeded
- [ ] Optional webhook notification for external alerting
- [ ] Budget reset on configured schedule

---

#### FR-3.3: Per-Project Cost Breakdown

**Priority:** P2 (Nice to Have)

**User Story:**
As a developer working on multiple projects, I want to see costs broken down by project so that I can track ROI.

**Acceptance Criteria:**
- [ ] Automatically detect project from working directory
- [ ] Manual project tagging via configuration
- [ ] Cost attribution per project
- [ ] Project comparison view
- [ ] Export project costs to CSV

---

#### FR-3.4: Model Efficiency Comparison

**Priority:** P2 (Nice to Have)

**User Story:**
As a developer, I want to compare efficiency across models so that I can choose the most cost-effective option.

**Acceptance Criteria:**
- [ ] Cost per successful task completion by model
- [ ] Token efficiency ratio (output tokens / input tokens)
- [ ] Tool call success rate by model
- [ ] Recommendation engine for model selection

---

### Phase 4: Advanced Analytics

#### FR-4.1: Session Pattern Analysis

**Priority:** P2 (Nice to Have)

**User Story:**
As a developer, I want to understand my usage patterns so that I can optimize my workflow.

**Acceptance Criteria:**
- [ ] Peak usage hours identification
- [ ] Average session duration and token usage
- [ ] Session success rate (completed vs. abandoned)
- [ ] Common tool sequences (workflow patterns)
- [ ] Day-of-week and time-of-day heatmap

---

#### FR-4.2: Multi-Agent Session Isolation

**Priority:** P2 (Nice to Have)

**User Story:**
As a developer using multi-agent workflows, I want to see metrics isolated by agent so that I can optimize individual agents.

**Acceptance Criteria:**
- [ ] Detect and track subagent sessions
- [ ] Per-agent token usage and cost
- [ ] Agent hierarchy visualization
- [ ] Filter dashboard by agent
- [ ] Agent performance comparison

---

#### FR-4.3: OpenTelemetry Export

**Priority:** P2 (Nice to Have)

**User Story:**
As a DevOps engineer, I want to export metrics to my existing observability stack so that I can integrate with Prometheus/Grafana.

**Acceptance Criteria:**
- [ ] Optional OTLP exporter (disabled by default)
- [ ] Configure OTLP endpoint via environment variable
- [ ] Export metrics using OTel GenAI Semantic Conventions
- [ ] Support for both HTTP and gRPC OTLP
- [ ] Configurable export interval (default: 60s)
- [ ] Graceful degradation if export target unavailable

---

#### FR-4.4: Team Aggregation

**Priority:** P3 (Future)

**User Story:**
As a team lead, I want to aggregate metrics across team members so that I can understand team-wide usage patterns.

**Acceptance Criteria:**
- [ ] Optional team mode (disabled by default)
- [ ] Central aggregation server component
- [ ] Privacy-conscious data collection (no prompt/response content)
- [ ] Anonymization option for individual metrics
- [ ] Team dashboard with aggregated views
- [ ] Individual opt-in required for data sharing

---

## 4. Non-Functional Requirements

### NFR-1: Privacy

**Priority:** P0 (Must Have)

**Requirements:**
- [ ] All data stored locally by default (no network calls)
- [ ] No telemetry or analytics sent to external services
- [ ] Prompt and response content NOT stored by default
- [ ] Configurable PII scrubbing for optional exports
- [ ] Clear documentation of all data collected
- [ ] Data deletion command (`mcp-lens purge`)

---

### NFR-2: Performance

**Priority:** P0 (Must Have)

**Requirements:**
- [ ] Hook event processing: <100ms latency impact
- [ ] Dashboard page load: <2s on localhost
- [ ] Database queries: <500ms for 30-day aggregations
- [ ] Memory usage: <100MB baseline
- [ ] CPU usage: <5% when idle (no active sessions)

---

### NFR-3: Reliability

**Priority:** P0 (Must Have)

**Requirements:**
- [ ] Graceful shutdown preserves all pending data
- [ ] Automatic recovery from database corruption
- [ ] No data loss during version upgrades
- [ ] Hook receiver continues operating if dashboard crashes
- [ ] Comprehensive error logging to file

---

### NFR-4: Security

**Priority:** P0 (Must Have)

**Requirements:**
- [ ] Dashboard binds to localhost only by default
- [ ] Optional authentication for non-localhost access
- [ ] No default credentials or API keys
- [ ] Input validation on all hook data
- [ ] SQL injection prevention via parameterized queries
- [ ] XSS prevention in dashboard

---

### NFR-5: Usability

**Priority:** P1 (Should Have)

**Requirements:**
- [ ] Zero-configuration startup possible
- [ ] Auto-generated Claude Code hook configuration
- [ ] Interactive setup wizard for first run
- [ ] Helpful error messages with remediation steps
- [ ] Comprehensive `--help` documentation

---

### NFR-6: Maintainability

**Priority:** P1 (Should Have)

**Requirements:**
- [ ] Comprehensive test coverage (>80%)
- [ ] Database schema migrations for upgrades
- [ ] Semantic versioning for releases
- [ ] Changelog maintained for each release
- [ ] Contributing guide for open source contributors

---

## 5. Out of Scope

The following items are explicitly NOT included in this project:

1. **Full LLM response logging** - Due to privacy concerns and storage requirements
2. **Real-time streaming dashboard** - WebSocket complexity not justified for MVP
3. **Enterprise SSO/SAML** - Focus on individual developers first
4. **Mobile app** - Web dashboard is sufficient
5. **Cloud-hosted version** - Contradicts privacy-first approach
6. **Competing with ccusage** - CLI usage tracking is well-served
7. **Competing with claude-code-otel** - Enterprise OTEL pipelines are well-served

---

## 6. Dependencies and Constraints

### Technical Constraints

1. **Go as primary language** - Enables single-binary distribution
2. **SQLite for storage** - No external database required
3. **Embedded web assets** - No separate frontend build
4. **Claude Code hooks API** - Dependent on hook event schema stability

### External Dependencies

1. **Claude Code** - Target environment
2. **MCP Protocol** - For MCP server discovery and metadata

### Assumptions

1. Users have Claude Code installed and configured
2. Users have basic familiarity with terminal/CLI
3. MCP servers are configured via standard Claude Code settings
4. Users can allocate localhost ports (9876, 9877)

---

## 7. Success Metrics

### Adoption Metrics

- GitHub stars within 6 months of release
- Number of unique installations (opt-in telemetry)
- Community contributions (PRs, issues)

### User Satisfaction

- GitHub issue resolution time
- Feature request implementation rate
- User feedback sentiment

### Technical Health

- Test coverage percentage
- Bug report frequency
- Time to resolve critical bugs

---

## 8. Glossary

| Term | Definition |
|------|------------|
| **MCP** | Model Context Protocol - Anthropic's standard for tool/server integration |
| **OTEL** | OpenTelemetry - Open standard for observability |
| **Hook** | Claude Code lifecycle event callback |
| **Session** | A single Claude Code conversation/interaction |
| **Tool** | An MCP-provided function that Claude can call |

---

## 9. Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-01-10 | MCP Lens Team | Initial requirements based on research |

---

## 10. Appendix: User Stories Summary

| ID | User Story | Priority | Phase |
|----|------------|----------|-------|
| FR-1.1 | Local SQLite Storage | P0 | 1 |
| FR-1.2 | Claude Code Hooks Integration | P0 | 1 |
| FR-1.3 | Embedded Web Dashboard | P0 | 1 |
| FR-1.4 | Basic Metrics Display | P0 | 1 |
| FR-1.5 | Single Binary Distribution | P0 | 1 |
| FR-2.1 | MCP Server Utilization Tracking | P0 | 2 |
| FR-2.2 | Tool Call Frequency and Success Rates | P0 | 2 |
| FR-2.3 | MCP Server Latency Monitoring | P1 | 2 |
| FR-2.4 | Unused MCP Server Detection | P1 | 2 |
| FR-2.5 | MCP Error Aggregation | P1 | 2 |
| FR-3.1 | Cost Forecasting | P1 | 3 |
| FR-3.2 | Budget Alerts and Thresholds | P1 | 3 |
| FR-3.3 | Per-Project Cost Breakdown | P2 | 3 |
| FR-3.4 | Model Efficiency Comparison | P2 | 3 |
| FR-4.1 | Session Pattern Analysis | P2 | 4 |
| FR-4.2 | Multi-Agent Session Isolation | P2 | 4 |
| FR-4.3 | OpenTelemetry Export | P2 | 4 |
| FR-4.4 | Team Aggregation | P3 | 4 |
