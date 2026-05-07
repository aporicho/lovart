package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/paths"
)

type fakeExecutor struct {
	projectSelect ProjectSelectArgs
	generate      GenerateArgs
	jobsRun       JobsRunArgs
	jobsStatus    JobsStatusArgs
	jobsResume    JobsResumeArgs
}

func (f *fakeExecutor) AuthStatus(ctx context.Context) envelope.Envelope {
	return okLocal(map[string]any{"operation": "auth_status"})
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
	return okPreflight(map[string]any{"operation": "project_select", "project_id": args.ProjectID, "project_context_ready": true})
}

func (f *fakeExecutor) Quote(ctx context.Context, args QuoteArgs) envelope.Envelope {
	return okPreflight(map[string]any{"operation": "quote"})
}

func (f *fakeExecutor) Generate(ctx context.Context, args GenerateArgs) envelope.Envelope {
	f.generate = args
	return okSubmit(map[string]any{"operation": "generate"}, true)
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
	if len(tools) != 13 {
		t.Fatalf("expected 13 tools, got %d", len(tools))
	}
	for _, tool := range tools {
		if tool.Name == "lovart_update_sync" || tool.Name == "lovart_auth_extract" || tool.Name == "lovart_auth_login" || tool.Name == "lovart_auth_import" || tool.Name == "lovart_generate_dry_run" || tool.Name == "lovart_jobs_quote" || tool.Name == "lovart_jobs_dry_run" {
			t.Fatalf("unsafe tool exposed: %s", tool.Name)
		}
		if tool.Name == "lovart_generate" {
			properties := tool.InputSchema["properties"].(map[string]any)
			if _, ok := properties["cid"]; ok {
				t.Fatalf("lovart_generate exposes cid: %#v", properties)
			}
		}
		if tool.Name == "lovart_jobs_run" {
			properties := tool.InputSchema["properties"].(map[string]any)
			assertSchemaExcludes(t, tool.Name, properties, []string{"cid", "out_dir", "detail", "wait", "download", "canvas", "canvas_layout", "download_dir_template", "download_file_template", "timeout_seconds", "poll_interval", "retry_failed"})
		}
		if tool.Name == "lovart_jobs_resume" {
			properties := tool.InputSchema["properties"].(map[string]any)
			assertSchemaExcludes(t, tool.Name, properties, []string{"cid", "out_dir", "detail", "wait", "download", "canvas", "canvas_layout", "download_dir_template", "download_file_template", "timeout_seconds", "poll_interval", "project_id"})
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

func TestProductionProjectCurrentDoesNotExposeCID(t *testing.T) {
	t.Cleanup(paths.Reset)
	t.Setenv("LOVART_REVERSE_ROOT", t.TempDir())
	paths.Reset()
	if err := auth.SaveSession(auth.Session{Cookie: "cookie", ProjectID: "project-123", CID: "cid-123"}); err != nil {
		t.Fatal(err)
	}

	env := ProductionExecutor{}.ProjectCurrent(context.Background())
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	for _, want := range []string{`"project_id":"project-123"`, `"project_context_ready":true`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("project current missing %s: %s", want, data)
		}
	}
	for _, forbidden := range []string{"cid-123", "cid_present", `"cid"`} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("project current exposed %s: %s", forbidden, data)
		}
	}
}

func TestGenerateDefaultsToCompletePostprocess(t *testing.T) {
	executor := &fakeExecutor{}
	server := NewServerWithExecutor(executor)
	env := server.CallTool(context.Background(), "lovart_generate", map[string]any{
		"model": "openai/gpt-image-2",
		"body":  map[string]any{"prompt": "test"},
	})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if !executor.generate.Wait || !executor.generate.Download || !executor.generate.Canvas {
		t.Fatalf("generate postprocess defaults = %#v", executor.generate)
	}

	executor = &fakeExecutor{}
	server = NewServerWithExecutor(executor)
	env = server.CallTool(context.Background(), "lovart_generate", map[string]any{
		"model": "openai/gpt-image-2",
		"body":  map[string]any{"prompt": "test"},
		"wait":  false,
	})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if executor.generate.Wait || executor.generate.Download || executor.generate.Canvas {
		t.Fatalf("no-wait normalization failed: %#v", executor.generate)
	}
}

func TestJobsRunArgsExposeUserSurface(t *testing.T) {
	executor := &fakeExecutor{}
	server := NewServerWithExecutor(executor)
	env := server.CallTool(context.Background(), "lovart_jobs_run", map[string]any{
		"jobs_file":         "runs/x/jobs.jsonl",
		"allow_paid":        true,
		"max_total_credits": 12.0,
		"project_id":        "proj_123",
		"download_dir":      "runs/x/images",
	})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if executor.jobsRun.JobsFile != "runs/x/jobs.jsonl" || !executor.jobsRun.AllowPaid || executor.jobsRun.MaxTotalCredits != 12 || executor.jobsRun.ProjectID != "proj_123" || executor.jobsRun.DownloadDir != "runs/x/images" {
		t.Fatalf("jobs run args = %#v", executor.jobsRun)
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

func TestDefaultMCPBatchOptionsCompleteAndShortWait(t *testing.T) {
	opts := defaultMCPBatchOptions()
	if !opts.Wait || !opts.Download || !opts.Canvas {
		t.Fatalf("batch postprocess defaults = %#v", opts)
	}
	if opts.CanvasLayout != "frame" || opts.TimeoutSeconds != MCPWaitTimeoutSeconds || opts.PollInterval != 5 || opts.Detail != "summary" {
		t.Fatalf("batch execution defaults = %#v", opts)
	}
}

func TestJobsResumeArgsExposeUserSurface(t *testing.T) {
	executor := &fakeExecutor{}
	server := NewServerWithExecutor(executor)
	env := server.CallTool(context.Background(), "lovart_jobs_resume", map[string]any{
		"run_dir":           "runs/x",
		"allow_paid":        true,
		"max_total_credits": 24.0,
		"download_dir":      "runs/x/images",
		"retry_failed":      true,
	})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if executor.jobsResume.RunDir != "runs/x" || !executor.jobsResume.AllowPaid || executor.jobsResume.MaxTotalCredits != 24 || executor.jobsResume.DownloadDir != "runs/x/images" || !executor.jobsResume.RetryFailed {
		t.Fatalf("jobs resume args = %#v", executor.jobsResume)
	}
}

func assertSchemaExcludes(t *testing.T, toolName string, properties map[string]any, names []string) {
	t.Helper()
	for _, name := range names {
		if _, ok := properties[name]; ok {
			t.Fatalf("%s exposes internal property %q: %#v", toolName, name, properties)
		}
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
