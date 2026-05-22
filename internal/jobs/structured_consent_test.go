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
	confirmed := 1.5
	setValidPaidConsent(&opts, 2, 2, 0)
	opts.ConfirmedMaxTotalCredits = &confirmed
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
	confirm := true
	opts.ConfirmPaid = &confirm
	opts.ConfirmedMaxTotalCredits = &credits
	opts.ConfirmedRemoteRequests = &remoteRequests
	opts.ConfirmedMaxActiveTasks = &maxActiveTasks
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
