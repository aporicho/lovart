package cli

import "testing"

func TestRootCommandExcludesRemovedRouteCommand(t *testing.T) {
	cmd := NewRootCommand()
	removed := "pl" + "an"
	if found, _, err := cmd.Find([]string{removed}); err == nil && found != cmd {
		t.Fatalf("root command still exposes removed route command: %s", found.CommandPath())
	}
}
