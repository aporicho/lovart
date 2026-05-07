package selfmanage

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	lovarterrors "github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/paths"
)

func TestUpgradeDownloadsVerifiesAndReplacesBinaryAndExtension(t *testing.T) {
	resetSelfManageRoot(t)
	binary := []byte("#!/bin/sh\necho upgraded\n")
	extensionZip := buildExtensionZip(t)
	server := releaseTestServer(t, "v1.2.3", map[string][]byte{
		"lovart-linux-x64": binary,
		extensionAssetName: extensionZip,
		"SHA256SUMS":       []byte(checksumLines(map[string][]byte{"lovart-linux-x64": binary, extensionAssetName: extensionZip})),
	})
	target := filepath.Join(t.TempDir(), "lovart")
	if err := os.WriteFile(target, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}
	extensionDir := filepath.Join(t.TempDir(), "lovart-connector")

	result, err := Upgrade(t.Context(), UpgradeOptions{
		Repo:           DefaultRepo,
		Version:        "latest",
		InstallPath:    target,
		ExtensionDir:   extensionDir,
		Yes:            true,
		CurrentVersion: "v1.0.0",
		APIBaseURL:     server.URL + "/api",
		HTTPClient:     server.Client(),
		RuntimeGOOS:    "linux",
		RuntimeGOARCH:  "amd64",
		RunSelfTest:    func(path string) error { return nil },
	})
	if err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	if !result.Upgraded || !result.ExtensionUpdated || result.ResolvedVersion != "v1.2.3" {
		t.Fatalf("unexpected result: %#v", result)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(binary) {
		t.Fatalf("binary = %q", string(got))
	}
	if _, err := os.Stat(filepath.Join(extensionDir, "manifest.json")); err != nil {
		t.Fatalf("extension not replaced: %v", err)
	}
}

func TestUpgradeDryRunDoesNotDownloadOrWrite(t *testing.T) {
	resetSelfManageRoot(t)
	downloads := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/releases/latest") {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"tag_name":"v1.2.3","assets":[{"name":"lovart-linux-x64","browser_download_url":"http://` + r.Host + `/asset"},{"name":"SHA256SUMS","browser_download_url":"http://` + r.Host + `/sums"}]}`))
			return
		}
		downloads++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	target := filepath.Join(t.TempDir(), "lovart")
	if err := os.WriteFile(target, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}
	result, err := Upgrade(t.Context(), UpgradeOptions{
		InstallPath:    target,
		DryRun:         true,
		NoExtension:    true,
		CurrentVersion: "v1.0.0",
		APIBaseURL:     server.URL,
		HTTPClient:     server.Client(),
		RuntimeGOOS:    "linux",
		RuntimeGOARCH:  "amd64",
	})
	if err != nil {
		t.Fatalf("Upgrade dry-run: %v", err)
	}
	if !result.DryRun || result.Upgraded || downloads != 0 {
		t.Fatalf("unexpected dry-run result=%#v downloads=%d", result, downloads)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "old" {
		t.Fatalf("dry-run wrote target: %q", got)
	}
}

func TestUpgradeChecksumMismatchReturnsInputError(t *testing.T) {
	resetSelfManageRoot(t)
	binary := []byte("new")
	server := releaseTestServer(t, "v1.2.3", map[string][]byte{
		"lovart-linux-x64": binary,
		"SHA256SUMS":       []byte("0000  lovart-linux-x64\n"),
	})
	target := filepath.Join(t.TempDir(), "lovart")
	if err := os.WriteFile(target, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}
	_, err := Upgrade(t.Context(), UpgradeOptions{
		InstallPath:    target,
		Yes:            true,
		NoExtension:    true,
		CurrentVersion: "v1.0.0",
		APIBaseURL:     server.URL + "/api",
		HTTPClient:     server.Client(),
		RuntimeGOOS:    "linux",
		RuntimeGOARCH:  "amd64",
		RunSelfTest:    func(path string) error { return nil },
	})
	if err == nil || !strings.Contains(err.Error(), lovarterrors.CodeInputError) {
		t.Fatalf("expected input checksum error, got %v", err)
	}
}

func releaseTestServer(t *testing.T, tag string, assets map[string][]byte) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/releases/latest") || strings.Contains(r.URL.Path, "/releases/tags/") {
			w.Header().Set("Content-Type", "application/json")
			var b strings.Builder
			b.WriteString(`{"tag_name":"` + tag + `","assets":[`)
			first := true
			for name := range assets {
				if !first {
					b.WriteByte(',')
				}
				first = false
				b.WriteString(`{"name":"` + name + `","browser_download_url":"http://`)
				b.WriteString(r.Host)
				b.WriteString(`/assets/` + name + `"}`)
			}
			b.WriteString(`]}`)
			w.Write([]byte(b.String()))
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/assets/")
		data, ok := assets[name]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write(data)
	}))
	return server
}

func checksumLines(assets map[string][]byte) string {
	var b strings.Builder
	for name, data := range assets {
		sum := sha256.Sum256(data)
		b.WriteString(hex.EncodeToString(sum[:]))
		b.WriteString("  ")
		b.WriteString(name)
		b.WriteByte('\n')
	}
	return b.String()
}

func buildExtensionZip(t *testing.T) []byte {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "extension-*.zip")
	if err != nil {
		t.Fatal(err)
	}
	zipper := zip.NewWriter(file)
	entry, err := zipper.Create("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	entry.Write([]byte(`{"manifest_version":3}`))
	if err := zipper.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func resetSelfManageRoot(t *testing.T) {
	t.Helper()
	t.Setenv("LOVART_HOME", t.TempDir())
	t.Cleanup(paths.Reset)
	paths.Reset()
	if err := paths.EnsureRuntimeDirs(); err != nil {
		t.Fatal(err)
	}
}
