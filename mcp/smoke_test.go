package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
)

type fakeSmokeRPCClient struct {
	authAvailable bool
	projectReady  bool
	tools         []Tool
	calls         []fakeSmokeCall
}

type fakeSmokeCall struct {
	method string
	tool   string
	args   map[string]any
}

func newFakeSmokeRPCClient() *fakeSmokeRPCClient {
	return &fakeSmokeRPCClient{
		authAvailable: true,
		projectReady:  true,
		tools:         Tools(),
	}
}

func (f *fakeSmokeRPCClient) Call(ctx context.Context, method string, params map[string]any) (map[string]any, *rpcErrorBody, error) {
	call := fakeSmokeCall{method: method}
	if method == "tools/call" {
		call.tool, _ = params["name"].(string)
		call.args, _ = params["arguments"].(map[string]any)
	}
	f.calls = append(f.calls, call)
	switch method {
	case "initialize":
		return map[string]any{
			"protocolVersion": ProtocolVersion,
			"serverInfo": map[string]any{
				"name":    ServerName,
				"version": "test",
			},
		}, nil, nil
	case "ping":
		return map[string]any{}, nil, nil
	case "tools/list":
		return map[string]any{"tools": roundTripTools(f.tools)}, nil, nil
	case "tools/call":
		return fakeToolResult(call.tool, f.authAvailable, f.projectReady), nil, nil
	default:
		return nil, &rpcErrorBody{Code: -32601, Message: "unknown method"}, nil
	}
}

func (f *fakeSmokeRPCClient) Close() error {
	return nil
}

func fakeToolResult(tool string, authAvailable bool, projectReady bool) map[string]any {
	switch tool {
	case "lovart_auth_status":
		return toolResult(okLocal(map[string]any{
			"available":             authAvailable,
			"project_id_present":    projectReady,
			"project_context_ready": projectReady,
		}, true))
	case "lovart_setup":
		return toolResult(okLocal(map[string]any{"status": "ok", "ready": true}, true))
	case "lovart_models":
		return toolResult(okLocal(map[string]any{"source": "registry", "count": 1}, true))
	case "lovart_config":
		return toolResult(okLocal(map[string]any{
			"model":  "openai/gpt-image-2",
			"fields": []map[string]any{{"name": "prompt", "type": "string"}},
		}, true))
	case "lovart_project_current":
		if !projectReady {
			return toolResult(envelope.Err(errors.CodeInputError, "no project context", map[string]any{
				"recommended_actions": []string{"run `lovart project list`", "run `lovart project select <project_id>`"},
			}))
		}
		return toolResult(okLocal(map[string]any{"project_id": "project-123", "project_context_ready": true}, true))
	case "lovart_balance":
		return toolResult(okPreflight(map[string]any{"balance": 100.0}))
	case "lovart_quote":
		return toolResult(okPreflight(map[string]any{"price": 0.5, "balance": 100.0}))
	case "lovart_generate":
		return toolResult(okSubmit(map[string]any{"submitted": true, "task_id": "task-123", "status": "submitted"}, true))
	default:
		return toolResult(envelope.Err(errors.CodeInputError, "unknown MCP tool", map[string]any{"tool": tool}))
	}
}

func TestSmokeReadyDoesNotSubmitByDefault(t *testing.T) {
	fake := newFakeSmokeRPCClient()
	withFakeSmokeClient(t, fake)

	env := Smoke(context.Background(), SmokeOptions{
		Model: "openai/gpt-image-2",
		Body:  map[string]any{"prompt": "test"},
		Mode:  "relax",
	})
	if !env.OK {
		t.Fatalf("Smoke failed: %#v", env)
	}
	report := smokeReportFromEnvelope(t, env)
	if report.Status != smokeStatusReady {
		t.Fatalf("status = %q, want ready: %#v", report.Status, report)
	}
	if calledTool(fake, "lovart_generate") {
		t.Fatalf("smoke called generate without --submit: %#v", fake.calls)
	}
	if !calledMethod(fake, "initialize") || !calledMethod(fake, "tools/list") {
		t.Fatalf("smoke did not exercise MCP contract: %#v", fake.calls)
	}
}

func TestSmokeNoAuthSkipsOnlineChecksAndDegrades(t *testing.T) {
	fake := newFakeSmokeRPCClient()
	fake.authAvailable = false
	fake.projectReady = false
	withFakeSmokeClient(t, fake)

	env := Smoke(context.Background(), SmokeOptions{
		Model: "openai/gpt-image-2",
		Body:  map[string]any{"prompt": "test"},
		Mode:  "relax",
	})
	if !env.OK {
		t.Fatalf("Smoke should degrade, not fail: %#v", env)
	}
	report := smokeReportFromEnvelope(t, env)
	if report.Status != smokeStatusDegraded {
		t.Fatalf("status = %q, want degraded: %#v", report.Status, report)
	}
	if calledTool(fake, "lovart_balance") || calledTool(fake, "lovart_quote") {
		t.Fatalf("smoke should skip online checks without auth: %#v", fake.calls)
	}
	if !hasStepStatus(report, "quote", smokeStepSkipped) {
		t.Fatalf("quote should be skipped: %#v", report.Steps)
	}
}

