package cli

import "testing"

func TestTaskStatusCommandSurface(t *testing.T) {
	cmd := newTaskCmd()
	status, _, err := cmd.Find([]string{"status"})
	if err != nil {
		t.Fatalf("task status missing: %v", err)
	}
	if status.Flags().Lookup("detail") == nil {
		t.Fatalf("task status missing --detail")
	}
	if got, want := status.Use, "status <task_id>"; got != want {
		t.Fatalf("task status use = %q, want %q", got, want)
	}

	wait, _, err := cmd.Find([]string{"wait"})
	if err != nil {
		t.Fatalf("task wait missing: %v", err)
	}
	for _, name := range []string{"detail", "timeout-seconds", "poll-interval"} {
		if wait.Flags().Lookup(name) == nil {
			t.Fatalf("task wait missing --%s", name)
		}
	}
	if got, want := wait.Use, "wait <task_id>"; got != want {
		t.Fatalf("task wait use = %q, want %q", got, want)
	}

	canvas, _, err := cmd.Find([]string{"canvas"})
	if err != nil {
		t.Fatalf("task canvas missing: %v", err)
	}
	for _, name := range []string{"project-id", "detail"} {
		if canvas.Flags().Lookup(name) == nil {
			t.Fatalf("task canvas missing --%s", name)
		}
	}
	if got, want := canvas.Use, "canvas <task_id>"; got != want {
		t.Fatalf("task canvas use = %q, want %q", got, want)
	}
}
