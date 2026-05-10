package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"sync"
	"time"

	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/pricing"
)

const (
	smokeStatusReady    = "ready"
	smokeStatusDegraded = "degraded"
	smokeStatusFailed   = "failed"

	smokeStepPassed  = "passed"
	smokeStepFailed  = "failed"
	smokeStepSkipped = "skipped"

	defaultSmokeTimeout = 30 * time.Second
)

// SmokeOptions configures the MCP smoke check.
type SmokeOptions struct {
	Model      string
	Body       map[string]any
	Mode       string
	Submit     bool
	AllowPaid  bool
	MaxCredits float64
	Executable string
	Timeout    time.Duration
}

// SmokeReport is the user-facing agent readiness report.
type SmokeReport struct {
	Status             string         `json:"status"`
	Model              string         `json:"model"`
	Mode               string         `json:"mode"`
	SubmitEnabled      bool           `json:"submit_enabled"`
	ExpectedToolCount  int            `json:"expected_tool_count"`
	ToolCount          int            `json:"tool_count,omitempty"`
	ToolNames          []string       `json:"tool_names,omitempty"`
	ProtocolVersion    string         `json:"protocol_version,omitempty"`
	ServerInfo         map[string]any `json:"server_info,omitempty"`
	Steps              []SmokeStep    `json:"steps"`
	RecommendedActions []string       `json:"recommended_actions,omitempty"`
}

// SmokeStep records one contract or tool check.
type SmokeStep struct {
	Name               string         `json:"name"`
	Tool               string         `json:"tool,omitempty"`
	Status             string         `json:"status"`
	ExecutionClass     string         `json:"execution_class,omitempty"`
	NetworkRequired    *bool          `json:"network_required,omitempty"`
	RemoteWrite        *bool          `json:"remote_write,omitempty"`
	Submitted          *bool          `json:"submitted,omitempty"`
	CacheUsed          *bool          `json:"cache_used,omitempty"`
	Data               map[string]any `json:"data,omitempty"`
	ErrorCode          string         `json:"error_code,omitempty"`
	Message            string         `json:"message,omitempty"`
	RecommendedActions []string       `json:"recommended_actions,omitempty"`
}

type smokeRPCClient interface {
	Call(ctx context.Context, method string, params map[string]any) (map[string]any, *rpcErrorBody, error)
	Close() error
}

var newSmokeRPCClient = newProcessSmokeRPCClient

