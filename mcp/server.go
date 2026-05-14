package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/jobs"
	"github.com/aporicho/lovart/internal/version"
)

// Server handles JSON-RPC MCP messages.
type Server struct {
	executor Executor
}

// NewServer returns a production MCP server.
func NewServer() *Server {
	return NewServerWithExecutor(ProductionExecutor{})
}

// NewServerWithExecutor returns a server backed by executor.
func NewServerWithExecutor(executor Executor) *Server {
	return &Server{executor: executor}
}

type rpcResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id,omitempty"`
	Result  any           `json:"result,omitempty"`
	Error   *rpcErrorBody `json:"error,omitempty"`
}

type rpcErrorBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Run serves newline-delimited JSON-RPC messages from r to w.
func (s *Server) Run(ctx context.Context, r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		response := s.HandleLine(ctx, []byte(line))
		if response == nil {
			continue
		}
		if err := encoder.Encode(response); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// HandleLine parses and handles one JSON-RPC message.
func (s *Server) HandleLine(ctx context.Context, data []byte) *rpcResponse {
	var message map[string]any
	if err := json.Unmarshal(data, &message); err != nil {
		return rpcError(nil, -32700, fmt.Sprintf("parse error: %v", err))
	}
	return s.HandleMessage(ctx, message)
}

// HandleMessage handles one decoded JSON-RPC message.
func (s *Server) HandleMessage(ctx context.Context, message map[string]any) *rpcResponse {
	id, hasID := message["id"]
	if !hasID {
		return nil
	}
	method, _ := message["method"].(string)
	switch method {
	case "initialize":
		return rpcResult(id, map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities": map[string]any{
				"tools": map[string]any{"listChanged": false},
			},
			"serverInfo": map[string]any{
				"name":    ServerName,
				"version": version.Version,
			},
		})
	case "ping":
		return rpcResult(id, map[string]any{})
	case "tools/list":
		return rpcResult(id, map[string]any{"tools": Tools()})
	case "tools/call":
		params, _ := message["params"].(map[string]any)
		name, _ := params["name"].(string)
		arguments := map[string]any{}
		if raw, ok := params["arguments"]; ok {
			var valid bool
			arguments, valid = raw.(map[string]any)
			if !valid {
				return rpcError(id, -32602, "arguments must be an object")
			}
		}
		return rpcResult(id, toolResult(s.CallTool(ctx, name, arguments)))
	default:
		return rpcError(id, -32601, "unknown method: "+method)
	}
}

