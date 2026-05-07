package mcp

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aporicho/lovart/internal/envelope"
)

func TestConfigStatusListsSupportedClients(t *testing.T) {
	home := t.TempDir()
	env := Status(ConfigOptions{Clients: "all", LovartPath: "/tmp/lovart", Home: home})
	if !env.OK {
		t.Fatalf("unexpected status envelope: %#v", env)
	}
	data := env.Data.(map[string]any)
	config := data["configuration"].(map[string]any)
	clients := config["clients"].([]map[string]any)
	if len(clients) != 4 {
		t.Fatalf("expected 4 client statuses, got %d", len(clients))
	}
	if data["protocol_version"] != ProtocolVersion {
		t.Fatalf("missing protocol version: %#v", data)
	}
}

func TestInstallCodexDryRunDoesNotWrite(t *testing.T) {
	home := t.TempDir()
	env := Install(ConfigOptions{Clients: "codex", LovartPath: "/tmp/lovart", Home: home, DryRun: true, Yes: true})
	if !env.OK {
		t.Fatalf("unexpected install envelope: %#v", env)
	}
	results := installResults(t, env)
	if results[0]["status"] != "dry_run" {
		t.Fatalf("unexpected result: %#v", results[0])
	}
	preview := results[0]["preview"].(map[string]any)
	if !strings.Contains(preview["toml"].(string), "[mcp_servers.lovart]") {
		t.Fatalf("missing toml preview: %#v", preview)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "config.toml")); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote config, stat err=%v", err)
	}
}

func TestInstallCodexWritesAndBacksUp(t *testing.T) {
	restoreClock(t)
	home := t.TempDir()
	path := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[other]\nvalue = true\n"), 0644); err != nil {
		t.Fatal(err)
	}
	env := Install(ConfigOptions{Clients: "codex", LovartPath: "/tmp/lovart", Home: home, Yes: true})
	if !env.OK {
		t.Fatalf("unexpected install envelope: %#v", env)
	}
	text := readText(path)
	if !strings.Contains(text, managedMarker) || !strings.Contains(text, "command = \"/tmp/lovart\"") {
		t.Fatalf("config not written correctly:\n%s", text)
	}
	backup := path + ".20260102T030405Z.bak"
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("backup missing: %v", err)
	}
}

func TestInstallCodexConflictAndForce(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[mcp_servers.lovart]\ncommand = \"/other/lovart\"\nargs = [\"mcp\"]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	env := Install(ConfigOptions{Clients: "codex", LovartPath: "/tmp/lovart", Home: home, Yes: true})
	if env.OK || env.Error == nil || env.Error.Code != "config_conflict" {
		t.Fatalf("expected config conflict, got %#v", env)
	}
	env = Install(ConfigOptions{Clients: "codex", LovartPath: "/tmp/lovart", Home: home, Yes: true, Force: true})
	if !env.OK {
		t.Fatalf("force install failed: %#v", env)
	}
	if !strings.Contains(readText(path), "command = \"/tmp/lovart\"") {
		t.Fatalf("force did not replace config:\n%s", readText(path))
	}
}

func TestInstallOpenCodeDryRunPreview(t *testing.T) {
	home := t.TempDir()
	env := Install(ConfigOptions{Clients: "opencode", LovartPath: "/tmp/lovart", Home: home, DryRun: true, Yes: true})
	if !env.OK {
		t.Fatalf("unexpected install envelope: %#v", env)
	}
	results := installResults(t, env)
	preview := results[0]["preview"].(map[string]any)
	jsonPreview := preview["json"].(map[string]any)
	command := jsonPreview["command"].([]string)
	if len(command) != 2 || command[0] != "/tmp/lovart" || command[1] != "mcp" {
		t.Fatalf("unexpected opencode command: %#v", command)
	}
}

func TestUninstallCodexRemovesManagedBlock(t *testing.T) {
	restoreClock(t)
	home := t.TempDir()
	path := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	text := "[other]\nvalue = true\n\n" + managedMarker + "\n[mcp_servers.lovart]\ncommand = \"/tmp/lovart\"\nargs = [\"mcp\"]\n\n[next]\nvalue = false\n"
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	env := Uninstall(ConfigOptions{Clients: "codex", LovartPath: "/tmp/lovart", Home: home, Yes: true})
	if !env.OK {
		t.Fatalf("unexpected uninstall envelope: %#v", env)
	}
	got := readText(path)
	if strings.Contains(got, "[mcp_servers.lovart]") || strings.Contains(got, managedMarker) {
		t.Fatalf("managed block still present:\n%s", got)
	}
	if !strings.Contains(got, "[other]") || !strings.Contains(got, "[next]") {
		t.Fatalf("unrelated config was removed:\n%s", got)
	}
	if _, err := os.Stat(path + ".20260102T030405Z.bak"); err != nil {
		t.Fatalf("backup missing: %v", err)
	}
}

