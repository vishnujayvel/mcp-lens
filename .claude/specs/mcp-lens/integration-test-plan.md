# MCP Lens Integration Test Plan

## Overview

This document outlines the integration testing strategy for MCP Lens using the **Everything MCP Server** as the test target. The Everything server is Anthropic's reference implementation designed specifically for testing MCP capabilities.

## Test Environment

### Components Under Test
1. **Hook Receiver** (`internal/hooks/receiver.go`) - HTTP endpoint receiving Claude Code hooks
2. **Event Processor** (`internal/hooks/processor.go`) - Processes and stores events
3. **SQLite Storage** (`internal/storage/sqlite.go`) - Persistent event storage
4. **Metrics Calculator** (`internal/metrics/calculator.go`) - Computes dashboard metrics
5. **Web Dashboard** (`internal/web/`) - Displays observability data

### External Dependencies
- **Everything MCP Server**: `@modelcontextprotocol/server-everything`
- **Claude Code CLI**: For generating real hook events

## Test Coverage Matrix

| Component | Unit Tests | Integration Tests | E2E Tests |
|-----------|------------|-------------------|-----------|
| Config | ✅ 7 tests | - | - |
| Storage | ✅ Written | ✅ IT-STOR-* | ✅ E2E-* |
| Hooks/Events | ✅ 5 tests | ✅ IT-HOOK-* | ✅ E2E-* |
| Hooks/Receiver | - | ✅ IT-RECV-* | ✅ E2E-* |
| Hooks/Processor | - | ✅ IT-PROC-* | ✅ E2E-* |
| Hooks/Identifier | - | ✅ IT-IDENT-* | ✅ E2E-* |
| Metrics | - | ✅ IT-METR-* | ✅ E2E-* |
| Web Dashboard | - | ✅ IT-WEB-* | ✅ E2E-* |

## Integration Test Cases

### IT-HOOK-001: Hook Receiver Accepts Valid Events
**Objective**: Verify the hook receiver correctly accepts and parses incoming hook events.

**Steps**:
1. Start MCP Lens hook receiver on port 9876
2. Send POST request with valid `PreToolUse` event JSON
3. Verify 200 response
4. Send POST request with valid `PostToolUse` event JSON
5. Verify 200 response

**Expected**: Both events accepted, no errors logged

---

### IT-HOOK-002: Hook Receiver Rejects Invalid Events
**Objective**: Verify the hook receiver properly handles malformed requests.

**Steps**:
1. Send POST with invalid JSON
2. Send POST with missing required fields
3. Send POST with unknown event type

**Expected**: 400 Bad Request for invalid, graceful handling for unknown

---

### IT-PROC-001: Processor Stores Events Correctly
**Objective**: Verify events flow from receiver to storage.

**Steps**:
1. Start receiver and processor
2. POST a PreToolUse event
3. POST a PostToolUse event
4. Query storage for events

**Expected**: Both events stored with correct fields

---

### IT-PROC-002: Processor Handles MCP Tool Identification
**Objective**: Verify MCP server name is extracted from tool calls.

**Steps**:
1. POST event with tool_name "mcp__filesystem__read_file"
2. POST event with tool_name "mcp__github__create_issue"
3. Query storage for MCP server stats

**Expected**: "filesystem" and "github" servers identified and tracked

---

### IT-STOR-001: Storage Persists Across Restarts
**Objective**: Verify SQLite persistence works correctly.

**Steps**:
1. Start MCP Lens, store events
2. Stop MCP Lens
3. Restart MCP Lens
4. Query for previously stored events

**Expected**: All events still present after restart

---

### IT-STOR-002: Storage Time Filtering Works
**Objective**: Verify time-based queries return correct results.

**Steps**:
1. Store events with different timestamps
2. Query with 1h filter
3. Query with 24h filter
4. Query with 7d filter

**Expected**: Only events within time range returned

---

### IT-METR-001: Metrics Calculator Aggregates Correctly
**Objective**: Verify metrics are calculated accurately.

**Steps**:
1. Store 10 PreToolUse + 10 PostToolUse events
2. Store 5 with tool_name containing "mcp__"
3. Calculate dashboard summary
4. Verify counts

**Expected**:
- Total events: 20
- MCP tool calls: 5
- Success rate calculated correctly

---

### IT-METR-002: MCP Server Stats Calculated
**Objective**: Verify per-MCP-server statistics.

**Steps**:
1. Store events for 3 different MCP servers
2. Include varying latencies (50ms, 100ms, 500ms)
3. Include 1 error event
4. Get MCP server stats

**Expected**:
- 3 servers listed
- Correct call counts per server
- Correct avg latency per server
- Error rate calculated

---

### IT-WEB-001: Dashboard Loads Successfully
**Objective**: Verify web dashboard serves correctly.

