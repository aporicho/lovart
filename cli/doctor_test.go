package cli

import (
	"strings"
	"testing"

	"github.com/aporicho/lovart/internal/paths"
)

func TestDoctorCommandDefaultsToLocalDiagnostics(t *testing.T) {
	t.Cleanup(paths.Reset)
	dir := t.TempDir()
	t.Setenv("LOVART_REVERSE_ROOT", dir)
	paths.Reset()

	cmd := newDoctorCmd()
	if cmd.Flags().Lookup("online") == nil {
		t.Fatal("doctor command missing --online flag")
	}

	output := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})

	for _, want := range []string{`"ok":true`, `"execution_class":"local"`, `"network_required":false`, `"checks":`} {
		if !strings.Contains(output, want) {
			t.Fatalf("doctor output missing %s: %s", want, output)
		}
	}
	if strings.Contains(output, `"online"`) {
		t.Fatalf("doctor default should not run online checks: %s", output)
	}
}
