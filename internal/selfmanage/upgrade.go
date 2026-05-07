package selfmanage

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aporicho/lovart/internal/paths"
	"github.com/aporicho/lovart/internal/version"
)

const extensionAssetName = "lovart-connector-extension.zip"

// UpgradeOptions controls the Lovart self-upgrade flow.
type UpgradeOptions struct {
	Repo         string
	Version      string
	InstallPath  string
	ExtensionDir string
	NoExtension  bool
	CheckOnly    bool
	DryRun       bool
	Yes          bool
	Force        bool

	ExecutablePath string
	CurrentVersion string
	APIBaseURL     string
	HTTPClient     *http.Client
	RuntimeGOOS    string
	RuntimeGOARCH  string
	RunSelfTest    func(path string) error
}

// UpgradeResult is the JSON payload returned by `lovart upgrade`.
type UpgradeResult struct {
	Repo               string   `json:"repo"`
	RequestedVersion   string   `json:"requested_version"`
	ResolvedVersion    string   `json:"resolved_version,omitempty"`
	CurrentVersion     string   `json:"current_version,omitempty"`
	Platform           string   `json:"platform"`
	Asset              string   `json:"asset,omitempty"`
	InstallPath        string   `json:"install_path,omitempty"`
	ExtensionAsset     string   `json:"extension_asset,omitempty"`
	ExtensionDir       string   `json:"extension_dir,omitempty"`
	CheckOnly          bool     `json:"check_only"`
	DryRun             bool     `json:"dry_run"`
	Upgraded           bool     `json:"upgraded"`
	ExtensionUpdated   bool     `json:"extension_updated"`
	UpdateAvailable    bool     `json:"update_available"`
	ManualRequired     bool     `json:"manual_required"`
	RecommendedActions []string `json:"recommended_actions,omitempty"`
}

