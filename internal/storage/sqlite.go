package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements Store using SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite store.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating database directory: %w", err)
	}

	// Open database with WAL mode for better concurrency
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	store := &SQLiteStore{db: db}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("initializing schema: %w", err)
	}

	return store, nil
}

func (s *SQLiteStore) initSchema() error {
	schema := `
	-- Core events table (append-only)
	CREATE TABLE IF NOT EXISTS events (
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
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_events_session ON events(session_id);
	CREATE INDEX IF NOT EXISTS idx_events_created ON events(created_at);
	CREATE INDEX IF NOT EXISTS idx_events_type ON events(event_type);
	CREATE INDEX IF NOT EXISTS idx_events_mcp_server ON events(mcp_server);
	CREATE INDEX IF NOT EXISTS idx_events_tool ON events(tool_name);

	-- Sessions table
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		cwd TEXT,
		started_at DATETIME NOT NULL,
		ended_at DATETIME,
		total_events INTEGER DEFAULT 0,
		total_tokens INTEGER DEFAULT 0,
		total_cost_usd REAL DEFAULT 0.0
	);

	CREATE INDEX IF NOT EXISTS idx_sessions_started ON sessions(started_at);

	-- MCP servers table
	CREATE TABLE IF NOT EXISTS mcp_servers (
		name TEXT PRIMARY KEY,
		first_seen_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_seen_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		total_calls INTEGER DEFAULT 0,
		total_errors INTEGER DEFAULT 0
	);

	-- Daily stats for performance
	CREATE TABLE IF NOT EXISTS daily_stats (
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

	CREATE INDEX IF NOT EXISTS idx_daily_stats_date ON daily_stats(date);

	-- Schema version
	CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	INSERT OR IGNORE INTO schema_version (version) VALUES (1);
	`

	_, err := s.db.Exec(schema)
	return err
}

