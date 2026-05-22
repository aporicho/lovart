package jobs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPreparedJobsWaitSchedulesRemoteTasksInBatches(t *testing.T) {
	setupRuntimeSchema(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.jsonl")
	var lines strings.Builder
	for i := 1; i <= 25; i++ {
		_, _ = fmt.Fprintf(&lines, `{"job_id":"job-%03d","model":"openai/gpt-image-2","body":{"prompt":"test %03d"}}`, i, i)
		if i < 25 {
			lines.WriteByte('\n')
		}
	}
	if err := os.WriteFile(path, []byte(lines.String()), 0644); err != nil {
		t.Fatal(err)
	}
	opts := JobsOptions{
		AllowPaid:                true,
		MaxTotalCredits:          30,
		ConfirmPaid:              true,
		ConfirmedMaxTotalCredits: 30,
		ConfirmedRemoteRequests:  25,
		ConfirmedMaxActiveTasks:  10,
		ProjectID:                "project",
		CID:                      "cid",
		Wait:                     true,
		MaxActiveTasks:           99,
		PollInterval:             0.01,
		TimeoutSeconds:           2,
	}
	state, validation, err := PrepareRun(path, opts)
	if err != nil || validation != nil {
		t.Fatalf("PrepareRun validation=%v err=%v", validation, err)
	}
	remote := &fakeRemote{price: 1, trackActive: true, completeOnFetch: true}
	result, err := RunPreparedJobs(context.Background(), remote, state, opts)
	if err != nil {
		t.Fatal(err)
	}
	if remote.submits != 25 {
		t.Fatalf("submits=%d, want 25", remote.submits)
	}
	if remote.maxActiveObserved > 10 {
		t.Fatalf("max active remote tasks=%d, want <= 10", remote.maxActiveObserved)
	}
	if result.Summary.LogicalJobs != 25 || result.Summary.RemoteRequests != 25 || result.Summary.RequestedOutputs != 25 {
		t.Fatalf("summary=%#v", result.Summary)
	}
	if result.Summary.RemoteStatusCounts[StatusCompleted] != 25 {
		t.Fatalf("remote status counts=%#v", result.Summary.RemoteStatusCounts)
	}
	if result.Submission == nil {
		t.Fatal("missing submission summary")
	}
	if result.Submission.ConcurrencyUnit != concurrencyUnitRemoteTask {
		t.Fatalf("concurrency unit=%q", result.Submission.ConcurrencyUnit)
	}
	if result.Submission.MaxActiveTasks != 10 {
		t.Fatalf("max active tasks=%d, want 10", result.Submission.MaxActiveTasks)
	}
	if result.Submission.LogicalJobCount != 25 || result.Submission.RemoteRequestCount != 25 || result.Submission.RequestedOutputCount != 25 {
		t.Fatalf("submission summary=%#v", result.Submission)
	}
	if result.Submission.Deferred || result.Submission.RemainingPending != 0 {
		t.Fatalf("submission summary=%#v", result.Submission)
	}
}
