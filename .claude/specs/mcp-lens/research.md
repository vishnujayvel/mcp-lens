# Claude Code Observability & Telemetry Research

**Research Date:** January 9, 2026
**Purpose:** Inform whether to build a new open-source repository for Claude Code observability

---

## 1. Competitive Landscape

### Existing Claude Code Observability Projects

| Project | Stars | URL | Key Features | Gaps/Limitations |
|---------|-------|-----|--------------|------------------|
| **ccusage** | 9.7k | [ryoppippi/ccusage](https://github.com/ryoppippi/ccusage) | CLI usage analysis from JSONL files, 5-hour billing windows, per-model cost breakdown, MCP server integration, companion tools for Codex/OpenCode | CLI-only (no web dashboard), no real-time monitoring, no team/org analytics |
| **Claude-Code-Usage-Monitor** | 6.1k | [Maciek-roboblog/Claude-Code-Usage-Monitor](https://github.com/Maciek-roboblog/Claude-Code-Usage-Monitor) | Real-time terminal UI, ML-based predictions, burn rate tracking, P90 auto-detection | Terminal-only, no web interface, no team analytics, no MCP insights |
| **claude-code-hooks-multi-agent-observability** | 892 | [disler/claude-code-hooks-multi-agent-observability](https://github.com/disler/claude-code-hooks-multi-agent-observability) | Real-time WebSocket streaming, multi-agent session tracking, Vue 3 dashboard, 9 event types, chat transcript storage | Complex setup (Python + Bun + Vue), SQLite only, no cost analytics |
| **claude-code-otel** | 221 | [ColeMurray/claude-code-otel](https://github.com/ColeMurray/claude-code-otel) | Full OTEL stack (Prometheus/Loki/Grafana), cost analysis, DAU/WAU/MAU, tool success rates, LOC metrics | Heavy infrastructure (requires Prometheus + Loki + Grafana), no retention policies documented |
| **claude_telemetry** | 8 | [TechNickAI/claude_telemetry](https://github.com/TechNickAI/claude_telemetry) | Drop-in CLI wrapper (`claudia`), multi-backend (Logfire/Sentry/Honeycomb/Datadog), async hooks | Requires Python, depends on external backends, no local visualization |
| **claude-code-monitor** | 4 | [zcquant/claude-code-monitor](https://github.com/zcquant/claude-code-monitor) | OTLP telemetry, web dashboard, Prometheus export, daily/weekly reports | JSON file storage (not scalable), single commit (immature), no auth |

### Related Hook Libraries

| Project | Stars | URL | Purpose |
|---------|-------|-----|---------|
| **claude-code-hooks-mastery** | - | [disler/claude-code-hooks-mastery](https://github.com/disler/claude-code-hooks-mastery) | Demo capturing all 8 hook lifecycle events with JSON payloads |
| **claude-hooks** | - | [johnlindquist/claude-hooks](https://github.com/johnlindquist/claude-hooks) | TypeScript-based hooks with full type safety |
| **define-claude-code-hooks** | - | [timoconnellaus/define-claude-code-hooks](https://github.com/timoconnellaus/define-claude-code-hooks) | Predefined hooks for common logging scenarios |

### MCP Monitoring Tools

| Project | Stars | URL | Relevance |
|---------|-------|-----|-----------|
| **MCP Hub** | 410 | [ravitemer/mcp-hub](https://github.com/ravitemer/mcp-hub) | Centralized MCP server coordinator with SSE health monitoring |
| **AgTrace** | New | [HN Show](https://news.ycombinator.com/item?id=46499885) | Rust-based log normalizer for Claude/Codex/Gemini, MCP integration |

### Competitor AI Coding Assistant Analytics

| Tool | Approach | Key Features |
|------|----------|--------------|
| **Cursor Analytics** | Built-in (Teams) | DAU/WAU/MAU, acceptance rates, admin CSV export, Analytics API |
| **GitHub Copilot Metrics Dashboard** | Open-source + Official | [microsoft/copilot-metrics-dashboard](https://github.com/microsoft/copilot-metrics-dashboard) - 164 stars, Azure-based, team filtering, acceptance rates |
| **Aider** | Opt-in anonymous | Built-in analytics (opt-in), minimal public tooling |
| **Continue.dev** | Local-first | PostHog telemetry (PII-stripped), local dev_data storage, privacy-focused |
| **AgentNotch** | macOS menu bar | Real-time tool tracking, token/cost monitoring for Claude Code + Codex |

---

## 2. Best Practices from Similar Tools

### Privacy-First Patterns (VS Code Extension Guidelines)

1. **Opt-in telemetry**: Always respect `isTelemetryEnabled` flag
2. **Data minimization**: Collect as little as possible
3. **Transparency**: Provide "Show Telemetry" command to see data being sent
4. **Local logging**: Write telemetry to local files (`telemetry.log`) for user inspection
5. **Data classification**: Categorize data (PublicNonPersonalData, EndUserPseudonymizedInformation)
6. **GDPR compliance**: Clear opt-out mechanisms, valid retention policies

### OpenTelemetry AI Agent Best Practices (2025)

1. **Use semantic conventions**: OTel GenAI Semantic Conventions v1.37+ define standard attributes
2. **Three primary signals**: Traces (spans), Metrics (counters/gauges), Events (logs)
3. **Nested spans**: Track tool calls within larger agent execution context
4. **Minimal overhead**: Well-implemented OTEL adds <3-5% latency
5. **Sampling for high-throughput**: Reduce overhead while maintaining insights
6. **Standardization**: Avoid vendor lock-in with framework-specific formats

### Continue.dev Privacy Pattern

- Local-first: Dev data saved to `.continue/dev_data` by default
- Air-gapped support: Works 100% offline with local LLMs
- PostHog for cloud telemetry: PII-stripped before transmission
- User control: Easy toggle in settings

---

## 3. Community Signals

### GitHub Issues (anthropics/claude-code)

| Issue | Type | Signal |
|-------|------|--------|
| [#1712](https://github.com/anthropics/claude-code/issues/1712) | Bug | OTEL data not exporting - users trying to set up monitoring |
| [#5508](https://github.com/anthropics/claude-code/issues/5508) | Bug | OTEL outputs PII on Windows, can't be disabled |
| [#2090](https://github.com/anthropics/claude-code/issues/2090) | Feature | Request to log full LLM response (only prompts currently logged) |
| [#11057](https://github.com/anthropics/claude-code/issues/11057) | Feature | Request for telemetry notification on startup |
| [#3913](https://github.com/anthropics/claude-code/issues/3913) | Feature | Request for telemetry debugging in Docker containers |
| [#7151](https://github.com/anthropics/claude-code/issues/7151) | Discussion | Concern about Statsig (now OpenAI-owned) as telemetry dependency |

### Hacker News Sentiment

**Positive signals:**
- [HN discussion](https://news.ycombinator.com/item?id=45325410) on Claude Code observability received engagement
- Users emphasize data-driven optimization over "ad-hoc prompt tweaking"
- Interest in using observability for automated agent platforms

**Concerns raised:**
- Surveillance concerns: Compared to "screenshot programs" for remote monitoring
- Workplace dynamics: Fear of "AI-usage KPI BS" for employee harassment
- Consensus: Better suited for agent orchestration than human monitoring

### Reddit (r/ClaudeAI) Pain Points

- **No visibility into actual limits**: Users frustrated by unclear usage tracking
- **Weekly limits added without notice**: Compounding restrictions create confusion
- **5-hour reset window**: Community-built tools like ccusage specifically address this

### Community Tools Signal

The 9.7k stars on ccusage and 6.1k stars on Claude-Code-Usage-Monitor demonstrate strong demand for usage visibility that official tools don't provide.

---

## 4. Technical Approaches

### Storage Patterns

| Approach | Used By | Pros | Cons |
|----------|---------|------|------|
| **Local JSONL files** | ccusage | No setup, privacy-first, portable | No real-time, limited querying |
| **SQLite** | hooks-multi-agent | Simple, local, good for moderate scale | Single-machine only |
| **JSON files** | claude-code-monitor | Simple | Not scalable |
| **Prometheus + Loki** | claude-code-otel | Industry standard, scalable | Heavy infrastructure |
| **External backends** | claude_telemetry | Leverage existing tools | Vendor dependency |

### Visualization Patterns

| Pattern | Used By | Suitability |
|---------|---------|-------------|
| **Terminal UI (Rich/Textual)** | Claude-Code-Usage-Monitor | Developers in terminal, real-time |
| **CLI tables** | ccusage | Quick checks, scriptable |
| **Vue 3 + WebSocket** | hooks-multi-agent | Real-time web dashboard |
| **Grafana dashboards** | claude-code-otel | Team/enterprise use |
| **macOS menu bar** | AgentNotch | Background monitoring |

### Data Collection Methods

1. **Claude Code Hooks** (8 events):
   - `PreToolUse`, `PostToolUse` - Tool-level instrumentation
   - `UserPromptSubmit` - Prompt capture
   - `Stop`, `SubagentStop` - Session/turn boundaries
   - `Notification` - User prompts/completions
   - `SessionStart`, `SessionEnd` - Session lifecycle
   - `PreCompact` - Context compaction events

2. **Native OTEL Export**:
   - Environment variables: `CLAUDE_CODE_ENABLE_TELEMETRY=1`, `OTEL_METRICS_EXPORTER=otlp`
   - Metrics: Token usage, costs, latency
   - Limitation: No full LLM response logging (only prompts via `OTEL_LOG_USER_PROMPTS`)

3. **JSONL File Parsing** (ccusage approach):
   - Reads local Claude Code logs
   - No configuration required
   - Works offline

---

## 5. Feature Ideas from Community

### Most Requested (Based on Issues & Discussions)

1. **Full LLM span logging** - Capture both prompt and response ([#2090](https://github.com/anthropics/claude-code/issues/2090))
2. **Telemetry startup notification** - Show where data goes ([#11057](https://github.com/anthropics/claude-code/issues/11057))
3. **Docker telemetry debugging** - Better error messages ([#3913](https://github.com/anthropics/claude-code/issues/3913))
4. **MCP server utilization** - Which MCP servers are actually used/unused
5. **Cost prediction/budgeting** - Set spending limits, get warnings
6. **Team/project aggregation** - Compare usage across projects
7. **Privacy-first defaults** - Local storage with optional cloud sync

### Gaps Not Yet Addressed

1. **MCP Server Analytics**: No tool tracks which MCP servers are used, their latency, or error rates
2. **Cross-session patterns**: Most tools are session-focused, not longitudinal
3. **Hook-to-OTEL bridge**: No unified solution combining hooks + native OTEL
4. **Unified dashboard**: Solutions are either CLI-only or require heavy infra
5. **Cost forecasting**: Predict monthly spend based on current patterns
6. **Agent-specific metrics**: Multi-agent scenarios lack proper isolation tracking

---

## 6. Opportunity Assessment

### Market Gap Analysis

| Dimension | Current State | Gap |
|-----------|--------------|-----|
| **CLI tools** | Strong (ccusage 9.7k stars) | Mature, not a gap |
| **Terminal UI** | Good (Claude-Code-Usage-Monitor 6.1k) | Mature, not a gap |
| **Web dashboard (simple)** | Weak (hooks-multi-agent complex) | **Opportunity** |
| **MCP monitoring** | None | **Strong opportunity** |
| **Local + lightweight** | Mixed | **Opportunity** for unified solution |
| **Enterprise OTEL** | Covered (claude-code-otel) | Not a gap |
| **Multi-agent tracking** | Emerging (hooks-multi-agent) | Partially addressed |

### Clear Opportunities

1. **MCP Server Observability**: Zero existing tools track MCP server health, utilization, latency
2. **Lightweight Local Dashboard**: Web UI without Prometheus/Grafana stack
3. **Hook + OTEL Unified**: Combine rich hook data with OTEL export
4. **Cost Intelligence**: Forecasting, budgeting, anomaly detection

### Crowded Spaces (Avoid)

1. Pure CLI usage tracking (ccusage dominates)
2. Terminal-based monitoring (Claude-Code-Usage-Monitor strong)
3. Enterprise OTEL pipelines (claude-code-otel, Datadog, Grafana Cloud)

---

## 7. Recommended Features for New Project

### If Building: "Claude Code Local Observatory" (Working Title)

**Core Differentiators:**
1. **MCP-first**: Track MCP server health, tool utilization, latency, errors
2. **Lightweight local stack**: SQLite + embedded web UI (no external deps)
3. **Hook + OTEL hybrid**: Combine hook richness with OTEL compatibility
4. **Privacy-first**: All data local by default, optional cloud export

**Recommended Feature Set:**

#### Phase 1: Foundation
- [ ] Local SQLite storage with configurable retention
- [ ] Claude Code hooks integration (all 8 events)
- [ ] Embedded web dashboard (single binary, no Node/Python deps)
- [ ] Basic metrics: tokens, costs, session duration

#### Phase 2: MCP Intelligence
- [ ] MCP server utilization tracking
- [ ] Tool call frequency and success rates
- [ ] Latency histograms per MCP server
- [ ] Unused MCP server detection
- [ ] MCP error aggregation

#### Phase 3: Cost Intelligence
- [ ] Cost forecasting (daily/weekly/monthly projections)
- [ ] Budget alerts and thresholds
- [ ] Per-project cost breakdown
- [ ] Model efficiency comparison

#### Phase 4: Advanced Analytics
- [ ] Session pattern analysis
- [ ] Multi-agent session isolation
- [ ] Export to OTEL backends (optional)
- [ ] Team aggregation (optional, privacy-conscious)

**Technical Stack Recommendation:**
- **Language**: Go or Rust (single binary, no runtime deps)
- **Storage**: SQLite (embedded, portable)
- **Web UI**: HTMX or embedded SPA (no separate build step)
- **Hook delivery**: Unix socket or HTTP POST
- **Configuration**: TOML/YAML in `.claude/` directory

**Name Ideas:**
- `claude-scope`
- `cc-insights`
- `claude-observatory`
- `mcp-lens`

---

## 8. Conclusion

### Is There a Clear Gap?

**Yes**, with caveats:

1. **MCP observability is completely unaddressed** - No existing tool tracks MCP server health or utilization
2. **Web dashboards are either too complex (Grafana stack) or too simple** - Opportunity for middle ground
3. **Hook + OTEL integration is fragmented** - No unified solution

### Recommendation

**Build a new project focused on:**
1. MCP server observability (unique differentiator)
2. Lightweight local web dashboard (SQLite + embedded UI)
3. Privacy-first design (local by default)

**Avoid competing with:**
- ccusage for CLI usage tracking
- claude-code-otel for enterprise OTEL pipelines

### Market Size Indicator

Combined stars on Claude Code observability tools: **16,800+**
- ccusage: 9,700
- Claude-Code-Usage-Monitor: 6,100
- hooks-multi-agent: 892
- Others: ~100

This indicates strong developer interest in Claude Code analytics, with room for differentiated solutions.

---

## Sources

### GitHub Repositories
- [ColeMurray/claude-code-otel](https://github.com/ColeMurray/claude-code-otel)
- [TechNickAI/claude_telemetry](https://github.com/TechNickAI/claude_telemetry)
- [zcquant/claude-code-monitor](https://github.com/zcquant/claude-code-monitor)
- [ryoppippi/ccusage](https://github.com/ryoppippi/ccusage)
- [Maciek-roboblog/Claude-Code-Usage-Monitor](https://github.com/Maciek-roboblog/Claude-Code-Usage-Monitor)
- [disler/claude-code-hooks-multi-agent-observability](https://github.com/disler/claude-code-hooks-multi-agent-observability)
- [disler/claude-code-hooks-mastery](https://github.com/disler/claude-code-hooks-mastery)
- [johnlindquist/claude-hooks](https://github.com/johnlindquist/claude-hooks)
- [timoconnellaus/define-claude-code-hooks](https://github.com/timoconnellaus/define-claude-code-hooks)
- [ravitemer/mcp-hub](https://github.com/ravitemer/mcp-hub)
- [microsoft/copilot-metrics-dashboard](https://github.com/microsoft/copilot-metrics-dashboard)
- [anthropics/claude-code-monitoring-guide](https://github.com/anthropics/claude-code-monitoring-guide)

### Documentation & Guides
- [Claude Code Monitoring Docs](https://code.claude.com/docs/en/monitoring-usage)
- [Claude Code Hooks Reference](https://code.claude.com/docs/en/hooks)
- [VS Code Telemetry Guide](https://code.visualstudio.com/api/extension-guides/telemetry)
- [Continue.dev Telemetry](https://docs.continue.dev/telemetry)
- [OpenTelemetry AI Agent Observability](https://opentelemetry.io/blog/2025/ai-agent-observability/)

### Community Discussions
- [HN: Bringing Observability to Claude Code](https://news.ycombinator.com/item?id=45325410)
- [HN: AgTrace - Observability for AI Coding Agents](https://news.ycombinator.com/item?id=46499885)
- [Claude Code Issue #2090 - Log full LLM span](https://github.com/anthropics/claude-code/issues/2090)
- [Claude Code Issue #11057 - Telemetry notification](https://github.com/anthropics/claude-code/issues/11057)

### Blog Posts & Articles
- [Claude Code Observability Stack: Visualize Token Spend with Grafana](https://murraycole.com/posts/claude-code-observability)
- [SigNoz: Claude Code Monitoring with OpenTelemetry](https://signoz.io/blog/claude-code-monitoring-with-opentelemetry/)
- [Cursor Analytics for Engineering Teams](https://workweave.dev/blog/cursor-analytics-tracking-ai-coding-tool-usage-for-engineering-teams)
- [Tribe AI: Measuring ROI of Claude Code](https://www.tribe.ai/applied-ai/a-quickstart-for-measuring-the-return-on-your-claude-code-investment)

### Official Resources
- [Cursor Analytics Docs](https://cursor.com/docs/account/teams/analytics)
- [GitHub Copilot Usage Metrics](https://docs.github.com/en/copilot/concepts/copilot-metrics)
- [Langfuse LLM Metrics](https://langfuse.com/docs/metrics/overview)
