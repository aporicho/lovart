package lovart_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestReleaseWorkflowBuildsGoBinaries(t *testing.T) {
	text := readRepoFile(t, ".github/workflows/release-binaries.yml")
	for _, want := range []string{
		"actions/setup-go@v5",
		"go-version-file: go.mod",
		"go test ./...",
		"go build -trimpath",
		"lovart-macos-arm64",
		"lovart-linux-x64",
		"lovart-windows-x64.exe",
		"install.sh",
		"install.ps1",
		"SHA256SUMS",
		`method":"tools/list`,
		"softprops/action-gh-release@v2",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("release workflow missing %q", want)
		}
	}
}

func TestMakeReleaseBuildsLocalAssetsWithoutGoReleaser(t *testing.T) {
	text := readRepoFile(t, "Makefile")
	for _, want := range []string{
		"lovart-macos-arm64",
		"lovart-linux-x64",
		"lovart-windows-x64.exe",
		"packaging/install/install.sh",
		"SHA256SUMS",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("Makefile release target missing %q", want)
		}
	}
	if strings.Contains(text, "goreleaser release") {
		t.Fatal("Makefile release target should not depend on missing GoReleaser config")
	}
}

func TestInstallShDryRunJSONMapsCurrentPlatform(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash installer dry-run is covered on Unix hosts")
	}
	if runtime.GOOS == "darwin" && runtime.GOARCH != "arm64" {
		t.Skip("release installer currently publishes macOS arm64 only")
	}
	result := exec.Command("bash", "packaging/install/install.sh", "--dry-run", "--json")
	out, err := result.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh dry-run failed: %v\n%s", err, out)
	}
	var payload struct {
		OK   bool `json:"ok"`
		Data struct {
			Asset      string `json:"asset"`
			MCPClients string `json:"mcp_clients"`
			DryRun     bool   `json:"dry_run"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("invalid dry-run JSON: %v\n%s", err, out)
	}
	if !payload.OK || !payload.Data.DryRun || payload.Data.MCPClients != "auto" {
		t.Fatalf("unexpected dry-run payload: %#v", payload)
	}
	wantAssets := map[string]string{
		"darwin": "lovart-macos-arm64",
		"linux":  "lovart-linux-x64",
	}
	if want, ok := wantAssets[runtime.GOOS]; ok && payload.Data.Asset != want {
		t.Fatalf("asset = %q, want %q", payload.Data.Asset, want)
	}
}

func TestInstallScriptsConfigureMCPAndCheckJSON(t *testing.T) {
	shellScript := readRepoFile(t, "packaging/install/install.sh")
	powershellScript := readRepoFile(t, "packaging/install/install.ps1")
	for _, want := range []string{
		"release_asset_url",
		"releases/latest/download",
		"curl -fL",
		"gh auth status",
		"gh release download",
		"private forks or API-limited access",
		"--mcp-clients",
		`"mcp" "install"`,
		`grep -q '"ok":true'`,
	} {
		if !strings.Contains(shellScript, want) {
			t.Fatalf("install.sh missing %q", want)
		}
	}
	if strings.Index(shellScript, "curl -fL") > strings.Index(shellScript, "gh auth status") {
		t.Fatal("install.sh should try public curl downloads before authenticated gh fallback")
	}
	for _, want := range []string{
		"Get-PublicAssetUrl",
		"Invoke-WebRequest",
		"Get-FileHash -Algorithm SHA256",
		"lovart-windows-x64.exe",
		"McpClients",
		"private forks or API-limited access",
		`"mcp", "install"`,
		"ConvertFrom-Json",
	} {
		if !strings.Contains(powershellScript, want) {
			t.Fatalf("install.ps1 missing %q", want)
		}
	}
	if strings.Index(powershellScript, "Invoke-WebRequest") > strings.Index(powershellScript, "gh auth status") {
		t.Fatal("install.ps1 should try public Invoke-WebRequest downloads before authenticated gh fallback")
	}
}

func TestVersionMetadataSupportsLDFlags(t *testing.T) {
	text := readRepoFile(t, "internal/version/version.go")
	if !strings.Contains(text, "var (") || !strings.Contains(text, `Version = "2.0.0-dev"`) {
		t.Fatalf("version metadata should remain mutable for release ldflags")
	}
}

func readRepoFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