// Upgrade checks, previews, or applies a Lovart binary upgrade.
func Upgrade(ctx context.Context, opts UpgradeOptions) (UpgradeResult, error) {
	opts = normalizeUpgradeOptions(opts)
	goos, goarch := runtimePlatform(opts.RuntimeGOOS, opts.RuntimeGOARCH)
	platform := goos + "/" + goarch
	result := UpgradeResult{
		Repo:             opts.Repo,
		RequestedVersion: opts.Version,
		CurrentVersion:   opts.CurrentVersion,
		Platform:         platform,
		CheckOnly:        opts.CheckOnly,
		DryRun:           opts.DryRun,
	}
	assetName, err := platformAsset(goos, goarch)
	if err != nil {
		return result, err
	}
	result.Asset = assetName

	installPath, explicitInstallPath, err := executablePath(opts.InstallPath, opts.ExecutablePath)
	if err != nil {
		return result, err
	}
	result.InstallPath = installPath
	if !explicitInstallPath && looksLikeGoRun(installPath) {
		return result, inputError("cannot upgrade a go run temporary binary", map[string]any{
			"install_path":        installPath,
			"recommended_actions": []string{"rerun with --install-path pointing at the installed lovart binary"},
		})
	}
	if !opts.NoExtension {
		result.ExtensionAsset = extensionAssetName
		result.ExtensionDir = opts.ExtensionDir
	}

	client := newReleaseClient(opts.HTTPClient, opts.APIBaseURL)
	release, err := client.fetch(ctx, opts.Repo, opts.Version)
	if err != nil {
		return result, err
	}
	result.ResolvedVersion = release.TagName
	result.UpdateAvailable = opts.CurrentVersion == "" || opts.CurrentVersion != release.TagName
	binaryAsset, err := releaseAsset(release, assetName)
	if err != nil {
		return result, err
	}
	sumsAsset, err := releaseAsset(release, "SHA256SUMS")
	if err != nil {
		return result, err
	}
	var extensionAsset ReleaseAsset
	if !opts.NoExtension {
		extensionAsset, err = releaseAsset(release, extensionAssetName)
		if err != nil {
			return result, err
		}
	}
	if opts.CheckOnly || opts.DryRun {
		return result, nil
	}
	if goos == "windows" {
		result.ManualRequired = true
		result.RecommendedActions = []string{"run the PowerShell install script with -Force to upgrade on Windows"}
		return result, nil
	}
	if !result.UpdateAvailable && !opts.Force {
		result.RecommendedActions = []string{"already on the requested version; rerun with --force to reinstall"}
		return result, nil
	}
	if !opts.Yes {
		return result, inputError("upgrade requires --yes", map[string]any{
			"recommended_actions": []string{"rerun with --dry-run to preview", "rerun with --yes to upgrade"},
		})
	}
	if _, err := os.Stat(installPath); os.IsNotExist(err) && !opts.Force {
		return result, inputError("install path does not exist", map[string]any{
			"install_path":        installPath,
			"recommended_actions": []string{"rerun with --force to install to this path", "rerun with --install-path pointing at the installed lovart binary"},
		})
	} else if err != nil && !os.IsNotExist(err) {
		return result, internalError("inspect install path", map[string]any{"path": installPath, "error": err.Error()})
	}

	stage, err := os.MkdirTemp(paths.TmpDir, "upgrade-")
	if err != nil {
		return result, internalError("create upgrade staging directory", map[string]any{"path": paths.TmpDir, "error": err.Error()})
	}
	defer os.RemoveAll(stage)

	binaryPath := filepath.Join(stage, assetName)
	sumsPath := filepath.Join(stage, "SHA256SUMS")
	if err := client.downloadFile(ctx, binaryAsset.DownloadURL, binaryPath); err != nil {
		return result, err
	}
	if err := client.downloadFile(ctx, sumsAsset.DownloadURL, sumsPath); err != nil {
		return result, err
	}
	expected, err := checksumForAsset(sumsPath, assetName)
	if err != nil {
		return result, err
	}
	if err := verifySHA256(binaryPath, expected); err != nil {
		return result, err
	}
	if err := os.Chmod(binaryPath, 0755); err != nil {
		return result, internalError("mark downloaded binary executable", map[string]any{"path": binaryPath, "error": err.Error()})
	}
	if err := selfTestRunner(opts.RunSelfTest)(binaryPath); err != nil {
		return result, internalError("upgraded binary self-test failed", map[string]any{"path": binaryPath, "error": err.Error()})
	}

	if !opts.NoExtension {
		extensionPath := filepath.Join(stage, extensionAssetName)
		if err := client.downloadFile(ctx, extensionAsset.DownloadURL, extensionPath); err != nil {
			return result, err
		}
		expected, err := checksumForAsset(sumsPath, extensionAssetName)
		if err != nil {
			return result, err
		}
		if err := verifySHA256(extensionPath, expected); err != nil {
			return result, err
		}
	}
	if err := installBinary(binaryPath, installPath); err != nil {
		return result, err
	}
	result.Upgraded = true
	if !opts.NoExtension {
		extensionPath := filepath.Join(stage, extensionAssetName)
		if err := replaceExtension(extensionPath, opts.ExtensionDir, stage); err != nil {
			return result, err
		}
		result.ExtensionUpdated = true
	}
	if result.ExtensionUpdated {
		result.RecommendedActions = []string{"reload Lovart Connector in chrome://extensions"}
	}
	return result, nil
}

func normalizeUpgradeOptions(opts UpgradeOptions) UpgradeOptions {
	if opts.Repo == "" {
		opts.Repo = DefaultRepo
	}
	if opts.Version == "" {
		opts.Version = "latest"
	}
	if opts.ExtensionDir == "" {
		opts.ExtensionDir = paths.ExtensionDir
	} else {
		if normalized, err := normalizePath(opts.ExtensionDir); err == nil {
			opts.ExtensionDir = normalized
		}
	}
	if opts.CurrentVersion == "" {
		opts.CurrentVersion = version.Version
	}
	return opts
}

func selfTestRunner(runner func(path string) error) func(path string) error {
	if runner != nil {
		return runner
	}
	return func(path string) error {
		for _, args := range [][]string{{"--version"}, {"self-test"}} {
			cmd := exec.Command(path, args...)
			var stderr bytes.Buffer
			cmd.Stdout = io.Discard
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				msg := strings.TrimSpace(stderr.String())
				if msg != "" {
					return internalError("run upgraded binary self-test", map[string]any{"error": err.Error(), "stderr": msg})
				}
				return err
			}
		}
		return nil
	}
}
