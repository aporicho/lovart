package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aporicho/lovart/internal/envelope"
)

type fakeExecutor struct {
	projectSelect ProjectSelectArgs
	jobsRun       JobsRunArgs
	jobsStatus    JobsStatusArgs
	jobsResume    JobsResumeArgs
}

func (f *fakeExecutor) Setup(ctx context.Context, args SetupArgs) envelope.Envelope {
	return okLocal(map[string]any{"operation": "setup"})
}

func (f *fakeExecutor) Models(ctx context.Context, args ModelsArgs) envelope.Envelope {
	return okLocal(map[string]any{"operation": "models", "refresh": args.Refresh})
}

func (f *fakeExecutor) Config(ctx context.Context, args ConfigArgs) envelope.Envelope {
	return okLocal(map[string]any{"operation": "config", "model": args.Model})
}

func (f *fakeExecutor) Balance(ctx context.Context) envelope.Envelope {
	return okPreflight(map[string]any{"operation": "balance"})
}

func (f *fakeExecutor) ProjectCurrent(ctx context.Context) envelope.Envelope {
	return okLocal(map[string]any{"operation": "project_current"})
}

func (f *fakeExecutor) ProjectList(ctx context.Context) envelope.Envelope {
	return okPreflight(map[string]any{"operation": "project_list"})
}

func (f *fakeExecutor) ProjectSelect(ctx context.Context, args ProjectSelectArgs) envelope.Envelope {
	f.projectSelect = args
	return okLocal(map[string]any{"operation": "project_select", "project_id": args.ProjectID})
}

func (f *fakeExecutor) Quote(ctx context.Context, args QuoteArgs) envelope.Envelope {
	return okPreflight(map[string]any{"operation": "quote"})
}

func (f *fakeExecutor) GenerateDryRun(ctx context.Context, args GenerateArgs) envelope.Envelope {
	return okPreflightSubmission(map[string]any{"operation": "generate_dry_run"}, false)
}

func (f *fakeExecutor) Generate(ctx context.Context, args GenerateArgs) envelope.Envelope {
	return okSubmit(map[string]any{"operation": "generate"}, true)
}

func (f *fakeExecutor) JobsQuote(ctx context.Context, args JobsQuoteArgs) envelope.Envelope {
	return okPreflight(map[string]any{"operation": "jobs_quote"})
}

func (f *fakeExecutor) JobsDryRun(ctx context.Context, args JobsDryRunArgs) envelope.Envelope {
	return okPreflightSubmission(map[string]any{"operation": "jobs_dry_run"}, false)
}

func (f *fakeExecutor) JobsRun(ctx context.Context, args JobsRunArgs) envelope.Envelope {
	f.jobsRun = args
	return okSubmit(map[string]any{"operation": "jobs_run"}, false)
}

func (f *fakeExecutor) JobsStatus(ctx context.Context, args JobsStatusArgs) envelope.Envelope {
	f.jobsStatus = args
	return okLocal(map[string]any{"operation": "jobs_status"})
}

func (f *fakeExecutor) JobsResume(ctx context.Context, args JobsResumeArgs) envelope.Envelope {
	f.jobsResume = args
	return okSubmit(map[string]any{"operation": "jobs_resume"}, false)
}

func TestHandleInitializeAndListTools(t *testing.T) {
	server := NewServerWithExecutor(&fakeExecutor{})
	initResp := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
	})
	if initResp == nil || initResp.Error != nil {
		t.Fatalf("initialize failed: %#v", initResp)
	}
	result := initResp.Result.(map[string]any)
	if result["protocolVersion"] != ProtocolVersion {
		t.Fatalf("unexpected protocol version: %#v", result["protocolVersion"])
	}

	listResp := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	})
	if listResp == nil || listResp.Error != nil {
		t.Fatalf("tools/list failed: %#v", listResp)
	}
	tools := listResp.Result.(map[string]any)["tools"].([]Tool)
	if len(tools) != 15 {
		t.Fatalf("expected 15 tools, got %d", len(tools))
	}
	for _, tool := range tools {
		if tool.Name == "lovart_update_sync" || tool.Name == "lovart_auth_extract" {
			t.Fatalf("unsafe tool exposed: %s", tool.Name)
		}
	}
}

func TestToolsCallReturnsEnvelopeText(t *testing.T) {
	server := NewServerWithExecutor(&fakeExecutor{})
	resp := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      "call-1",
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "lovart_models",
			"arguments": map[string]any{"refresh": true},
		},
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("tools/call failed: %#v", resp)
	}
	result := resp.Result.(map[string]any)
	if isError, _ := result["isError"].(bool); isError {
		t.Fatalf("unexpected MCP error result: %#v", result)
	}
	env := envelopeFromToolResult(t, result)
	if !env.OK || env.ExecutionClass != executionLocal {
		t.Fatalf("unexpected envelope: %#v", env)
	}
}