// Smoke runs a real MCP JSON-RPC contract check through stdio.
func Smoke(ctx context.Context, opts SmokeOptions) envelope.Envelope {
	normalized, err := normalizeSmokeOptions(opts)
	if err != nil {
		return envelope.Err(errors.CodeInputError, "invalid smoke options", map[string]any{"error": err.Error()})
	}
	opts = normalized

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	report := SmokeReport{
		Status:            smokeStatusReady,
		Model:             opts.Model,
		Mode:              opts.Mode,
		SubmitEnabled:     opts.Submit,
		ExpectedToolCount: len(ToolNames()),
	}

	client, err := newSmokeRPCClient(ctx, opts.Executable)
	if err != nil {
		report.Status = smokeStatusFailed
		report.Steps = append(report.Steps, failedStep("start_mcp_server", "", errors.CodeInternal, err.Error(), nil))
		report.RecommendedActions = []string{"run `lovart mcp status`", "run `lovart setup`"}
		return failedSmokeEnvelope(report, "MCP smoke failed")
	}
	defer client.Close()

	if !runContractChecks(ctx, client, &report) {
		return failedSmokeEnvelope(report, "MCP smoke failed")
	}

	authEnv := runToolSmokeStep(ctx, client, &report, "auth_status", "lovart_auth_status", nil, true)
	runToolSmokeStep(ctx, client, &report, "setup", "lovart_setup", nil, true)
	runToolSmokeStep(ctx, client, &report, "models", "lovart_models", map[string]any{"refresh": false}, true)
	runToolSmokeStep(ctx, client, &report, "config", "lovart_config", map[string]any{"model": opts.Model}, true)
	projectEnv := runToolSmokeStep(ctx, client, &report, "project_current", "lovart_project_current", nil, false)

	submitted := false
	authAvailable := boolFromEnvelopeData(authEnv, "available")
	projectReady := boolFromEnvelopeData(authEnv, "project_context_ready") || boolFromEnvelopeData(projectEnv, "project_context_ready")
	if !authAvailable {
		appendSkippedStep(&report, "balance", "lovart_balance", "auth is not connected", []string{"run `lovart auth login`"})
		appendSkippedStep(&report, "quote", "lovart_quote", "auth is not connected", []string{"run `lovart auth login`"})
	} else {
		runToolSmokeStep(ctx, client, &report, "balance", "lovart_balance", nil, false)
		runToolSmokeStep(ctx, client, &report, "quote", "lovart_quote", map[string]any{
			"model": opts.Model,
			"body":  opts.Body,
			"mode":  opts.Mode,
		}, false)
	}

	if opts.Submit {
		args := map[string]any{
			"model":       opts.Model,
			"body":        opts.Body,
			"mode":        opts.Mode,
			"allow_paid":  opts.AllowPaid,
			"max_credits": opts.MaxCredits,
			"wait":        false,
			"download":    false,
			"canvas":      false,
		}
		if !projectReady {
			appendSkippedStep(&report, "generate", "lovart_generate", "project context is not ready", []string{"run `lovart project list`", "run `lovart project select <project_id>`"})
		} else {
			generateEnv := runToolSmokeStep(ctx, client, &report, "generate", "lovart_generate", args, true)
			submitted = envelopeSubmitted(generateEnv)
		}
	}

	finalizeSmokeStatus(&report)
	if report.Status == smokeStatusFailed {
		return failedSmokeEnvelope(report, "MCP smoke failed")
	}
	return envelope.OKWithMetadata(report, envelope.ExecutionMetadata{
		ExecutionClass:  executionPreflight,
		NetworkRequired: true,
		RemoteWrite:     submitted,
		Submitted:       boolPtr(submitted),
	})
}

func normalizeSmokeOptions(opts SmokeOptions) (SmokeOptions, error) {
	if opts.Model == "" {
		opts.Model = "openai/gpt-image-2"
	}
	if opts.Body == nil {
		opts.Body = map[string]any{"prompt": "lovart mcp smoke test"}
	}
	if opts.Mode == "" {
		opts.Mode = pricing.ModeRelax
	}
	if _, err := pricing.NormalizeMode(opts.Mode); err != nil {
		return opts, err
	}
	if opts.Submit && (!opts.AllowPaid || opts.MaxCredits <= 0) {
		return opts, fmt.Errorf("--submit requires --allow-paid and --max-credits > 0")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = defaultSmokeTimeout
	}
	if opts.Executable == "" {
		executable, err := os.Executable()
		if err != nil {
			return opts, fmt.Errorf("resolve executable: %w", err)
		}
		opts.Executable = executable
	}
	return opts, nil
}

