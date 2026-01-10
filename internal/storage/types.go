// Package storage provides persistence for MCP Lens data.
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
	ID           int64
	SessionID    string
	EventType    string
	ToolName     string
	MCPServer    string
	Success      bool
	DurationMs   int64
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
	RawPayload   []byte
	CreatedAt    time.Time
}

// Session represents a Claude Code session.
type Session struct {
	ID           string
	Cwd          string
	StartedAt    time.Time
	EndedAt      *time.Time
	TotalEvents  int
	TotalTokens  int64
	TotalCostUSD float64
}

// MCPServerStats holds aggregated metrics for an MCP server.
type MCPServerStats struct {
	ServerName   string
	TotalCalls   int64
	SuccessCount int64
	ErrorCount   int64
	AvgLatencyMs float64
	P50LatencyMs float64
	P90LatencyMs float64
	P99LatencyMs float64
	LastUsedAt   time.Time
}

// ToolStats holds aggregated metrics for a tool.
type ToolStats struct {
	ToolName     string
	MCPServer    string
	TotalCalls   int64
	SuccessCount int64
	ErrorCount   int64
	AvgLatencyMs float64
}

// CostSummary holds aggregated cost metrics.
type CostSummary struct {
	TotalTokens  int64
	InputTokens  int64
	OutputTokens int64
	TotalCostUSD float64
}

// ModelCost holds cost breakdown by model.
type ModelCost struct {
	Model        string
	InputTokens  int64
	OutputTokens int64
	TotalCostUSD float64
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

// SessionFilter specifies criteria for session queries.
type SessionFilter struct {
	TimeFilter
	Cwd    string
	Limit  int
	Offset int
}
