package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aporicho/lovart/internal/auth"
	lovarthttp "github.com/aporicho/lovart/internal/http"
	"github.com/aporicho/lovart/internal/paths"
)

type contractServer struct {
	server       *httptest.Server
	quoteCalls   atomic.Int32
	submitCalls  atomic.Int32
	listCalls    atomic.Int32
	fetchCalls   atomic.Int32
	takeSlotCode int
	takeSlotMsg  string
	status       string
	artifacts    []map[string]any
	failure      map[string]any
	runningTasks []any
	submitStatus int
	submitBody   string
}

func newContractServer(t *testing.T) *contractServer {
	t.Helper()
	cs := &contractServer{
		status:     "completed",
		artifacts:  []map[string]any{{"url": "https://cdn.test/out.png", "mime_type": "image/png", "width": 1024}},
		submitBody: `{"code":0,"data":{"generator_task_id":"task-123"}}`,
	}
	cs.server = httptest.NewServer(http.HandlerFunc(cs.handle))
	t.Cleanup(cs.server.Close)
	return cs
}

func (cs *contractServer) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/v1/generator/pricing":
		cs.quoteCalls.Add(1)
		writeJSON(w, map[string]any{
			"code": 0,
			"data": map[string]any{
				"price":   12.5,
				"balance": 40,
				"price_detail": map[string]any{
					"unit_price":     12.5,
					"unit_count":     1,
					"unit_name":      "image",
					"total_price":    12.5,
					"search_key":     "openai/gpt-image-2",
					"generator_name": "openai/gpt-image-2",
				},
			},
		})
	case "/api/canva/agent-cashier/task/take/slot":
		if cs.takeSlotCode != 0 {
			writeJSON(w, map[string]any{"code": cs.takeSlotCode, "message": cs.takeSlotMsg})
			return
		}
		writeJSON(w, map[string]any{"code": 0})
	case "/v1/generator/tasks":
		if r.Method == http.MethodPost {
			cs.submitCalls.Add(1)
			if cs.submitStatus != 0 {
				w.WriteHeader(cs.submitStatus)
			}
			_, _ = w.Write([]byte(cs.submitBody))
			return
		}
		cs.fetchCalls.Add(1)
		data := map[string]any{
			"generator_task_id": r.URL.Query().Get("task_id"),
			"status":            cs.status,
		}
		if len(cs.artifacts) > 0 {
			data["artifacts"] = cs.artifacts
		}
		for k, v := range cs.failure {
			data[k] = v
		}
		writeJSON(w, map[string]any{"code": 0, "data": data})
	case "/v1/tasks/running":
		cs.listCalls.Add(1)
		writeJSON(w, map[string]any{"code": 0, "data": map[string]any{"tasks": cs.runningTasks}})
	default:
		w.WriteHeader(http.StatusNotFound)
		writeJSON(w, map[string]any{"code": 404, "message": r.URL.Path})
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	_ = json.NewEncoder(w).Encode(value)
}

func TestHTTPRemoteContractQuoteEntersStateGateAndNormalizedBody(t *testing.T) {
	cs := newContractServer(t)
	remote := newHTTPContractRemote(t, cs)
	state := newContractRunState(t, 1)

	result, err := DryRunPreparedJobs(context.Background(), remote, state, JobsOptions{AllowPaid: true, MaxTotalCredits: 20})
	if err != nil {
		t.Fatalf("DryRunPreparedJobs: %v", err)
	}
	request := state.Jobs[0].RemoteRequests[0]
	if got := request.Status; got != StatusQuoted {
		t.Fatalf("request status = %q, want %q; errors=%#v quote_calls=%d", got, StatusQuoted, request.Errors, cs.quoteCalls.Load())
	}
	if request.Quote == nil || request.Quote.Price != 12.5 || request.Quote.Balance != 40 {
		t.Fatalf("quote not recorded in state: %#v", request.Quote)
	}
	if got := request.NormalizedBody["size"]; got != "2048*2048" {
		t.Fatalf("normalized size = %#v, want default 2048*2048; body=%#v", got, request.NormalizedBody)
	}
	if result.BatchGate == nil || !result.BatchGate.Allowed || result.BatchGate.TotalCredits != 12.5 {
		t.Fatalf("batch gate not populated from quote: %#v", result.BatchGate)
	}
	if got := cs.quoteCalls.Load(); got != 1 {
		t.Fatalf("quote calls = %d, want 1", got)
	}
}

