// MCP Lens - Lightweight observability dashboard for Claude Code with MCP server intelligence.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anthropics/mcp-lens/internal/config"
	"github.com/anthropics/mcp-lens/internal/hooks"
	"github.com/anthropics/mcp-lens/internal/storage"
	"github.com/anthropics/mcp-lens/internal/web"
)

var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		runServe(os.Args[2:])
	case "init":
		runInit(os.Args[2:])
	case "status":
		runStatus(os.Args[2:])
	case "purge":
		runPurge(os.Args[2:])
	case "version":
		runVersion()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`MCP Lens - Claude Code Observability Dashboard

Usage:
  mcp-lens <command> [options]

Commands:
  serve     Start the MCP Lens server (hook receiver + dashboard)
  init      Generate Claude Code hook configuration
  status    Show server status
  purge     Delete all stored data
  version   Show version information
  help      Show this help message

Examples:
  mcp-lens serve                    # Start with default settings
  mcp-lens serve --hook-port 9876   # Start with custom hook port
  mcp-lens init                     # Generate hook config for Claude Code
  mcp-lens purge --confirm          # Delete all data

For more information, visit: https://github.com/anthropics/mcp-lens`)
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	hookPort := fs.Int("hook-port", 0, "Hook receiver port (default: 9876)")
	dashboardPort := fs.Int("dashboard-port", 0, "Dashboard port (default: 9877)")
	bind := fs.String("bind", "", "Bind address (default: 127.0.0.1)")
	configPath := fs.String("config", "", "Path to config file")
	fs.Parse(args)

	// Load configuration
	var cfg *config.Config
	var err error

	if *configPath != "" {
		cfg, err = config.LoadFromFile(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}
	} else {
		cfg, err = config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}
	}

	// Apply command-line overrides
	if *hookPort != 0 {
		cfg.Server.HookPort = *hookPort
	}
	if *dashboardPort != 0 {
		cfg.Server.DashboardPort = *dashboardPort
	}
	if *bind != "" {
		cfg.Server.BindAddress = *bind
	}

	// Ensure data directory exists
	if err := cfg.EnsureDataDir(); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating data directory: %v\n", err)
		os.Exit(1)
	}

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Initialize storage
	store, err := storage.NewSQLiteStore(cfg.Storage.DatabasePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	// Initialize hook receiver
	receiver := hooks.NewReceiver(hooks.ReceiverConfig{
		Port:        cfg.Server.HookPort,
		BindAddress: cfg.Server.BindAddress,
	})

	// Initialize event processor
	processor := hooks.NewProcessor(store, receiver.Events())

	// Initialize web dashboard
	webServer, err := web.NewServer(web.ServerConfig{
		Port:            cfg.Server.DashboardPort,
		BindAddress:     cfg.Server.BindAddress,
		RefreshInterval: cfg.Dashboard.RefreshInterval,
	}, store)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating web server: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("MCP Lens starting...\n")
	fmt.Printf("  Hook receiver: http://%s\n", cfg.HookAddress())
	fmt.Printf("  Dashboard:     http://%s\n", cfg.DashboardAddress())
	fmt.Printf("  Database:      %s\n", cfg.Storage.DatabasePath)
	fmt.Println()

	// Start hook receiver
	if err := receiver.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting hook receiver: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Hook receiver started")

	// Start event processor
	processor.Start(ctx)
	fmt.Println("Event processor started")

	// Start web dashboard
	if err := webServer.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting web server: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Dashboard started")

	fmt.Println()
	fmt.Println("Press Ctrl+C to stop")

	// Wait for shutdown signal
	select {
	case <-sigCh:
		fmt.Println("\nShutting down...")
		cancel()
	case <-ctx.Done():
	}

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	processor.Stop()
	receiver.Stop(shutdownCtx)
	webServer.Stop(shutdownCtx)

	fmt.Println("MCP Lens stopped")
}

func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	output := fs.String("output", "", "Output file (default: stdout)")
	hookPort := fs.Int("hook-port", 9876, "Hook receiver port")
	fs.Parse(args)

	hookConfig := generateHookConfig(*hookPort)

	if *output != "" {
		err := os.WriteFile(*output, []byte(hookConfig), 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Hook configuration written to: %s\n", *output)
	} else {
		fmt.Println(hookConfig)
	}
}

func generateHookConfig(hookPort int) string {
	return fmt.Sprintf(`{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "curl -s -X POST -H 'Content-Type: application/json' -d @- http://localhost:%d/hook"
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "curl -s -X POST -H 'Content-Type: application/json' -d @- http://localhost:%d/hook"
          }
        ]
      }
    ],
    "SessionStart": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "curl -s -X POST -H 'Content-Type: application/json' -d @- http://localhost:%d/hook"
          }
        ]
      }
    ],
    "SessionEnd": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "curl -s -X POST -H 'Content-Type: application/json' -d @- http://localhost:%d/hook"
          }
        ]
      }
    ],
    "Stop": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "curl -s -X POST -H 'Content-Type: application/json' -d @- http://localhost:%d/hook"
          }
        ]
      }
    ],
    "SubagentStop": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "curl -s -X POST -H 'Content-Type: application/json' -d @- http://localhost:%d/hook"
          }
        ]
      }
    ]
  }
}`, hookPort, hookPort, hookPort, hookPort, hookPort, hookPort)
}

func runStatus(args []string) {
	// TODO: Implement status check
	fmt.Println("MCP Lens Status")
	fmt.Println("---------------")
	fmt.Println("Server: not running")
	fmt.Println()
	fmt.Println("To start the server, run: mcp-lens serve")
}

func runPurge(args []string) {
	fs := flag.NewFlagSet("purge", flag.ExitOnError)
	confirm := fs.Bool("confirm", false, "Confirm data deletion")
	fs.Parse(args)

	if !*confirm {
		fmt.Println("This will delete all MCP Lens data.")
		fmt.Println("To confirm, run: mcp-lens purge --confirm")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Remove database file
	if err := os.Remove(cfg.Storage.DatabasePath); err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error removing database: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("All MCP Lens data has been deleted.")
}

func runVersion() {
	fmt.Printf("MCP Lens %s\n", version)
	fmt.Printf("  Commit:     %s\n", commit)
	fmt.Printf("  Build date: %s\n", buildDate)
}
