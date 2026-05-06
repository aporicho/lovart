package mcp

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/version"
)

// Status returns local MCP server metadata and a manual config snippet.
func Status() envelope.Envelope {
	path := lovartPath()
	names := ToolNames()
	return okLocal(map[string]any{
		"protocol_version": ProtocolVersion,
		"server": map[string]any{
			"name":    ServerName,
			"version": version.Version,
		},
		"tools": map[string]any{
			"count": len(names),
			"names": names,
		},
		"manual_config": map[string]any{
			"codex_toml": "[mcp_servers.lovart]\ncommand = \"" + tomlString(path) + "\"\nargs = [\"mcp\"]\n",
			"command":    path,
			"args":       []string{"mcp"},
		},
	}, true)
}

func lovartPath() string {
	if executable, err := os.Executable(); err == nil {
		if absolute, err := filepath.Abs(executable); err == nil {
			return absolute
		}
		return executable
	}
	if len(os.Args) > 0 {
		if absolute, err := filepath.Abs(os.Args[0]); err == nil {
			return absolute
		}
		return os.Args[0]
	}
	return "lovart"
}

func tomlString(value string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "\"", "\\\"")
	return replacer.Replace(value)
}