func runContractChecks(ctx context.Context, client smokeRPCClient, report *SmokeReport) bool {
	initResult, rpcErr, err := client.Call(ctx, "initialize", nil)
	if err != nil || rpcErr != nil {
		appendRPCFailure(report, "initialize", "", rpcErr, err)
		return false
	}
	protocol, _ := initResult["protocolVersion"].(string)
	if protocol != ProtocolVersion {
		report.Steps = append(report.Steps, failedStep("initialize", "", errors.CodeInternal, fmt.Sprintf("protocol version %q does not match %q", protocol, ProtocolVersion), nil))
		report.Status = smokeStatusFailed
		return false
	}
	report.ProtocolVersion = protocol
	if serverInfo, ok := initResult["serverInfo"].(map[string]any); ok {
		report.ServerInfo = serverInfo
	}
	report.Steps = append(report.Steps, SmokeStep{Name: "initialize", Status: smokeStepPassed, Data: map[string]any{"protocol_version": protocol}})

	if _, rpcErr, err := client.Call(ctx, "ping", nil); err != nil || rpcErr != nil {
		appendRPCFailure(report, "ping", "", rpcErr, err)
		return false
	}
	report.Steps = append(report.Steps, SmokeStep{Name: "ping", Status: smokeStepPassed})

	listResult, rpcErr, err := client.Call(ctx, "tools/list", nil)
	if err != nil || rpcErr != nil {
		appendRPCFailure(report, "tools_list", "", rpcErr, err)
		return false
	}
	names, err := toolNamesFromListResult(listResult)
	if err != nil {
		report.Steps = append(report.Steps, failedStep("tools_list", "", errors.CodeInternal, err.Error(), nil))
		report.Status = smokeStatusFailed
		return false
	}
	report.ToolNames = names
	report.ToolCount = len(names)
	missing := missingToolNames(names)
	if len(names) != len(ToolNames()) || len(missing) > 0 {
		report.Steps = append(report.Steps, failedStep("tools_list", "", errors.CodeInternal, "MCP tool list does not match expected Lovart tools", map[string]any{
			"expected_count": len(ToolNames()),
			"actual_count":   len(names),
			"missing_tools":  missing,
		}))
		report.Status = smokeStatusFailed
		return false
	}
	report.Steps = append(report.Steps, SmokeStep{Name: "tools_list", Status: smokeStepPassed, Data: map[string]any{"count": len(names)}})
	return true
}

func runToolSmokeStep(ctx context.Context, client smokeRPCClient, report *SmokeReport, name string, tool string, args map[string]any, critical bool) *envelope.Envelope {
	result, rpcErr, err := client.Call(ctx, "tools/call", map[string]any{
		"name":      tool,
		"arguments": argsOrEmpty(args),
	})
	if err != nil || rpcErr != nil {
		appendRPCFailure(report, name, tool, rpcErr, err)
		if critical {
			report.Status = smokeStatusFailed
		}
		return nil
	}
	env, err := envelopeFromToolCallResult(result)
	if err != nil {
		report.Steps = append(report.Steps, failedStep(name, tool, errors.CodeInternal, err.Error(), nil))
		if critical {
			report.Status = smokeStatusFailed
		}
		return nil
	}
	step := stepFromEnvelope(name, tool, env)
	report.Steps = append(report.Steps, step)
	if !env.OK {
		if critical {
			report.Status = smokeStatusFailed
		} else if report.Status != smokeStatusFailed {
			report.Status = smokeStatusDegraded
		}
	}
	return &env
}

func argsOrEmpty(args map[string]any) map[string]any {
	if args == nil {
		return map[string]any{}
	}
	return args
}

func stepFromEnvelope(name, tool string, env envelope.Envelope) SmokeStep {
	step := SmokeStep{
		Name:            name,
		Tool:            tool,
		Status:          smokeStepPassed,
		ExecutionClass:  env.ExecutionClass,
		NetworkRequired: env.NetworkRequired,
		RemoteWrite:     env.RemoteWrite,
		Submitted:       env.Submitted,
		CacheUsed:       env.CacheUsed,
		Data:            summaryData(name, env.Data),
	}
	if !env.OK {
		step.Status = smokeStepFailed
		if env.Error != nil {
			step.ErrorCode = env.Error.Code
			step.Message = env.Error.Message
			step.RecommendedActions = recommendedActions(env.Error.Details)
		}
	}
	return step
}

