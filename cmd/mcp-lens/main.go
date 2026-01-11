// MCP Lens - Lightweight observability dashboard for Claude Code with MCP server intelligence.
package main

import (
	"fmt"
	"os"

	"github.com/anthropics/mcp-lens/internal/cli"
)

var (
	// These are set at build time
	version   = "dev"
	buildDate = "unknown"
)

func main() {
	// Set version info for CLI
	cli.Version = version
	cli.BuildDate = buildDate

	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
