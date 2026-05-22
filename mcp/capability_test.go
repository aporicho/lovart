package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestToolListCapabilityMetadata(t *testing.T) {
	server := NewServerWithExecutor(&fakeExecutor{})
	response := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	})
	if response == nil || response.Error != nil {
		t.Fatalf("tools/list failed: %#v", response)
	}

	encoded, err := json.Marshal(response.Result)
	if err != nil {
		t.Fatalf("marshal tools/list result: %v", err)
	}
	if strings.Contains(string(encoded), "test-secret-value") {
		t.Fatalf("tools/list leaked a secret value: %s", string(encoded))
	}

	tools := response.Result.(map[string]any)["tools"].([]Tool)
	for _, tool := range tools {
		properties := tool.InputSchema["properties"].(map[string]any)
		assertSchemaExcludes(t, tool.Name, properties, []string{"cid", "cookie", "token", "csrf", "api_key"})
	}
	byName := toolsByName(tools)
	for _, name := range []string{
		"lovart_project_delete",
		"lovart_task_cancel",
		"lovart_task_canvas",
		"lovart_generate",
		"lovart_jobs_run",
		"lovart_jobs_finalize",
	} {
		capability := requireToolCapability(t, byName, name)
		if capability.ExecutionClass == "" {
			t.Fatalf("%s missing execution_class", name)
		}
		if byName[name].Annotations["execution_class"] != capability.ExecutionClass {
			t.Fatalf("%s annotations do not mirror capability: %#v", name, byName[name].Annotations)
		}
	}
}

func TestReadOnlyToolsAreNotRemoteWrite(t *testing.T) {
	byName := toolsByName(Tools())
	for _, name := range []string{
		"lovart_auth_status",
		"lovart_extension_status",
		"lovart_setup",
		"lovart_config",
		"lovart_project_current",
		"lovart_models",
		"lovart_balance",
		"lovart_project_list",
		"lovart_project_show",
		"lovart_task_list",
		"lovart_task_status",
		"lovart_task_wait",
		"lovart_canvas_artifacts",
		"lovart_canvas_artifact",
		"lovart_quote",
	} {
		capability := requireToolCapability(t, byName, name)
		if capability.RemoteWrite {
			t.Fatalf("%s marked remote_write", name)
		}
	}
}

func TestHighRiskToolCapabilities(t *testing.T) {
	byName := toolsByName(Tools())

	assertCapability(t, byName, "lovart_project_delete", ToolCapability{
		ExecutionClass:           executionSubmit,
		NetworkRequired:          true,
		RemoteWrite:              true,
		Destructive:              true,
		RequiresUserConfirmation: true,
	})
	assertCapability(t, byName, "lovart_task_cancel", ToolCapability{
		ExecutionClass:           executionSubmit,
		NetworkRequired:          true,
		RemoteWrite:              true,
		Destructive:              true,
		RequiresUserConfirmation: true,
	})
	assertCapability(t, byName, "lovart_task_canvas", ToolCapability{
		ExecutionClass:  executionSubmit,
		NetworkRequired: true,
		RemoteWrite:     true,
	})
	assertCapability(t, byName, "lovart_generate", ToolCapability{
		ExecutionClass:       executionSubmit,
		NetworkRequired:      true,
		RemoteWrite:          true,
		PaidPossible:         true,
		LocalFilesystemWrite: true,
	})
	assertCapability(t, byName, "lovart_jobs_run", ToolCapability{
		ExecutionClass:       executionSubmit,
		NetworkRequired:      true,
		RemoteWrite:          true,
		PaidPossible:         true,
		LocalFilesystemWrite: true,
	})
	assertCapability(t, byName, "lovart_jobs_finalize", ToolCapability{
		ExecutionClass:       executionSubmit,
		NetworkRequired:      true,
		RemoteWrite:          true,
		LocalFilesystemWrite: true,
	})
}

func toolsByName(tools []Tool) map[string]Tool {
	byName := make(map[string]Tool, len(tools))
	for _, tool := range tools {
		byName[tool.Name] = tool
	}
	return byName
}

func requireToolCapability(t *testing.T, tools map[string]Tool, name string) ToolCapability {
	t.Helper()
	tool, ok := tools[name]
	if !ok {
		t.Fatalf("missing tool %s", name)
	}
	if tool.Capability.ExecutionClass == "" {
		t.Fatalf("%s has no capability metadata", name)
	}
	return tool.Capability
}

func assertCapability(t *testing.T, tools map[string]Tool, name string, expected ToolCapability) {
	t.Helper()
	actual := requireToolCapability(t, tools, name)
	if actual.ExecutionClass != expected.ExecutionClass {
		t.Fatalf("%s execution_class: got %q want %q", name, actual.ExecutionClass, expected.ExecutionClass)
	}
	if actual.NetworkRequired != expected.NetworkRequired {
		t.Fatalf("%s network_required: got %t want %t", name, actual.NetworkRequired, expected.NetworkRequired)
	}
	if actual.RemoteWrite != expected.RemoteWrite {
		t.Fatalf("%s remote_write: got %t want %t", name, actual.RemoteWrite, expected.RemoteWrite)
	}
	if actual.PaidPossible != expected.PaidPossible {
		t.Fatalf("%s paid_possible: got %t want %t", name, actual.PaidPossible, expected.PaidPossible)
	}
	if actual.Destructive != expected.Destructive {
		t.Fatalf("%s destructive: got %t want %t", name, actual.Destructive, expected.Destructive)
	}
	if actual.LocalFilesystemWrite != expected.LocalFilesystemWrite {
		t.Fatalf("%s local_filesystem_write: got %t want %t", name, actual.LocalFilesystemWrite, expected.LocalFilesystemWrite)
	}
	if actual.RequiresUserConfirmation != expected.RequiresUserConfirmation {
		t.Fatalf("%s requires_user_confirmation: got %t want %t", name, actual.RequiresUserConfirmation, expected.RequiresUserConfirmation)
	}
}