// CallTool validates arguments, applies MCP defaults, and invokes executor.
func (s *Server) CallTool(ctx context.Context, name string, args map[string]any) envelope.Envelope {
	switch name {
	case "lovart_auth_status":
		return s.executor.AuthStatus(ctx)
	case "lovart_auth_login":
		timeout := numberArg(args, "timeout_seconds", 300)
		if timeout <= 0 {
			return inputErr(fmt.Errorf("timeout_seconds must be positive"))
		}
		return s.executor.AuthLogin(ctx, AuthLoginArgs{TimeoutSeconds: timeout})
	case "lovart_extension_status":
		return s.executor.ExtensionStatus(ctx, ExtensionStatusArgs{
			ExtensionDir: stringArg(args, "extension_dir", ""),
		})
	case "lovart_extension_install":
		return s.executor.ExtensionInstall(ctx, ExtensionInstallArgs{
			SourceDir:    stringArg(args, "source_dir", ""),
			ExtensionDir: stringArg(args, "extension_dir", ""),
			DryRun:       boolArg(args, "dry_run", false),
			Open:         boolArg(args, "open", true),
		})
	case "lovart_extension_open":
		return s.executor.ExtensionOpen(ctx, ExtensionOpenArgs{
			ExtensionDir: stringArg(args, "extension_dir", ""),
		})
	case "lovart_setup":
		return s.executor.Setup(ctx, SetupArgs{})
	case "lovart_models":
		return s.executor.Models(ctx, ModelsArgs{Refresh: boolArg(args, "refresh", false)})
	case "lovart_config":
		model, err := requiredString(args, "model")
		if err != nil {
			return inputErr(err)
		}
		return s.executor.Config(ctx, ConfigArgs{
			Model:      model,
			IncludeAll: boolArg(args, "include_all", false),
		})
	case "lovart_balance":
		return s.executor.Balance(ctx)
	case "lovart_project_current":
		return s.executor.ProjectCurrent(ctx)
	case "lovart_project_list":
		return s.executor.ProjectList(ctx)
	case "lovart_project_create":
		return s.executor.ProjectCreate(ctx, ProjectCreateArgs{
			Name:   stringArg(args, "name", ""),
			Select: boolArg(args, "select", true),
		})
	case "lovart_project_select":
		projectID, err := requiredString(args, "project_id")
		if err != nil {
			return inputErr(err)
		}
		return s.executor.ProjectSelect(ctx, ProjectSelectArgs{ProjectID: projectID})
	case "lovart_project_show":
		return s.executor.ProjectShow(ctx, ProjectShowArgs{
			ProjectID: stringArg(args, "project_id", ""),
		})
	case "lovart_project_open":
		return s.executor.ProjectOpen(ctx, ProjectOpenArgs{
			ProjectID: stringArg(args, "project_id", ""),
		})
	case "lovart_project_rename":
		projectID, err := requiredString(args, "project_id")
		if err != nil {
			return inputErr(err)
		}
		newName, err := requiredString(args, "new_name")
		if err != nil {
			return inputErr(err)
		}
		return s.executor.ProjectRename(ctx, ProjectRenameArgs{ProjectID: projectID, NewName: newName})
	case "lovart_project_delete":
		projectID, err := requiredString(args, "project_id")
		if err != nil {
			return inputErr(err)
		}
		confirmProjectID, err := requiredString(args, "confirm_project_id")
		if err != nil {
			return inputErr(err)
		}
		if confirmProjectID != projectID {
			return inputErr(fmt.Errorf("confirm_project_id must match project_id"))
		}
		return s.executor.ProjectDelete(ctx, ProjectDeleteArgs{ProjectID: projectID, ConfirmProjectID: confirmProjectID})
	case "lovart_task_list":
		listArgs, err := parseTaskListArgs(args)
		if err != nil {
			return inputErr(err)
		}
		return s.executor.TaskList(ctx, listArgs)
	case "lovart_task_cancel":
		cancelArgs, err := parseTaskCancelArgs(args)
		if err != nil {
			return inputErr(err)
		}
		return s.executor.TaskCancel(ctx, cancelArgs)
	case "lovart_task_status":
		statusArgs, err := parseTaskStatusArgs(args)
		if err != nil {
			return inputErr(err)
		}
		return s.executor.TaskStatus(ctx, statusArgs)
	case "lovart_task_wait":
		waitArgs, err := parseTaskWaitArgs(args)
		if err != nil {
			return inputErr(err)
		}
		return s.executor.TaskWait(ctx, waitArgs)
	case "lovart_task_canvas":
		canvasArgs, err := parseTaskCanvasArgs(args)
		if err != nil {
			return inputErr(err)
		}
		return s.executor.TaskCanvas(ctx, canvasArgs)
	case "lovart_task_download":
		downloadArgs, err := parseTaskDownloadArgs(args)
		if err != nil {
			return inputErr(err)
		}
		return s.executor.TaskDownload(ctx, downloadArgs)
	case "lovart_canvas_artifacts":
		artifactArgs, err := parseCanvasArtifactsArgs(args)
		if err != nil {
			return inputErr(err)
		}
		return s.executor.CanvasArtifacts(ctx, artifactArgs)
	case "lovart_canvas_artifact":
		artifactArgs, err := parseCanvasArtifactArgs(args)
		if err != nil {
			return inputErr(err)
		}
		return s.executor.CanvasArtifact(ctx, artifactArgs)
	case "lovart_canvas_download":
		downloadArgs, err := parseCanvasDownloadArgs(args)
		if err != nil {
			return inputErr(err)
		}
		return s.executor.CanvasDownload(ctx, downloadArgs)
	case "lovart_quote":
		quoteArgs, err := parseQuoteArgs(args)
		if err != nil {
			return inputErr(err)
		}
		return s.executor.Quote(ctx, quoteArgs)
	case "lovart_generate":
		genArgs, err := parseGenerateArgs(args)
		if err != nil {
			return inputErr(err)
		}
		normalizePostprocess(&genArgs.Wait, &genArgs.Download, &genArgs.Canvas)
		return s.executor.Generate(ctx, genArgs)
	case "lovart_jobs_run":
		runArgs, err := parseJobsRunArgs(args)
		if err != nil {
			return inputErr(err)
		}
		return s.executor.JobsRun(ctx, runArgs)
	case "lovart_jobs_status":
		statusArgs, err := parseJobsStatusArgs(args)
		if err != nil {
			return inputErr(err)
		}
		return s.executor.JobsStatus(ctx, statusArgs)
	case "lovart_jobs_resume":
		resumeArgs, err := parseJobsResumeArgs(args)
		if err != nil {
			return inputErr(err)
		}
		return s.executor.JobsResume(ctx, resumeArgs)
	case "lovart_jobs_finalize":
		finalizeArgs, err := parseJobsFinalizeArgs(args)
		if err != nil {
			return inputErr(err)
		}
		return s.executor.JobsFinalize(ctx, finalizeArgs)
	default:
		return envelope.Err(errors.CodeInputError, "unknown MCP tool", map[string]any{"tool": name})
	}
}

