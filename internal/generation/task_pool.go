package generation

import (
	"context"
	"fmt"
	"net/url"

	"github.com/aporicho/lovart/internal/http"
)

// TaskPoolResult is the current Lovart running-task pool for the user.
type TaskPoolResult struct {
	UserUUID    string           `json:"user_uuid,omitempty"`
	ActiveCount int              `json:"active_count"`
	Tasks       []map[string]any `json:"tasks"`
}

// TaskCancelResult reports a bulk running-task cancellation request.
type TaskCancelResult struct {
	UserUUID       string   `json:"user_uuid,omitempty"`
	TaskIDs        []string `json:"task_ids"`
	RequestedCount int      `json:"requested_count"`
	Cancelled      bool     `json:"cancelled"`
}

// ListRunningTasks lists running tasks for the current signed-in user.
func ListRunningTasks(ctx context.Context, client *http.Client) (*TaskPoolResult, error) {
	if client == nil {
		return nil, fmt.Errorf("generation: task list requires a client")
	}
	userUUID := client.UserUUID()
	if userUUID == "" {
		return nil, fmt.Errorf("generation: task list requires cid/webid in credentials")
	}

	path := "/v1/tasks/running?user_uuid=" + url.QueryEscape(userUUID)
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Msg     string `json:"msg"`
		Data    struct {
			Tasks []any `json:"tasks"`
		} `json:"data"`
	}
	if err := client.GetJSON(ctx, http.LGWBase, path, &resp); err != nil {
		return nil, fmt.Errorf("generation: list running tasks: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("generation: list running tasks returned code %d: %s", resp.Code, firstNonEmptyTaskPool(resp.Message, resp.Msg))
	}

	tasks := normalizeTaskPoolTasks(resp.Data.Tasks)
	return &TaskPoolResult{
		UserUUID:    userUUID,
		ActiveCount: len(tasks),
		Tasks:       tasks,
	}, nil
}

// CancelRunningTasks terminates running tasks for the current signed-in user.
func CancelRunningTasks(ctx context.Context, client *http.Client, taskIDs []string) (*TaskCancelResult, error) {
	if client == nil {
		return nil, fmt.Errorf("generation: task cancel requires a client")
	}
	userUUID := client.UserUUID()
	if userUUID == "" {
		return nil, fmt.Errorf("generation: task cancel requires cid/webid in credentials")
	}
	taskIDs = normalizeTaskIDs(taskIDs)
	if len(taskIDs) == 0 {
		return nil, fmt.Errorf("generation: task cancel requires at least one task_id")
	}

	body := map[string]any{
		"user_uuid": userUUID,
		"task_ids":  taskIDs,
	}
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Msg     string `json:"msg"`
	}
	if err := client.PostJSON(ctx, http.LGWBase, "/v1/tasks/terminate", body, &resp); err != nil {
		return nil, fmt.Errorf("generation: cancel running tasks: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("generation: cancel running tasks returned code %d: %s", resp.Code, firstNonEmptyTaskPool(resp.Message, resp.Msg))
	}

	return &TaskCancelResult{
		UserUUID:       userUUID,
		TaskIDs:        taskIDs,
		RequestedCount: len(taskIDs),
		Cancelled:      true,
	}, nil
}

func normalizeTaskIDs(taskIDs []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(taskIDs))
	for _, taskID := range taskIDs {
		if taskID == "" || seen[taskID] {
			continue
		}
		seen[taskID] = true
		out = append(out, taskID)
	}
	return out
}

func normalizeTaskPoolTasks(values []any) []map[string]any {
	if len(values) == 0 {
		return []map[string]any{}
	}
	tasks := make([]map[string]any, 0, len(values))
	for _, value := range values {
		switch typed := value.(type) {
		case map[string]any:
			tasks = append(tasks, typed)
		case string:
			if typed != "" {
				tasks = append(tasks, map[string]any{"task_id": typed})
			}
		default:
			tasks = append(tasks, map[string]any{"value": typed})
		}
	}
	return tasks
}

func firstNonEmptyTaskPool(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
