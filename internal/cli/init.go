package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Generate Claude Code hook config",
		Long:  `Display the hook configuration to add to Claude Code settings.`,
		RunE:  runInit,
	}
}

func runInit(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	eventsFile := expandPath(cfg.Storage.EventsFile)
	dataDir := expandPath(cfg.Storage.DataDir)
	dbPath := expandPath(cfg.Storage.DatabasePath)

	fmt.Println("Add to ~/.claude/settings.json:")
	fmt.Println()
	fmt.Println(`{
  "hooks": {
    "PostToolUse": [{
      "matcher": "*",
      "hooks": [{
        "type": "command",
        "command": "mkdir -p ~/.mcp-lens && jq -c '{ts: now | todate, sid: .session_id, type: .hook_event_name, tool: .tool_name, ok: (if .tool_response.is_error then false else true end)}' >> ~/.mcp-lens/events.jsonl"
      }]
    }],
    "SessionStart": [{
      "matcher": "",
      "hooks": [{
        "type": "command",
        "command": "mkdir -p ~/.mcp-lens && jq -c '{ts: now | todate, sid: .session_id, type: .hook_event_name, cwd: .cwd}' >> ~/.mcp-lens/events.jsonl"
      }]
    }],
    "Stop": [{
      "matcher": "",
      "hooks": [{
        "type": "command",
        "command": "mkdir -p ~/.mcp-lens && jq -c '{ts: now | todate, sid: .session_id, type: .hook_event_name}' >> ~/.mcp-lens/events.jsonl"
      }]
    }]
  }
}`)
	fmt.Println()
	fmt.Println("Alternative: Store full payloads (simpler, larger files):")
	fmt.Println()
	fmt.Println(`{
  "hooks": {
    "PostToolUse": [{
      "matcher": "*",
      "hooks": [{
        "type": "command",
        "command": "mkdir -p ~/.mcp-lens && cat >> ~/.mcp-lens/events.jsonl"
      }]
    }]
  }
}`)
	fmt.Println()
	fmt.Printf("Data directory: %s\n", dataDir)
	fmt.Printf("Events file:    %s\n", eventsFile)
	fmt.Printf("Database:       %s\n", dbPath)
	fmt.Println()

	return nil
}
