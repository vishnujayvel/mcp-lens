package web

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/anthropics/mcp-lens/internal/storage"
)

// Page data structures
type dashboardData struct {
	Title           string
	RefreshInterval int
	Summary         interface{}
	MCPUtilization  interface{}
	RecentEvents    []storage.Event
}

type mcpData struct {
	Title           string
	RefreshInterval int
	Servers         interface{}
	TotalCalls      int64
}

type toolsData struct {
	Title           string
	RefreshInterval int
	Tools           interface{}
}

type costsData struct {
	Title           string
	RefreshInterval int
	Summary         interface{}
	Forecast        interface{}
	ByModel         []storage.ModelCost
}

type sessionsData struct {
	Title           string
	RefreshInterval int
	Sessions        []storage.Session
	Page            int
	HasMore         bool
}

type sessionDetailData struct {
	Title   string
	Session *storage.Session
	Events  []storage.Event
}

// Dashboard handler
func (s *Server) handleDashboard(c echo.Context) error {
	ctx := c.Request().Context()

	// Get time filter from query params
	filter := s.parseTimeFilter(c)

	summary, err := s.calculator.GetDashboardSummary(ctx, filter)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	mcpUtil, err := s.calculator.GetMCPUtilization(ctx, filter)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.Render(http.StatusOK, "dashboard.html", dashboardData{
		Title:           "MCP Lens Dashboard",
		RefreshInterval: s.config.RefreshInterval,
		Summary:         summary,
		MCPUtilization:  mcpUtil,
		RecentEvents:    summary.RecentEvents,
	})
}

// MCP servers handler
func (s *Server) handleMCPServers(c echo.Context) error {
	ctx := c.Request().Context()
	filter := s.parseTimeFilter(c)

	servers, err := s.calculator.GetMCPUtilization(ctx, filter)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	var totalCalls int64
	for _, srv := range servers {
		totalCalls += srv.CallCount
	}

	return c.Render(http.StatusOK, "mcp.html", mcpData{
		Title:           "MCP Servers",
		RefreshInterval: s.config.RefreshInterval,
		Servers:         servers,
		TotalCalls:      totalCalls,
	})
}

// Tools handler
func (s *Server) handleTools(c echo.Context) error {
	ctx := c.Request().Context()
	filter := s.parseTimeFilter(c)

	tools, err := s.calculator.GetToolSuccessRates(ctx, filter)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.Render(http.StatusOK, "tools.html", toolsData{
		Title:           "Tool Analytics",
		RefreshInterval: s.config.RefreshInterval,
		Tools:           tools,
	})
}

// Costs handler
func (s *Server) handleCosts(c echo.Context) error {
	ctx := c.Request().Context()
	filter := s.parseTimeFilter(c)

	summary, err := s.store.GetCostSummary(ctx, filter)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	forecast, err := s.calculator.GetCostForecast(ctx, 14)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	byModel, err := s.store.GetCostByModel(ctx, filter)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.Render(http.StatusOK, "costs.html", costsData{
		Title:           "Cost Analytics",
		RefreshInterval: s.config.RefreshInterval,
		Summary:         summary,
		Forecast:        forecast,
		ByModel:         byModel,
	})
}

// Sessions handler
func (s *Server) handleSessions(c echo.Context) error {
	ctx := c.Request().Context()
	filter := s.parseTimeFilter(c)

	sessions, err := s.store.GetSessions(ctx, storage.SessionFilter{
		TimeFilter: filter,
		Limit:      50,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.Render(http.StatusOK, "sessions.html", sessionsData{
		Title:           "Sessions",
		RefreshInterval: s.config.RefreshInterval,
		Sessions:        sessions,
		Page:            1,
		HasMore:         len(sessions) == 50,
	})
}

// Session detail handler
func (s *Server) handleSessionDetail(c echo.Context) error {
	ctx := c.Request().Context()
	sessionID := c.Param("id")

	session, err := s.store.GetSession(ctx, sessionID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if session == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Session not found")
	}

	events, err := s.store.GetEvents(ctx, storage.EventFilter{
		SessionID: sessionID,
		Limit:     100,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.Render(http.StatusOK, "session-detail.html", sessionDetailData{
		Title:   "Session " + sessionID[:8],
		Session: session,
		Events:  events,
	})
}

// HTMX partial handlers
func (s *Server) handleMetricsPartial(c echo.Context) error {
	ctx := c.Request().Context()
	filter := s.parseTimeFilter(c)

	summary, err := s.calculator.GetDashboardSummary(ctx, filter)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.Render(http.StatusOK, "partials/metrics.html", summary)
}

func (s *Server) handleMCPTablePartial(c echo.Context) error {
	ctx := c.Request().Context()
	filter := s.parseTimeFilter(c)

	servers, err := s.calculator.GetMCPUtilization(ctx, filter)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.Render(http.StatusOK, "partials/mcp-table.html", servers)
}

func (s *Server) handleRecentEventsPartial(c echo.Context) error {
	ctx := c.Request().Context()

	events, err := s.store.GetEvents(ctx, storage.EventFilter{
		Limit: 10,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.Render(http.StatusOK, "partials/recent-events.html", events)
}

// Helper to parse time filter from query params
func (s *Server) parseTimeFilter(c echo.Context) storage.TimeFilter {
	filter := storage.TimeFilter{}

	// Default to last 24 hours
	filter.From = time.Now().Add(-24 * time.Hour)

	// Override with query params if provided
	if from := c.QueryParam("from"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			filter.From = t
		}
	}

	if to := c.QueryParam("to"); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			filter.To = t
		}
	}

	// Handle preset ranges
	switch c.QueryParam("range") {
	case "1h":
		filter.From = time.Now().Add(-1 * time.Hour)
	case "6h":
		filter.From = time.Now().Add(-6 * time.Hour)
	case "24h":
		filter.From = time.Now().Add(-24 * time.Hour)
	case "7d":
		filter.From = time.Now().AddDate(0, 0, -7)
	case "30d":
		filter.From = time.Now().AddDate(0, 0, -30)
	}

	return filter
}
