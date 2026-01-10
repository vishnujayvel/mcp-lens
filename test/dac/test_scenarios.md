# Director-Actor-Critic (DAC) Meta-Testing Framework

## Overview

This framework implements a Director-Actor-Critic pattern for meta-testing MCP Lens:

- **Director** (this document): Defines test scenarios and success criteria
- **Actor** (subagents): Execute scenarios and report results
- **Critic** (analysis): Evaluate actor outputs against criteria

## Test Scenarios

### Scenario 1: Hook Receiver Stress Test
**Actor Task**: Send 100 rapid hook events and verify all are processed

**Inputs**:
- 100 PreToolUse + PostToolUse event pairs
- Mix of MCP and built-in tools
- Varying payloads sizes

**Success Criteria**:
- All events accepted (200 OK)
- No events lost
- Processing time < 100ms per event

---

### Scenario 2: MCP Identification Edge Cases
**Actor Task**: Test tool name patterns that may confuse the identifier

**Inputs**:
```
mcp__server__tool          → "server"
mcp_server_tool            → "server"
mcp__server_with_underscore__tool → "server_with_underscore"
Read                       → "" (built-in)
mcp__                      → "" (invalid)
_mcp_server_tool           → NOT "mcp" (no prefix)
```

**Success Criteria**:
- Correct extraction for all patterns
- No crashes on edge cases
- Built-in tools not flagged as MCP

---

### Scenario 3: Dashboard Data Accuracy
**Actor Task**: Insert known data, verify dashboard reflects it correctly

**Inputs**:
- 5 sessions with known event counts
- 3 MCP servers with known call counts
- Known success/error ratios

**Success Criteria**:
- Dashboard metrics match inserted data
- MCP table shows correct stats
- Recent events show in correct order

---

### Scenario 4: Time Filtering Boundary
**Actor Task**: Test time filter edge cases

**Inputs**:
- Events at exactly T-1h, T-1h-1ms, T-1h+1ms
- Events at timezone boundaries
- Empty time ranges

**Success Criteria**:
- Correct inclusion/exclusion at boundaries
- Empty result for future time ranges
- All events included for open-ended queries

---

### Scenario 5: Error Handling Robustness
**Actor Task**: Send malformed and edge-case inputs

**Inputs**:
- Invalid JSON
- Missing required fields
- Extremely large payloads (1MB+)
- Unicode edge cases
- Null/empty values

**Success Criteria**:
- Graceful error responses (400/500)
- No server crashes
- Error messages are informative
- Server remains functional after errors

---

### Scenario 6: Concurrent Request Handling
**Actor Task**: Send parallel requests to test thread safety

**Inputs**:
- 10 concurrent hook events
- Parallel dashboard requests
- Simultaneous read/write operations

**Success Criteria**:
- No race conditions
- No data corruption
- All requests complete successfully

---

## Actor Execution Template

Each actor should:
1. Read the scenario requirements
2. Execute the test steps
3. Capture all outputs
4. Report results in structured format

## Critic Evaluation Rubric

| Rating | Criteria |
|--------|----------|
| ✅ PASS | All success criteria met |
| ⚠️ WARN | Minor issues, non-blocking |
| 🔴 FAIL | Critical success criteria not met |

## Meta-Testing Goals

1. **Coverage**: Ensure tests cover edge cases not in main test suite
2. **Robustness**: Verify system handles unexpected inputs gracefully
3. **Accuracy**: Confirm metrics and data are computed correctly
4. **Performance**: Validate response times under load
