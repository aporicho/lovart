package generation

import (
	"fmt"
	"strings"

	apperrors "github.com/aporicho/lovart/internal/errors"
)

const (
	// Normalized task statuses shared by CLI, MCP, and batch jobs.
	TaskStatusCompleted = "completed"
	TaskStatusFailed    = "failed"
	TaskStatusRunning   = "running"

	FailureTypeTaskFailed            = apperrors.CodeTaskFailed
	FailureTypeContentPolicyRejected = apperrors.CodeContentPolicyRejected
)

// TaskFailure is the stable public summary for a failed remote task.
type TaskFailure struct {
	Type          string `json:"type"`
	Code          string `json:"code"`
	Message       string `json:"message"`
	Status        string `json:"status,omitempty"`
	RemoteCode    string `json:"remote_code,omitempty"`
	RemoteMessage string `json:"remote_message,omitempty"`
	RemoteReason  string `json:"remote_reason,omitempty"`
	Retryable     bool   `json:"retryable"`
}

// BuildTaskResult converts the remote task data into the stable task shape.
func BuildTaskResult(fallbackTaskID string, data map[string]any) map[string]any {
	raw := copyMap(data)
	taskID := firstString(raw["generator_task_id"], raw["task_id"], fallbackTaskID)
	status := stringValue(raw["status"])
	result := map[string]any{
		"task_id":           taskID,
		"status":            status,
		"normalized_status": TaskStatusRunning,
		"raw_task":          raw,
	}
	result["normalized_status"] = NormalizeTaskStatus(result)

	if NormalizeTaskStatus(result) == TaskStatusCompleted {
		urls, details := artifactDetailsFromRaw(raw["artifacts"])
		if len(urls) > 0 {
			result["artifacts"] = urls
		}
		if len(details) > 0 {
			result["artifact_details"] = details
		}
	}
	if failure := ClassifyTaskFailure(result); failure != nil {
		result["failure"] = failure
	}
	return result
}

// TaskView returns a public task view. detail="full" includes raw_task.
func TaskView(task map[string]any, detail string) map[string]any {
	if task == nil {
		return nil
	}
	view := copyMap(task)
	if detail != "full" {
		delete(view, "raw_task")
	}
	return view
}

// NormalizeTaskStatus collapses remote task status variants into stable states.
func NormalizeTaskStatus(task map[string]any) string {
	status := strings.ToLower(strings.TrimSpace(stringValue(task["status"])))
	if status == "" {
		if raw, _ := task["raw_task"].(map[string]any); raw != nil {
			status = strings.ToLower(strings.TrimSpace(stringValue(raw["status"])))
		}
	}
	switch status {
	case "completed", "complete", "done", "finished", "success", "succeeded":
		return TaskStatusCompleted
	case "failed", "failure", "error", "cancelled", "canceled", "rejected", "task_failed":
		return TaskStatusFailed
	default:
		return TaskStatusRunning
	}
}

// ClassifyTaskFailure returns a stable failure summary for terminal failed tasks.
func ClassifyTaskFailure(task map[string]any) *TaskFailure {
	if NormalizeTaskStatus(task) != TaskStatusFailed {
		return nil
	}
	info := collectFailureEvidence(task)
	status := stringValue(task["status"])
	message := firstNonEmpty(info.RemoteMessage, info.RemoteReason, "remote Lovart task failed")
	failureType := FailureTypeTaskFailed
	code := apperrors.CodeTaskFailed
	if hasContentPolicyEvidence(info.Texts) {
		failureType = FailureTypeContentPolicyRejected
		code = apperrors.CodeContentPolicyRejected
		if info.RemoteMessage == "" && info.RemoteReason == "" {
			message = "remote generation was rejected by content policy"
		}
	}
	return &TaskFailure{
		Type:          failureType,
		Code:          code,
		Message:       message,
		Status:        status,
		RemoteCode:    info.RemoteCode,
		RemoteMessage: info.RemoteMessage,
		RemoteReason:  info.RemoteReason,
		Retryable:     false,
	}
}

type failureEvidence struct {
	RemoteCode    string
	RemoteMessage string
	RemoteReason  string
	Texts         []string
}

