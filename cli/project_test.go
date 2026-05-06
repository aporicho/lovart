package cli

import "testing"

func TestProjectCommandMovesAdvancedActionsUnderAdmin(t *testing.T) {
	cmd := newProjectCmd()
	if cmd.Commands() == nil {
		t.Fatalf("project command has no subcommands")
	}
	for _, name := range []string{"rename", "delete", "repair-canvas"} {
		if found, _, err := cmd.Find([]string{name}); err == nil && found != cmd {
			t.Fatalf("project command exposes advanced action at top level: %s", found.CommandPath())
		}
		found, _, err := cmd.Find([]string{"admin", name})
		if err != nil {
			t.Fatalf("project admin missing %s: %v", name, err)
		}
		if got, want := found.CommandPath(), "project admin "+name; got != want {
			t.Fatalf("advanced command path = %q, want %q", got, want)
		}
	}
}
