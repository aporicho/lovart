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
	if !strings.Contains(output, `"execution_class":"local"`) {
		t.Fatalf("expected local execution metadata, got %s", output)
	}
	if !strings.Contains(output, `"cache_used":true`) {
		t.Fatalf("expected cache metadata, got %s", output)
	}
	if strings.Contains(output, "request_schema") {
		t.Fatalf("models output should not expose full request schemas: %s", output)
	}
}

func TestModelsCommandUsesRefreshFlag(t *testing.T) {
	cmd := newModelsCmd()
	if cmd.Flags().Lookup("refresh") == nil {
		t.Fatal("models command missing --refresh flag")
	}
	if cmd.Flags().Lookup("live") != nil {
		t.Fatal("models command should not expose --live")
	}
}
