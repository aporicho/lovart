package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
)

const managedMarker = "# Managed by lovart"

var supportedMCPClients = []string{"codex", "claude", "opencode", "openclaw"}

var (
	lookPath    = exec.LookPath
	runCommand  = runCommandDefault
	nowUTC      = func() time.Time { return time.Now().UTC() }
	userHomeDir = os.UserHomeDir
)

// ConfigOptions controls MCP client status and install operations.
type ConfigOptions struct {
	Clients    string
	LovartPath string
	Home       string
	DryRun     bool
	Yes        bool
	Force      bool
}

type configContext struct {
	lovartPath string
	home       string
	dryRun     bool
	yes        bool
	force      bool
}

type commandResult struct {
	ReturnCode int
	Stdout     string
	Stderr     string
}

// Install configures selected local MCP clients to use this lovart binary.
func Install(opts ConfigOptions) envelope.Envelope {
	if !opts.DryRun && !opts.Yes {
		return envelope.Err(errors.CodeInputError, "--yes is required for mcp install", map[string]any{
			"recommended_actions": []string{"rerun with --yes", "rerun with --dry-run to preview changes"},
		})
	}
	ctx, err := newConfigContext(opts)
	if err != nil {
		return envelope.Err(errors.CodeInputError, "mcp install setup failed", map[string]any{"error": err.Error()})
	}
	selected, err := selectMCPClients(clientSpec(opts.Clients), ctx, false)
	if err != nil {
		return envelope.Err(errors.CodeInputError, "unknown MCP client", map[string]any{
			"error":             err.Error(),
			"supported_clients": supportedMCPClients,
		})
	}
	results := make([]map[string]any, 0, len(selected))
	for _, client := range selected {
		result, err := installClient(client, ctx)
		if err != nil {
			return configErrorEnvelope(err)
		}
		results = append(results, result)
	}
	return okLocal(map[string]any{
		"lovart_path":            ctx.lovartPath,
		"dry_run":                ctx.dryRun,
		"force":                  ctx.force,
		"mcp_clients_requested":  clientSpec(opts.Clients),
		"mcp_clients_selected":   selected,
		"supported_mcp_clients":  supportedMCPClients,
		"results":                results,
		"recommended_next_steps": []string{"run `lovart mcp status`", "restart your MCP client"},
	}, true)
}

// Uninstall removes Lovart MCP config from selected local clients.
func Uninstall(opts ConfigOptions) envelope.Envelope {
	if !opts.DryRun && !opts.Yes {
		return envelope.Err(errors.CodeInputError, "--yes is required for mcp uninstall", map[string]any{
			"recommended_actions": []string{"rerun with --yes", "rerun with --dry-run to preview changes"},
		})
	}
	ctx, err := newConfigContext(opts)
	if err != nil {
		return envelope.Err(errors.CodeInputError, "mcp uninstall setup failed", map[string]any{"error": err.Error()})
	}
	selected, err := selectMCPClientsForUninstall(clientSpec(opts.Clients), ctx)
	if err != nil {
		return envelope.Err(errors.CodeInputError, "unknown MCP client", map[string]any{
			"error":             err.Error(),
			"supported_clients": supportedMCPClients,
		})
	}
	results := make([]map[string]any, 0, len(selected))
	for _, client := range selected {
		result, err := uninstallClient(client, ctx)
		if err != nil {
			return configErrorEnvelope(err)
		}
		results = append(results, result)
	}
	return okLocal(map[string]any{
		"lovart_path":            ctx.lovartPath,
		"dry_run":                ctx.dryRun,
		"force":                  ctx.force,
		"mcp_clients_requested":  clientSpec(opts.Clients),
		"mcp_clients_selected":   selected,
		"supported_mcp_clients":  supportedMCPClients,
		"results":                results,
		"recommended_next_steps": []string{"restart your MCP client"},
	}, true)
}

