package jobs

import (
	"context"
	"time"
)

const concurrencyUnitRemoteTask = "remote_task"

type scheduler struct {
	remote  RemoteClient
	state   *RunState
	opts    JobsOptions
	summary *SubmissionSummary
}

func schedulePending(ctx context.Context, remote RemoteClient, state *RunState, opts JobsOptions) error {
	s := scheduler{
		remote: remote,
		state:  state,
		opts:   opts,
		summary: &SubmissionSummary{
			ConcurrencyUnit:       concurrencyUnitRemoteTask,
			SubmitIntervalSeconds: normalizedSubmitInterval(opts),
			SubmitLimit:           normalizedSubmitLimit(opts),
			MaxActiveTasks:        normalizedMaxActiveTasks(opts),
			LogicalJobCount:       len(state.Jobs),
			RemoteRequestCount:    countRemoteRequests(state),
			RequestedOutputCount:  countRequestedOutputs(state),
		},
	}
	return s.run(ctx)
}

func (s *scheduler) run(ctx context.Context) error {
	deadline := schedulerDeadline(s.opts)
	for {
		if err := refreshActiveOnceIfNeeded(ctx, s.remote, s.state); err != nil {
			return err
		}
		if schedulerTimedOut(deadline) {
			s.state.TimedOut = true
			setSubmissionDeferred(s.state, s.summary, "scheduler_timeout", nextPendingRequestID(s.state))
			return SaveState(s.state)
		}
		if countPendingSubmissions(s.state) == 0 && countActive(s.state) == 0 {
			s.state.TimedOut = false
			s.state.LastSubmission = finalizeSubmissionSummary(s.state, s.summary)
			return SaveState(s.state)
		}
		capacity, err := s.availableCapacity(ctx)
		if err != nil {
			return err
		}
		if capacity > 0 && countPendingSubmissions(s.state) > 0 {
			submitted, err := s.submitAvailable(ctx, capacity)
			if err != nil || s.summary.Deferred {
				return err
			}
			if submitted > 0 {
				continue
			}
		}
		if err := sleepSchedulerPoll(ctx, s.opts); err != nil {
			return err
		}
	}
}

func (s *scheduler) availableCapacity(ctx context.Context) (int, error) {
	if s.summary.MaxActiveTasks <= 0 {
		return countPendingSubmissions(s.state), nil
	}
	pool, err := s.remote.ListRunningTasks(ctx)
	if err != nil {
		active := countActive(s.state)
		s.summary.ActiveTaskCount = active
		if active >= s.summary.MaxActiveTasks {
			return 0, nil
		}
		return s.summary.MaxActiveTasks - active, nil
	}
	s.summary.ActiveTaskCount = pool.ActiveCount
	if pool.ActiveCount >= s.summary.MaxActiveTasks {
		return 0, nil
	}
	return s.summary.MaxActiveTasks - pool.ActiveCount, nil
}

func (s *scheduler) submitAvailable(ctx context.Context, capacity int) (int, error) {
	submitted := 0
	for capacity > 0 {
		request := nextPendingRequest(s.state)
		if request == nil {
			break
		}
		if s.summary.SubmitLimit > 0 && s.summary.AttemptedCount >= s.summary.SubmitLimit {
			setSubmissionDeferred(s.state, s.summary, "submit_limit_reached", request.RequestID)
			return submitted, SaveState(s.state)
		}
		if s.summary.AttemptedCount > 0 && s.summary.SubmitIntervalSeconds > 0 {
			if err := sleepSubmitInterval(ctx, s.summary.SubmitIntervalSeconds); err != nil {
				return submitted, err
			}
		}
		if err := submitOne(ctx, s.remote, s.state, s.opts, s.summary, request); err != nil {
			return submitted, err
		}
		if s.summary.Deferred {
			return submitted, nil
		}
		if request.TaskID != "" {
			submitted++
			capacity--
		}
	}
	return submitted, nil
}

func refreshActiveOnceIfNeeded(ctx context.Context, remote RemoteClient, state *RunState) error {
	if countActive(state) == 0 {
		return nil
	}
	return refreshActiveOnce(ctx, remote, state)
}

func schedulerDeadline(opts JobsOptions) time.Time {
	timeout := opts.TimeoutSeconds
	if timeout <= 0 {
		timeout = 3600
	}
	return time.Now().Add(time.Duration(timeout * float64(time.Second)))
}

func schedulerTimedOut(deadline time.Time) bool {
	return !deadline.IsZero() && time.Now().After(deadline)
}

func sleepSchedulerPoll(ctx context.Context, opts JobsOptions) error {
	poll := opts.PollInterval
	if poll <= 0 {
		poll = 5
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Duration(poll * float64(time.Second))):
		return nil
	}
}

func nextPendingRequest(state *RunState) *RemoteRequest {
	for i := range state.Jobs {
		for j := range state.Jobs[i].RemoteRequests {
			request := &state.Jobs[i].RemoteRequests[j]
			if request.TaskID == "" && (request.Status == StatusPending || request.Status == StatusQuoted) {
				return request
			}
		}
	}
	return nil
}

func nextPendingRequestID(state *RunState) string {
	request := nextPendingRequest(state)
	if request == nil {
		return ""
	}
	return request.RequestID
}

func countRemoteRequests(state *RunState) int {
	return len(allRequests(state))
}

func countRequestedOutputs(state *RunState) int {
	total := 0
	for _, job := range state.Jobs {
		total += job.Outputs
	}
	return total
}