func TestUninstallOpenCodeConflictAndForce(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"mcp":{"lovart":{"type":"local","managed_by":"someone-else"}}}`), 0644); err != nil {
		t.Fatal(err)
	}
	env := Uninstall(ConfigOptions{Clients: "opencode", LovartPath: "/tmp/lovart", Home: home, Yes: true})
	if env.OK || env.Error == nil || env.Error.Code != "config_conflict" {
		t.Fatalf("expected config conflict, got %#v", env)
	}
	env = Uninstall(ConfigOptions{Clients: "opencode", LovartPath: "/tmp/lovart", Home: home, Yes: true, Force: true})
	if !env.OK {
		t.Fatalf("force uninstall failed: %#v", env)
	}
	if strings.Contains(readText(path), `"lovart"`) {
		t.Fatalf("lovart entry still present:\n%s", readText(path))
	}
}

func TestUninstallClaudeRunsRemoveCommand(t *testing.T) {
	restoreCommandHooks(t)
	lookPath = func(name string) (string, error) { return "/bin/" + name, nil }
	var got []string
	runCommand = func(command []string) (commandResult, error) {
		got = append([]string(nil), command...)
		return commandResult{ReturnCode: 0, Stdout: "removed"}, nil
	}
	env := Uninstall(ConfigOptions{Clients: "claude", LovartPath: "/tmp/lovart", Home: t.TempDir(), Yes: true})
	if !env.OK {
		t.Fatalf("unexpected uninstall envelope: %#v", env)
	}
	want := []string{"claude", "mcp", "remove", "--scope", "user", "lovart"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("command = %#v, want %#v", got, want)
	}
}

func TestInstallRequiresYesUnlessDryRun(t *testing.T) {
	env := Install(ConfigOptions{Clients: "codex", LovartPath: "/tmp/lovart", Home: t.TempDir()})
	if env.OK || env.Error == nil || env.Error.Code != "input_error" {
		t.Fatalf("expected input_error, got %#v", env)
	}
}

func TestClientSelection(t *testing.T) {
	restoreCommandHooks(t)
	lookPath = func(name string) (string, error) { return "", errors.New("missing") }
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0755); err != nil {
		t.Fatal(err)
	}
	ctx, err := newConfigContext(ConfigOptions{LovartPath: "/tmp/lovart", Home: home})
	if err != nil {
		t.Fatal(err)
	}
	selected, err := selectMCPClients("auto", ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 || selected[0] != "codex" {
		t.Fatalf("unexpected auto selection: %#v", selected)
	}
	selected, err = selectMCPClients("none", ctx, false)
	if err != nil || len(selected) != 0 {
		t.Fatalf("unexpected none selection: %#v err=%v", selected, err)
	}
	selected, err = selectMCPClients("opencode,codex,codex", ctx, false)
	if err != nil || len(selected) != 2 || selected[0] != "opencode" || selected[1] != "codex" {
		t.Fatalf("unexpected explicit selection: %#v err=%v", selected, err)
	}
}

func TestCommandClientManualRequiredWhenUnavailable(t *testing.T) {
	restoreCommandHooks(t)
	lookPath = func(name string) (string, error) { return "", errors.New("missing") }
	env := Install(ConfigOptions{Clients: "claude", LovartPath: "/tmp/lovart", Home: t.TempDir(), Yes: true})
	if !env.OK {
		t.Fatalf("manual-required install should not fail: %#v", env)
	}
	results := installResults(t, env)
	if results[0]["status"] != "manual_required" {
		t.Fatalf("unexpected command result: %#v", results[0])
	}
}

func TestCommandClientRunsExpectedCommand(t *testing.T) {
	restoreCommandHooks(t)
	lookPath = func(name string) (string, error) { return "/bin/" + name, nil }
	var got []string
	runCommand = func(command []string) (commandResult, error) {
		got = append([]string(nil), command...)
		return commandResult{ReturnCode: 0, Stdout: "ok"}, nil
	}
	env := Install(ConfigOptions{Clients: "openclaw", LovartPath: "/tmp/lovart", Home: t.TempDir(), Yes: true})
	if !env.OK {
		t.Fatalf("unexpected install envelope: %#v", env)
	}
	if len(got) < 5 || got[0] != "openclaw" || got[1] != "mcp" || got[2] != "set" || got[3] != "lovart" {
		t.Fatalf("unexpected command: %#v", got)
	}
	if !strings.Contains(got[4], `"command":"/tmp/lovart"`) {
		t.Fatalf("unexpected payload: %s", got[4])
	}
}

func installResults(t *testing.T, env envelope.Envelope) []map[string]any {
	t.Helper()
	data := env.Data.(map[string]any)
	results := data["results"].([]map[string]any)
	if len(results) == 0 {
		t.Fatalf("missing results: %#v", data)
	}
	return results
}

func restoreCommandHooks(t *testing.T) {
	t.Helper()
	oldLookPath := lookPath
	oldRunCommand := runCommand
	t.Cleanup(func() {
		lookPath = oldLookPath
		runCommand = oldRunCommand
	})
}

func restoreClock(t *testing.T) {
	t.Helper()
	oldNow := nowUTC
	nowUTC = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }
	t.Cleanup(func() { nowUTC = oldNow })
}