// ConfigStatus reports selected local MCP client configuration state.
func ConfigStatus(opts ConfigOptions) (map[string]any, error) {
	ctx, err := newConfigContext(opts)
	if err != nil {
		return nil, err
	}
	selected, err := selectMCPClients(clientSpec(opts.Clients), ctx, true)
	if err != nil {
		return nil, err
	}
	statuses := make([]map[string]any, 0, len(selected))
	for _, client := range selected {
		statuses = append(statuses, statusForClient(client, ctx))
	}
	return map[string]any{
		"lovart_path":           ctx.lovartPath,
		"mcp_clients_requested": clientSpec(opts.Clients),
		"mcp_clients_selected":  selected,
		"supported_clients":     supportedMCPClients,
		"clients":               statuses,
	}, nil
}

func newConfigContext(opts ConfigOptions) (configContext, error) {
	path, err := resolveLovartPath(opts.LovartPath)
	if err != nil {
		return configContext{}, err
	}
	home := opts.Home
	if home == "" {
		home, err = userHomeDir()
		if err != nil {
			return configContext{}, err
		}
	}
	home, err = filepath.Abs(expandHome(home))
	if err != nil {
		return configContext{}, err
	}
	return configContext{lovartPath: path, home: home, dryRun: opts.DryRun, yes: opts.Yes, force: opts.Force}, nil
}

func resolveLovartPath(path string) (string, error) {
	if path == "" {
		path = lovartPath()
	}
	abs, err := filepath.Abs(expandHome(path))
	if err != nil {
		return "", err
	}
	return abs, nil
}