func parseQuoteArgs(args map[string]any) (QuoteArgs, error) {
	model, err := requiredString(args, "model")
	if err != nil {
		return QuoteArgs{}, err
	}
	body, err := requiredBody(args)
	if err != nil {
		return QuoteArgs{}, err
	}
	mode, err := modeArg(args, "mode", "auto")
	if err != nil {
		return QuoteArgs{}, err
	}
	return QuoteArgs{Model: model, Body: body, Mode: mode}, nil
}

func parseTaskStatusArgs(args map[string]any) (TaskStatusArgs, error) {
	taskID, err := requiredString(args, "task_id")
	if err != nil {
		return TaskStatusArgs{}, err
	}
	detail, err := artifactDetailArg(args, "detail", "summary")
	if err != nil {
		return TaskStatusArgs{}, err
	}
	return TaskStatusArgs{TaskID: taskID, Detail: detail}, nil
}

func parseTaskWaitArgs(args map[string]any) (TaskWaitArgs, error) {
	taskID, err := requiredString(args, "task_id")
	if err != nil {
		return TaskWaitArgs{}, err
	}
	detail, err := artifactDetailArg(args, "detail", "summary")
	if err != nil {
		return TaskWaitArgs{}, err
	}
	timeoutSeconds := numberArg(args, "timeout_seconds", MCPWaitTimeoutSeconds)
	if timeoutSeconds <= 0 {
		return TaskWaitArgs{}, fmt.Errorf("timeout_seconds must be positive")
	}
	pollInterval := numberArg(args, "poll_interval", 2)
	if pollInterval <= 0 {
		return TaskWaitArgs{}, fmt.Errorf("poll_interval must be positive")
	}
	return TaskWaitArgs{TaskID: taskID, Detail: detail, TimeoutSeconds: timeoutSeconds, PollInterval: pollInterval}, nil
}

func parseTaskCanvasArgs(args map[string]any) (TaskCanvasArgs, error) {
	taskID, err := requiredString(args, "task_id")
	if err != nil {
		return TaskCanvasArgs{}, err
	}
	detail, err := artifactDetailArg(args, "detail", "summary")
	if err != nil {
		return TaskCanvasArgs{}, err
	}
	return TaskCanvasArgs{
		TaskID:    taskID,
		ProjectID: stringArg(args, "project_id", ""),
		Detail:    detail,
	}, nil
}

