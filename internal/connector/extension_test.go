package connector

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallDryRunDoesNotWrite(t *testing.T) {
	source := testExtensionSource(t)
	target := filepath.Join(t.TempDir(), "lovart-connector")
	result, err := Install(Options{SourceDir: source, ExtensionDir: target, DryRun: true})
	if err != nil {
		t.Fatalf("Install dry run: %v", err)
	}
	if result.Status != "dry_run" || result.DryRun != true {
		t.Fatalf("unexpected dry run result: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(target, "manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote target, stat err=%v", err)
	}
}

func TestInstallCopiesExtensionFiles(t *testing.T) {
	source := testExtensionSource(t)
	target := filepath.Join(t.TempDir(), "lovart-connector")
	result, err := Install(Options{SourceDir: source, ExtensionDir: target})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if result.Status != "installed" || !result.Installed || !result.Exists {
		t.Fatalf("unexpected install result: %#v", result)
	}
	for _, rel := range []string{"manifest.json", filepath.Join("src", "background", "service_worker.js")} {
		if _, err := os.Stat(filepath.Join(target, rel)); err != nil {
			t.Fatalf("missing copied file %s: %v", rel, err)
		}
	}
}

func TestInstallMissingSourceReturnsInputError(t *testing.T) {
	_, err := Install(Options{SourceDir: filepath.Join(t.TempDir(), "missing"), ExtensionDir: filepath.Join(t.TempDir(), "target")})
	if err == nil || !strings.Contains(err.Error(), "input_error") {
		t.Fatalf("expected input error, got %v", err)
	}
}

func TestOpenFailureDoesNotFailInstall(t *testing.T) {
	source := testExtensionSource(t)
	target := filepath.Join(t.TempDir(), "lovart-connector")
	result, err := Install(Options{
		SourceDir:    source,
		ExtensionDir: target,
		Open:         true,
		OpenURL: func(url string) error {
			if url != ChromeExtensionsURL {
				t.Fatalf("unexpected URL: %s", url)
			}
			return errTestOpen
		},
	})
	if err != nil {
		t.Fatalf("Install should not fail on open error: %v", err)
	}
	if result.OpenedBrowser || !strings.Contains(result.OpenError, "open failed") {
		t.Fatalf("open error not recorded: %#v", result)
	}
}

func TestStatusReportsMissingAndInstalled(t *testing.T) {
	target := filepath.Join(t.TempDir(), "lovart-connector")
	result, err := Status(Options{ExtensionDir: target})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "missing" || result.Exists {
		t.Fatalf("unexpected missing status: %#v", result)
	}
	if _, err := Install(Options{SourceDir: testExtensionSource(t), ExtensionDir: target}); err != nil {
		t.Fatal(err)
	}
	result, err = Status(Options{ExtensionDir: target})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "installed" || result.ManifestName != "Lovart Connector" || result.Version != "0.1.0" {
		t.Fatalf("unexpected installed status: %#v", result)
	}
}

func TestWindowsExtensionDirUsesConvertedPathWhenAvailable(t *testing.T) {
	target := filepath.Join(t.TempDir(), "lovart-connector")
	result, err := Status(Options{
		ExtensionDir: target,
		WindowsPathFunc: func(path string) (string, error) {
			if path != target {
				t.Fatalf("path = %q, want %q", path, target)
			}
			return `\\wsl.localhost\Distro\home\user\.lovart\extension\lovart-connector`, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.WindowsExtensionDir == "" {
		t.Fatalf("missing windows extension dir: %#v", result)
	}
	if !strings.Contains(result.ManualSteps[len(result.ManualSteps)-1], result.WindowsExtensionDir) {
		t.Fatalf("manual steps do not use windows path: %#v", result.ManualSteps)
	}
}

func TestWindowsExtensionDirConversionFailureIsIgnored(t *testing.T) {
	target := filepath.Join(t.TempDir(), "lovart-connector")
	result, err := Status(Options{
		ExtensionDir: target,
		WindowsPathFunc: func(path string) (string, error) {
			return "", errTestOpen
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.WindowsExtensionDir != "" {
		t.Fatalf("unexpected windows extension dir: %#v", result)
	}
	if !strings.Contains(result.ManualSteps[len(result.ManualSteps)-1], target) {
		t.Fatalf("manual steps should fall back to linux path: %#v", result.ManualSteps)
	}
}

var errTestOpen = &testOpenError{}

type testOpenError struct{}

func (e *testOpenError) Error() string { return "open failed" }

func testExtensionSource(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"manifest_version":3,"name":"Lovart Connector","version":"0.1.0"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "src", "background"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "background", "service_worker.js"), []byte("self.addEventListener('install', () => {})\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}
