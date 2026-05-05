package jobs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/aporicho/lovart/internal/generation"
	"github.com/aporicho/lovart/internal/pricing"
)

func TestParseJobsFileDefaultsAndConflicts(t *testing.T) {
	setupRuntimeSchema(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.jsonl")
	data := `{"model":"openai/gpt-image-2","body":{"prompt":"test"}}
{"job_id":"explicit","model":"openai/gpt-image-2","mode":"relax","outputs":2,"body":{"prompt":"test"}}
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	lines, err := ParseJobsFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if lines[0].JobID != "line-000001" || lines[0].Mode != "auto" || lines[0].Outputs != 1 {
		t.Fatalf("defaults not applied: %#v", lines[0])
	}
	if !lines[1].OutputsExplicit {
		t.Fatalf("expected explicit outputs: %#v", lines[1])
	}

	conflict := filepath.Join(dir, "conflict.jsonl")
	if err := os.WriteFile(conflict, []byte(`{"model":"openai/gpt-image-2","outputs":1,"body":{"prompt":"test","n":1}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseJobsFile(conflict); err == nil {
		t.Fatal("expected quantity conflict")
	}
}

func TestRunPreparedJobsSavesStateAndResumeDoesNotResubmit(t *testing.T) {
	setupRuntimeSchema(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.jsonl")
	data := `{"job_id":"a","model":"openai/gpt-image-2","outputs":2,"body":{"prompt":"test"}}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	opts := JobsOptions{AllowPaid: true, MaxTotalCredits: 10, ProjectID: "project", CID: "cid", PollInterval: 0.01, TimeoutSeconds: 1}
	state, validation, err := PrepareRun(path, opts)
	if err != nil || validation != nil {
		t.Fatalf("PrepareRun validation=%v err=%v", validation, err)
	}
	remote := &fakeRemote{price: 0}
	result, err := RunPreparedJobs(context.Background(), remote, state, opts)
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.RemoteRequests != 1 || remote.submits != 1 {
		t.Fatalf("run result=%#v submits=%d", result.Summary, remote.submits)
	}
	if _, err := os.Stat(state.StateFile); err != nil {
		t.Fatalf("state not saved: %v", err)
	}

	resumed, err := ResumeJobs(context.Background(), remote, state.RunDir, opts)
	if err != nil {
		t.Fatal(err)
	}
	if remote.submits != 1 {
		t.Fatalf("resume resubmitted task, submits=%d", remote.submits)
	}
	if resumed.Summary.RemoteStatusCounts[StatusSubmitted] != 1 {
		t.Fatalf("resume summary=%#v", resumed.Summary)
	}
}

func TestRunPreparedJobsBlocksPaidWithoutAllowance(t *testing.T) {
	setupRuntimeSchema(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.jsonl")
	if err := os.WriteFile(path, []byte(`{"job_id":"a","model":"openai/gpt-image-2","body":{"prompt":"test"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	state, validation, err := PrepareRun(path, JobsOptions{})
	if err != nil || validation != nil {
		t.Fatalf("PrepareRun validation=%v err=%v", validation, err)
	}
	remote := &fakeRemote{price: 5}
	_, err = RunPreparedJobs(context.Background(), remote, state, JobsOptions{})
	var gateErr *GateError
	if !errors.As(err, &gateErr) {
		t.Fatalf("expected gate error, got %v", err)
	}
	if gateErr.Code != "credit_risk" || remote.submits != 0 {
		t.Fatalf("gate=%#v submits=%d", gateErr, remote.submits)
	}
}

func TestStatusJobsRefreshesActiveTasks(t *testing.T) {
	setupRuntimeSchema(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.jsonl")
	if err := os.WriteFile(path, []byte(`{"job_id":"a","model":"openai/gpt-image-2","body":{"prompt":"test"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	opts := JobsOptions{AllowPaid: true, MaxTotalCredits: 10, ProjectID: "project", CID: "cid"}
	state, validation, err := PrepareRun(path, opts)
	if err != nil || validation != nil {
		t.Fatalf("PrepareRun validation=%v err=%v", validation, err)
	}
	remote := &fakeRemote{price: 0}
	if _, err := RunPreparedJobs(context.Background(), remote, state, opts); err != nil {
		t.Fatal(err)
	}
	remote.taskStatus = "completed"
	result, err := StatusJobs(context.Background(), remote, state.RunDir, JobsOptions{Refresh: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.RemoteStatusCounts[StatusCompleted] != 1 {
		t.Fatalf("status summary=%#v", result.Summary)
	}
}

type fakeRemote struct {
	price      float64
	submits    int
	quotes     int
	taskStatus string
}

func (f *fakeRemote) Quote(ctx context.Context, model string, body map[string]any) (*pricing.QuoteResult, error) {
	f.quotes++
	return &pricing.QuoteResult{Price: f.price, Balance: 100, PriceDetail: pricing.PriceDetail{UnitPrice: f.price}}, nil
}

func (f *fakeRemote) Submit(ctx context.Context, model string, body map[string]any, opts generation.Options) (*generation.SubmitResult, error) {
	f.submits++
	return &generation.SubmitResult{TaskID: "task-1", Status: "submitted"}, nil
}

func (f *fakeRemote) FetchTask(ctx context.Context, taskID string) (map[string]any, error) {
	status := f.taskStatus
	if status == "" {
		status = "running"
	}
	return map[string]any{
		"task_id": taskID,
		"status":  status,
		"artifact_details": []map[string]any{{
			"url": "https://example.test/a.png",
		}},
	}, nil
}