func parseTaskDownloadArgs(args map[string]any) (TaskDownloadArgs, error) {
	taskID, err := requiredString(args, "task_id")
	if err != nil {
		return TaskDownloadArgs{}, err
	}
	detail, err := artifactDetailArg(args, "detail", "summary")
	if err != nil {
		return TaskDownloadArgs{}, err
	}
	return TaskDownloadArgs{
		TaskID:               taskID,
		ArtifactIndex:        intArg(args, "artifact_index", 0),
		DownloadDir:          stringArg(args, "download_dir", ""),
		DownloadDirTemplate:  stringArg(args, "download_dir_template", ""),
		DownloadFileTemplate: stringArg(args, "download_file_template", ""),
		Overwrite:            boolArg(args, "overwrite", false),
		Detail:               detail,
	}, nil
}

func parseTaskListArgs(args map[string]any) (TaskListArgs, error) {
	return TaskListArgs{Active: boolArg(args, "active", true)}, nil
}

func parseTaskCancelArgs(args map[string]any) (TaskCancelArgs, error) {
	taskIDs, err := requiredStringSlice(args, "task_ids")
	if err != nil {
		return TaskCancelArgs{}, err
	}
	return TaskCancelArgs{TaskIDs: taskIDs}, nil
}

func parseCanvasArtifactsArgs(args map[string]any) (CanvasArtifactsArgs, error) {
	detail, err := artifactDetailArg(args, "detail", "summary")
	if err != nil {
		return CanvasArtifactsArgs{}, err
	}
	return CanvasArtifactsArgs{
		ProjectID: stringArg(args, "project_id", ""),
		TaskID:    stringArg(args, "task_id", ""),
		Limit:     intArg(args, "limit", 0),
		Offset:    intArg(args, "offset", 0),
		Detail:    detail,
	}, nil
}

func parseCanvasArtifactArgs(args map[string]any) (CanvasArtifactArgs, error) {
	artifactID, err := requiredString(args, "artifact_id")
	if err != nil {
		return CanvasArtifactArgs{}, err
	}
	return CanvasArtifactArgs{
		ProjectID:  stringArg(args, "project_id", ""),
		ArtifactID: artifactID,
		IncludeRaw: boolArg(args, "include_raw", false),
	}, nil
}

func parseCanvasDownloadArgs(args map[string]any) (CanvasDownloadArgs, error) {
	parsed := CanvasDownloadArgs{
		ProjectID:            stringArg(args, "project_id", ""),
		ArtifactID:           stringArg(args, "artifact_id", ""),
		ArtifactIndex:        intArg(args, "artifact_index", 0),
		TaskID:               stringArg(args, "task_id", ""),
		All:                  boolArg(args, "all", false),
		Original:             boolArg(args, "original", false),
		DownloadDir:          stringArg(args, "download_dir", ""),
		DownloadDirTemplate:  stringArg(args, "download_dir_template", ""),
		DownloadFileTemplate: stringArg(args, "download_file_template", ""),
		Overwrite:            boolArg(args, "overwrite", false),
	}
	if err := validateCanvasDownloadArgs(parsed); err != nil {
		return CanvasDownloadArgs{}, err
	}
	return parsed, nil
}

func parseGenerateArgs(args map[string]any) (GenerateArgs, error) {
	model, err := requiredString(args, "model")
	if err != nil {
		return GenerateArgs{}, err
	}
	body, err := requiredBody(args)
	if err != nil {
		return GenerateArgs{}, err
	}
	mode, err := modeArg(args, "mode", "auto")
	if err != nil {
		return GenerateArgs{}, err
	}
	wait := boolArg(args, "wait", false)
	download := boolArg(args, "download", false)
	canvas := boolArg(args, "canvas", false)
	return GenerateArgs{
		Model:                model,
		Body:                 body,
		Mode:                 mode,
		AllowPaid:            boolArg(args, "allow_paid", false),
		MaxCredits:           numberArg(args, "max_credits", 0),
		ProjectID:            stringArg(args, "project_id", ""),
		Wait:                 wait,
		Download:             download,
		Canvas:               canvas,
		DownloadDir:          stringArg(args, "download_dir", ""),
		DownloadDirTemplate:  stringArg(args, "download_dir_template", ""),
		DownloadFileTemplate: stringArg(args, "download_file_template", ""),
	}, nil
}