// StoreEvent stores a hook event.
func (s *SQLiteStore) StoreEvent(ctx context.Context, event *Event) error {
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	successInt := 0
	if event.Success {
		successInt = 1
	}

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO events (session_id, event_type, tool_name, mcp_server, success,
			duration_ms, input_tokens, output_tokens, cost_usd, raw_payload, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.SessionID, event.EventType, event.ToolName, event.MCPServer, successInt,
		event.DurationMs, event.InputTokens, event.OutputTokens, event.CostUSD,
		event.RawPayload, event.CreatedAt)
	if err != nil {
		return fmt.Errorf("inserting event: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting last insert ID: %w", err)
	}
	event.ID = id

	// Update or create session
	if event.EventType == "SessionStart" {
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO sessions (id, cwd, started_at, total_events)
			VALUES (?, ?, ?, 1)
			ON CONFLICT(id) DO UPDATE SET total_events = total_events + 1`,
			event.SessionID, "", event.CreatedAt)
	} else {
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO sessions (id, cwd, started_at, total_events, total_tokens, total_cost_usd)
			VALUES (?, '', ?, 1, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				total_events = total_events + 1,
				total_tokens = total_tokens + excluded.total_tokens,
				total_cost_usd = total_cost_usd + excluded.total_cost_usd`,
			event.SessionID, event.CreatedAt,
			event.InputTokens+event.OutputTokens, event.CostUSD)
	}
	if err != nil {
		return fmt.Errorf("updating session: %w", err)
	}

	// Update MCP server stats if applicable
	if event.MCPServer != "" {
		errorInc := 0
		if !event.Success {
			errorInc = 1
		}
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO mcp_servers (name, first_seen_at, last_seen_at, total_calls, total_errors)
			VALUES (?, ?, ?, 1, ?)
			ON CONFLICT(name) DO UPDATE SET
				last_seen_at = excluded.last_seen_at,
				total_calls = total_calls + 1,
				total_errors = total_errors + excluded.total_errors`,
			event.MCPServer, event.CreatedAt, event.CreatedAt, errorInc)
		if err != nil {
			return fmt.Errorf("updating MCP server: %w", err)
		}
	}

	return nil
}

// GetEvents retrieves events matching the filter.
func (s *SQLiteStore) GetEvents(ctx context.Context, filter EventFilter) ([]Event, error) {
	query := `SELECT id, session_id, event_type, tool_name, mcp_server, success,
		duration_ms, input_tokens, output_tokens, cost_usd, raw_payload, created_at
		FROM events WHERE 1=1`

	var args []interface{}

	if filter.SessionID != "" {
		query += " AND session_id = ?"
		args = append(args, filter.SessionID)
	}

	if len(filter.EventTypes) > 0 {
		placeholders := make([]string, len(filter.EventTypes))
		for i, et := range filter.EventTypes {
			placeholders[i] = "?"
			args = append(args, et)
		}
		query += " AND event_type IN (" + strings.Join(placeholders, ",") + ")"
	}

	if len(filter.ToolNames) > 0 {
		placeholders := make([]string, len(filter.ToolNames))
		for i, tn := range filter.ToolNames {
			placeholders[i] = "?"
			args = append(args, tn)
		}
		query += " AND tool_name IN (" + strings.Join(placeholders, ",") + ")"
	}

	if len(filter.MCPServers) > 0 {
		placeholders := make([]string, len(filter.MCPServers))
		for i, ms := range filter.MCPServers {
			placeholders[i] = "?"
			args = append(args, ms)
		}
		query += " AND mcp_server IN (" + strings.Join(placeholders, ",") + ")"
	}

	if !filter.From.IsZero() {
		query += " AND created_at >= ?"
		args = append(args, filter.From)
	}

	if !filter.To.IsZero() {
		query += " AND created_at <= ?"
		args = append(args, filter.To)
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var success int
		var toolName, mcpServer sql.NullString
		var rawPayload []byte

		err := rows.Scan(&e.ID, &e.SessionID, &e.EventType, &toolName, &mcpServer,
			&success, &e.DurationMs, &e.InputTokens, &e.OutputTokens, &e.CostUSD,
			&rawPayload, &e.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scanning event: %w", err)
		}

		e.Success = success == 1
		e.ToolName = toolName.String
		e.MCPServer = mcpServer.String
		e.RawPayload = rawPayload
		events = append(events, e)
	}

	return events, rows.Err()
}

// GetSession retrieves a session by ID.
func (s *SQLiteStore) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, cwd, started_at, ended_at, total_events, total_tokens, total_cost_usd
		FROM sessions WHERE id = ?`, sessionID)

	var session Session
	var cwd sql.NullString
	var endedAt sql.NullTime

	err := row.Scan(&session.ID, &cwd, &session.StartedAt, &endedAt,
		&session.TotalEvents, &session.TotalTokens, &session.TotalCostUSD)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scanning session: %w", err)
	}

	session.Cwd = cwd.String
	if endedAt.Valid {
		session.EndedAt = &endedAt.Time
	}

	return &session, nil
}

// GetSessions retrieves sessions matching the filter.
func (s *SQLiteStore) GetSessions(ctx context.Context, filter SessionFilter) ([]Session, error) {
	query := `SELECT id, cwd, started_at, ended_at, total_events, total_tokens, total_cost_usd
		FROM sessions WHERE 1=1`

	var args []interface{}

	if filter.Cwd != "" {
		query += " AND cwd = ?"
		args = append(args, filter.Cwd)
	}

	if !filter.From.IsZero() {
		query += " AND started_at >= ?"
		args = append(args, filter.From)
	}

	if !filter.To.IsZero() {
		query += " AND started_at <= ?"
		args = append(args, filter.To)
	}

	query += " ORDER BY started_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var sess Session
		var cwd sql.NullString
		var endedAt sql.NullTime

		err := rows.Scan(&sess.ID, &cwd, &sess.StartedAt, &endedAt,
			&sess.TotalEvents, &sess.TotalTokens, &sess.TotalCostUSD)
		if err != nil {
			return nil, fmt.Errorf("scanning session: %w", err)
		}

		sess.Cwd = cwd.String
		if endedAt.Valid {
			sess.EndedAt = &endedAt.Time
		}
		sessions = append(sessions, sess)
	}

	return sessions, rows.Err()
}

