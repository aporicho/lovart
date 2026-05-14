package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestJobsRunAndResumeExposeUserCapabilityFlags(t *testing.T) {
	cases := []struct {
		cmd       *cobra.Command
		allowed   []string
		forbidden []string
	}{
		{
			cmd:       newJobsRunCmd(),
			allowed:   []string{"allow-paid", "max-total-credits", "download-dir", "project-id", "wait", "download", "canvas"},
			forbidden: []string{"out-dir", "detail", "no-wait", "no-download", "no-canvas", "canvas-layout", "download-dir-template", "download-file-template", "timeout-seconds", "poll-interval", "cid", "retry-failed"},
		},
		{
			cmd:       newJobsResumeCmd(),
			allowed:   []string{"allow-paid", "max-total-credits", "download-dir", "retry-failed"},
			forbidden: []string{"out-dir", "detail", "wait", "download", "canvas", "no-wait", "no-download", "no-canvas", "canvas-layout", "download-dir-template", "download-file-template", "timeout-seconds", "poll-interval", "project-id", "cid"},
		},
	}
	for _, tc := range cases {
		for _, name := range tc.allowed {
			if tc.cmd.Flags().Lookup(name) == nil {
				t.Fatalf("%s command missing --%s flag", tc.cmd.Name(), name)
			}
		}
		for _, name := range tc.forbidden {
			if tc.cmd.Flags().Lookup(name) != nil {
				t.Fatalf("%s command exposes internal --%s flag", tc.cmd.Name(), name)
			}
		}
	}
}

func TestJobsRunPostprocessFlagsDefaultToAsync(t *testing.T) {
	cmd := newJobsRunCmd()
	for _, name := range []string{"wait", "download", "canvas"} {
		flag := cmd.Flags().Lookup(name)
		if flag == nil {
			t.Fatalf("jobs run missing --%s", name)
		}
		if flag.DefValue != "false" {
			t.Fatalf("jobs run --%s default = %q, want false", name, flag.DefValue)
		}
	}
}

func TestJobsFinalizeCommandSurface(t *testing.T) {
	cmd := newJobsFinalizeCmd()
	allowed := []string{"download", "canvas", "download-dir", "project-id", "detail", "canvas-layout"}
	for _, name := range allowed {
		if cmd.Flags().Lookup(name) == nil {
			t.Fatalf("jobs finalize missing --%s flag", name)
		}
	}
	forbidden := []string{"allow-paid", "max-total-credits", "retry-failed", "wait", "no-wait", "timeout-seconds", "poll-interval", "cid"}
	for _, name := range forbidden {
		if cmd.Flags().Lookup(name) != nil {
			t.Fatalf("jobs finalize exposes internal --%s flag", name)
		}
	}
}

func TestJobsCommandDoesNotExposeAtomicQuoteOrDryRun(t *testing.T) {
	cmd := newJobsCmd()
	for _, subcommand := range cmd.Commands() {
		if subcommand.Name() == "quote" || subcommand.Name() == "dry-run" {
			t.Fatalf("jobs exposes internal atomic command %q", subcommand.Name())
		}
	}
}

func TestJobsRunValidatesSchemaBeforeSignedClient(t *testing.T) {
	setupCLIRuntimeMetadata(t)
	dir := t.TempDir()
	jobsPath := filepath.Join(dir, "jobs.jsonl")
	data := `{"job_id":"bad","model":"openai/gpt-image-2","body":{"prompt":"test","n":20}}`
	if err := os.WriteFile(jobsPath, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		cmd := newJobsRunCmd()
		cmd.SetArgs([]string{jobsPath})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})
	if !strings.Contains(output, `"code":"schema_invalid"`) {
		t.Fatalf("expected schema_invalid before auth/signing, got %s", output)
	}
}
