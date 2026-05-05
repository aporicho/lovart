package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQuoteValidatesSchemaBeforeSignedClient(t *testing.T) {
	setupCLIRuntimeMetadata(t)
	dir := t.TempDir()
	bodyPath := filepath.Join(dir, "body.json")
	if err := os.WriteFile(bodyPath, []byte(`{"n":20}`), 0644); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		cmd := newQuoteCmd()
		cmd.SetArgs([]string{"openai/gpt-image-2", "--body-file", bodyPath})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})
	if !strings.Contains(output, `"code":"schema_invalid"`) {
		t.Fatalf("expected schema_invalid before auth/signing, got %s", output)
	}
}
