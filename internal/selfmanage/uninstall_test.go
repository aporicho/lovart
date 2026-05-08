package selfmanage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aporicho/lovart/internal/paths"
)

func TestUninstallDeletesBinaryAndExtensionButKeepsDataByDefault(t *testing.T) {
	resetSelfManageRoot(t)
	binary := filepath.Join(t.TempDir(), "lovart")
	extensionDir := filepath.Join(t.TempDir(), "extension")
	if err := os.WriteFile(binary, []byte("bin"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(extensionDir, 0755); err != nil {
		t.Fatal(err)
	}
	dataFile := filepath.Join(paths.Root, "creds.json")
	if err := os.WriteFile(dataFile, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := Uninstall(UninstallOptions{InstallPath: binary, ExtensionDir: extensionDir, Yes: true, RuntimeGOOS: "linux"})
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if !result.Deleted {
		t.Fatalf("unexpected result: %#v", result)
	}
	if _, err := os.Stat(binary); !os.IsNotExist(err) {
		t.Fatalf("binary should be removed, err=%v", err)
	}
	if _, err := os.Stat(extensionDir); !os.IsNotExist(err) {
		t.Fatalf("extension should be removed, err=%v", err)
	}
	if _, err := os.Stat(dataFile); err != nil {
		t.Fatalf("data should remain, err=%v", err)
	}
}

func TestUninstallDataDeletesRuntimeRoot(t *testing.T) {
	resetSelfManageRoot(t)
	binary := filepath.Join(t.TempDir(), "lovart")
	if err := os.WriteFile(binary, []byte("bin"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(paths.Root, "creds.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(UninstallOptions{InstallPath: binary, KeepExtension: true, Data: true, Yes: true, RuntimeGOOS: "linux"}); err != nil {
		t.Fatalf("Uninstall data: %v", err)
	}
	if _, err := os.Stat(paths.Root); !os.IsNotExist(err) {
		t.Fatalf("runtime root should be removed, err=%v", err)
	}
}

func TestUninstallDryRunDoesNotDelete(t *testing.T) {
	resetSelfManageRoot(t)
	binary := filepath.Join(t.TempDir(), "lovart")
	if err := os.WriteFile(binary, []byte("bin"), 0755); err != nil {
		t.Fatal(err)
	}
	result, err := Uninstall(UninstallOptions{InstallPath: binary, DryRun: true, Data: true})
	if err != nil {
		t.Fatalf("Uninstall dry-run: %v", err)
	}
	if !result.DryRun || result.Deleted {
		t.Fatalf("unexpected dry-run result: %#v", result)
	}
	if _, err := os.Stat(binary); err != nil {
		t.Fatalf("binary should remain, err=%v", err)
	}
	if _, err := os.Stat(paths.Root); err != nil {
		t.Fatalf("runtime root should remain, err=%v", err)
	}
}