func appendSkippedStep(report *SmokeReport, name, tool, message string, actions []string) {
	report.Steps = append(report.Steps, SmokeStep{
		Name:               name,
		Tool:               tool,
		Status:             smokeStepSkipped,
		Message:            message,
		RecommendedActions: actions,
	})
	if report.Status != smokeStatusFailed {
		report.Status = smokeStatusDegraded
	}
}

func appendRPCFailure(report *SmokeReport, name, tool string, rpcErr *rpcErrorBody, err error) {
	message := ""
	if err != nil {
		message = err.Error()
	}
	if rpcErr != nil {
		message = rpcErr.Message
	}
	report.Steps = append(report.Steps, failedStep(name, tool, errors.CodeInternal, message, nil))
	report.Status = smokeStatusFailed
}

func failedStep(name, tool, code, message string, data map[string]any) SmokeStep {
	return SmokeStep{
		Name:      name,
		Tool:      tool,
		Status:    smokeStepFailed,
		ErrorCode: code,
		Message:   message,
		Data:      data,
	}
}

func finalizeSmokeStatus(report *SmokeReport) {
	if report.Status == smokeStatusFailed {
		report.RecommendedActions = appendUnique(report.RecommendedActions, "run `lovart mcp status`", "run `lovart setup`")
		return
	}
	for _, step := range report.Steps {
		if step.Status == smokeStepSkipped || step.Status == smokeStepFailed {
			report.Status = smokeStatusDegraded
			report.RecommendedActions = appendUnique(report.RecommendedActions, step.RecommendedActions...)
		}
	}
	if report.Status == "" {
		report.Status = smokeStatusReady
	}
}

func failedSmokeEnvelope(report SmokeReport, message string) envelope.Envelope {
	return envelope.Err(errors.CodeInternal, message, map[string]any{
		"report": report,
	})
}

func toolNamesFromListResult(result map[string]any) ([]string, error) {
	rawTools, ok := result["tools"]
	if !ok {
		return nil, fmt.Errorf("tools/list result missing tools")
	}
	data, err := json.Marshal(rawTools)
	if err != nil {
		return nil, err
	}
	var tools []Tool
	if err := json.Unmarshal(data, &tools); err != nil {
		return nil, fmt.Errorf("parse tools/list result: %w", err)
	}
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		if tool.Name != "" {
			names = append(names, tool.Name)
		}
	}
	sort.Strings(names)
	return names, nil
}

