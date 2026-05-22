package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/aporicho/lovart/internal/audit"
)

func TestCallToolWritesAuditEvents(t *testing.T) {
	var output bytes.Buffer
	server := NewServerWithExecutorAndAudit(&fakeExecutor{}, audit.NewWriter(&output))

	env := server.CallTool(context.Background(), "lovart_models", map[string]any{"refresh": true})
	if !env.OK {
		t.Fatalf("models failed: %#v", env)
	}

	events := decodeAuditEvents(t, output.String())
	if len(events) != 3 {
		t.Fatalf("audit event count = %d, want 3: %#v", len(events), events)
	}
	if events[0]["operation"] != "mcp.tool_call" || events[0]["action"] != "start" || events[0]["tool"] != "lovart_models" {
		t.Fatalf("unexpected start event: %#v", events[0])
	}
	if events[1]["operation"] != "mcp.policy_gate" || events[1]["decision"] != "allow" {
		t.Fatalf("unexpected policy event: %#v", events[1])
	}
	if events[2]["operation"] != "mcp.tool_call" || events[2]["action"] != "finish" || events[2]["decision"] != "success" {
		t.Fatalf("unexpected finish event: %#v", events[2])
	}
}

func TestPolicyGateDenialWritesAuditEvents(t *testing.T) {
	var output bytes.Buffer
	server := NewServerWithExecutorAndAudit(&fakeExecutor{}, audit.NewWriter(&output))

	env := server.CallTool(context.Background(), "lovart_generate", map[string]any{"model": "openai/gpt-image-2", "body": map[string]any{"prompt": "x"}})
	if env.OK {
		t.Fatalf("generate without confirmation should fail: %#v", env)
	}

	events := decodeAuditEvents(t, output.String())
	if len(events) != 3 {
		t.Fatalf("audit event count = %d, want 3: %#v", len(events), events)
	}
	if events[1]["operation"] != "mcp.policy_gate" || events[1]["decision"] != "confirmation_required" {
		t.Fatalf("unexpected policy denial event: %#v", events[1])
	}
	if events[2]["decision"] != "error" {
		t.Fatalf("denied call should finish as error: %#v", events[2])
	}
}

func decodeAuditEvents(t *testing.T, text string) []map[string]any {
	t.Helper()
	lines := bytes.Split(bytes.TrimSpace([]byte(text)), []byte("\n"))
	events := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatalf("decode audit event %q: %v", line, err)
		}
		events = append(events, event)
	}
	return events
}
