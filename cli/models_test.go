package cli

import (
	"strings"
	"testing"
)

func TestModelsCommandUsesCompactRegistryOutput(t *testing.T) {
	setupCLIRuntimeMetadata(t)

	output := captureStdout(t, func() {
		cmd := newModelsCmd()
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})
	if !strings.Contains(output, `"source":"registry"`) {
		t.Fatalf("expected registry source, got %s", output)
	}
	if strings.Contains(output, "request_schema") {
		t.Fatalf("models output should not expose full request schemas: %s", output)
	}
}