func missingToolNames(actual []string) []string {
	seen := map[string]bool{}
	for _, name := range actual {
		seen[name] = true
	}
	var missing []string
	for _, name := range ToolNames() {
		if !seen[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

func envelopeFromToolCallResult(result map[string]any) (envelope.Envelope, error) {
	var normalized struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	data, err := json.Marshal(result)
	if err != nil {
		return envelope.Envelope{}, err
	}
	if err := json.Unmarshal(data, &normalized); err != nil {
		return envelope.Envelope{}, fmt.Errorf("parse tool result: %w", err)
	}
	if len(normalized.Content) == 0 || normalized.Content[0].Text == "" {
		return envelope.Envelope{}, fmt.Errorf("tool result did not contain envelope text")
	}
	var env envelope.Envelope
	if err := json.Unmarshal([]byte(normalized.Content[0].Text), &env); err != nil {
		return envelope.Envelope{}, fmt.Errorf("parse envelope: %w", err)
	}
	return env, nil
}

func summaryData(stepName string, data any) map[string]any {
	value := anyMap(data)
	if value == nil {
		return nil
	}
	switch stepName {
	case "auth_status":
		return pickKeys(value, "available", "source", "credential_path", "fields", "project_id_present", "project_context_ready", "updated_at")
	case "setup":
		return pickKeys(value, "status", "ready", "version")
	case "models":
		return pickKeys(value, "source", "count")
	case "config":
		summary := pickKeys(value, "model")
		if summary == nil {
			summary = map[string]any{}
		}
		if fields, ok := value["fields"].([]any); ok {
			summary["field_count"] = len(fields)
		}
		return summary
	case "project_current":
		return pickKeys(value, "project_id", "project_context_ready")
	case "balance":
		return pickKeys(value, "balance")
	case "quote":
		return pickKeys(value, "price", "balance", "pricing_context")
	case "generate":
		return pickKeys(value, "submitted", "task_id", "status", "project_id")
	default:
		return value
	}
}

func anyMap(data any) map[string]any {
	if data == nil {
		return nil
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil
	}
	return value
}

func pickKeys(value map[string]any, keys ...string) map[string]any {
	result := map[string]any{}
	for _, key := range keys {
		if v, ok := value[key]; ok {
			result[key] = v
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func boolFromEnvelopeData(env *envelope.Envelope, key string) bool {
	if env == nil || !env.OK {
		return false
	}
	value := anyMap(env.Data)
	if value == nil {
		return false
	}
	boolean, _ := value[key].(bool)
	return boolean
}

func envelopeSubmitted(env *envelope.Envelope) bool {
	if env == nil || !env.OK {
		return false
	}
	if env.Submitted != nil && *env.Submitted {
		return true
	}
	data := anyMap(env.Data)
	if data == nil {
		return false
	}
	submitted, _ := data["submitted"].(bool)
	return submitted
}

func recommendedActions(details map[string]any) []string {
	if details == nil {
		return nil
	}
	raw, ok := details["recommended_actions"]
	if !ok {
		return nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var actions []string
	if err := json.Unmarshal(data, &actions); err != nil {
		return nil
	}
	return actions
}

func appendUnique(values []string, additions ...string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		if value != "" {
			seen[value] = true
		}
	}
	for _, value := range additions {
		if value != "" && !seen[value] {
			values = append(values, value)
			seen[value] = true
		}
	}
	return values
}

type processSmokeRPCClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *json.Decoder
	stderr *bytes.Buffer
	mu     sync.Mutex
	nextID int
}

func newProcessSmokeRPCClient(ctx context.Context, executable string) (smokeRPCClient, error) {
	cmd := exec.CommandContext(ctx, executable, "mcp")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &processSmokeRPCClient{
		cmd:    cmd,
		stdin:  stdin,
		stdout: json.NewDecoder(stdoutPipe),
		stderr: stderr,
		nextID: 1,
	}, nil
}

func (c *processSmokeRPCClient) Call(ctx context.Context, method string, params map[string]any) (map[string]any, *rpcErrorBody, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID
	c.nextID++
	request := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		request["params"] = params
	}
	data, err := json.Marshal(request)
	if err != nil {
		return nil, nil, err
	}
	if _, err := c.stdin.Write(append(data, '\n')); err != nil {
		return nil, nil, c.withStderr(err)
	}

	type decodeResult struct {
		response rpcResponse
		err      error
	}
	done := make(chan decodeResult, 1)
	go func() {
		var response rpcResponse
		err := c.stdout.Decode(&response)
		done <- decodeResult{response: response, err: err}
	}()

	select {
	case <-ctx.Done():
		_ = c.cmd.Process.Kill()
		return nil, nil, ctx.Err()
	case result := <-done:
		if result.err != nil {
			return nil, nil, c.withStderr(result.err)
		}
		if result.response.Error != nil {
			return nil, result.response.Error, nil
		}
		if result.response.Result == nil {
			return nil, nil, c.withStderr(fmt.Errorf("missing JSON-RPC result"))
		}
		resultMap, ok := result.response.Result.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("JSON-RPC result must be an object")
		}
		return resultMap, nil, nil
	}
}

func (c *processSmokeRPCClient) Close() error {
	_ = c.stdin.Close()
	return c.cmd.Wait()
}

func (c *processSmokeRPCClient) withStderr(err error) error {
	if c.stderr == nil || c.stderr.Len() == 0 {
		return err
	}
	return fmt.Errorf("%w: %s", err, c.stderr.String())
}