func expandHome(path string) string {
	if path == "~" {
		if home, err := userHomeDir(); err == nil {
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := userHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func clientSpec(value string) string {
	if strings.TrimSpace(value) == "" {
		return "auto"
	}
	return strings.TrimSpace(strings.ToLower(value))
}

func selectMCPClients(spec string, ctx configContext, includeMissing bool) ([]string, error) {
	switch spec {
	case "none":
		return []string{}, nil
	case "all":
		return append([]string(nil), supportedMCPClients...), nil
	case "auto":
		var selected []string
		for _, client := range supportedMCPClients {
			if clientDetected(client, ctx) {
				selected = append(selected, client)
			}
		}
		if len(selected) > 0 {
			return selected, nil
		}
		if includeMissing {
			return append([]string(nil), supportedMCPClients...), nil
		}
		return []string{"codex"}, nil
	default:
		parts := strings.Split(spec, ",")
		seen := map[string]bool{}
		var selected []string
		var unknown []string
		for _, part := range parts {
			client := strings.TrimSpace(strings.ToLower(part))
			if client == "" {
				continue
			}
			if !isSupportedClient(client) {
				unknown = append(unknown, client)
				continue
			}
			if !seen[client] {
				seen[client] = true
				selected = append(selected, client)
			}
		}
		if len(unknown) > 0 {
			sort.Strings(unknown)
			return nil, fmt.Errorf("unknown clients: %s", strings.Join(unknown, ", "))
		}
		return selected, nil
	}
}

func selectMCPClientsForUninstall(spec string, ctx configContext) ([]string, error) {
	if spec != "auto" {
		return selectMCPClients(spec, ctx, false)
	}
	var selected []string
	for _, client := range supportedMCPClients {
		if clientDetected(client, ctx) {
			selected = append(selected, client)
		}
	}
	return selected, nil
}

func isSupportedClient(client string) bool {
	for _, supported := range supportedMCPClients {
		if client == supported {
			return true
		}
	}
	return false
}

func clientDetected(client string, ctx configContext) bool {
	switch client {
	case "codex":
		return pathExists(filepath.Join(ctx.home, ".codex"))
	case "claude":
		return commandAvailable("claude")
	case "opencode":
		return commandAvailable("opencode") || pathExists(filepath.Join(ctx.home, ".config", "opencode"))
	case "openclaw":
		return commandAvailable("openclaw")
	default:
		return false
	}
}

func statusForClient(client string, ctx configContext) map[string]any {
	switch client {
	case "codex":
		return codexStatus(ctx)
	case "claude":
		return commandStatus("claude", claudeCommand(ctx))
	case "opencode":
		return opencodeStatus(ctx)
	case "openclaw":
		return commandStatus("openclaw", openclawCommand(ctx))
	default:
		return map[string]any{"client": client, "status": "unknown"}
	}
}

func installClient(client string, ctx configContext) (map[string]any, error) {
	switch client {
	case "codex":
		return installCodex(ctx)
	case "claude":
		return installClaude(ctx)
	case "opencode":
		return installOpenCode(ctx)
	case "openclaw":
		return installCommandClient("openclaw", openclawCommand(ctx), ctx)
	default:
		return nil, configInputError{Message: "unknown MCP client", Details: map[string]any{"client": client}}
	}
}

func uninstallClient(client string, ctx configContext) (map[string]any, error) {
	switch client {
	case "codex":
		return uninstallCodex(ctx)
	case "claude":
		return uninstallCommandClient("claude", claudeRemoveCommand(), ctx)
	case "opencode":
		return uninstallOpenCode(ctx)
	case "openclaw":
		return map[string]any{
			"client":             "openclaw",
			"type":               "command",
			"available":          commandAvailable("openclaw"),
			"status":             "manual_required",
			"changed":            false,
			"recommended_action": "remove the Lovart MCP server from OpenClaw manually",
		}, nil
	default:
		return nil, configInputError{Message: "unknown MCP client", Details: map[string]any{"client": client}}
	}
}

func commandStatus(client string, command []string) map[string]any {
	return map[string]any{
		"client":         client,
		"type":           "command",
		"available":      commandAvailable(command[0]),
		"configured":     nil,
		"manual_command": shellJoin(command),
		"command":        command,
	}
}

func installCommandClient(client string, command []string, ctx configContext) (map[string]any, error) {
	result := commandStatus(client, command)
	result["changed"] = false
	if ctx.dryRun {
		result["status"] = "dry_run"
		return result, nil
	}
	if !commandAvailable(command[0]) {
		result["status"] = "manual_required"
		return result, nil
	}
	completed, err := runCommand(command)
	if err != nil {
		result["status"] = "failed"
		result["error"] = err.Error()
		return nil, configCommandError{Message: client + " MCP configuration failed", Details: result}
	}
	result["returncode"] = completed.ReturnCode
	result["stdout"] = tail(completed.Stdout, 2000)
	result["stderr"] = tail(completed.Stderr, 2000)
	if completed.ReturnCode != 0 {
		result["status"] = "failed"
		return nil, configCommandError{Message: client + " MCP configuration failed", Details: result}
	}
	result["status"] = "configured"
	result["changed"] = true
	return result, nil
}

func installClaude(ctx configContext) (map[string]any, error) {
	command := claudeCommand(ctx)
	result := commandStatus("claude", command)
	result["changed"] = false
	if ctx.dryRun {
		result["status"] = "dry_run"
		return result, nil
	}
	if !commandAvailable(command[0]) {
		result["status"] = "manual_required"
		return result, nil
	}

	completed, err := runCommand(command)
	recordCommandResult(result, completed)
	if err != nil {
		result["status"] = "failed"
		result["error"] = err.Error()
		return nil, configCommandError{Message: "claude MCP configuration failed", Details: result}
	}
	if completed.ReturnCode == 0 {
		result["status"] = "configured"
		result["changed"] = true
		return result, nil
	}
	if !commandAlreadyExists(completed) {
		result["status"] = "failed"
		return nil, configCommandError{Message: "claude MCP configuration failed", Details: result}
	}

	result["configured"] = true
	if !ctx.force {
		result["status"] = "already_exists"
		result["recommended_action"] = "rerun with --force to replace the existing Claude Lovart MCP server"
		return result, nil
	}

	result["initial_returncode"] = completed.ReturnCode
	result["initial_stdout"] = tail(completed.Stdout, 2000)
	result["initial_stderr"] = tail(completed.Stderr, 2000)
	removeCommand := claudeRemoveCommand()
	result["remove_command"] = removeCommand
	removed, err := runCommand(removeCommand)
	result["remove_returncode"] = removed.ReturnCode
	result["remove_stdout"] = tail(removed.Stdout, 2000)
	result["remove_stderr"] = tail(removed.Stderr, 2000)
	if err != nil {
		result["status"] = "failed"
		result["error"] = err.Error()
		return nil, configCommandError{Message: "claude MCP replacement failed", Details: result}
	}
	if removed.ReturnCode != 0 {
		result["status"] = "failed"
		return nil, configCommandError{Message: "claude MCP replacement failed", Details: result}
	}

	retried, err := runCommand(command)
	recordCommandResult(result, retried)
	if err != nil {
		result["status"] = "failed"
		result["error"] = err.Error()
		return nil, configCommandError{Message: "claude MCP configuration failed", Details: result}
	}
	if retried.ReturnCode != 0 {
		result["status"] = "failed"
		return nil, configCommandError{Message: "claude MCP configuration failed", Details: result}
	}
	result["status"] = "configured"
	result["changed"] = true
	result["replaced"] = true
	return result, nil
}

func uninstallCommandClient(client string, command []string, ctx configContext) (map[string]any, error) {
	result := commandStatus(client, command)
	result["changed"] = false
	if ctx.dryRun {
		result["status"] = "dry_run"
		return result, nil
	}
	if !commandAvailable(command[0]) {
		result["status"] = "manual_required"
		return result, nil
	}
	completed, err := runCommand(command)
	if err != nil {
		result["status"] = "failed"
		result["error"] = err.Error()
		return nil, configCommandError{Message: client + " MCP removal failed", Details: result}
	}
	result["returncode"] = completed.ReturnCode
	result["stdout"] = tail(completed.Stdout, 2000)
	result["stderr"] = tail(completed.Stderr, 2000)
	if completed.ReturnCode != 0 {
		result["status"] = "failed"
		return nil, configCommandError{Message: client + " MCP removal failed", Details: result}
	}
	result["status"] = "removed"
	result["changed"] = true
	return result, nil
}

func recordCommandResult(result map[string]any, completed commandResult) {
	result["returncode"] = completed.ReturnCode
	result["stdout"] = tail(completed.Stdout, 2000)
	result["stderr"] = tail(completed.Stderr, 2000)
}

func commandAlreadyExists(completed commandResult) bool {
	text := strings.ToLower(completed.Stdout + "\n" + completed.Stderr)
	return strings.Contains(text, "already exists")
}

func claudeCommand(ctx configContext) []string {
	return []string{"claude", "mcp", "add", "--transport", "stdio", "lovart", "--scope", "user", "--", ctx.lovartPath, "mcp"}
}

func claudeRemoveCommand() []string {
	return []string{"claude", "mcp", "remove", "--scope", "user", "lovart"}
}

func openclawCommand(ctx configContext) []string {
	payload, _ := json.Marshal(map[string]any{"command": ctx.lovartPath, "args": []string{"mcp"}})
	return []string{"openclaw", "mcp", "set", "lovart", string(payload)}
}

func commandAvailable(name string) bool {
	_, err := lookPath(name)
	return err == nil
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func runCommandDefault(command []string) (commandResult, error) {
	cmd := exec.Command(command[0], command[1:]...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := commandResult{Stdout: stdout.String(), Stderr: stderr.String()}
	if exitErr, ok := err.(*exec.ExitError); ok {
		result.ReturnCode = exitErr.ExitCode()
		return result, nil
	}
	if err != nil {
		return result, err
	}
	result.ReturnCode = 0
	return result, nil
}

func shellJoin(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "" || strings.ContainsAny(arg, " \t\n'\"\\$`") {
			quoted = append(quoted, "'"+strings.ReplaceAll(arg, "'", "'\\''")+"'")
		} else {
			quoted = append(quoted, arg)
		}
	}
	return strings.Join(quoted, " ")
}

func tail(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[len(value)-limit:]
}