func TestSmokeToolListMismatchFails(t *testing.T) {
	fake := newFakeSmokeRPCClient()
	fake.tools = fake.tools[:len(fake.tools)-1]
	withFakeSmokeClient(t, fake)

	env := Smoke(context.Background(), SmokeOptions{
		Model: "openai/gpt-image-2",
		Body:  map[string]any{"prompt": "test"},
		Mode:  "relax",
	})
	if env.OK {
		t.Fatalf("Smoke should fail on tool mismatch: %#v", env)
	}
	if env.Error == nil || env.Error.Code != errors.CodeInternal {
		t.Fatalf("unexpected error: %#v", env.Error)
	}
	report := smokeReportFromError(t, env)
	if report.Status != smokeStatusFailed || !hasStepStatus(report, "tools_list", smokeStepFailed) {
		t.Fatalf("unexpected report: %#v", report)
	}
}

func TestSmokeSubmitRequiresPaidBudget(t *testing.T) {
	fake := newFakeSmokeRPCClient()
	withFakeSmokeClient(t, fake)

	env := Smoke(context.Background(), SmokeOptions{
		Model:     "openai/gpt-image-2",
		Body:      map[string]any{"prompt": "test"},
		Mode:      "relax",
		Submit:    true,
		AllowPaid: false,
	})
	if env.OK {
		t.Fatalf("Smoke submit without budget should fail: %#v", env)
	}
	if env.Error == nil || env.Error.Code != errors.CodeInputError {
		t.Fatalf("unexpected error: %#v", env.Error)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("invalid submit options should not start MCP calls: %#v", fake.calls)
	}
}

func TestSmokeSubmitCallsGenerateWithPostprocessDisabled(t *testing.T) {
	fake := newFakeSmokeRPCClient()
	withFakeSmokeClient(t, fake)

	env := Smoke(context.Background(), SmokeOptions{
		Model:      "openai/gpt-image-2",
		Body:       map[string]any{"prompt": "test"},
		Mode:       "relax",
		Submit:     true,
		AllowPaid:  true,
		MaxCredits: 1,
	})
	if !env.OK {
		t.Fatalf("Smoke submit failed: %#v", env)
	}
	if !calledTool(fake, "lovart_generate") {
		t.Fatalf("smoke submit did not call generate: %#v", fake.calls)
	}
	generateArgs := toolArgs(fake, "lovart_generate")
	if generateArgs["wait"] != false || generateArgs["download"] != false || generateArgs["canvas"] != false {
		t.Fatalf("generate postprocess should be disabled: %#v", generateArgs)
	}
	if env.RemoteWrite == nil || !*env.RemoteWrite || env.Submitted == nil || !*env.Submitted {
		t.Fatalf("submit metadata not set: %#v", env)
	}
}

func withFakeSmokeClient(t *testing.T, client *fakeSmokeRPCClient) {
	t.Helper()
	original := newSmokeRPCClient
	newSmokeRPCClient = func(ctx context.Context, executable string) (smokeRPCClient, error) {
		return client, nil
	}
	t.Cleanup(func() { newSmokeRPCClient = original })
}

func roundTripTools(tools []Tool) []any {
	data, _ := json.Marshal(tools)
	var result []any
	_ = json.Unmarshal(data, &result)
	return result
}

func calledMethod(client *fakeSmokeRPCClient, method string) bool {
	for _, call := range client.calls {
		if call.method == method {
			return true
		}
	}
	return false
}

func calledTool(client *fakeSmokeRPCClient, tool string) bool {
	for _, call := range client.calls {
		if call.tool == tool {
			return true
		}
	}
	return false
}

func toolArgs(client *fakeSmokeRPCClient, tool string) map[string]any {
	for _, call := range client.calls {
		if call.tool == tool {
			return call.args
		}
	}
	return nil
}

func hasStepStatus(report SmokeReport, name string, status string) bool {
	for _, step := range report.Steps {
		if step.Name == name && step.Status == status {
			return true
		}
	}
	return false
}

func smokeReportFromEnvelope(t *testing.T, env envelope.Envelope) SmokeReport {
	t.Helper()
	var report SmokeReport
	data, err := json.Marshal(env.Data)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("parse report: %v\n%s", err, data)
	}
	return report
}

func smokeReportFromError(t *testing.T, env envelope.Envelope) SmokeReport {
	t.Helper()
	if env.Error == nil {
		t.Fatal("missing error")
	}
	raw, ok := env.Error.Details["report"]
	if !ok {
		t.Fatalf("missing report: %#v", env.Error.Details)
	}
	var report SmokeReport
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("parse report: %v\n%s", err, data)
	}
	return report
}
