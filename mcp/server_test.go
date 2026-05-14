package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/paths"
)

type fakeExecutor struct {
	authLogin        AuthLoginArgs
	extensionStatus  ExtensionStatusArgs
	extensionInstall ExtensionInstallArgs
	extensionOpen    ExtensionOpenArgs
	projectCreate    ProjectCreateArgs
	projectSelect    ProjectSelectArgs
	projectShow      ProjectShowArgs
	projectOpen      ProjectOpenArgs
	projectRename    ProjectRenameArgs
	projectDelete    ProjectDeleteArgs
	taskList         TaskListArgs
	taskCancel       TaskCancelArgs
	taskStatus       TaskStatusArgs
	taskWait         TaskWaitArgs
	taskCanvas       TaskCanvasArgs
	taskDownload     TaskDownloadArgs
	canvasArtifacts  CanvasArtifactsArgs
	canvasArtifact   CanvasArtifactArgs
	canvasDownload   CanvasDownloadArgs
	generate         GenerateArgs
	jobsRun          JobsRunArgs
	jobsStatus       JobsStatusArgs
	jobsResume       JobsResumeArgs
	jobsFinalize     JobsFinalizeArgs
}

func (f *fakeExecutor) AuthStatus(ctx context.Context) envelope.Envelope {
	return okLocal(map[string]any{"operation": "auth_status"})
}

func (f *fakeExecutor) AuthLogin(ctx context.Context, args AuthLoginArgs) envelope.Envelope {
	f.authLogin = args
	return okLocal(map[string]any{"operation": "auth_login", "timeout_seconds": args.TimeoutSeconds})
}

func (f *fakeExecutor) ExtensionStatus(ctx context.Context, args ExtensionStatusArgs) envelope.Envelope {
	f.extensionStatus = args
	return okLocal(map[string]any{"operation": "extension_status", "extension_dir": args.ExtensionDir})
}

func (f *fakeExecutor) ExtensionInstall(ctx context.Context, args ExtensionInstallArgs) envelope.Envelope {
	f.extensionInstall = args
	return okLocal(map[string]any{"operation": "extension_install", "open": args.Open, "dry_run": args.DryRun})
}

