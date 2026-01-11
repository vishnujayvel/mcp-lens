package tui

import (
	"fmt"
	"time"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"

	"github.com/anthropics/mcp-lens/internal/analytics"
	"github.com/anthropics/mcp-lens/internal/storage"
)

// DashboardData holds all data for the dashboard.
type DashboardData struct {
	MCPServers   []storage.MCPServerStats
	RecentEvents []storage.RecentEvent
	CallVolume   []storage.HourlyCallVolume
	SessionCount int
	TimeRange    string
	UpdatedAt    time.Time

	// Analytics data
	Utilization []analytics.ServerUtilization
	ErrorTotals *analytics.ErrorTotals
}

// Dashboard holds all TUI widgets.
type Dashboard struct {
	header         *widgets.Paragraph
	mcpTable       *widgets.Table
	sparkline      *widgets.Sparkline
	sparklineGroup *widgets.SparklineGroup
	eventsList     *widgets.List
	footer         *widgets.Paragraph

	timeRange   string
	data        *DashboardData
	eventsStart int // For scrolling
}

// NewDashboard creates a new dashboard.
func NewDashboard(timeRange string) *Dashboard {
	d := &Dashboard{
		timeRange:   timeRange,
		eventsStart: 0,
	}

	// Header
	d.header = widgets.NewParagraph()
	d.header.Border = true
	d.header.BorderStyle = ui.NewStyle(ui.ColorCyan)
	d.header.TitleStyle = ui.NewStyle(ui.ColorCyan, ui.ColorClear, ui.ModifierBold)

	// MCP Server table
	d.mcpTable = widgets.NewTable()
	d.mcpTable.Title = " MCP Servers "
	d.mcpTable.TextStyle = ui.NewStyle(ui.ColorWhite)
	d.mcpTable.RowSeparator = false
	d.mcpTable.BorderStyle = ui.NewStyle(ui.ColorBlue)
	d.mcpTable.TitleStyle = ui.NewStyle(ui.ColorBlue, ui.ColorClear, ui.ModifierBold)

	// Sparkline
	d.sparkline = widgets.NewSparkline()
	d.sparkline.LineColor = ui.ColorGreen
	d.sparklineGroup = widgets.NewSparklineGroup(d.sparkline)
	d.sparklineGroup.Title = " Tool Calls "
	d.sparklineGroup.BorderStyle = ui.NewStyle(ui.ColorMagenta)
	d.sparklineGroup.TitleStyle = ui.NewStyle(ui.ColorMagenta, ui.ColorClear, ui.ModifierBold)

	// Events list
	d.eventsList = widgets.NewList()
	d.eventsList.Title = " Recent Events "
	d.eventsList.TextStyle = ui.NewStyle(ui.ColorWhite)
	d.eventsList.BorderStyle = ui.NewStyle(ui.ColorYellow)
	d.eventsList.TitleStyle = ui.NewStyle(ui.ColorYellow, ui.ColorClear, ui.ModifierBold)

	// Footer
	d.footer = widgets.NewParagraph()
	d.footer.Border = false
	d.footer.Text = " [q](fg:red) quit  [r](fg:green) refresh  [1-4](fg:cyan) time range  [j/k](fg:yellow) scroll "
	d.footer.TextStyle = ui.NewStyle(ui.ColorWhite)

	return d
}

// SetTimeRange changes the time filter.
func (d *Dashboard) SetTimeRange(r string) {
	d.timeRange = r
}

// Update updates the dashboard data.
func (d *Dashboard) Update(data *DashboardData) {
	d.data = data
	d.updateHeader()
	d.updateMCPTable()
	d.updateSparkline()
	d.updateEventsList()
}

// updateHeader updates the header widget.
func (d *Dashboard) updateHeader() {
	if d.data == nil {
		return
	}

	// Calculate stats from raw data if analytics not available
	totalCalls := int64(0)
	totalErrors := int64(0)
	for _, s := range d.data.MCPServers {
		totalCalls += s.TotalCalls
		totalErrors += s.ErrorCount
	}

	errorRate := 0.0
	if totalCalls > 0 {
		errorRate = float64(totalErrors) / float64(totalCalls) * 100
	}

	// Build health summary if analytics available
	healthSummary := ""
	if d.data.ErrorTotals != nil {
		et := d.data.ErrorTotals
		if et.CriticalServers > 0 {
			healthSummary = fmt.Sprintf("  |  [⚠ %d critical](fg:red)", et.CriticalServers)
		} else if et.HighErrorServers > 0 {
			healthSummary = fmt.Sprintf("  |  [%d high errors](fg:yellow)", et.HighErrorServers)
		} else {
			healthSummary = "  |  [✓ healthy](fg:green)"
		}
	}

	d.header.Title = fmt.Sprintf(" MCP Lens - %s ", d.data.UpdatedAt.Format("15:04:05"))
	d.header.Text = fmt.Sprintf(
		" Sessions: [%d](fg:cyan)  |  Tool Calls: [%d](fg:green)  |  Errors: [%d](fg:red) (%.1f%%)  |  Time: [%s](fg:yellow)%s ",
		d.data.SessionCount,
		totalCalls,
		totalErrors,
		errorRate,
		d.timeRange,
		healthSummary,
	)
}

