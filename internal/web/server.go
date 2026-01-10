// Package web provides the HTTP dashboard interface.
package web

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/anthropics/mcp-lens/internal/metrics"
	"github.com/anthropics/mcp-lens/internal/storage"
)

//go:embed templates static
var embeddedFS embed.FS

// ServerConfig configures the web server.
type ServerConfig struct {
	Port            int
	BindAddress     string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	RefreshInterval int // seconds
}

// DefaultServerConfig returns default server configuration.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Port:            9877,
		BindAddress:     "127.0.0.1",
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		RefreshInterval: 30,
	}
}

// Server represents the web dashboard server.
type Server struct {
	config     ServerConfig
	echo       *echo.Echo
	store      storage.Store
	calculator *metrics.Calculator
	templates  *template.Template
}

// NewServer creates a new web dashboard server.
func NewServer(config ServerConfig, store storage.Store) (*Server, error) {
	if config.Port == 0 {
		config.Port = 9877
	}
	if config.BindAddress == "" {
		config.BindAddress = "127.0.0.1"
	}
	if config.RefreshInterval == 0 {
		config.RefreshInterval = 30
	}

	s := &Server{
		config:     config,
		store:      store,
		calculator: metrics.NewCalculator(store),
	}

	// Parse templates
	templates, err := s.loadTemplates()
	if err != nil {
		return nil, fmt.Errorf("loading templates: %w", err)
	}
	s.templates = templates

	// Setup Echo
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Middleware
	e.Use(middleware.Recover())
	e.Use(middleware.Logger())

	// Template renderer
	e.Renderer = &templateRenderer{templates: s.templates}

	// Routes
	s.setupRoutes(e)
	s.echo = e

	return s, nil
}

// Start begins serving the dashboard.
func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.config.BindAddress, s.config.Port)

	// Start in goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := s.echo.Start(addr); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Check for immediate startup errors
	select {
	case err := <-errCh:
		return err
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}

// Stop gracefully shuts down the server.
func (s *Server) Stop(ctx context.Context) error {
	return s.echo.Shutdown(ctx)
}

// Address returns the server's listening address.
func (s *Server) Address() string {
	return fmt.Sprintf("%s:%d", s.config.BindAddress, s.config.Port)
}

func (s *Server) loadTemplates() (*template.Template, error) {
	funcMap := template.FuncMap{
		"formatDuration": formatDuration,
		"formatCost":     formatCost,
		"formatPercent":  formatPercent,
		"formatNumber":   formatNumber,
		"formatTime":     formatTime,
		"sub":            func(a, b int) int { return a - b },
		"add":            func(a, b int) int { return a + b },
	}

	tmpl := template.New("").Funcs(funcMap)

	// Walk embedded filesystem and parse templates
	err := fs.WalkDir(embeddedFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || len(path) < 5 || path[len(path)-5:] != ".html" {
			return nil
		}

		content, err := fs.ReadFile(embeddedFS, path)
		if err != nil {
			return err
		}

		name := path[len("templates/"):]
		_, err = tmpl.New(name).Parse(string(content))
		return err
	})

	if err != nil {
		return nil, err
	}

	return tmpl, nil
}

func (s *Server) setupRoutes(e *echo.Echo) {
	// Static files
	staticFS, _ := fs.Sub(embeddedFS, "static")
	e.GET("/static/*", echo.WrapHandler(http.StripPrefix("/static/", http.FileServer(http.FS(staticFS)))))

	// Dashboard pages
	e.GET("/", s.handleDashboard)
	e.GET("/mcp", s.handleMCPServers)
	e.GET("/tools", s.handleTools)
	e.GET("/costs", s.handleCosts)
	e.GET("/sessions", s.handleSessions)
	e.GET("/sessions/:id", s.handleSessionDetail)

	// HTMX partials
	e.GET("/partials/metrics", s.handleMetricsPartial)
	e.GET("/partials/mcp-table", s.handleMCPTablePartial)
	e.GET("/partials/recent-events", s.handleRecentEventsPartial)

	// Health check
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
}

// templateRenderer implements echo.Renderer
type templateRenderer struct {
	templates *template.Template
}

func (t *templateRenderer) Render(w http.ResponseWriter, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

// Template helper functions
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}

func formatCost(cost float64) string {
	if cost < 0.01 {
		return fmt.Sprintf("$%.4f", cost)
	}
	if cost < 1 {
		return fmt.Sprintf("$%.3f", cost)
	}
	return fmt.Sprintf("$%.2f", cost)
}

func formatPercent(pct float64) string {
	return fmt.Sprintf("%.1f%%", pct)
}

func formatNumber(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(n)/1000000)
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("Jan 02 15:04")
}