func parseJobsRunArgs(args map[string]any) (JobsRunArgs, error) {
	jobsFile, err := requiredString(args, "jobs_file")
	if err != nil {
		return JobsRunArgs{}, err
	}
	return JobsRunArgs{
		JobsFile:              jobsFile,
		AllowPaid:             boolArg(args, "allow_paid", false),
		MaxTotalCredits:       numberArg(args, "max_total_credits", 0),
		ProjectID:             stringArg(args, "project_id", ""),
		DownloadDir:           stringArg(args, "download_dir", ""),
		SubmitIntervalSeconds: numberArg(args, "submit_interval_seconds", 2),
		SubmitLimit:           intArg(args, "submit_limit", 0),
		MaxActiveTasks:        intArg(args, "max_active_tasks", 10),
	}, nil
}

func parseJobsStatusArgs(args map[string]any) (JobsStatusArgs, error) {
	runDir, err := requiredString(args, "run_dir")
	if err != nil {
		return JobsStatusArgs{}, err
	}
	detail, err := detailArg(args, "detail", "summary")
	if err != nil {
		return JobsStatusArgs{}, err
	}
	return JobsStatusArgs{
		RunDir:  runDir,
		Detail:  detail,
		Refresh: boolArg(args, "refresh", false),
	}, nil
}

func parseJobsResumeArgs(args map[string]any) (JobsResumeArgs, error) {
	runDir, err := requiredString(args, "run_dir")
	if err != nil {
		return JobsResumeArgs{}, err
	}
	return JobsResumeArgs{
		RunDir:                runDir,
		AllowPaid:             boolArg(args, "allow_paid", false),
		MaxTotalCredits:       numberArg(args, "max_total_credits", 0),
		DownloadDir:           stringArg(args, "download_dir", ""),
		RetryFailed:           boolArg(args, "retry_failed", false),
		SubmitIntervalSeconds: numberArg(args, "submit_interval_seconds", 2),
		SubmitLimit:           intArg(args, "submit_limit", 0),
		MaxActiveTasks:        intArg(args, "max_active_tasks", 10),
	}, nil
}

func parseJobsFinalizeArgs(args map[string]any) (JobsFinalizeArgs, error) {
	runDir, err := requiredString(args, "run_dir")
	if err != nil {
		return JobsFinalizeArgs{}, err
	}
	detail, err := detailArg(args, "detail", "summary")
	if err != nil {
		return JobsFinalizeArgs{}, err
	}
	parsed := JobsFinalizeArgs{
		RunDir:       runDir,
		Download:     boolArg(args, "download", false),
		Canvas:       boolArg(args, "canvas", false),
		ProjectID:    stringArg(args, "project_id", ""),
		DownloadDir:  stringArg(args, "download_dir", ""),
		Detail:       detail,
		CanvasLayout: stringArg(args, "canvas_layout", jobs.CanvasLayoutFrame),
	}
	if !parsed.Download && !parsed.Canvas {
		return JobsFinalizeArgs{}, fmt.Errorf("choose at least one finalization action: download or canvas")
	}
	if parsed.CanvasLayout != jobs.CanvasLayoutFrame && parsed.CanvasLayout != jobs.CanvasLayoutPlain {
		return JobsFinalizeArgs{}, fmt.Errorf("canvas_layout must be one of %s, %s", jobs.CanvasLayoutFrame, jobs.CanvasLayoutPlain)
	}
	return parsed, nil
}

