package jobs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestRunPreparedJobsStructuredConsentMissingConfirmPaidBlocks(t *testing.T) {
	state, opts := preparedPaidConsentState(t, 2)
	remote := &fakeRemote{price: 1}

	_, err := RunPreparedJobs(context.Background(), remote, state, opts)
	assertGateError(t, err, "structured_consent_missing_confirm_paid")
	if remote.submits != 0 {
		t.Fatalf("submits=%d, want 0", remote.submits)
	}
}

func TestRunPreparedJobsStructuredConsentCreditMismatchBlocks(t *testing.T) {
	state, opts := preparedPaidConsentState(t, 2)
	setValidPaidConsent(&opts, 2, 2, 0)
	opts.ConfirmedMaxTotalCredits = 1.5
	remote := &fakeRemote{price: 1}

	_, err := RunPreparedJobs(context.Background(), remote, state, opts)
	assertGateError(t, err, "structured_consent_max_total_credits_mismatch")
	if remote.submits != 0 {
		t.Fatalf("submits=%d, want 0", remote.submits)
	}
}

func TestRunPreparedJobsStructuredConsentRemoteRequestMismatchBlocks(t *testing.T) {
	state, opts := preparedPaidConsentState(t, 2)
	setValidPaidConsent(&opts, 2, 1, 0)
	remote := &fakeRemote{price: 1}

	_, err := RunPreparedJobs(context.Background(), remote, state, opts)
	assertGateError(t, err, "structured_consent_remote_requests_mismatch")
	if remote.submits != 0 {
		t.Fatalf("submits=%d, want 0", remote.submits)
	}
}

func TestRunPreparedJobsStructuredConsentRemoteRequestsUsesPlannedSubmitLimit(t *testing.T) {
	state, opts := preparedPaidConsentState(t, 3)
	opts.SubmitLimit = 2
	setValidPaidConsent(&opts, 3, 2, 0)
	remote := &fakeRemote{price: 1}

	result, err := RunPreparedJobs(context.Background(), remote, state, opts)
	if err != nil {
		t.Fatal(err)
	}
	if remote.submits != 2 {
		t.Fatalf("submits=%d, want 2", remote.submits)
	}
	if result.BatchGate == nil || result.BatchGate.SelectedRemoteRequests != 3 || result.BatchGate.PlannedRemoteRequests != 2 {
		t.Fatalf("batch gate=%#v", result.BatchGate)
	}
	if result.Submission == nil || !result.Submission.Deferred || result.Submission.DeferredReason != "submit_limit_reached" {
		t.Fatalf("submission=%#v", result.Submission)
	}
}

func TestRunPreparedJobsStructuredConsentMaxActiveMismatchBlocks(t *testing.T) {
	state, opts := preparedPaidConsentState(t, 2)
	opts.MaxActiveTasks = 99
	setValidPaidConsent(&opts, 2, 2, 9)
	remote := &fakeRemote{price: 1}

	_, err := RunPreparedJobs(context.Background(), remote, state, opts)
	assertGateError(t, err, "structured_consent_max_active_tasks_mismatch")
	if remote.submits != 0 {
		t.Fatalf("submits=%d, want 0", remote.submits)
	}
}

func preparedPaidConsentState(t *testing.T, count int) (*RunState, JobsOptions) {
	t.Helper()
	setupRuntimeSchema(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.jsonl")
	data := ""
	for i := 0; i < count; i++ {
		if i > 0 {
			data += "\n"
		}
		data += fmt.Sprintf("{\"job_id\":\"job-%c\",\"model\":\"openai/gpt-image-2\",\"body\":{\"prompt\":\"test\"}}", 'a'+i)
	}
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	opts := JobsOptions{AllowPaid: true, MaxTotalCredits: 10, ProjectID: "project", CID: "cid"}
	state, validation, err := PrepareRun(path, opts)
	if err != nil || validation != nil {
		t.Fatalf("PrepareRun validation=%v err=%v", validation, err)
	}
	return state, opts
}

func setValidPaidConsent(opts *JobsOptions, credits float64, remoteRequests int, maxActiveTasks int) {
	opts.ConfirmPaid = true
	opts.ConfirmedMaxTotalCredits = credits
	opts.ConfirmedRemoteRequests = remoteRequests
	opts.ConfirmedMaxActiveTasks = maxActiveTasks
	if opts.ProjectID != "" {
		opts.ConfirmedProjectID = opts.ProjectID
	}
}

func assertGateError(t *testing.T, err error, code string) {
	t.Helper()
	var gateErr *GateError
	if !errors.As(err, &gateErr) {
		t.Fatalf("err=%v, want GateError", err)
	}
	if gateErr.Code != code {
		t.Fatalf("code=%q, want %q", gateErr.Code, code)
	}
}
