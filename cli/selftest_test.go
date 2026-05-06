package cli

import (
	"strings"
	"testing"

	"github.com/aporicho/lovart/internal/paths"
)

func TestSelfTestCommandReturnsDiagnostics(t *testing.T) {
	t.Cleanup(paths.Reset)
	dir := t.TempDir()
	t.Setenv("LOVART_REVERSE_ROOT", dir)
	paths.Reset()

	output := captureStdout(t, func() {
		cmd := newSelfTestCmd()
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})

	for _, want := range []string{`"ok":true`, `"status":"needs_setup"`, `"checks":`, `"auth":`, `"signer":`} {
		if !strings.Contains(output, want) {
			t.Fatalf("self-test output missing %s: %s", want, output)
		}
	}
	if strings.Contains(output, "not implemented") {
		t.Fatalf("self-test still returns placeholder output: %s", output)
	}
}