// GetMCPServerStats retrieves aggregated stats for MCP servers.
func (s *SQLiteStore) GetMCPServerStats(ctx context.Context, filter TimeFilter) ([]MCPServerStats, error) {
	query := `
		SELECT
			mcp_server,
			COUNT(*) as total_calls,
			SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) as success_count,
			SUM(CASE WHEN success = 0 THEN 1 ELSE 0 END) as error_count,
			AVG(duration_ms) as avg_latency,
			MAX(created_at) as last_used
		FROM events
		WHERE mcp_server != '' AND mcp_server IS NOT NULL`

	var args []interface{}

	if !filter.From.IsZero() {
		query += " AND created_at >= ?"
		args = append(args, filter.From)
	}

	if !filter.To.IsZero() {
		query += " AND created_at <= ?"
		args = append(args, filter.To)
	}

	query += " GROUP BY mcp_server ORDER BY total_calls DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying MCP stats: %w", err)
	}
	defer rows.Close()

	var stats []MCPServerStats
	for rows.Next() {
		var st MCPServerStats
		var avgLatency sql.NullFloat64

		err := rows.Scan(&st.ServerName, &st.TotalCalls, &st.SuccessCount,
			&st.ErrorCount, &avgLatency, &st.LastUsedAt)
		if err != nil {
			return nil, fmt.Errorf("scanning MCP stats: %w", err)
		}

		st.AvgLatencyMs = avgLatency.Float64
		stats = append(stats, st)
	}

	return stats, rows.Err()
}

// GetToolStats retrieves aggregated stats for tools.
func (s *SQLiteStore) GetToolStats(ctx context.Context, filter TimeFilter) ([]ToolStats, error) {
	query := `
		SELECT
			tool_name,
			mcp_server,
			COUNT(*) as total_calls,
			SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) as success_count,
			SUM(CASE WHEN success = 0 THEN 1 ELSE 0 END) as error_count,
			AVG(duration_ms) as avg_latency
		FROM events
		WHERE tool_name != '' AND tool_name IS NOT NULL`

	var args []interface{}

	if !filter.From.IsZero() {
		query += " AND created_at >= ?"
		args = append(args, filter.From)
	}

	if !filter.To.IsZero() {
		query += " AND created_at <= ?"
		args = append(args, filter.To)
	}

	query += " GROUP BY tool_name, mcp_server ORDER BY total_calls DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying tool stats: %w", err)
	}
	defer rows.Close()

	var stats []ToolStats
	for rows.Next() {
		var st ToolStats
		var avgLatency sql.NullFloat64
		var mcpServer sql.NullString

		err := rows.Scan(&st.ToolName, &mcpServer, &st.TotalCalls,
			&st.SuccessCount, &st.ErrorCount, &avgLatency)
		if err != nil {
			return nil, fmt.Errorf("scanning tool stats: %w", err)
		}

		st.MCPServer = mcpServer.String
		st.AvgLatencyMs = avgLatency.Float64
		stats = append(stats, st)
	}

	return stats, rows.Err()
}

// GetCostSummary retrieves aggregated cost metrics.
func (s *SQLiteStore) GetCostSummary(ctx context.Context, filter TimeFilter) (*CostSummary, error) {
	query := `
		SELECT
			COALESCE(SUM(input_tokens), 0) as input_tokens,
			COALESCE(SUM(output_tokens), 0) as output_tokens,
			COALESCE(SUM(cost_usd), 0) as total_cost
		FROM events WHERE 1=1`

	var args []interface{}

	if !filter.From.IsZero() {
		query += " AND created_at >= ?"
		args = append(args, filter.From)
	}

	if !filter.To.IsZero() {
		query += " AND created_at <= ?"
		args = append(args, filter.To)
	}

	row := s.db.QueryRowContext(ctx, query, args...)

	var summary CostSummary
	err := row.Scan(&summary.InputTokens, &summary.OutputTokens, &summary.TotalCostUSD)
	if err != nil {
		return nil, fmt.Errorf("scanning cost summary: %w", err)
	}

	summary.TotalTokens = summary.InputTokens + summary.OutputTokens
	return &summary, nil
}

// GetCostByModel retrieves cost breakdown by model.
func (s *SQLiteStore) GetCostByModel(ctx context.Context, filter TimeFilter) ([]ModelCost, error) {
	// Note: Model information would need to be stored in events
	// For now, return empty - this would be implemented when we track model info
	return []ModelCost{}, nil
}

// Cleanup removes events older than the specified time.
func (s *SQLiteStore) Cleanup(ctx context.Context, olderThan time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx,
		"DELETE FROM events WHERE created_at < ?", olderThan)
	if err != nil {
		return 0, fmt.Errorf("deleting old events: %w", err)
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("getting rows affected: %w", err)
	}

	return deleted, nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