func TestInvalidArgumentsReturnJSONRPCError(t *testing.T) {
	server := NewServerWithExecutor(&fakeExecutor{})
	resp := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "lovart_models",
			"arguments": []any{},
		},
	})
	if resp == nil || resp.Error == nil {
		t.Fatalf("expected JSON-RPC error, got %#v", resp)
	}
	if resp.Error.Code != -32602 {
		t.Fatalf("unexpected error code: %d", resp.Error.Code)
	}
}

func TestUnknownToolReturnsEnvelopeError(t *testing.T) {
	server := NewServerWithExecutor(&fakeExecutor{})
	env := server.CallTool(context.Background(), "lovart_update_sync", map[string]any{})
	if env.OK {
		t.Fatalf("expected error envelope")
	}
	if env.Error == nil || env.Error.Code != "input_error" {
		t.Fatalf("unexpected error: %#v", env.Error)
	}
}

func TestProjectSelectRequiresProjectID(t *testing.T) {
	executor := &fakeExecutor{}
	server := NewServerWithExecutor(executor)
	env := server.CallTool(context.Background(), "lovart_project_select", map[string]any{})
	if env.OK {
		t.Fatalf("expected missing project_id error")
	}

	env = server.CallTool(context.Background(), "lovart_project_select", map[string]any{"project_id": "proj_123"})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if executor.projectSelect.ProjectID != "proj_123" {
		t.Fatalf("project select args = %#v", executor.projectSelect)
	}
}

func TestJobsRunDefaultsAndWaitCap(t *testing.T) {
	executor := &fakeExecutor{}
	server := NewServerWithExecutor(executor)
	env := server.CallTool(context.Background(), "lovart_jobs_run", map[string]any{
		"jobs_file":       "runs/x/jobs.jsonl",
		"wait":            true,
		"timeout_seconds": 999.0,
	})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if executor.jobsRun.TimeoutSeconds != MCPWaitTimeoutSeconds {
		t.Fatalf("timeout not capped: %v", executor.jobsRun.TimeoutSeconds)
	}
	if executor.jobsRun.Detail != "summary" {
		t.Fatalf("detail default mismatch: %q", executor.jobsRun.Detail)
	}
	if len(env.Warnings) == 0 {
		t.Fatalf("expected cap warning")
	}

	executor = &fakeExecutor{}
	server = NewServerWithExecutor(executor)
	env = server.CallTool(context.Background(), "lovart_jobs_run", map[string]any{
		"jobs_file": "runs/x/jobs.jsonl",
	})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if executor.jobsRun.Wait || executor.jobsRun.Download || executor.jobsRun.Canvas {
		t.Fatalf("unexpected postprocess defaults: %#v", executor.jobsRun)
	}
}

func TestJobsStatusDefaultsToSummary(t *testing.T) {
	executor := &fakeExecutor{}
	server := NewServerWithExecutor(executor)
	env := server.CallTool(context.Background(), "lovart_jobs_status", map[string]any{
		"run_dir": "runs/x",
	})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if executor.jobsStatus.Detail != "summary" {
		t.Fatalf("detail default mismatch: %q", executor.jobsStatus.Detail)
	}
}

func TestJobsResumeWaitCap(t *testing.T) {
	executor := &fakeExecutor{}
	server := NewServerWithExecutor(executor)
	env := server.CallTool(context.Background(), "lovart_jobs_resume", map[string]any{
		"run_dir": "runs/x",
		"wait":    true,
	})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if executor.jobsResume.TimeoutSeconds != MCPWaitTimeoutSeconds {
		t.Fatalf("timeout not capped: %v", executor.jobsResume.TimeoutSeconds)
	}
	if executor.jobsResume.Detail != "summary" {
		t.Fatalf("detail default mismatch: %q", executor.jobsResume.Detail)
	}
}

func envelopeFromToolResult(t *testing.T, result map[string]any) envelope.Envelope {
	t.Helper()
	content := result["content"].([]map[string]string)
	if len(content) != 1 {
		t.Fatalf("unexpected content: %#v", content)
	}
	var env envelope.Envelope
	if err := json.Unmarshal([]byte(content[0]["text"]), &env); err != nil {
		t.Fatalf("parse envelope: %v", err)
	}
	return env
}
