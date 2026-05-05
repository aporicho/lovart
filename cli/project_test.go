package cli

import "testing"

func TestProjectCommandExposesRepairCanvas(t *testing.T) {
	cmd := newProjectCmd()
	if cmd.Commands() == nil {
		t.Fatalf("project command has no subcommands")
	}
	if _, _, err := cmd.Find([]string{"repair-canvas"}); err != nil {
		t.Fatalf("project command missing repair-canvas: %v", err)
	}
}
