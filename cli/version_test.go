package cli

import (
	"runtime"
	"strings"
	"testing"
)

func TestRootVersionFlagsMatch(t *testing.T) {
	longOutput := captureStdout(t, func() {
		cmd := NewRootCommand()
		cmd.SetArgs([]string{"--version"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute root --version: %v", err)
		}
	})
	shortOutput := captureStdout(t, func() {
		cmd := NewRootCommand()
		cmd.SetArgs([]string{"-v"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute root -v: %v", err)
		}
	})

	if shortOutput != longOutput {
		t.Fatalf("root -v output = %s, want %s", shortOutput, longOutput)
	}
	if !strings.Contains(longOutput, `"go_version":"`+runtime.Version()+`"`) {
		t.Fatalf("version output does not include runtime Go version: %s", longOutput)
	}
}

func TestVersionSubcommandIsNotRegistered(t *testing.T) {
	cmd := NewRootCommand()
	cmd.SetArgs([]string{"version"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected lovart version to be unavailable")
	}
}
