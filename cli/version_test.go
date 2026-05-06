package cli

import (
	"runtime"
	"strings"
	"testing"
)

func TestRootVersionFlagMatchesVersionCommand(t *testing.T) {
	versionOutput := captureStdout(t, func() {
		cmd := newVersionCmd()
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute version: %v", err)
		}
	})

	rootOutput := captureStdout(t, func() {
		cmd := NewRootCommand()
		cmd.SetArgs([]string{"--version"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute root --version: %v", err)
		}
	})

	if rootOutput != versionOutput {
		t.Fatalf("root --version output = %s, want %s", rootOutput, versionOutput)
	}
	if !strings.Contains(rootOutput, `"go_version":"`+runtime.Version()+`"`) {
		t.Fatalf("version output does not include runtime Go version: %s", rootOutput)
	}
}
