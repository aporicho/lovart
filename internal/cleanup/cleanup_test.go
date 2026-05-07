package cleanup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aporicho/lovart/internal/paths"
)

func TestRunDefaultsRequireSelectedScope(t *testing.T) {
	resetCleanupRoot(t)

	if _, err := Run(Options{DryRun: true}); err == nil {
		t.Fatal("expected no-scope error")
	}
}

func TestRunDryRunDoesNotRemoveData(t *testing.T) {
	resetCleanupRoot(t)
	writeCleanupFile(t, filepath.Join(paths.DownloadsDir, "artifact.png"), []byte("image"))

	result, err := Run(Options{Downloads: true, DryRun: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.DryRun || result.Deleted {
		t.Fatalf("dry-run flags = %#v", result)
	}
	if result.TotalBytes != 5 {
		t.Fatalf("TotalBytes = %d, want 5", result.TotalBytes)
	}
	if _, err := os.Stat(filepath.Join(paths.DownloadsDir, "artifact.png")); err != nil {
		t.Fatalf("download should remain: %v", err)
	}
}

func TestRunDeletesOnlySelectedScopeWithYes(t *testing.T) {
	resetCleanupRoot(t)
	writeCleanupFile(t, filepath.Join(paths.DownloadsDir, "artifact.png"), []byte("image"))
	writeCleanupFile(t, paths.CredsFile, []byte(`{"cookie":"secret"}`))

	result, err := Run(Options{Downloads: true, Yes: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.DryRun || !result.Deleted {
		t.Fatalf("delete flags = %#v", result)
	}
	if _, err := os.Stat(paths.DownloadsDir); !os.IsNotExist(err) {
		t.Fatalf("downloads should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(paths.CredsFile); err != nil {
		t.Fatalf("creds should remain: %v", err)
	}
}

func TestRunDeleteRequiresYes(t *testing.T) {
	resetCleanupRoot(t)
	writeCleanupFile(t, filepath.Join(paths.RunsDir, "run.json"), []byte("{}"))

	if _, err := Run(Options{Runs: true}); err == nil {
		t.Fatal("expected --yes error")
	}
	if _, err := os.Stat(filepath.Join(paths.RunsDir, "run.json")); err != nil {
		t.Fatalf("run should remain: %v", err)
	}
}

func resetCleanupRoot(t *testing.T) {
	t.Helper()
	t.Cleanup(paths.Reset)
	t.Setenv("LOVART_HOME", t.TempDir())
	paths.Reset()
	if err := paths.EnsureRuntimeDirs(); err != nil {
		t.Fatal(err)
	}
}

func writeCleanupFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}
