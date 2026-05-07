package cli

import (
	"strings"
	"testing"

	"github.com/aporicho/lovart/internal/auth"
)

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

func TestProjectCurrentDoesNotExposeCID(t *testing.T) {
	resetCLIRuntimeRoot(t)
	if err := auth.SaveSession(auth.Session{Cookie: "cookie", ProjectID: "project-123", CID: "cid-123"}); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		cmd := newProjectCurrentCmd()
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute current: %v", err)
		}
	})
	for _, want := range []string{`"project_id":"project-123"`, `"project_context_ready":true`} {
		if !strings.Contains(output, want) {
			t.Fatalf("project current missing %s: %s", want, output)
		}
	}
	for _, forbidden := range []string{"cid-123", `"cid"`} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("project current exposed %s: %s", forbidden, output)
		}
	}
}

func TestProjectRepairCanvasDoesNotExposeCIDFlag(t *testing.T) {
	if newProjectRepairCanvasCmd().Flags().Lookup("cid") != nil {
		t.Fatalf("repair-canvas exposes --cid")
	}
}
