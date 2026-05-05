package jobs

import (
	"context"
	"fmt"
	"time"
)

func waitActive(ctx context.Context, remote RemoteClient, state *RunState, opts JobsOptions) error {
	timeout := opts.TimeoutSeconds
	if timeout <= 0 {
		timeout = 3600
	}
	poll := opts.PollInterval
	if poll <= 0 {
		poll = 5
	}
	deadline := time.Now().Add(time.Duration(timeout * float64(time.Second)))
	for {
		active := countActive(state)
		if active == 0 {
			state.TimedOut = false
			return SaveState(state)
		}
		if time.Now().After(deadline) {
			state.TimedOut = true
			return SaveState(state)
		}
		if err := refreshActiveOnce(ctx, remote, state); err != nil {
			return err
		}
		if countActive(state) == 0 {
			state.TimedOut = false
			return SaveState(state)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(poll * float64(time.Second))):
		}
	}
}

func refreshActiveOnce(ctx context.Context, remote RemoteClient, state *RunState) error {
	if remote == nil {
		return fmt.Errorf("jobs: refresh requires remote client")
	}
	for i := range state.Jobs {
		for j := range state.Jobs[i].RemoteRequests {
			request := &state.Jobs[i].RemoteRequests[j]
			if request.TaskID == "" || !activeStatus(request.Status) {
				continue
			}
			task, err := remote.FetchTask(ctx, request.TaskID)
			if err != nil {
				addRequestError(request, "remote_error", "task polling failed", map[string]any{"error": err.Error()})
				request.Status = StatusRunning
				continue
			}
			request.Task = task
			request.Artifacts = artifactDetails(task)
			switch normalizeRemoteStatus(task) {
			case StatusCompleted:
				request.Status = StatusCompleted
			case StatusFailed:
				request.Status = StatusFailed
				addRequestError(request, "task_failed", "remote Lovart task failed", map[string]any{"task": task})
			default:
				request.Status = StatusRunning
			}
			request.UpdatedAt = time.Now().UTC()
		}
	}
	RefreshStatuses(state)
	return SaveState(state)
}

func countActive(state *RunState) int {
	total := 0
	for _, job := range state.Jobs {
		for _, request := range job.RemoteRequests {
			if request.TaskID != "" && activeStatus(request.Status) {
				total++
			}
		}
	}
	return total
}

func normalizeRemoteStatus(task map[string]any) string {
	status, _ := task["status"].(string)
	switch status {
	case "completed", "complete", "done", "finished", "success", "succeeded":
		return StatusCompleted
	case "failed", "failure", "error", "cancelled", "canceled", "rejected":
		return StatusFailed
	default:
		return StatusRunning
	}
}

func artifactDetails(task map[string]any) []map[string]any {
	raw, _ := task["artifact_details"].([]map[string]any)
	if raw != nil {
		return raw
	}
	if values, _ := task["artifact_details"].([]any); len(values) > 0 {
		out := make([]map[string]any, 0, len(values))
		for _, value := range values {
			if item, ok := value.(map[string]any); ok {
				out = append(out, item)
			}
		}
		return out
	}
	items, _ := task["artifacts"].([]string)
	out := make([]map[string]any, 0, len(items))
	for _, url := range items {
		out = append(out, map[string]any{"url": url})
	}
	return out
}
