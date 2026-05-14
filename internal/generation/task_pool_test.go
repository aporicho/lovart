package generation

import "testing"

func TestNormalizeTaskPoolTasksAcceptsIDsAndObjects(t *testing.T) {
	tasks := normalizeTaskPoolTasks([]any{
		"task-a",
		map[string]any{"task_id": "task-b", "status": "running"},
	})
	if len(tasks) != 2 {
		t.Fatalf("len(tasks) = %d, want 2", len(tasks))
	}
	if tasks[0]["task_id"] != "task-a" || tasks[1]["task_id"] != "task-b" {
		t.Fatalf("tasks = %#v", tasks)
	}
}