func TestHTTPRemoteContractSubmitReturnsTaskAndResumeDoesNotResubmit(t *testing.T) {
	cs := newContractServer(t)
	remote := newHTTPContractRemote(t, cs)
	state := newContractRunState(t, 1)
	opts := JobsOptions{AllowPaid: true, MaxTotalCredits: 20, ProjectID: "project-1", CID: "cid-1"}

	result, err := RunPreparedJobs(context.Background(), remote, state, opts)
	if err != nil {
		t.Fatalf("RunPreparedJobs: %v; request_errors=%#v quote_calls=%d", err, state.Jobs[0].RemoteRequests[0].Errors, cs.quoteCalls.Load())
	}
	request := state.Jobs[0].RemoteRequests[0]
	if request.TaskID != "task-123" || request.Status != StatusSubmitted {
		t.Fatalf("submitted request = task_id %q status %q", request.TaskID, request.Status)
	}
	if result.Submission == nil || result.Submission.SubmittedCount != 1 {
		t.Fatalf("submission summary = %#v", result.Submission)
	}

	resumed, err := ResumeJobs(context.Background(), remote, state.RunDir, opts)
	if err != nil {
		t.Fatalf("ResumeJobs: %v", err)
	}
	if got := cs.submitCalls.Load(); got != 1 {
		t.Fatalf("submit calls after resume = %d, want 1", got)
	}
	if resumed.Submission == nil || resumed.Submission.SubmittedCount != 0 {
		t.Fatalf("resume should not resubmit: %#v", resumed.Submission)
	}
}

func TestHTTPRemoteContractStatusRefreshCompletedArtifacts(t *testing.T) {
	cs := newContractServer(t)
	remote := newHTTPContractRemote(t, cs)
	state := newSubmittedContractRunState(t, "task-done")

	result, err := StatusJobs(context.Background(), remote, state.RunDir, JobsOptions{Refresh: true, Detail: "full"})
	if err != nil {
		t.Fatalf("StatusJobs refresh: %v", err)
	}
	request := result.Jobs[0].RemoteRequests[0]
	if request.Status != StatusCompleted {
		t.Fatalf("request status = %q, want completed", request.Status)
	}
	if len(request.Artifacts) != 1 || request.Artifacts[0]["url"] != "https://cdn.test/out.png" {
		t.Fatalf("artifacts not classified from fetch response: %#v", request.Artifacts)
	}
	if got := cs.fetchCalls.Load(); got != 1 {
		t.Fatalf("fetch calls = %d, want 1", got)
	}
}

func TestHTTPRemoteContractStatusRefreshModerationFailure(t *testing.T) {
	cs := newContractServer(t)
	cs.status = "rejected"
	cs.artifacts = nil
	cs.failure = map[string]any{"moderation_result": map[string]any{"message": "blocked by content policy"}}
	remote := newHTTPContractRemote(t, cs)
	state := newSubmittedContractRunState(t, "task-rejected")

	result, err := StatusJobs(context.Background(), remote, state.RunDir, JobsOptions{Refresh: true, Detail: "full"})
	if err != nil {
		t.Fatalf("StatusJobs refresh: %v", err)
	}
	request := result.Jobs[0].RemoteRequests[0]
	if request.Status != StatusFailed {
		t.Fatalf("request status = %q, want failed", request.Status)
	}
	if len(request.Errors) == 0 || request.Errors[len(request.Errors)-1].Code != "content_policy_rejected" {
		t.Fatalf("content policy failure not recorded: %#v", request.Errors)
	}
}

func TestHTTPRemoteContractListRunningTasksDefersAtActiveCap(t *testing.T) {
	cs := newContractServer(t)
	cs.runningTasks = []any{"task-a", map[string]any{"task_id": "task-b"}}
	remote := newHTTPContractRemote(t, cs)
	state := newContractRunState(t, 1)
	opts := JobsOptions{AllowPaid: true, MaxTotalCredits: 20, ProjectID: "project-1", CID: "cid-1", MaxActiveTasks: 2}

	result, err := RunPreparedJobs(context.Background(), remote, state, opts)
	if err != nil {
		t.Fatalf("RunPreparedJobs: %v; request_errors=%#v quote_calls=%d", err, state.Jobs[0].RemoteRequests[0].Errors, cs.quoteCalls.Load())
	}
	if result.Submission == nil || !result.Submission.Deferred || result.Submission.DeferredReason != "active_task_limit_reached" {
		t.Fatalf("submission not deferred at active cap: %#v", result.Submission)
	}
	if got := cs.submitCalls.Load(); got != 0 {
		t.Fatalf("submit calls = %d, want 0 when active cap reached", got)
	}
	if got := cs.listCalls.Load(); got != 1 {
		t.Fatalf("list running calls = %d, want 1", got)
	}
}

func TestHTTPRemoteContract429SubmitDefersAsTransient(t *testing.T) {
	cs := newContractServer(t)
	cs.submitStatus = http.StatusTooManyRequests
	cs.submitBody = `{"code":429,"message":"too many requests"}`
	remote := newHTTPContractRemote(t, cs)
	state := newContractRunState(t, 1)
	opts := JobsOptions{AllowPaid: true, MaxTotalCredits: 20, ProjectID: "project-1", CID: "cid-1"}

	result, err := RunPreparedJobs(context.Background(), remote, state, opts)
	if err != nil {
		t.Fatalf("RunPreparedJobs: %v; request_errors=%#v quote_calls=%d", err, state.Jobs[0].RemoteRequests[0].Errors, cs.quoteCalls.Load())
	}
	request := state.Jobs[0].RemoteRequests[0]
	if request.Status == StatusFailed {
		t.Fatalf("429 should defer, not permanently fail: %#v", request)
	}
	if result.Submission == nil || !result.Submission.Deferred || result.Submission.DeferredReason != "transient_submit_error" {
		t.Fatalf("429 not classified as transient defer: %#v", result.Submission)
	}
}

