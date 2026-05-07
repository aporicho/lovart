package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func codexConfigPath(ctx configContext) string {
	return filepath.Join(ctx.home, ".codex", "config.toml")
}

func codexBlock(ctx configContext) string {
	return strings.Join([]string{
		managedMarker,
		"[mcp_servers.lovart]",
		"command = \"" + tomlString(ctx.lovartPath) + "\"",
		"args = [\"mcp\"]",
		"",
	}, "\n")
}

func codexStatus(ctx configContext) map[string]any {
	path := codexConfigPath(ctx)
	text := readText(path)
	return map[string]any{
		"client":     "codex",
		"type":       "file",
		"path":       path,
		"exists":     pathExists(path),
		"configured": strings.Contains(text, "[mcp_servers.lovart]") && strings.Contains(text, ctx.lovartPath) && strings.Contains(text, "args = [\"mcp\"]"),
		"managed":    strings.Contains(text, managedMarker) && strings.Contains(text, "[mcp_servers.lovart]"),
	}
}

func installCodex(ctx configContext) (map[string]any, error) {
	path := codexConfigPath(ctx)
	text := readText(path)
	hasBlock := strings.Contains(text, "[mcp_servers.lovart]")
	managed := strings.Contains(text, managedMarker) && hasBlock
	if hasBlock && !managed && !ctx.force {
		return nil, configConflictError{Message: "existing unmanaged Codex Lovart MCP config", Details: map[string]any{
			"client":              "codex",
			"path":                path,
			"recommended_actions": []string{"rerun with --force", "edit the config manually"},
		}}
	}
	block := codexBlock(ctx)
	next := replaceTOMLLovartBlock(text, block)
	return writeConfigResult("codex", path, next, ctx, map[string]any{"toml": block})
}

func uninstallCodex(ctx configContext) (map[string]any, error) {
	path := codexConfigPath(ctx)
	text := readText(path)
	next, changed, err := removeTOMLLovartBlock(text, ctx.force)
	if err != nil {
		return nil, err
	}
	return writeConfigRemovalResult("codex", path, next, changed, ctx, map[string]any{"remove": "[mcp_servers.lovart]"})
}

func replaceTOMLLovartBlock(text string, block string) string {
	lines := strings.Split(text, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "[mcp_servers.lovart]" {
			start = i
			break
		}
	}
	if start == -1 {
		prefix := strings.TrimRight(text, "\n")
		if prefix == "" {
			return block
		}
		return prefix + "\n\n" + block
	}
	if start > 0 && strings.TrimSpace(lines[start-1]) == managedMarker {
		start--
	}
	end := start + 1
	for end < len(lines) {
		stripped := strings.TrimSpace(lines[end])
		if strings.HasPrefix(stripped, "[") && strings.HasSuffix(stripped, "]") && stripped != "[mcp_servers.lovart]" {
			break
		}
		end++
	}
	newLines := append([]string{}, lines[:start]...)
	newLines = append(newLines, strings.Split(strings.TrimRight(block, "\n"), "\n")...)
	newLines = append(newLines, lines[end:]...)
	return strings.TrimRight(strings.Join(newLines, "\n"), "\n") + "\n"
}

func removeTOMLLovartBlock(text string, force bool) (string, bool, error) {
	lines := strings.Split(text, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "[mcp_servers.lovart]" {
			start = i
			break
		}
	}
	if start == -1 {
		return text, false, nil
	}
	managed := start > 0 && strings.TrimSpace(lines[start-1]) == managedMarker
	if !managed && !force {
		return "", false, configConflictError{Message: "existing unmanaged Codex Lovart MCP config", Details: map[string]any{
			"client":              "codex",
			"recommended_actions": []string{"rerun with --force", "edit the config manually"},
		}}
	}
	if managed {
		start--
	}
	end := start + 1
	for end < len(lines) {
		stripped := strings.TrimSpace(lines[end])
		if strings.HasPrefix(stripped, "[") && strings.HasSuffix(stripped, "]") && stripped != "[mcp_servers.lovart]" {
			break
		}
		end++
	}
	newLines := append([]string{}, lines[:start]...)
	newLines = append(newLines, lines[end:]...)
	next := strings.TrimRight(strings.Join(newLines, "\n"), "\n")
	if next != "" {
		next += "\n"
	}
	return next, true, nil
}

func opencodeConfigPath(ctx configContext) string {
	return filepath.Join(ctx.home, ".config", "opencode", "opencode.json")
}

func opencodeStatus(ctx configContext) map[string]any {
	path := opencodeConfigPath(ctx)
	data := readJSON(path)
	config := mapValue(mapValue(data, "mcp"), "lovart")
	command, _ := config["command"].([]any)
	configured := len(command) == 2 && command[0] == ctx.lovartPath && command[1] == "mcp" && config["enabled"] == true
	return map[string]any{
		"client":     "opencode",
		"type":       "file",
		"path":       path,
		"exists":     pathExists(path),
		"configured": configured,
		"managed":    config["managed_by"] == "lovart",
	}
}

