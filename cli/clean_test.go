package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aporicho/lovart/internal/paths"
)

func TestCleanCommandNoArgsPreviewsAllScopes(t *testing.T) {
	resetCleanCLIRoot(t)
	writeCleanCLIFile(t, filepath.Join(paths.DownloadsDir, "artifact.png"), []byte("image"))

	output := captureStdout(t, func() {
		cmd := newCleanCmd()
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})

	for _, want := range []string{`"ok":true`, `"dry_run":true`, `"downloads"`, `"auth"`, `"extension"`} {
		if !strings.Contains(output, want) {
			t.Fatalf("clean preview missing %s: %s", want, output)
		}
	}
	if _, err := os.Stat(filepath.Join(paths.DownloadsDir, "artifact.png")); err != nil {
		t.Fatalf("download should remain: %v", err)
	}
}

func TestCleanCommandSelectedScopeRequiresYes(t *testing.T) {
	resetCleanCLIRoot(t)

	output := captureStdout(t, func() {
		cmd := newCleanCmd()
		cmd.SetArgs([]string{"--downloads"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})

	if !strings.Contains(output, `"code":"input_error"`) {
		t.Fatalf("clean without --yes should fail: %s", output)
	}
}

func TestCleanCommandDeletesSelectedScope(t *testing.T) {
	resetCleanCLIRoot(t)
	writeCleanCLIFile(t, filepath.Join(paths.DownloadsDir, "artifact.png"), []byte("image"))
	writeCleanCLIFile(t, paths.CredsFile, []byte(`{"cookie":"secret"}`))

	output := captureStdout(t, func() {
		cmd := newCleanCmd()
		cmd.SetArgs([]string{"--downloads", "--yes"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})

	if !strings.Contains(output, `"deleted":true`) {
		t.Fatalf("clean output = %s", output)
	}
	if _, err := os.Stat(paths.DownloadsDir); !os.IsNotExist(err) {
		t.Fatalf("downloads should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(paths.CredsFile); err != nil {
		t.Fatalf("creds should remain: %v", err)
	}
}

func resetCleanCLIRoot(t *testing.T) {
	t.Helper()
	t.Cleanup(paths.Reset)
	t.Setenv("LOVART_HOME", t.TempDir())
	paths.Reset()
	if err := paths.EnsureRuntimeDirs(); err != nil {
		t.Fatal(err)
	}
}

func writeCleanCLIFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}
