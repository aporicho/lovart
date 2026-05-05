package jobs

import (
	"context"
	"fmt"
)

const taskSampleLimit = 20

// DryRunJobs validates, quotes, gates, and saves a non-submitted batch state.
func DryRunJobs(ctx context.Context, remote RemoteClient, jobsFile string, opts JobsOptions) (*BatchResult, error) {
	state, validation, err := PrepareRun(jobsFile, opts)
	if err != nil {
		return nil, err
	}
	if validation != nil {
		return nil, validation
	}
	return DryRunPreparedJobs(ctx, remote, state, opts)
}

// DryRunPreparedJobs quotes and gates an already prepared state.
func DryRunPreparedJobs(ctx context.Context, remote RemoteClient, state *RunState, opts JobsOptions) (*BatchResult, error) {
	if err := quoteState(ctx, remote, state); err != nil {
		return nil, err
	}
	state.BatchGate = evaluateGate(state, opts, nil)
	if err := SaveState(state); err != nil {
		return nil, err
	}
	return Result(state, "dry_run", opts.Detail), nil
}

// RunJobs validates, quotes, submits, optionally waits, and saves state.
func RunJobs(ctx context.Context, remote RemoteClient, jobsFile string, opts JobsOptions) (*BatchResult, error) {
	state, validation, err := PrepareRun(jobsFile, opts)
	if err != nil {
		return nil, err
	}
	if validation != nil {
		return nil, validation
	}
	return RunPreparedJobs(ctx, remote, state, opts)
}

// RunPreparedJobs submits an already prepared state.
func RunPreparedJobs(ctx context.Context, remote RemoteClient, state *RunState, opts JobsOptions) (*BatchResult, error) {
	if ExistingStateHasRemoteTasks(state.RunDir) {
		return nil, fmt.Errorf("jobs: existing state has submitted tasks; use `lovart jobs resume %s`", state.RunDir)
	}
	if err := quoteState(ctx, remote, state); err != nil {
		return nil, err
	}
	if err := ensureGateAllowed(state, opts, nil); err != nil {
		_ = SaveState(state)
		return nil, err
	}
	if err := SaveState(state); err != nil {
		return nil, err
	}
	if err := submitPending(ctx, remote, state, opts); err != nil {
		return nil, err
	}
	if opts.Wait {
		if err := waitActive(ctx, remote, state, opts); err != nil {
			return nil, err
		}
	}
	return Result(state, "run", opts.Detail), nil
}

// ResumeJobs resumes a saved run state.
func ResumeJobs(ctx context.Context, remote RemoteClient, runDir string, opts JobsOptions) (*BatchResult, error) {
	state, err := LoadState(runDir)
	if err != nil {
		return nil, err
	}
	if opts.ProjectID != "" && state.ProjectID == "" {
		state.ProjectID = opts.ProjectID
	}
	if opts.CID != "" && state.CID == "" {
		state.CID = opts.CID
	}
	if opts.RetryFailed {
		resetRetryableFailures(state)
	}
	if err := quoteState(ctx, remote, state); err != nil {
		return nil, err
	}
	pendingStatuses := map[string]bool{StatusPending: true, StatusQuoted: true}
	if err := ensureGateAllowed(state, opts, pendingStatuses); err != nil {
		_ = SaveState(state)
		return nil, err
	}
	if err := SaveState(state); err != nil {
		return nil, err
	}
	if err := submitPending(ctx, remote, state, opts); err != nil {
		return nil, err
	}
	if opts.Wait {
		if err := waitActive(ctx, remote, state, opts); err != nil {
			return nil, err
		}
	}
	return Result(state, "resume", opts.Detail), nil
}

// StatusJobs returns saved state, optionally refreshing active task statuses.
func StatusJobs(ctx context.Context, remote RemoteClient, runDir string, opts JobsOptions) (*BatchResult, error) {
	state, err := LoadState(runDir)
	if err != nil {
		return nil, err
	}
	if opts.Refresh {
		if err := refreshActiveOnce(ctx, remote, state); err != nil {
			return nil, err
		}
		if err := SaveState(state); err != nil {
			return nil, err
		}
	}
	return Result(state, "status", opts.Detail), nil
}