func installOpenCode(ctx configContext) (map[string]any, error) {
	path := opencodeConfigPath(ctx)
	data := readJSON(path)
	mcp := mapValue(data, "mcp")
	existing, hasExisting := mcp["lovart"].(map[string]any)
	if hasExisting && existing["managed_by"] != "lovart" && !ctx.force {
		return nil, configConflictError{Message: "existing unmanaged OpenCode Lovart MCP config", Details: map[string]any{
			"client":              "opencode",
			"path":                path,
			"recommended_actions": []string{"rerun with --force", "edit the config manually"},
		}}
	}
	config := map[string]any{
		"type":       "local",
		"command":    []string{ctx.lovartPath, "mcp"},
		"enabled":    true,
		"managed_by": "lovart",
	}
	mcp["lovart"] = config
	data["mcp"] = mcp
	text, _ := json.MarshalIndent(data, "", "  ")
	return writeConfigResult("opencode", path, string(text)+"\n", ctx, map[string]any{"json": config})
}

func uninstallOpenCode(ctx configContext) (map[string]any, error) {
	path := opencodeConfigPath(ctx)
	data := readJSON(path)
	mcp := mapValue(data, "mcp")
	existing, hasExisting := mcp["lovart"].(map[string]any)
	if !hasExisting {
		return writeConfigRemovalResult("opencode", path, "", false, ctx, map[string]any{"remove": "mcp.lovart"})
	}
	if existing["managed_by"] != "lovart" && !ctx.force {
		return nil, configConflictError{Message: "existing unmanaged OpenCode Lovart MCP config", Details: map[string]any{
			"client":              "opencode",
			"path":                path,
			"recommended_actions": []string{"rerun with --force", "edit the config manually"},
		}}
	}
	delete(mcp, "lovart")
	data["mcp"] = mcp
	text, _ := json.MarshalIndent(data, "", "  ")
	return writeConfigRemovalResult("opencode", path, string(text)+"\n", true, ctx, map[string]any{"remove": "mcp.lovart"})
}

func writeConfigResult(client string, path string, text string, ctx configContext, preview map[string]any) (map[string]any, error) {
	backup := backupPath(path)
	result := map[string]any{
		"client":  client,
		"type":    "file",
		"path":    path,
		"backup":  backup,
		"changed": false,
		"dry_run": ctx.dryRun,
		"preview": preview,
	}
	if ctx.dryRun {
		result["status"] = "dry_run"
		return result, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, configInputError{Message: "create config directory failed", Details: map[string]any{"path": path, "error": err.Error()}}
	}
	hadExisting := pathExists(path)
	if hadExisting {
		if err := os.WriteFile(backup, []byte(readText(path)), 0644); err != nil {
			return nil, configInputError{Message: "write config backup failed", Details: map[string]any{"path": backup, "error": err.Error()}}
		}
	}
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		return nil, configInputError{Message: "write config failed", Details: map[string]any{"path": path, "error": err.Error()}}
	}
	result["status"] = "configured"
	result["changed"] = true
	result["backup_created"] = hadExisting
	return result, nil
}

func writeConfigRemovalResult(client string, path string, text string, changed bool, ctx configContext, preview map[string]any) (map[string]any, error) {
	backup := backupPath(path)
	result := map[string]any{
		"client":  client,
		"type":    "file",
		"path":    path,
		"backup":  backup,
		"changed": false,
		"dry_run": ctx.dryRun,
		"preview": preview,
	}
	if !changed {
		result["status"] = "not_configured"
		return result, nil
	}
	if ctx.dryRun {
		result["status"] = "dry_run"
		return result, nil
	}
	hadExisting := pathExists(path)
	if hadExisting {
		if err := os.WriteFile(backup, []byte(readText(path)), 0644); err != nil {
			return nil, configInputError{Message: "write config backup failed", Details: map[string]any{"path": backup, "error": err.Error()}}
		}
	}
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		return nil, configInputError{Message: "write config failed", Details: map[string]any{"path": path, "error": err.Error()}}
	}
	result["status"] = "removed"
	result["changed"] = true
	result["backup_created"] = hadExisting
	return result, nil
}

func backupPath(path string) string {
	stamp := nowUTC().Format("20060102T150405Z")
	return filepath.Join(filepath.Dir(path), filepath.Base(path)+"."+stamp+".bak")
}

func readText(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func readJSON(path string) map[string]any {
	data := map[string]any{}
	raw, err := os.ReadFile(path)
	if err != nil {
		return data
	}
	_ = json.Unmarshal(raw, &data)
	return data
}

func mapValue(data map[string]any, key string) map[string]any {
	if value, ok := data[key].(map[string]any); ok {
		return value
	}
	value := map[string]any{}
	data[key] = value
	return value
}

func tomlString(value string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "\"", "\\\"")
	return replacer.Replace(value)
}
