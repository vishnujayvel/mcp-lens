package storage

import (
	"context"
	"sync"
	"time"
)

// MockStore is an in-memory implementation of Store for testing.
type MockStore struct {
	mu       sync.RWMutex
	events   []Event
	sessions map[string]*Session
	nextID   int64
}

// NewMockStore creates a new mock store.
func NewMockStore() *MockStore {
	return &MockStore{
		events:   make([]Event, 0),
		sessions: make(map[string]*Session),
		nextID:   1,
	}
}

// StoreEvent stores an event in memory.
func (m *MockStore) StoreEvent(ctx context.Context, event *Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	event.ID = m.nextID
	m.nextID++

	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	m.events = append(m.events, *event)

	// Update or create session
	if _, exists := m.sessions[event.SessionID]; !exists {
		m.sessions[event.SessionID] = &Session{
			ID:           event.SessionID,
			StartedAt:    event.CreatedAt,
			TotalEvents:  0,
			TotalTokens:  0,
			TotalCostUSD: 0,
		}
	}
	sess := m.sessions[event.SessionID]
	sess.TotalEvents++
	sess.TotalTokens += event.InputTokens + event.OutputTokens
	sess.TotalCostUSD += event.CostUSD

	return nil
}

// GetEvents retrieves events matching the filter.
func (m *MockStore) GetEvents(ctx context.Context, filter EventFilter) ([]Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Event
	for i := len(m.events) - 1; i >= 0; i-- {
		e := m.events[i]
		if m.matchesFilter(e, filter) {
			result = append(result, e)
		}
	}

	// Apply limit
	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}

	return result, nil
}

func (m *MockStore) matchesFilter(e Event, f EventFilter) bool {
	if f.SessionID != "" && e.SessionID != f.SessionID {
		return false
	}

	if len(f.EventTypes) > 0 {
		found := false
		for _, et := range f.EventTypes {
			if e.EventType == et {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if len(f.ToolNames) > 0 {
		found := false
		for _, tn := range f.ToolNames {
			if e.ToolName == tn {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if len(f.MCPServers) > 0 {
		found := false
		for _, ms := range f.MCPServers {
			if e.MCPServer == ms {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if !f.From.IsZero() && e.CreatedAt.Before(f.From) {
		return false
	}

	if !f.To.IsZero() && e.CreatedAt.After(f.To) {
		return false
	}

	return true
}

// GetSession retrieves a session by ID.
func (m *MockStore) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if sess, ok := m.sessions[sessionID]; ok {
		return sess, nil
	}
	return nil, nil
}

// GetSessions retrieves sessions matching the filter.
func (m *MockStore) GetSessions(ctx context.Context, filter SessionFilter) ([]Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Session
	for _, sess := range m.sessions {
		if m.matchesSessionFilter(*sess, filter) {
			result = append(result, *sess)
		}
	}

	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}

	return result, nil
}

func (m *MockStore) matchesSessionFilter(s Session, f SessionFilter) bool {
	if f.Cwd != "" && s.Cwd != f.Cwd {
		return false
	}
	if !f.From.IsZero() && s.StartedAt.Before(f.From) {
		return false
	}
	if !f.To.IsZero() && s.StartedAt.After(f.To) {
		return false
	}
	return true
}

// GetMCPServerStats retrieves aggregated stats for MCP servers.
func (m *MockStore) GetMCPServerStats(ctx context.Context, filter TimeFilter) ([]MCPServerStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statsMap := make(map[string]*MCPServerStats)

	for _, e := range m.events {
		if e.MCPServer == "" {
			continue
		}

		// Apply time filter
		if !filter.From.IsZero() && e.CreatedAt.Before(filter.From) {
			continue
		}
		if !filter.To.IsZero() && e.CreatedAt.After(filter.To) {
			continue
		}

		if _, exists := statsMap[e.MCPServer]; !exists {
			statsMap[e.MCPServer] = &MCPServerStats{
				ServerName: e.MCPServer,
			}
		}

		stats := statsMap[e.MCPServer]
		stats.TotalCalls++
		if e.Success {
			stats.SuccessCount++
		} else {
			stats.ErrorCount++
		}
		stats.AvgLatencyMs = (stats.AvgLatencyMs*float64(stats.TotalCalls-1) + float64(e.DurationMs)) / float64(stats.TotalCalls)
		stats.LastUsedAt = e.CreatedAt
	}

	var result []MCPServerStats
	for _, stats := range statsMap {
		result = append(result, *stats)
	}

	return result, nil
}

// GetToolStats retrieves aggregated stats for tools.
func (m *MockStore) GetToolStats(ctx context.Context, filter TimeFilter) ([]ToolStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statsMap := make(map[string]*ToolStats)

	for _, e := range m.events {
		if e.ToolName == "" {
			continue
		}

		// Apply time filter
		if !filter.From.IsZero() && e.CreatedAt.Before(filter.From) {
			continue
		}
		if !filter.To.IsZero() && e.CreatedAt.After(filter.To) {
			continue
		}

		key := e.ToolName + ":" + e.MCPServer
		if _, exists := statsMap[key]; !exists {
			statsMap[key] = &ToolStats{
				ToolName:  e.ToolName,
				MCPServer: e.MCPServer,
			}
		}

		stats := statsMap[key]
		stats.TotalCalls++
		if e.Success {
			stats.SuccessCount++
		} else {
			stats.ErrorCount++
		}
		stats.AvgLatencyMs = (stats.AvgLatencyMs*float64(stats.TotalCalls-1) + float64(e.DurationMs)) / float64(stats.TotalCalls)
	}

	var result []ToolStats
	for _, stats := range statsMap {
		result = append(result, *stats)
	}

	return result, nil
}

// GetCostSummary retrieves aggregated cost metrics.
func (m *MockStore) GetCostSummary(ctx context.Context, filter TimeFilter) (*CostSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := &CostSummary{}

	for _, e := range m.events {
		// Apply time filter
		if !filter.From.IsZero() && e.CreatedAt.Before(filter.From) {
			continue
		}
		if !filter.To.IsZero() && e.CreatedAt.After(filter.To) {
			continue
		}

		summary.InputTokens += e.InputTokens
		summary.OutputTokens += e.OutputTokens
		summary.TotalCostUSD += e.CostUSD
	}

	summary.TotalTokens = summary.InputTokens + summary.OutputTokens
	return summary, nil
}

// GetCostByModel retrieves cost breakdown by model.
func (m *MockStore) GetCostByModel(ctx context.Context, filter TimeFilter) ([]ModelCost, error) {
	return []ModelCost{}, nil
}

// Cleanup removes events older than the specified time.
func (m *MockStore) Cleanup(ctx context.Context, olderThan time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var newEvents []Event
	var deleted int64

	for _, e := range m.events {
		if e.CreatedAt.Before(olderThan) {
			deleted++
		} else {
			newEvents = append(newEvents, e)
		}
	}

	m.events = newEvents
	return deleted, nil
}

// Close is a no-op for mock store.
func (m *MockStore) Close() error {
	return nil
}

// EventCount returns the number of stored events (for testing).
func (m *MockStore) EventCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.events)
}

// Reset clears all data (for testing).
func (m *MockStore) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = make([]Event, 0)
	m.sessions = make(map[string]*Session)
	m.nextID = 1
}
