package mcp

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestMCPToolCatalogConsistency(t *testing.T) {
	expected := ToolNames()
	if len(expected) == 0 {
		t.Fatal("ToolNames returned no tools")
	}

	tools := Tools()
	toolNames := make([]string, 0, len(tools))
	seen := map[string]bool{}
	for _, tool := range tools {
		if tool.Name == "" {
			t.Fatalf("Tools contains empty tool name: %#v", tool)
		}
		if seen[tool.Name] {
			t.Fatalf("Tools contains duplicate tool name %q", tool.Name)
		}
		seen[tool.Name] = true
		toolNames = append(toolNames, tool.Name)
	}
	assertToolNamesEqual(t, "Tools()", expected, toolNames)

	server := NewServerWithExecutor(&fakeExecutor{})
	listResp := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	})
	if listResp == nil || listResp.Error != nil {
		t.Fatalf("tools/list failed: %#v", listResp)
	}
	listedTools := listResp.Result.(map[string]any)["tools"].([]Tool)
	listedNames := make([]string, 0, len(listedTools))
	for _, tool := range listedTools {
		listedNames = append(listedNames, tool.Name)
	}
	assertToolNamesEqual(t, "tools/list", expected, listedNames)

	status := Status(ConfigOptions{Clients: "none", Home: t.TempDir(), LovartPath: "/tmp/lovart"})
	if !status.OK {
		t.Fatalf("mcp status failed: %#v", status)
	}
	data := status.Data.(map[string]any)
	statusTools := data["tools"].(map[string]any)
	statusCount := statusTools["count"].(int)
	if statusCount != len(expected) {
		t.Fatalf("mcp status tools.count drift: got %d want %d", statusCount, len(expected))
	}
	statusNames := statusTools["names"].([]string)
	assertToolNamesEqual(t, "mcp status tools.names", expected, statusNames)
}

func TestREADMEMCPToolListConsistency(t *testing.T) {
	readme, err := os.ReadFile("../README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	text := string(readme)
	expected := ToolNames()

	countRE := regexp.MustCompile(`MCP server 声明 protocol version ` + "`" + regexp.QuoteMeta(ProtocolVersion) + "`" + `，并提供 ([0-9]+) 个 tools：`)
	countMatch := countRE.FindStringSubmatch(text)
	if len(countMatch) != 2 {
		t.Fatalf("README MCP tool count marker not found for protocol %s", ProtocolVersion)
	}
	readmeCount, err := strconv.Atoi(countMatch[1])
	if err != nil {
		t.Fatalf("parse README MCP tool count %q: %v", countMatch[1], err)
	}
	if readmeCount != len(expected) {
		t.Fatalf("README MCP tool count drift: got %d want %d", readmeCount, len(expected))
	}

	readmeNames, err := readmeMCPToolNames(text)
	if err != nil {
		t.Fatal(err)
	}
	assertToolNamesEqual(t, "README MCP tool list", expected, readmeNames)
}

func readmeMCPToolNames(readme string) ([]string, error) {
	marker := "MCP server 声明 protocol version"
	idx := strings.Index(readme, marker)
	if idx < 0 {
		return nil, fmt.Errorf("README MCP tool list marker not found")
	}
	afterMarker := readme[idx:]
	blockStart := strings.Index(afterMarker, "```text")
	if blockStart < 0 {
		return nil, fmt.Errorf("README MCP tool list code block start not found")
	}
	afterStart := afterMarker[blockStart+len("```text"):]
	blockEnd := strings.Index(afterStart, "```")
	if blockEnd < 0 {
		return nil, fmt.Errorf("README MCP tool list code block end not found")
	}
	block := afterStart[:blockEnd]
	var names []string
	for _, line := range strings.Split(block, "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	return names, nil
}

func assertToolNamesEqual(t *testing.T, label string, expected []string, actual []string) {
	t.Helper()
	if len(expected) == len(actual) {
		matches := true
		for i := range expected {
			if expected[i] != actual[i] {
				matches = false
				break
			}
		}
		if matches {
			return
		}
	}
	missing, extra := diffToolNames(expected, actual)
	t.Fatalf("%s drift: got %d tools want %d; missing_tools=%v extra_tools=%v actual_tools=%v expected_tools=%v", label, len(actual), len(expected), missing, extra, actual, expected)
}

func diffToolNames(expected []string, actual []string) ([]string, []string) {
	actualSet := make(map[string]bool, len(actual))
	for _, name := range actual {
		actualSet[name] = true
	}
	expectedSet := make(map[string]bool, len(expected))
	for _, name := range expected {
		expectedSet[name] = true
	}
	var missing []string
	for _, name := range expected {
		if !actualSet[name] {
			missing = append(missing, name)
		}
	}
	var extra []string
	for _, name := range actual {
		if !expectedSet[name] {
			extra = append(extra, name)
		}
	}
	return missing, extra
}
