package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestDefaultRootUsesLovartHomeDirectory(t *testing.T) {
	t.Cleanup(Reset)
	home := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
		t.Setenv("HOMEDRIVE", "")
		t.Setenv("HOMEPATH", "")
	} else {
		t.Setenv("HOME", home)
	}
	t.Setenv("LOVART_HOME", "")

	Reset()

	want := filepath.Join(home, ".lovart")
	if Root != want {
		t.Fatalf("Root = %q, want %q", Root, want)
	}
	if CredsFile != filepath.Join(want, "creds.json") {
		t.Fatalf("CredsFile = %q", CredsFile)
	}
	if MetadataDir != filepath.Join(want, "metadata") {
		t.Fatalf("MetadataDir = %q", MetadataDir)
	}
	if DownloadsDir != filepath.Join(want, "downloads") {
		t.Fatalf("DownloadsDir = %q", DownloadsDir)
	}
}

func TestLovartHomeOverridesDefaultRoot(t *testing.T) {
	t.Cleanup(Reset)
	root := filepath.Join(t.TempDir(), "custom")
	t.Setenv("LOVART_HOME", root)

	Reset()

	if Root != root {
		t.Fatalf("Root = %q, want %q", Root, root)
	}
	if ExtensionDir != filepath.Join(root, "extension", "lovart-connector") {
		t.Fatalf("ExtensionDir = %q", ExtensionDir)
	}
}

func TestPrepareRuntimeCreatesRuntimeDirs(t *testing.T) {
	t.Cleanup(Reset)
	root := t.TempDir()
	t.Setenv("LOVART_HOME", root)
	Reset()

	if err := PrepareRuntime(); err != nil {
		t.Fatalf("PrepareRuntime: %v", err)
	}

	for _, dir := range []string{Root, MetadataDir, SignerDir, RunsDir, DownloadsDir, TmpDir, filepath.Dir(ExtensionDir)} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("runtime dir missing %s: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("runtime path is not a dir: %s", dir)
		}
	}
}

func TestCleanupIntermediateFilesRemovesOnlyStaleTemps(t *testing.T) {
	t.Cleanup(Reset)
	root := t.TempDir()
	t.Setenv("LOVART_HOME", root)
	Reset()
	if err := EnsureRuntimeDirs(); err != nil {
		t.Fatal(err)
	}

	staleTmp := filepath.Join(TmpDir, "stale")
	freshTmp := filepath.Join(TmpDir, "fresh")
	staleAtomic := filepath.Join(MetadataDir, ".manifest.json.123.tmp")
	freshAtomic := filepath.Join(MetadataDir, ".manifest.json.456.tmp")
	regular := filepath.Join(MetadataDir, "manifest.json")
	for _, path := range []string{staleTmp, freshTmp, staleAtomic, freshAtomic, regular} {
		writeFile(t, path, []byte("x"))
	}
	old := time.Now().Add(-48 * time.Hour)
	for _, path := range []string{staleTmp, staleAtomic} {
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}

	if err := CleanupIntermediateFiles(24 * time.Hour); err != nil {
		t.Fatalf("CleanupIntermediateFiles: %v", err)
	}
	for _, path := range []string{staleTmp, staleAtomic} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s should be removed, stat err = %v", path, err)
		}
	}
	for _, path := range []string{freshTmp, freshAtomic, regular} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s should remain: %v", path, err)
		}
	}
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}