func TestHTTPRemoteContractConcurrencyLimitDefersAsActiveLimit(t *testing.T) {
	cs := newContractServer(t)
	cs.takeSlotCode = 4001
	cs.takeSlotMsg = "too many running tasks: concurrency limit reached"
	remote := newHTTPContractRemote(t, cs)
	state := newContractRunState(t, 1)
	opts := JobsOptions{AllowPaid: true, MaxTotalCredits: 20, ProjectID: "project-1", CID: "cid-1"}

	result, err := RunPreparedJobs(context.Background(), remote, state, opts)
	if err != nil {
		t.Fatalf("RunPreparedJobs: %v; request_errors=%#v quote_calls=%d", err, state.Jobs[0].RemoteRequests[0].Errors, cs.quoteCalls.Load())
	}
	if result.Submission == nil || !result.Submission.Deferred || result.Submission.DeferredReason != "active_task_limit_reached" {
		t.Fatalf("concurrency limit not classified as active limit defer: %#v", result.Submission)
	}
}

func newHTTPContractRemote(t *testing.T, cs *contractServer) RemoteClient {
	t.Helper()
	client := lovarthttp.NewClient(&auth.Credentials{Cookie: "lovart=test", Token: "token", WebID: "web-123"}, nil)
	client.SetHTTPClient(&http.Client{Transport: contractRoundTripper{handler: http.HandlerFunc(cs.handle)}, Timeout: 5 * time.Second})
	return NewHTTPRemoteClient(client)
}

type contractRoundTripper struct {
	handler http.Handler
}

func (t contractRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	body, err := cloneBody(req.Body)
	if err != nil {
		return nil, err
	}
	clone := req.Clone(req.Context())
	clone.Body = io.NopCloser(bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	t.handler.ServeHTTP(recorder, clone)
	return recorder.Result(), nil
}

func cloneBody(body io.ReadCloser) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	defer body.Close()
	return io.ReadAll(body)
}

func newContractRunState(t *testing.T, count int) *RunState {
	t.Helper()
	setupHTTPContractRegistryMetadata(t)
	dir := t.TempDir()
	jobsFile := filepath.Join(dir, "jobs.jsonl")
	var b strings.Builder
	for i := 1; i <= count; i++ {
		b.WriteString(`{"job_id":"job-`)
		b.WriteString(string(rune('0' + i)))
		b.WriteString(`","model":"openai/gpt-image-2","mode":"auto","body":{"prompt":"a contract test image"}}`)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(jobsFile, []byte(b.String()), 0644); err != nil {
		t.Fatalf("write jobs file: %v", err)
	}
	lines, err := ParseJobsFile(jobsFile)
	if err != nil {
		t.Fatalf("ParseJobsFile: %v", err)
	}
	state, err := NewRunState(jobsFile, lines, JobsOptions{OutDir: filepath.Join(dir, "run"), ProjectID: "project-1", CID: "cid-1"})
	if err != nil {
		t.Fatalf("NewRunState: %v", err)
	}
	return state
}

func setupHTTPContractRegistryMetadata(t *testing.T) {
	t.Helper()
	t.Cleanup(paths.Reset)
	dir := t.TempDir()
	t.Setenv("LOVART_HOME", dir)
	paths.Reset()
	if err := os.MkdirAll(paths.MetadataDir, 0755); err != nil {
		t.Fatal(err)
	}
	list := []byte(`{"items":[{"name":"openai/gpt-image-2","display_name":"GPT Image 2","type":"image"}]}`)
	schema := []byte(`{
  "paths": {
    "/openai/gpt-image-2": {
      "post": {
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/GPTImage2Request"}
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "GPTImage2Request": {
        "required": ["prompt"],
        "properties": {
          "prompt": {"type": "string"},
          "size": {"type": "string", "default": "2048*2048"},
          "quality": {"type": "string", "default": "low"},
          "n": {"type": "integer", "default": 1, "minimum": 1, "maximum": 10}
        }
      }
    }
  }
}`)
	if err := os.WriteFile(paths.GeneratorListFile, list, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.GeneratorSchemaFile, schema, 0644); err != nil {
		t.Fatal(err)
	}
}

func newSubmittedContractRunState(t *testing.T, taskID string) *RunState {
	t.Helper()
	state := newContractRunState(t, 1)
	request := &state.Jobs[0].RemoteRequests[0]
	request.TaskID = taskID
	request.Status = StatusSubmitted
	request.Response = map[string]any{"task_id": taskID, "status": "submitted"}
	RefreshStatuses(state)
	if err := SaveState(state); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	return state
}
