package hooks

import (
	"strings"
)

// MCPIdentifier extracts MCP server name from tool information.
type MCPIdentifier interface {
	// Identify returns the MCP server name for a given tool.
	Identify(toolName string, toolInput map[string]interface{}) string
}

// RuleBasedIdentifier implements MCPIdentifier using pattern matching rules.
type RuleBasedIdentifier struct {
	prefixRules map[string]string // prefix -> server name
	exactRules  map[string]string // exact tool name -> server name
}

// NewRuleBasedIdentifier creates a new rule-based identifier with default rules.
func NewRuleBasedIdentifier() *RuleBasedIdentifier {
	return &RuleBasedIdentifier{
		prefixRules: map[string]string{
			"mcp_filesystem": "filesystem",
			"mcp_github":     "github",
			"mcp_slack":      "slack",
			"mcp_postgres":   "postgres",
			"mcp_redis":      "redis",
			"mcp_docker":     "docker",
			"mcp_kubernetes": "kubernetes",
			"mcp_aws":        "aws",
			"mcp_gcp":        "gcp",
			"mcp_azure":      "azure",
			"filesystem_":    "filesystem",
			"github_":        "github",
			"slack_":         "slack",
			"postgres_":      "postgres",
			"docker_":        "docker",
			"git_":           "git",
			"npm_":           "npm",
			"brave_":         "brave-search",
			"puppeteer_":     "puppeteer",
			"sqlite_":        "sqlite",
			"memory_":        "memory",
			"fetch_":         "fetch",
		},
		exactRules: map[string]string{
			// Built-in Claude Code tools (not MCP)
			"Read":          "",
			"Write":         "",
			"Edit":          "",
			"Bash":          "",
			"Glob":          "",
			"Grep":          "",
			"LS":            "",
			"MultiEdit":     "",
			"NotebookEdit":  "",
			"WebFetch":      "",
			"WebSearch":     "",
			"TodoRead":      "",
			"TodoWrite":     "",
			"Task":          "",
			"Skill":         "",
			"KillShell":     "",
			"TaskOutput":    "",
			// Common MCP tools
			"create_issue":       "github",
			"list_issues":        "github",
			"create_pull":        "github",
			"search_repos":       "github",
			"brave_search":       "brave-search",
			"brave_local_search": "brave-search",
			"navigate":           "puppeteer",
			"screenshot":         "puppeteer",
			"click":              "puppeteer",
			"read_file":          "filesystem",
			"write_file":         "filesystem",
			"list_directory":     "filesystem",
			"search_files":       "filesystem",
			"read_query":         "postgres",
			"write_query":        "postgres",
		},
	}
}

// Identify returns the MCP server name for a given tool.
func (r *RuleBasedIdentifier) Identify(toolName string, toolInput map[string]interface{}) string {
	// Check exact match first
	if server, ok := r.exactRules[toolName]; ok {
		return server
	}

	// Check for double-underscore MCP naming convention: mcp__servername__toolname
	if strings.HasPrefix(toolName, "mcp__") {
		parts := strings.SplitN(toolName, "__", 3)
		if len(parts) >= 2 {
			return parts[1] // Return the server name
		}
	}

	// Check for single-underscore MCP naming convention: mcp_servername_toolname
	if strings.HasPrefix(toolName, "mcp_") {
		// Remove "mcp_" prefix and split by underscore
		rest := toolName[4:]
		parts := strings.SplitN(rest, "_", 2)
		if len(parts) >= 1 {
			return parts[0] // Return the server name
		}
	}

	// Check prefix rules
	lowerName := strings.ToLower(toolName)
	for prefix, server := range r.prefixRules {
		if strings.HasPrefix(lowerName, strings.ToLower(prefix)) {
			return server
		}
	}

	// Try to extract from underscore-separated name
	// e.g., "my_custom_server_do_thing" -> "my_custom_server"
	parts := strings.Split(toolName, "_")
	if len(parts) >= 2 && !strings.HasPrefix(toolName, "mcp") {
		// Take all but the last part as the server name (only if not mcp prefix)
		return strings.Join(parts[:len(parts)-1], "_")
	}

	// Unknown - could be a custom tool or built-in
	return ""
}

// AddPrefixRule adds a prefix matching rule.
func (r *RuleBasedIdentifier) AddPrefixRule(prefix, serverName string) {
	r.prefixRules[prefix] = serverName
}

// AddExactRule adds an exact match rule.
func (r *RuleBasedIdentifier) AddExactRule(toolName, serverName string) {
	r.exactRules[toolName] = serverName
}

// IsBuiltInTool returns true if the tool is a built-in Claude Code tool.
func IsBuiltInTool(toolName string) bool {
	builtInTools := map[string]bool{
		"Read":         true,
		"Write":        true,
		"Edit":         true,
		"Bash":         true,
		"Glob":         true,
		"Grep":         true,
		"LS":           true,
		"MultiEdit":    true,
		"NotebookEdit": true,
		"WebFetch":     true,
		"WebSearch":    true,
		"TodoRead":     true,
		"TodoWrite":    true,
		"Task":         true,
		"Skill":        true,
		"KillShell":    true,
		"TaskOutput":   true,
	}
	return builtInTools[toolName]
}