func (f *fakeExecutor) ExtensionOpen(ctx context.Context, args ExtensionOpenArgs) envelope.Envelope {
	f.extensionOpen = args
	return okLocal(map[string]any{"operation": "extension_open", "extension_dir": args.ExtensionDir})
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

func (f *fakeExecutor) ProjectCreate(ctx context.Context, args ProjectCreateArgs) envelope.Envelope {
	f.projectCreate = args
	return okSubmit(map[string]any{"operation": "project_create", "project_name": args.Name, "selected": args.Select}, true)
}

func (f *fakeExecutor) ProjectSelect(ctx context.Context, args ProjectSelectArgs) envelope.Envelope {
	f.projectSelect = args
	return okPreflight(map[string]any{"operation": "project_select", "project_id": args.ProjectID, "project_context_ready": true})
}

func (f *fakeExecutor) ProjectShow(ctx context.Context, args ProjectShowArgs) envelope.Envelope {
	f.projectShow = args
	return okPreflight(map[string]any{"operation": "project_show", "project_id": args.ProjectID})
}

func (f *fakeExecutor) ProjectOpen(ctx context.Context, args ProjectOpenArgs) envelope.Envelope {
	f.projectOpen = args
	return okLocal(map[string]any{"operation": "project_open", "project_id": args.ProjectID})
}

func (f *fakeExecutor) ProjectRename(ctx context.Context, args ProjectRenameArgs) envelope.Envelope {
	f.projectRename = args
	return okSubmit(map[string]any{"operation": "project_rename", "project_id": args.ProjectID, "project_name": args.NewName}, true)
}

func (f *fakeExecutor) ProjectDelete(ctx context.Context, args ProjectDeleteArgs) envelope.Envelope {
	f.projectDelete = args
	return okSubmit(map[string]any{"operation": "project_delete", "project_id": args.ProjectID}, true)
}

func (f *fakeExecutor) TaskStatus(ctx context.Context, args TaskStatusArgs) envelope.Envelope {
	f.taskStatus = args
	return okPreflight(map[string]any{"operation": "task_status", "task_id": args.TaskID})
}

func (f *fakeExecutor) TaskList(ctx context.Context, args TaskListArgs) envelope.Envelope {
	f.taskList = args
	return okPreflight(map[string]any{"operation": "task_list", "active": args.Active})
}

func (f *fakeExecutor) TaskCancel(ctx context.Context, args TaskCancelArgs) envelope.Envelope {
	f.taskCancel = args
	return okSubmit(map[string]any{"operation": "task_cancel", "task_ids": args.TaskIDs}, true)
}

func (f *fakeExecutor) TaskWait(ctx context.Context, args TaskWaitArgs) envelope.Envelope {
	f.taskWait = args
	return okPreflight(map[string]any{"operation": "task_wait", "task_id": args.TaskID})
}

func (f *fakeExecutor) TaskCanvas(ctx context.Context, args TaskCanvasArgs) envelope.Envelope {
	f.taskCanvas = args
	return okSubmit(map[string]any{"operation": "task_canvas", "task_id": args.TaskID}, true)
}

func (f *fakeExecutor) TaskDownload(ctx context.Context, args TaskDownloadArgs) envelope.Envelope {
	f.taskDownload = args
	return okPreflight(map[string]any{"operation": "task_download", "task_id": args.TaskID})
}

func (f *fakeExecutor) CanvasArtifacts(ctx context.Context, args CanvasArtifactsArgs) envelope.Envelope {
	f.canvasArtifacts = args
	return okPreflight(map[string]any{"operation": "canvas_artifacts", "project_id": args.ProjectID})
}

func (f *fakeExecutor) CanvasArtifact(ctx context.Context, args CanvasArtifactArgs) envelope.Envelope {
	f.canvasArtifact = args
	return okPreflight(map[string]any{"operation": "canvas_artifact", "artifact_id": args.ArtifactID})
}

func (f *fakeExecutor) CanvasDownload(ctx context.Context, args CanvasDownloadArgs) envelope.Envelope {
	f.canvasDownload = args
	return okPreflight(map[string]any{"operation": "canvas_download", "project_id": args.ProjectID})
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

func (f *fakeExecutor) JobsFinalize(ctx context.Context, args JobsFinalizeArgs) envelope.Envelope {
	f.jobsFinalize = args
	return okSubmit(map[string]any{"operation": "jobs_finalize"}, args.Canvas)
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
	if len(tools) != 32 {
		t.Fatalf("expected 32 tools, got %d", len(tools))
	}
	for _, tool := range tools {
		if tool.Name == "lovart_project_repair_canvas" {
			t.Fatalf("removed repair canvas tool is still exposed")
		}
		if tool.Name == "lovart_update_sync" || tool.Name == "lovart_auth_extract" || tool.Name == "lovart_auth_import" || tool.Name == "lovart_generate_dry_run" || tool.Name == "lovart_jobs_quote" || tool.Name == "lovart_jobs_dry_run" {
			t.Fatalf("unsafe tool exposed: %s", tool.Name)
		}
		if strings.HasPrefix(tool.Name, "lovart_auth_") || strings.HasPrefix(tool.Name, "lovart_extension_") {
			properties := tool.InputSchema["properties"].(map[string]any)
			assertSchemaExcludes(t, tool.Name, properties, []string{"cid", "cookie", "token", "csrf"})
		}
		if strings.HasPrefix(tool.Name, "lovart_project_") {
			properties := tool.InputSchema["properties"].(map[string]any)
			assertSchemaExcludes(t, tool.Name, properties, []string{"cid", "cookie", "token", "csrf"})
		}
		if strings.HasPrefix(tool.Name, "lovart_canvas_") || strings.HasPrefix(tool.Name, "lovart_task_") {
			properties := tool.InputSchema["properties"].(map[string]any)
			assertSchemaExcludes(t, tool.Name, properties, []string{"cid", "cookie", "token", "csrf"})
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
		if tool.Name == "lovart_jobs_finalize" {
			properties := tool.InputSchema["properties"].(map[string]any)
			assertSchemaExcludes(t, tool.Name, properties, []string{"cid", "out_dir", "wait", "download_dir_template", "download_file_template", "timeout_seconds", "poll_interval", "allow_paid", "max_total_credits", "retry_failed"})
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

func TestAuthLoginAndExtensionToolsParseArgs(t *testing.T) {
	executor := &fakeExecutor{}
	server := NewServerWithExecutor(executor)
	env := server.CallTool(context.Background(), "lovart_auth_login", map[string]any{"timeout_seconds": 12.5})
	if !env.OK || executor.authLogin.TimeoutSeconds != 12.5 {
		t.Fatalf("auth login args not parsed: env=%#v args=%#v", env, executor.authLogin)
	}
	env = server.CallTool(context.Background(), "lovart_auth_login", map[string]any{"timeout_seconds": 0})
	if env.OK || env.Error == nil || env.Error.Code != "input_error" {
		t.Fatalf("expected invalid timeout error, got %#v", env)
	}

	env = server.CallTool(context.Background(), "lovart_extension_install", map[string]any{
		"source_dir":    "/source",
		"extension_dir": "/target",
		"dry_run":       true,
	})
	if !env.OK {
		t.Fatalf("extension install failed: %#v", env)
	}
	if executor.extensionInstall.SourceDir != "/source" || executor.extensionInstall.ExtensionDir != "/target" || !executor.extensionInstall.DryRun || !executor.extensionInstall.Open {
		t.Fatalf("extension install args = %#v", executor.extensionInstall)
	}

	env = server.CallTool(context.Background(), "lovart_extension_open", map[string]any{"extension_dir": "/target"})
	if !env.OK || executor.extensionOpen.ExtensionDir != "/target" {
		t.Fatalf("extension open args not parsed: env=%#v args=%#v", env, executor.extensionOpen)
	}
}

func TestProjectCreateDefaultsToSelect(t *testing.T) {
	executor := &fakeExecutor{}
	server := NewServerWithExecutor(executor)
	env := server.CallTool(context.Background(), "lovart_project_create", map[string]any{
		"name": "Campaign draft",
	})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if executor.projectCreate.Name != "Campaign draft" || !executor.projectCreate.Select {
		t.Fatalf("project create args = %#v", executor.projectCreate)
	}

	env = server.CallTool(context.Background(), "lovart_project_create", map[string]any{
		"select": false,
	})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if executor.projectCreate.Select {
		t.Fatalf("project create select override failed: %#v", executor.projectCreate)
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

func TestProjectShowOpenRepairAcceptOptionalProjectID(t *testing.T) {
	executor := &fakeExecutor{}
	server := NewServerWithExecutor(executor)

	env := server.CallTool(context.Background(), "lovart_project_show", map[string]any{})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if executor.projectShow.ProjectID != "" {
		t.Fatalf("project show args = %#v", executor.projectShow)
	}

	env = server.CallTool(context.Background(), "lovart_project_open", map[string]any{"project_id": "proj_123"})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if executor.projectOpen.ProjectID != "proj_123" {
		t.Fatalf("project open args = %#v", executor.projectOpen)
	}

	env = server.CallTool(context.Background(), "lovart_project_repair_canvas", map[string]any{"project_id": "proj_456"})
	if env.OK {
		t.Fatalf("removed repair tool unexpectedly succeeded: %#v", env)
	}
}

func TestProjectRenameRequiresInputs(t *testing.T) {
	executor := &fakeExecutor{}
	server := NewServerWithExecutor(executor)
	env := server.CallTool(context.Background(), "lovart_project_rename", map[string]any{"project_id": "proj_123"})
	if env.OK {
		t.Fatalf("expected missing new_name error")
	}

	env = server.CallTool(context.Background(), "lovart_project_rename", map[string]any{"project_id": "proj_123", "new_name": "New name"})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if executor.projectRename.ProjectID != "proj_123" || executor.projectRename.NewName != "New name" {
		t.Fatalf("project rename args = %#v", executor.projectRename)
	}
}

func TestProjectDeleteRequiresMatchingConfirmProjectID(t *testing.T) {
	executor := &fakeExecutor{}
	server := NewServerWithExecutor(executor)
	env := server.CallTool(context.Background(), "lovart_project_delete", map[string]any{"project_id": "proj_123"})
	if env.OK {
		t.Fatalf("expected missing confirm_project_id error")
	}

	env = server.CallTool(context.Background(), "lovart_project_delete", map[string]any{"project_id": "proj_123", "confirm_project_id": "proj_other"})
	if env.OK {
		t.Fatalf("expected mismatched confirm_project_id error")
	}

	env = server.CallTool(context.Background(), "lovart_project_delete", map[string]any{"project_id": "proj_123", "confirm_project_id": "proj_123"})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if executor.projectDelete.ProjectID != "proj_123" || executor.projectDelete.ConfirmProjectID != "proj_123" {
		t.Fatalf("project delete args = %#v", executor.projectDelete)
	}
}

func TestDownloadToolsParseArgs(t *testing.T) {
	executor := &fakeExecutor{}
	server := NewServerWithExecutor(executor)

	env := server.CallTool(context.Background(), "lovart_task_list", map[string]any{
		"active": true,
	})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if !executor.taskList.Active {
		t.Fatalf("task list args = %#v", executor.taskList)
	}

	env = server.CallTool(context.Background(), "lovart_task_cancel", map[string]any{
		"task_ids": []any{"task-a", "task-b"},
	})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if len(executor.taskCancel.TaskIDs) != 2 || executor.taskCancel.TaskIDs[0] != "task-a" || executor.taskCancel.TaskIDs[1] != "task-b" {
		t.Fatalf("task cancel args = %#v", executor.taskCancel)
	}

	env = server.CallTool(context.Background(), "lovart_task_status", map[string]any{
		"task_id": "task-123",
		"detail":  "full",
	})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if executor.taskStatus.TaskID != "task-123" || executor.taskStatus.Detail != "full" {
		t.Fatalf("task status args = %#v", executor.taskStatus)
	}

	env = server.CallTool(context.Background(), "lovart_task_wait", map[string]any{
		"task_id":         "task-123",
		"detail":          "full",
		"timeout_seconds": 12.0,
		"poll_interval":   0.5,
	})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if executor.taskWait.TaskID != "task-123" || executor.taskWait.Detail != "full" || executor.taskWait.TimeoutSeconds != 12 || executor.taskWait.PollInterval != 0.5 {
		t.Fatalf("task wait args = %#v", executor.taskWait)
	}

	env = server.CallTool(context.Background(), "lovart_task_canvas", map[string]any{
		"task_id":    "task-123",
		"project_id": "project-123",
		"detail":     "full",
	})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if executor.taskCanvas.TaskID != "task-123" || executor.taskCanvas.ProjectID != "project-123" || executor.taskCanvas.Detail != "full" {
		t.Fatalf("task canvas args = %#v", executor.taskCanvas)
	}

	env = server.CallTool(context.Background(), "lovart_task_download", map[string]any{
		"task_id":                "task-123",
		"artifact_index":         2.0,
		"download_dir":           "/tmp/images",
		"download_dir_template":  "{{task.id}}",
		"download_file_template": "img-{{artifact.index:02}}.{{ext}}",
		"overwrite":              true,
		"detail":                 "full",
	})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if executor.taskDownload.TaskID != "task-123" || executor.taskDownload.ArtifactIndex != 2 || executor.taskDownload.DownloadDir != "/tmp/images" || !executor.taskDownload.Overwrite || executor.taskDownload.Detail != "full" {
		t.Fatalf("task download args = %#v", executor.taskDownload)
	}

	env = server.CallTool(context.Background(), "lovart_canvas_artifacts", map[string]any{
		"project_id": "project-123",
		"task_id":    "task-123",
		"limit":      5.0,
		"offset":     2.0,
		"detail":     "full",
	})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if executor.canvasArtifacts.ProjectID != "project-123" || executor.canvasArtifacts.TaskID != "task-123" || executor.canvasArtifacts.Limit != 5 || executor.canvasArtifacts.Offset != 2 || executor.canvasArtifacts.Detail != "full" {
		t.Fatalf("canvas artifacts args = %#v", executor.canvasArtifacts)
	}

	env = server.CallTool(context.Background(), "lovart_canvas_artifact", map[string]any{
		"artifact_id": "shape:one",
		"include_raw": true,
	})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if executor.canvasArtifact.ArtifactID != "shape:one" || !executor.canvasArtifact.IncludeRaw {
		t.Fatalf("canvas artifact args = %#v", executor.canvasArtifact)
	}

	env = server.CallTool(context.Background(), "lovart_canvas_download", map[string]any{
		"project_id":     "project-123",
		"artifact_index": 1.0,
		"original":       true,
		"overwrite":      true,
	})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if executor.canvasDownload.ProjectID != "project-123" || executor.canvasDownload.ArtifactIndex != 1 || !executor.canvasDownload.Original || !executor.canvasDownload.Overwrite {
		t.Fatalf("canvas download args = %#v", executor.canvasDownload)
	}
}

func TestCanvasDownloadRequiresExactlyOneSelector(t *testing.T) {
	server := NewServerWithExecutor(&fakeExecutor{})
	for _, args := range []map[string]any{
		{},
		{"artifact_id": "shape:one", "all": true},
		{"artifact_index": -1.0},
	} {
		env := server.CallTool(context.Background(), "lovart_canvas_download", args)
		if env.OK || env.Error == nil || env.Error.Code != "input_error" {
			t.Fatalf("expected selector input error for %#v, got %#v", args, env)
		}
	}
}

func TestProductionProjectCurrentDoesNotExposeCID(t *testing.T) {
	t.Cleanup(paths.Reset)
	t.Setenv("LOVART_HOME", t.TempDir())
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

func TestProductionAuthLoginReturnsPendingWhenBrowserOpenFails(t *testing.T) {
	t.Cleanup(paths.Reset)
	t.Setenv("LOVART_HOME", t.TempDir())
	paths.Reset()

	openedURL := ""
	originalOpenAuthURL := openAuthURL
	openAuthURL = func(url string) error {
		openedURL = url
		return errors.New("open failed")
	}
	t.Cleanup(func() { openAuthURL = originalOpenAuthURL })
	originalAuthLoginPorts := authLoginPorts
	authLoginPorts = []int{0}
	t.Cleanup(func() { authLoginPorts = originalAuthLoginPorts })

	env := ProductionExecutor{}.AuthLogin(context.Background(), AuthLoginArgs{TimeoutSeconds: 0.2})
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	result, ok := env.Data.(auth.BrowserLoginResult)
	if !ok {
		t.Fatalf("unexpected auth login data type: %T", env.Data)
	}
	if !result.Pending || result.Authenticated || result.LoginURL == "" || result.OpenError != "open failed" {
		t.Fatalf("unexpected auth login result: %#v", result)
	}
	if openedURL == "" || openedURL != result.LoginURL {
		t.Fatalf("opened url = %q, result url = %q", openedURL, result.LoginURL)
	}
	for _, want := range []string{`"pending":true`, `"login_url":"https://www.lovart.ai/?lovart_cli_auth=1`, `"open_error":"open failed"`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("auth login missing %s: %s", want, data)
		}
	}
}

func TestProductionProjectOpenDoesNotExposeCID(t *testing.T) {
	t.Cleanup(paths.Reset)
	t.Setenv("LOVART_HOME", t.TempDir())
	paths.Reset()
	if err := auth.SaveSession(auth.Session{Cookie: "cookie", ProjectID: "project-123", CID: "cid-123"}); err != nil {
		t.Fatal(err)
	}

	openedURL := ""
	originalOpenProjectURL := openProjectURL
	openProjectURL = func(url string) error {
		openedURL = url
		return nil
	}
	t.Cleanup(func() { openProjectURL = originalOpenProjectURL })

	env := ProductionExecutor{}.ProjectOpen(context.Background(), ProjectOpenArgs{})
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if !strings.Contains(string(data), "https://www.lovart.ai/canvas?projectId=project-123") {
		t.Fatalf("project open missing url: %s", data)
	}
	if openedURL != "https://www.lovart.ai/canvas?projectId=project-123" {
		t.Fatalf("opened url = %q", openedURL)
	}
	for _, forbidden := range []string{"cid-123", "cid_present", `"cid"`} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("project open exposed %s: %s", forbidden, data)
		}
	}
}

func TestGenerateDefaultsToAsyncPostprocess(t *testing.T) {
	executor := &fakeExecutor{}
	server := NewServerWithExecutor(executor)
	env := server.CallTool(context.Background(), "lovart_generate", map[string]any{
		"model": "openai/gpt-image-2",
		"body":  map[string]any{"prompt": "test"},
	})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if executor.generate.Wait || executor.generate.Download || executor.generate.Canvas {
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
		t.Fatalf("async normalization failed: %#v", executor.generate)
	}

	executor = &fakeExecutor{}
	server = NewServerWithExecutor(executor)
	env = server.CallTool(context.Background(), "lovart_generate", map[string]any{
		"model":    "openai/gpt-image-2",
		"body":     map[string]any{"prompt": "test"},
		"download": true,
	})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if !executor.generate.Wait || !executor.generate.Download || executor.generate.Canvas {
		t.Fatalf("download normalization failed: %#v", executor.generate)
	}
}

func TestJobsRunArgsExposeUserSurface(t *testing.T) {
	executor := &fakeExecutor{}
	server := NewServerWithExecutor(executor)
	env := server.CallTool(context.Background(), "lovart_jobs_run", map[string]any{
		"jobs_file":               "runs/x/jobs.jsonl",
		"allow_paid":              true,
		"max_total_credits":       12.0,
		"project_id":              "proj_123",
		"download_dir":            "runs/x/images",
		"submit_interval_seconds": 1.5,
		"submit_limit":            7.0,
		"max_active_tasks":        9.0,
	})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if executor.jobsRun.JobsFile != "runs/x/jobs.jsonl" || !executor.jobsRun.AllowPaid || executor.jobsRun.MaxTotalCredits != 12 || executor.jobsRun.ProjectID != "proj_123" || executor.jobsRun.DownloadDir != "runs/x/images" || executor.jobsRun.SubmitIntervalSeconds != 1.5 || executor.jobsRun.SubmitLimit != 7 || executor.jobsRun.MaxActiveTasks != 9 {
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
	if opts.Wait || opts.Download || opts.Canvas {
		t.Fatalf("batch postprocess defaults = %#v", opts)
	}
	if opts.CanvasLayout != "frame" || opts.TimeoutSeconds != MCPWaitTimeoutSeconds || opts.PollInterval != 5 || opts.SubmitIntervalSeconds != 2 || opts.MaxActiveTasks != 10 || opts.Detail != "summary" {
		t.Fatalf("batch execution defaults = %#v", opts)
	}
}

func TestJobsResumeArgsExposeUserSurface(t *testing.T) {
	executor := &fakeExecutor{}
	server := NewServerWithExecutor(executor)
	env := server.CallTool(context.Background(), "lovart_jobs_resume", map[string]any{
		"run_dir":                 "runs/x",
		"allow_paid":              true,
		"max_total_credits":       24.0,
		"download_dir":            "runs/x/images",
		"retry_failed":            true,
		"submit_interval_seconds": 3.0,
		"submit_limit":            5.0,
		"max_active_tasks":        8.0,
	})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if executor.jobsResume.RunDir != "runs/x" || !executor.jobsResume.AllowPaid || executor.jobsResume.MaxTotalCredits != 24 || executor.jobsResume.DownloadDir != "runs/x/images" || !executor.jobsResume.RetryFailed || executor.jobsResume.SubmitIntervalSeconds != 3 || executor.jobsResume.SubmitLimit != 5 || executor.jobsResume.MaxActiveTasks != 8 {
		t.Fatalf("jobs resume args = %#v", executor.jobsResume)
	}
}

func TestJobsFinalizeArgsExposeUserSurface(t *testing.T) {
	executor := &fakeExecutor{}
	server := NewServerWithExecutor(executor)
	env := server.CallTool(context.Background(), "lovart_jobs_finalize", map[string]any{
		"run_dir":       "runs/x",
		"download":      true,
		"canvas":        true,
		"project_id":    "proj_123",
		"download_dir":  "runs/x/images",
		"detail":        "requests",
		"canvas_layout": "plain",
	})
	if !env.OK {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if executor.jobsFinalize.RunDir != "runs/x" || !executor.jobsFinalize.Download || !executor.jobsFinalize.Canvas || executor.jobsFinalize.ProjectID != "proj_123" || executor.jobsFinalize.DownloadDir != "runs/x/images" || executor.jobsFinalize.Detail != "requests" || executor.jobsFinalize.CanvasLayout != "plain" {
		t.Fatalf("jobs finalize args = %#v", executor.jobsFinalize)
	}

	env = server.CallTool(context.Background(), "lovart_jobs_finalize", map[string]any{"run_dir": "runs/x"})
	if env.OK || env.Error == nil || env.Error.Code != "input_error" {
		t.Fatalf("expected finalize action input error, got %#v", env)
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