// updateMCPTable updates the MCP servers table.
func (d *Dashboard) updateMCPTable() {
	if d.data == nil {
		return
	}

	// Build utilization lookup map
	utilMap := make(map[string]analytics.ServerUtilization)
	for _, u := range d.data.Utilization {
		utilMap[u.ServerName] = u
	}

	rows := [][]string{
		{"Server", "Calls", "Util %", "Errors", "Avg Latency", "Health"},
	}

	for _, s := range d.data.MCPServers {
		// Get utilization data if available
		util, hasUtil := utilMap[s.ServerName]

		// Utilization percentage
		utilPct := "-"
		if hasUtil {
			utilPct = fmt.Sprintf("%.1f%%", util.UtilizationPct)
		}

		// Health status with color
		health := statusIndicator(s.ErrorCount, s.TotalCalls)
		if hasUtil {
			health = healthStatusIndicator(util.HealthStatus)
		}

		latency := formatLatency(s.AvgLatencyMs)

		rows = append(rows, []string{
			s.ServerName,
			fmt.Sprintf("%d", s.TotalCalls),
			utilPct,
			fmt.Sprintf("%d", s.ErrorCount),
			latency,
			health,
		})
	}

	if len(rows) == 1 {
		rows = append(rows, []string{"(no MCP servers)", "-", "-", "-", "-", "-"})
	}

	d.mcpTable.Rows = rows
}

// updateSparkline updates the call volume sparkline.
func (d *Dashboard) updateSparkline() {
	if d.data == nil || len(d.data.CallVolume) == 0 {
		d.sparkline.Data = []float64{0}
		return
	}

	data := make([]float64, len(d.data.CallVolume))
	for i, v := range d.data.CallVolume {
		data[i] = float64(v.TotalCalls)
	}

	d.sparkline.Data = data
}

// updateEventsList updates the recent events list.
func (d *Dashboard) updateEventsList() {
	if d.data == nil {
		return
	}

	rows := make([]string, 0, len(d.data.RecentEvents))

	for _, e := range d.data.RecentEvents {
		status := "[✓](fg:green)"
		if !e.Success {
			status = "[✗](fg:red)"
		}

		// Truncate tool name if too long
		toolName := e.ToolName
		if len(toolName) > 35 {
			toolName = toolName[:32] + "..."
		}

		timeStr := e.Timestamp.Format("15:04:05")
		latencyStr := ""
		if e.DurationMs > 0 {
			latencyStr = fmt.Sprintf("%dms", e.DurationMs)
		}

		row := fmt.Sprintf(" %s  %-12s  %-35s  %6s  %s",
			timeStr,
			e.EventType,
			toolName,
			latencyStr,
			status,
		)
		rows = append(rows, row)
	}

	if len(rows) == 0 {
		rows = []string{" (no recent events)"}
	}

	d.eventsList.Rows = rows
}

// Render draws the dashboard to the terminal.
func (d *Dashboard) Render() {
	termWidth, termHeight := ui.TerminalDimensions()

	// Layout calculations
	headerHeight := 3
	tableHeight := min(12, termHeight/3)
	sparklineHeight := 6
	footerHeight := 1

	eventsHeight := termHeight - headerHeight - tableHeight - sparklineHeight - footerHeight

	y := 0

	// Header
	d.header.SetRect(0, y, termWidth, y+headerHeight)
	y += headerHeight

	// MCP Table
	d.mcpTable.SetRect(0, y, termWidth, y+tableHeight)
	y += tableHeight

	// Sparkline
	d.sparklineGroup.SetRect(0, y, termWidth, y+sparklineHeight)
	y += sparklineHeight

	// Events list
	d.eventsList.SetRect(0, y, termWidth, y+eventsHeight)
	y += eventsHeight

	// Footer
	d.footer.SetRect(0, y, termWidth, termHeight)

	ui.Render(d.header, d.mcpTable, d.sparklineGroup, d.eventsList, d.footer)
}

// ScrollDown scrolls the events list down.
func (d *Dashboard) ScrollDown() {
	d.eventsList.ScrollDown()
	d.Render()
}

// ScrollUp scrolls the events list up.
func (d *Dashboard) ScrollUp() {
	d.eventsList.ScrollUp()
	d.Render()
}

// Helper functions

func statusIndicator(errors, total int64) string {
	if total == 0 {
		return "[●](fg:white) idle"
	}

	errorRate := float64(errors) / float64(total)

	if errorRate >= 0.1 {
		return "[●](fg:red) error"
	} else if errorRate > 0 {
		return "[●](fg:yellow) warn"
	}
	return "[●](fg:green) ok"
}

func healthStatusIndicator(status analytics.HealthStatus) string {
	switch status {
	case analytics.HealthCritical:
		return "[●](fg:red) critical"
	case analytics.HealthWarning:
		return "[●](fg:yellow) warning"
	case analytics.HealthUnused:
		return "[●](fg:white) unused"
	case analytics.HealthGood:
		return "[●](fg:green) good"
	default:
		return "[●](fg:white) unknown"
	}
}

func formatLatency(ms float64) string {
	if ms == 0 {
		return "-"
	}

	color := "green"
	if ms > 500 {
		color = "red"
	} else if ms > 100 {
		color = "yellow"
	}

	return fmt.Sprintf("[%dms](fg:%s)", int(ms), color)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
