package jobs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestRunPreparedJobsHonorsSubmitLimit(t *testing.T) {
	setupRuntimeSchema(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.jsonl")
	data := `{"job_id":"a","model":"openai/gpt-image-2","body":{"prompt":"test a"}}
{"job_id":"b","model":"openai/gpt-image-2","body":{"prompt":"test b"}}
{"job_id":"c","model":"openai/gpt-image-2","body":{"prompt":"test c"}}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	opts := JobsOptions{AllowPaid: true, MaxTotalCredits: 10, ProjectID: "project", CID: "cid", SubmitLimit: 2}
	state, validation, err := PrepareRun(path, opts)
	if err != nil || validation != nil {
		t.Fatalf("PrepareRun validation=%v err=%v", validation, err)
	}
	remote := &fakeRemote{price: 0}
	result, err := RunPreparedJobs(context.Background(), remote, state, opts)
	if err != nil {
		t.Fatal(err)
	}
	if remote.submits != 2 {
		t.Fatalf("submits=%d, want 2", remote.submits)
	}
	if result.Submission == nil || !result.Submission.Deferred || result.Submission.DeferredReason != "submit_limit_reached" || result.Submission.DeferredCount != 1 {
		t.Fatalf("submission summary=%#v", result.Submission)
	}
	if result.Summary.RemoteStatusCounts[StatusSubmitted] != 2 || result.Summary.RemoteStatusCounts[StatusQuoted] != 1 {
		t.Fatalf("summary=%#v", result.Summary)
	}
}

func TestRunPreparedJobsDefersTransientSubmitErrorAndResumeContinues(t *testing.T) {
	setupRuntimeSchema(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.jsonl")
	data := `{"job_id":"a","model":"openai/gpt-image-2","body":{"prompt":"test a"}}
{"job_id":"b","model":"openai/gpt-image-2","body":{"prompt":"test b"}}
{"job_id":"c","model":"openai/gpt-image-2","body":{"prompt":"test c"}}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	opts := JobsOptions{AllowPaid: true, MaxTotalCredits: 10, ProjectID: "project", CID: "cid"}
	state, validation, err := PrepareRun(path, opts)
	if err != nil || validation != nil {
		t.Fatalf("PrepareRun validation=%v err=%v", validation, err)
	}
	remote := &fakeRemote{price: 0, submitErrors: []error{nil, fmt.Errorf("429 too many requests")}}
	result, err := RunPreparedJobs(context.Background(), remote, state, opts)
	if err != nil {
		t.Fatal(err)
	}
	if remote.submits != 2 {
		t.Fatalf("submits=%d, want 2", remote.submits)
	}
	if result.Summary.RemoteStatusCounts[StatusFailed] != 0 || result.Summary.RemoteStatusCounts[StatusSubmitted] != 1 || result.Summary.RemoteStatusCounts[StatusQuoted] != 2 {
		t.Fatalf("summary after defer=%#v", result.Summary)
	}
	if result.Submission == nil || !result.Submission.Deferred || result.Submission.DeferredReason != "transient_submit_error" || result.Submission.DeferredRequestID != "b-001" {
		t.Fatalf("submission summary=%#v", result.Submission)
	}
	second := state.Jobs[1].RemoteRequests[0]
	if second.Status != StatusQuoted || lastRequestError(second).Code != "submission_deferred" {
		t.Fatalf("second request=%#v", second)
	}

	resumed, err := ResumeJobs(context.Background(), remote, state.RunDir, opts)
	if err != nil {
		t.Fatal(err)
	}
	if remote.submits != 4 {
		t.Fatalf("submits after resume=%d, want 4", remote.submits)
	}
	if resumed.Summary.RemoteStatusCounts[StatusSubmitted] != 3 || resumed.Summary.RemoteStatusCounts[StatusFailed] != 0 {
		t.Fatalf("summary after resume=%#v", resumed.Summary)
	}
	if resumed.Submission == nil || resumed.Submission.Deferred || resumed.Submission.SubmittedCount != 2 || resumed.Submission.RemainingPending != 0 {
		t.Fatalf("resume submission summary=%#v", resumed.Submission)
	}
}

func TestRunPreparedJobsDefersWhenActiveTaskLimitReached(t *testing.T) {
	setupRuntimeSchema(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.jsonl")
	data := `{"job_id":"a","model":"openai/gpt-image-2","body":{"prompt":"test a"}}
{"job_id":"b","model":"openai/gpt-image-2","body":{"prompt":"test b"}}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	opts := JobsOptions{AllowPaid: true, MaxTotalCredits: 10, ProjectID: "project", CID: "cid", MaxActiveTasks: 1}
	state, validation, err := PrepareRun(path, opts)
	if err != nil || validation != nil {
		t.Fatalf("PrepareRun validation=%v err=%v", validation, err)
	}
	remote := &fakeRemote{
		price:        0,
		runningTasks: []map[string]any{{"task_id": "already-running"}},
	}
	result, err := RunPreparedJobs(context.Background(), remote, state, opts)
	if err != nil {
		t.Fatal(err)
	}
	if remote.submits != 0 {
		t.Fatalf("submits=%d, want 0", remote.submits)
	}
	if remote.listRunningCalls != 1 {
		t.Fatalf("listRunningCalls=%d, want 1", remote.listRunningCalls)
	}
	if result.Submission == nil || !result.Submission.Deferred || result.Submission.DeferredReason != "active_task_limit_reached" || result.Submission.ActiveTaskCount != 1 || result.Submission.MaxActiveTasks != 1 {
		t.Fatalf("submission summary=%#v", result.Submission)
	}
	if result.Summary.RemoteStatusCounts[StatusQuoted] != 2 {
		t.Fatalf("summary=%#v", result.Summary)
	}
}

func TestRunPreparedJobsDefersConcurrencyLimitSubmitError(t *testing.T) {
	setupRuntimeSchema(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.jsonl")
	data := `{"job_id":"a","model":"openai/gpt-image-2","body":{"prompt":"test a"}}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	opts := JobsOptions{AllowPaid: true, MaxTotalCredits: 10, ProjectID: "project", CID: "cid"}
	state, validation, err := PrepareRun(path, opts)
	if err != nil || validation != nil {
		t.Fatalf("PrepareRun validation=%v err=%v", validation, err)
	}
	remote := &fakeRemote{price: 0, submitErrors: []error{fmt.Errorf("已达当前并发上限")}}
	result, err := RunPreparedJobs(context.Background(), remote, state, opts)
	if err != nil {
		t.Fatal(err)
	}
	if result.Submission == nil || !result.Submission.Deferred || result.Submission.DeferredReason != "active_task_limit_reached" {
		t.Fatalf("submission summary=%#v", result.Submission)
	}
	request := state.Jobs[0].RemoteRequests[0]
	if request.Status != StatusQuoted || lastRequestError(request).Code != "submission_deferred" {
		t.Fatalf("request=%#v", request)
	}
}