func collectFailureEvidence(task map[string]any) failureEvidence {
	var info failureEvidence
	collectFailureValue(task, "", false, &info)
	if raw, _ := task["raw_task"].(map[string]any); raw != nil {
		collectFailureValue(raw, "", false, &info)
	}
	return info
}

func collectFailureValue(value any, key string, parentInteresting bool, info *failureEvidence) {
	normalizedKey := normalizeEvidenceKey(key)
	interesting := parentInteresting || interestingFailureKey(normalizedKey)
	switch v := value.(type) {
	case map[string]any:
		for childKey, childValue := range v {
			switch normalizeEvidenceKey(childKey) {
			case "rawtask", "failure":
				continue
			}
			collectFailureValue(childValue, childKey, interesting, info)
		}
	case []any:
		for _, item := range v {
			collectFailureValue(item, key, interesting, info)
		}
	case string:
		if interesting {
			recordFailureText(normalizedKey, strings.TrimSpace(v), info)
		}
	case float64, int, int64:
		if interestingCodeKey(normalizedKey) {
			recordFailureText(normalizedKey, fmt.Sprint(v), info)
		}
	}
}

func recordFailureText(key string, text string, info *failureEvidence) {
	if text == "" {
		return
	}
	info.Texts = append(info.Texts, text)
	switch {
	case interestingCodeKey(key) && info.RemoteCode == "":
		info.RemoteCode = text
	case strings.Contains(key, "reason") && info.RemoteReason == "":
		info.RemoteReason = text
	case (strings.Contains(key, "message") || key == "msg" || strings.Contains(key, "error") || strings.Contains(key, "detail")) && info.RemoteMessage == "":
		info.RemoteMessage = text
	}
}

func interestingFailureKey(key string) bool {
	switch {
	case key == "error", key == "errors", key == "err", key == "msg", key == "message", key == "detail", key == "details":
		return true
	case strings.Contains(key, "error"), strings.Contains(key, "fail"), strings.Contains(key, "reject"), strings.Contains(key, "reason"):
		return true
	case strings.Contains(key, "moderation"), strings.Contains(key, "policy"), strings.Contains(key, "safety"), strings.Contains(key, "audit"), strings.Contains(key, "review"):
		return true
	default:
		return false
	}
}

func interestingCodeKey(key string) bool {
	return key == "code" || strings.Contains(key, "code")
}

func hasContentPolicyEvidence(texts []string) bool {
	for _, text := range texts {
		lower := strings.ToLower(text)
		for _, token := range []string{
			"content policy",
			"policy",
			"moderation",
			"moderated",
			"safety",
			"unsafe",
			"prohibited",
			"disallowed",
			"violat",
			"blocked",
			"copyright",
			"audit",
			"审核",
			"未通过",
			"不通过",
			"违规",
			"违禁",
			"策略",
			"政策",
			"安全",
			"拒绝",
			"禁止",
			"版权",
		} {
			if strings.Contains(lower, token) {
				return true
			}
		}
	}
	return false
}

func artifactDetailsFromRaw(value any) ([]string, []map[string]any) {
	values, ok := value.([]any)
	if !ok {
		return nil, nil
	}
	urls := make([]string, 0, len(values))
	details := make([]map[string]any, 0, len(values))
	for _, item := range values {
		switch artifact := item.(type) {
		case string:
			if artifact == "" {
				continue
			}
			urls = append(urls, artifact)
			details = append(details, map[string]any{"url": artifact})
		case map[string]any:
			url := firstString(artifact["content"], artifact["url"])
			if url == "" {
				continue
			}
			detail := map[string]any{"url": url}
			metadata, _ := artifact["metadata"].(map[string]any)
			if width := numberValue(firstNonNil(metadata["width"], artifact["width"])); width > 0 {
				detail["width"] = width
			}
			if height := numberValue(firstNonNil(metadata["height"], artifact["height"])); height > 0 {
				detail["height"] = height
			}
			urls = append(urls, url)
			details = append(details, detail)
		}
	}
	return urls, details
}

func copyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func firstString(values ...any) string {
	for _, value := range values {
		if text := strings.TrimSpace(stringValue(value)); text != "" {
			return text
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return ""
	}
}

func numberValue(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	default:
		return 0
	}
}

func normalizeEvidenceKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, "_", "")
	key = strings.ReplaceAll(key, "-", "")
	return key
}
