package generation

import "testing"

func TestBuildTaskResultPreservesFailureAndClassifiesPolicy(t *testing.T) {
	task := BuildTaskResult("fallback-task", map[string]any{
		"generator_task_id": "remote-task",
		"status":            "rejected",
		"error_code":        "moderation_failed",
		"error_msg":         "内容审核不通过",
	})

	if got := task["task_id"]; got != "remote-task" {
		t.Fatalf("task_id = %#v", got)
	}
	if got := task["normalized_status"]; got != TaskStatusFailed {
		t.Fatalf("normalized_status = %#v", got)
	}
	if _, ok := task["raw_task"].(map[string]any); !ok {
		t.Fatalf("raw_task missing: %#v", task)
	}
	failure, ok := task["failure"].(*TaskFailure)
	if !ok {
		t.Fatalf("failure missing: %#v", task["failure"])
	}
	if failure.Code != FailureTypeContentPolicyRejected || failure.Type != FailureTypeContentPolicyRejected {
		t.Fatalf("failure = %#v", failure)
	}
	if failure.RemoteCode != "moderation_failed" || failure.RemoteMessage != "内容审核不通过" || failure.Retryable {
		t.Fatalf("failure details = %#v", failure)
	}
}

func TestClassifyTaskFailureKeepsGenericFailureWhenNoPolicyEvidence(t *testing.T) {
	task := BuildTaskResult("task-1", map[string]any{
		"task_id": "task-1",
		"status":  "failed",
		"message": "renderer crashed",
	})

	failure := ClassifyTaskFailure(task)
	if failure == nil {
		t.Fatal("expected failure")
	}
	if failure.Code != FailureTypeTaskFailed || failure.Message != "renderer crashed" {
		t.Fatalf("failure = %#v", failure)
	}
}

func TestBuildTaskResultKeepsCompletedArtifacts(t *testing.T) {
	task := BuildTaskResult("task-1", map[string]any{
		"task_id": "task-1",
		"status":  "succeeded",
		"artifacts": []any{
			map[string]any{
				"content": "https://example.test/a.png",
				"metadata": map[string]any{
					"width":  float64(2048),
					"height": float64(1152),
				},
			},
		},
	})

	if got := task["normalized_status"]; got != TaskStatusCompleted {
		t.Fatalf("normalized_status = %#v", got)
	}
	details, ok := task["artifact_details"].([]map[string]any)
	if !ok || len(details) != 1 {
		t.Fatalf("artifact_details = %#v", task["artifact_details"])
	}
	if details[0]["url"] != "https://example.test/a.png" || details[0]["width"] != 2048 || details[0]["height"] != 1152 {
		t.Fatalf("artifact detail = %#v", details[0])
	}
}

func TestTaskViewHidesRawTaskUnlessFull(t *testing.T) {
	task := BuildTaskResult("task-1", map[string]any{"task_id": "task-1", "status": "running"})
	if _, ok := TaskView(task, "summary")["raw_task"]; ok {
		t.Fatalf("summary view exposed raw_task")
	}
	if _, ok := TaskView(task, "full")["raw_task"]; !ok {
		t.Fatalf("full view omitted raw_task")
	}
}