func requiredString(args map[string]any, key string) (string, error) {
	value := stringArg(args, key, "")
	if value == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

func requiredBody(args map[string]any) (map[string]any, error) {
	raw, ok := args["body"]
	if !ok {
		return nil, fmt.Errorf("body is required")
	}
	body, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("body must be an object")
	}
	return body, nil
}

func requiredStringSlice(args map[string]any, key string) ([]string, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return nil, fmt.Errorf("%s is required", key)
	}
	values, ok := raw.([]any)
	if !ok {
		if typed, ok := raw.([]string); ok {
			values = make([]any, 0, len(typed))
			for _, value := range typed {
				values = append(values, value)
			}
		} else {
			return nil, fmt.Errorf("%s must be an array of strings", key)
		}
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%s must be an array of strings", key)
		}
		if text != "" {
			out = append(out, text)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s must contain at least one task id", key)
	}
	return out, nil
}

func stringArg(args map[string]any, key string, fallback string) string {
	value, ok := args[key]
	if !ok || value == nil {
		return fallback
	}
	text, ok := value.(string)
	if !ok {
		return fallback
	}
	return text
}

func boolArg(args map[string]any, key string, fallback bool) bool {
	value, ok := args[key]
	if !ok || value == nil {
		return fallback
	}
	boolean, ok := value.(bool)
	if !ok {
		return fallback
	}
	return boolean
}

func numberArg(args map[string]any, key string, fallback float64) float64 {
	value, ok := args[key]
	if !ok || value == nil {
		return fallback
	}
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return fallback
	}
}

func intArg(args map[string]any, key string, fallback int) int {
	return int(numberArg(args, key, float64(fallback)))
}

func modeArg(args map[string]any, key string, fallback string) (string, error) {
	mode := stringArg(args, key, fallback)
	switch mode {
	case "auto", "fast", "relax":
		return mode, nil
	default:
		return "", fmt.Errorf("%s must be one of auto, fast, relax", key)
	}
}

func artifactDetailArg(args map[string]any, key string, fallback string) (string, error) {
	detail := stringArg(args, key, fallback)
	switch detail {
	case "summary", "full":
		return detail, nil
	default:
		return "", fmt.Errorf("%s must be one of summary, full", key)
	}
}

func detailArg(args map[string]any, key string, fallback string) (string, error) {
	detail := stringArg(args, key, fallback)
	switch detail {
	case "summary", "requests", "full":
		return detail, nil
	default:
		return "", fmt.Errorf("%s must be one of summary, requests, full", key)
	}
}

func validateCanvasDownloadArgs(args CanvasDownloadArgs) error {
	count := 0
	if args.ArtifactID != "" {
		count++
	}
	if args.ArtifactIndex != 0 {
		count++
	}
	if args.TaskID != "" {
		count++
	}
	if args.All {
		count++
	}
	if count != 1 {
		return fmt.Errorf("choose exactly one canvas selector: artifact_id, artifact_index, task_id, or all")
	}
	if args.ArtifactIndex < 0 {
		return fmt.Errorf("artifact_index must be greater than zero")
	}
	return nil
}

func normalizePostprocess(wait, download, canvas *bool) {
	if *download || *canvas {
		*wait = true
	}
	if !*wait {
		*download = false
		*canvas = false
	}
}

func inputErr(err error) envelope.Envelope {
	return envelope.Err(errors.CodeInputError, err.Error(), nil)
}

func toolResult(env envelope.Envelope) map[string]any {
	text, _ := json.Marshal(env)
	return map[string]any{
		"content": []map[string]string{{
			"type": "text",
			"text": string(text),
		}},
		"isError": !env.OK,
	}
}

func rpcResult(id any, result any) *rpcResponse {
	return &rpcResponse{JSONRPC: "2.0", ID: id, Result: result}
}

func rpcError(id any, code int, message string) *rpcResponse {
	return &rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcErrorBody{Code: code, Message: message}}
}
