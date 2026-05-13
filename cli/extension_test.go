package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtensionStatusAndInstallDryRun(t *testing.T) {
	resetCLIRuntimeRoot(t)
	source := testCLIExtensionSource(t)
	target := filepath.Join(t.TempDir(), "lovart-connector")

	output := captureStdout(t, func() {
		cmd := newExtensionCmd()
		cmd.SetArgs([]string{"status", "--extension-dir", target})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("extension status: %v", err)
		}
	})
	if !strings.Contains(output, `"status":"missing"`) {
		t.Fatalf("unexpected status output: %s", output)
	}

	output = captureStdout(t, func() {
		cmd := newExtensionCmd()
		cmd.SetArgs([]string{"install", "--source-dir", source, "--extension-dir", target, "--dry-run"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("extension install dry-run: %v", err)
		}
	})
	if !strings.Contains(output, `"dry_run":true`) || !strings.Contains(output, `"status":"dry_run"`) {
		t.Fatalf("unexpected dry-run output: %s", output)
	}
	if _, err := os.Stat(filepath.Join(target, "manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote extension target, stat err=%v", err)
	}
}

func TestExtensionInstallRequiresYes(t *testing.T) {
	output := captureStdout(t, func() {
		cmd := newExtensionCmd()
		cmd.SetArgs([]string{"install", "--source-dir", testCLIExtensionSource(t), "--extension-dir", filepath.Join(t.TempDir(), "target")})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("extension install: %v", err)
		}
	})
	if !strings.Contains(output, `"ok":false`) || !strings.Contains(output, "extension install requires --yes") {
		t.Fatalf("expected --yes error, got %s", output)
	}
}

func TestRootCommandExposesExtension(t *testing.T) {
	cmd := NewRootCommand()
	found, _, err := cmd.Find([]string{"extension", "install"})
	if err != nil {
		t.Fatalf("extension install missing: %v", err)
	}
	if got, want := found.CommandPath(), "lovart extension install"; got != want {
		t.Fatalf("extension install path = %q, want %q", got, want)
	}
}

func testCLIExtensionSource(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"manifest_version":3,"name":"Lovart Connector","version":"0.1.0"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "content.js"), []byte("console.log('lovart')\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}
