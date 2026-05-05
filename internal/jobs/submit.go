package jobs

import (
	"context"
	"time"

	"github.com/aporicho/lovart/internal/generation"
)

func submitPending(ctx context.Context, remote RemoteClient, state *RunState, opts JobsOptions) error {
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
			request.Attempts++
			result, err := remote.Submit(ctx, request.Model, request.Body, generation.Options{
				Mode:      request.Mode,
				ProjectID: projectID,
				CID:       cid,
			})
			if err != nil {
				request.Status = StatusFailed
				addRequestError(request, "task_failed", "batch request submission failed", map[string]any{"error": err.Error()})
				RefreshStatuses(state)
				if saveErr := SaveState(state); saveErr != nil {
					return saveErr
				}
				continue
			}
			request.TaskID = result.TaskID
			request.Status = StatusSubmitted
			request.Response = map[string]any{"task_id": result.TaskID, "status": result.Status}
			request.UpdatedAt = time.Now().UTC()
			RefreshStatuses(state)
			if err := SaveState(state); err != nil {
				return err
			}
		}
	}
	return nil
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
