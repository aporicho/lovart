package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aporicho/lovart/internal/generation"
)

func submitPending(ctx context.Context, remote RemoteClient, state *RunState, opts JobsOptions) error {
	summary := &SubmissionSummary{
		SubmitIntervalSeconds: normalizedSubmitInterval(opts),
		SubmitLimit:           normalizedSubmitLimit(opts),
		MaxActiveTasks:        normalizedMaxActiveTasks(opts),
	}
	projectID := opts.ProjectID
	if projectID == "" {
		projectID = state.ProjectID
	}
	cid := opts.CID
	if cid == "" {
		cid = state.CID
	}
	for i := range state.Jobs {
		for j := range state.Jobs[i].RemoteRequests {
			request := &state.Jobs[i].RemoteRequests[j]
			if request.TaskID != "" || (request.Status != StatusPending && request.Status != StatusQuoted) {
				continue
			}
			if summary.SubmitLimit > 0 && summary.AttemptedCount >= summary.SubmitLimit {
				setSubmissionDeferred(state, summary, "submit_limit_reached", request.RequestID)
				return SaveState(state)
			}
			deferred, err := deferIfActiveTaskLimitReached(ctx, remote, state, summary, request.RequestID)
			if err != nil {
				return err
			}
			if deferred {
				return nil
			}
			if summary.AttemptedCount > 0 && summary.SubmitIntervalSeconds > 0 {
				if err := sleepSubmitInterval(ctx, summary.SubmitIntervalSeconds); err != nil {
					return err
				}
			}
			body := requestEffectiveBody(*request)
			request.Attempts++
			summary.AttemptedCount++
			result, err := remote.Submit(ctx, request.Model, body, generation.Options{
				Mode:      request.Mode,
				ProjectID: projectID,
				CID:       cid,
			})
			if err != nil {
				if generation.IsConcurrencyLimitError(err) {
					addRequestError(request, "submission_deferred", "batch request submission deferred because the active task limit was reached", map[string]any{"error": err.Error()})
					RefreshStatuses(state)
					setSubmissionDeferred(state, summary, "active_task_limit_reached", request.RequestID)
					if saveErr := SaveState(state); saveErr != nil {
						return saveErr
					}
					return nil
				}
				if isTransientSubmitError(err) {
					addRequestError(request, "submission_deferred", "batch request submission deferred after transient submit error", map[string]any{"error": err.Error()})
					RefreshStatuses(state)
					setSubmissionDeferred(state, summary, "transient_submit_error", request.RequestID)
					if saveErr := SaveState(state); saveErr != nil {
						return saveErr
					}
					return nil
				}
				summary.FailedCount++
				request.Status = StatusFailed
				addRequestError(request, "task_failed", "batch request submission failed", map[string]any{"error": err.Error()})
				RefreshStatuses(state)
				state.LastSubmission = finalizeSubmissionSummary(state, summary)
				if saveErr := SaveState(state); saveErr != nil {
					return saveErr
				}
				continue
			}
			clearRequestErrorsByCode(request, "submission_deferred")
			summary.SubmittedCount++
			request.TaskID = result.TaskID
			request.Status = StatusSubmitted
			if len(result.NormalizedBody) > 0 {
				request.NormalizedBody = result.NormalizedBody
			}
			request.Response = map[string]any{
				"task_id":         result.TaskID,
				"status":          result.Status,
				"normalized_body": request.NormalizedBody,
			}
			request.UpdatedAt = time.Now().UTC()
			RefreshStatuses(state)
			state.LastSubmission = finalizeSubmissionSummary(state, summary)
			if err := SaveState(state); err != nil {
				return err
			}
		}
	}
	state.LastSubmission = finalizeSubmissionSummary(state, summary)
	if err := SaveState(state); err != nil {
		return err
	}
	return nil
}

func clearRequestErrorsByCode(request *RemoteRequest, code string) {
	if len(request.Errors) == 0 {
		return
	}
	filtered := request.Errors[:0]
	for _, err := range request.Errors {
		if err.Code != code {
			filtered = append(filtered, err)
		}
	}
	request.Errors = filtered
}

func normalizedSubmitInterval(opts JobsOptions) float64 {
	if opts.SubmitIntervalSeconds <= 0 {
		return 0
	}
	return opts.SubmitIntervalSeconds
}

func normalizedSubmitLimit(opts JobsOptions) int {
	if opts.SubmitLimit <= 0 {
		return 0
	}
	return opts.SubmitLimit
}

func normalizedMaxActiveTasks(opts JobsOptions) int {
	if opts.MaxActiveTasks <= 0 {
		return 0
	}
	return opts.MaxActiveTasks
}

func deferIfActiveTaskLimitReached(ctx context.Context, remote RemoteClient, state *RunState, summary *SubmissionSummary, requestID string) (bool, error) {
	if summary.MaxActiveTasks <= 0 {
		return false, nil
	}
	pool, err := remote.ListRunningTasks(ctx)
	if err != nil {
		return false, nil
	}
	summary.ActiveTaskCount = pool.ActiveCount
	if pool.ActiveCount < summary.MaxActiveTasks {
		return false, nil
	}
	setSubmissionDeferred(state, summary, "active_task_limit_reached", requestID)
	if err := SaveState(state); err != nil {
		return false, err
	}
	return true, nil
}

func sleepSubmitInterval(ctx context.Context, seconds float64) error {
	timer := time.NewTimer(time.Duration(seconds * float64(time.Second)))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func setSubmissionDeferred(state *RunState, summary *SubmissionSummary, reason, requestID string) {
	summary.Deferred = true
	summary.DeferredReason = reason
	summary.DeferredRequestID = requestID
	summary.NextAction = fmt.Sprintf("lovart jobs resume %s", state.RunDir)
	state.LastSubmission = finalizeSubmissionSummary(state, summary)
}

func finalizeSubmissionSummary(state *RunState, summary *SubmissionSummary) *SubmissionSummary {
	copySummary := *summary
	copySummary.RemainingPending = countPendingSubmissions(state)
	if copySummary.Deferred {
		copySummary.DeferredCount = copySummary.RemainingPending
		if copySummary.NextAction == "" && copySummary.RemainingPending > 0 {
			copySummary.NextAction = fmt.Sprintf("lovart jobs resume %s", state.RunDir)
		}
	}
	return &copySummary
}

func countPendingSubmissions(state *RunState) int {
	count := 0
	for _, request := range allRequests(state) {
		if request.TaskID == "" && (request.Status == StatusPending || request.Status == StatusQuoted) {
			count++
		}
	}
	return count
}

func isTransientSubmitError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	for _, marker := range []string{
		"429",
		"rate limit",
		"rate-limit",
		"too many requests",
		"temporarily unavailable",
		"temporary",
		"timeout",
		"timed out",
		"deadline exceeded",
		"connection reset",
		"connection refused",
		"service unavailable",
		"bad gateway",
		"gateway timeout",
		"502",
		"503",
		"504",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func resetRetryableFailures(state *RunState) {
	for i := range state.Jobs {
		for j := range state.Jobs[i].RemoteRequests {
			request := &state.Jobs[i].RemoteRequests[j]
			if failedWithoutTask(*request) {
				request.Status = StatusPending
				request.Errors = nil
				request.Quote = nil
			}
		}
	}
	RefreshStatuses(state)
}