**Steps**:
1. Start web server on port 9877
2. GET /
3. GET /mcp
4. GET /tools
5. GET /costs

**Expected**: All pages return 200 with valid HTML

---

### IT-WEB-002: HTMX Partials Return Data
**Objective**: Verify HTMX partial endpoints work.

**Steps**:
1. Store some test events
2. GET /partials/metrics
3. GET /partials/mcp-table
4. GET /partials/recent-events

**Expected**: HTML fragments with actual data

---

### IT-IDENT-001: Built-in Tools Not Flagged as MCP
**Objective**: Verify Claude's built-in tools are correctly identified.

**Steps**:
1. Process events with tool_name: "Read", "Write", "Bash", "Glob"
2. Check MCP server identification

**Expected**: All return empty string (not MCP)

---

### IT-IDENT-002: MCP Tools Correctly Identified
**Objective**: Verify MCP tool naming patterns are recognized.

**Steps**:
1. Process "mcp__filesystem__read" → "filesystem"
2. Process "mcp__github__pr" → "github"
3. Process "mcp_notion_search" → "notion"

**Expected**: Server names correctly extracted

---

## End-to-End Test Cases

### E2E-001: Full Flow with Everything MCP Server
**Objective**: Verify complete flow from MCP call to dashboard display.

**Preconditions**:
- Everything MCP server installed and configured
- MCP Lens running (hook receiver + dashboard)
- Claude Code configured with MCP Lens hook

**Steps**:
1. Start MCP Lens: `./mcp-lens serve`
2. Configure Claude Code hook to POST to localhost:9876
3. In Claude Code, invoke Everything MCP tools:
   - `echo` tool (simple echo back)
   - `add` tool (number addition)
   - `longRunningOperation` tool (test latency tracking)
4. Open dashboard at localhost:9877
5. Verify:
   - Events appear in recent events
   - MCP server "everything" shows in MCP tab
   - Tool calls counted correctly
   - Latencies recorded

**Expected**: All MCP calls visible in dashboard with correct metadata

---

### E2E-002: Error Tracking with Deliberate Failures
**Objective**: Verify error events are captured and displayed.

**Steps**:
1. Invoke Everything MCP with invalid parameters
2. Check dashboard for error indicators
3. Verify error rate metrics update

**Expected**: Errors tracked, error rate displayed correctly

---

### E2E-003: Session Tracking
**Objective**: Verify sessions are tracked correctly.

**Steps**:
1. Start new Claude Code session
2. Make several MCP calls
3. End session
4. Check sessions tab in dashboard

**Expected**: Session listed with correct duration and event count

---

### E2E-004: Multi-MCP Server Tracking
**Objective**: Verify multiple MCP servers tracked independently.

**Steps**:
1. Configure both Everything and Fetch MCP servers
2. Make calls to both
3. Check MCP tab

**Expected**: Both servers listed separately with individual stats

---

## Test Data

### Sample Hook Event Payloads

```json
// PreToolUse Event
{
  "session_id": "test-session-001",
  "hook_event_name": "PreToolUse",
  "tool_name": "mcp__everything__echo",
  "tool_input": {
    "message": "Hello, MCP Lens!"
  }
}

// PostToolUse Event (Success)
{
  "session_id": "test-session-001",
  "hook_event_name": "PostToolUse",
  "tool_name": "mcp__everything__echo",
  "tool_input": {
    "message": "Hello, MCP Lens!"
  },
  "tool_response": {
    "content": "Echo: Hello, MCP Lens!"
  }
}

// PostToolUse Event (Error)
{
  "session_id": "test-session-001",
  "hook_event_name": "PostToolUse",
  "tool_name": "mcp__everything__add",
  "tool_input": {
    "a": "not_a_number",
    "b": 5
  },
  "tool_response": {
    "error": "Invalid input: 'a' must be a number"
  }
}
```

---

## Automation

### Running Integration Tests

```bash
# Run all integration tests
go test -v -tags=integration ./...

# Run specific test
go test -v -tags=integration -run IT_HOOK_001 ./internal/hooks/

# Run with coverage
go test -v -tags=integration -coverprofile=coverage.out ./...
```

### CI Pipeline Integration

```yaml
integration-test:
  steps:
    - name: Install Everything MCP
      run: npm install -g @modelcontextprotocol/server-everything

    - name: Start MCP Lens
      run: ./mcp-lens serve &

    - name: Run Integration Tests
      run: go test -v -tags=integration ./...
```

---

## Success Criteria

| Metric | Target |
|--------|--------|
| Integration test pass rate | 100% |
| Code coverage (integration) | > 70% |
| E2E scenarios passing | 4/4 |
| No data loss under load | Verified |
| Dashboard response time | < 200ms |

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Network issues in test env | Use localhost only |
| SQLite locking | Use WAL mode |
| Port conflicts | Configurable ports |
| Test pollution | Fresh DB per test |

