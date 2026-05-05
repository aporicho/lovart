package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestJobsRunAndResumeDefaultPostprocessFlags(t *testing.T) {
	for _, cmd := range []*cobra.Command{newJobsRunCmd(), newJobsResumeCmd()} {
		for _, name := range []string{"wait", "download", "canvas"} {
			flag := cmd.Flags().Lookup(name)
			if flag == nil {
				t.Fatalf("%s command missing --%s flag", cmd.Name(), name)
			}
			if flag.DefValue != "true" {
				t.Fatalf("%s --%s default = %q, want true", cmd.Name(), name, flag.DefValue)
			}
		}
		for _, name := range []string{"no-wait", "no-download", "no-canvas"} {
			if cmd.Flags().Lookup(name) == nil {
				t.Fatalf("%s command missing --%s flag", cmd.Name(), name)
			}
		}
	}
}

func TestJobsQuoteValidatesSchemaBeforeSignedClient(t *testing.T) {
	setupCLIRuntimeMetadata(t)
	dir := t.TempDir()
	jobsPath := filepath.Join(dir, "jobs.jsonl")
	data := `{"job_id":"bad","model":"openai/gpt-image-2","body":{"prompt":"test","n":20}}`
	if err := os.WriteFile(jobsPath, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		cmd := newJobsQuoteCmd()
		cmd.SetArgs([]string{jobsPath})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})
	if !strings.Contains(output, `"code":"schema_invalid"`) {
		t.Fatalf("expected schema_invalid before auth/signing, got %s", output)
	}
}

func TestJobsDryRunValidatesSchemaBeforeSignedClient(t *testing.T) {
	setupCLIRuntimeMetadata(t)
	dir := t.TempDir()
	jobsPath := filepath.Join(dir, "jobs.jsonl")
	data := `{"job_id":"bad","model":"openai/gpt-image-2","body":{"prompt":"test","n":20}}`
	if err := os.WriteFile(jobsPath, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		cmd := newJobsDryRunCmd()
		cmd.SetArgs([]string{jobsPath})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})
	if !strings.Contains(output, `"code":"schema_invalid"`) {
		t.Fatalf("expected schema_invalid before auth/signing, got %s", output)
	}
}
