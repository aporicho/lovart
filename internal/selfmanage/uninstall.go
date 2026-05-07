package selfmanage

import (
	"os"
	"path/filepath"

	"github.com/aporicho/lovart/internal/paths"
)

// UninstallOptions controls local Lovart removal.
type UninstallOptions struct {
	DryRun        bool
	Yes           bool
	Data          bool
	KeepExtension bool
	InstallPath   string
	ExtensionDir  string

	ExecutablePath string
	RuntimeGOOS    string
	RuntimeGOARCH  string
}

// UninstallItem describes one local target that may be removed.
type UninstallItem struct {
	Scope   string `json:"scope"`
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
	Removed bool   `json:"removed"`
}

// UninstallResult is the JSON payload returned by `lovart uninstall`.
type UninstallResult struct {
	DryRun             bool            `json:"dry_run"`
	Deleted            bool            `json:"deleted"`
	ManualRequired     bool            `json:"manual_required"`
	Platform           string          `json:"platform"`
	InstallPath        string          `json:"install_path,omitempty"`
	ExtensionDir       string          `json:"extension_dir,omitempty"`
	RuntimeRoot        string          `json:"runtime_root,omitempty"`
	Items              []UninstallItem `json:"items"`
	RecommendedActions []string        `json:"recommended_actions,omitempty"`
}

// Uninstall removes local Lovart files selected by opts.
func Uninstall(opts UninstallOptions) (UninstallResult, error) {
	opts = normalizeUninstallOptions(opts)
	goos, goarch := runtimePlatform(opts.RuntimeGOOS, opts.RuntimeGOARCH)
	result := UninstallResult{
		DryRun:       opts.DryRun,
		Deleted:      !opts.DryRun,
		Platform:     goos + "/" + goarch,
		ExtensionDir: opts.ExtensionDir,
		RuntimeRoot:  paths.Root,
	}
	installPath, explicitInstallPath, err := executablePath(opts.InstallPath, opts.ExecutablePath)
	if err != nil {
		return result, err
	}
	result.InstallPath = installPath
	if !explicitInstallPath && looksLikeGoRun(installPath) {
		return result, inputError("cannot uninstall a go run temporary binary", map[string]any{
			"install_path":        installPath,
			"recommended_actions": []string{"rerun with --install-path pointing at the installed lovart binary"},
		})
	}
	if !opts.DryRun && !opts.Yes {
		return result, inputError("uninstall requires --yes", map[string]any{
			"recommended_actions": []string{"rerun with --dry-run to preview", "rerun with --yes to uninstall"},
		})
	}
	result.Items = append(result.Items, inspectUninstallItem("binary", installPath))
	if !opts.KeepExtension {
		result.Items = append(result.Items, inspectUninstallItem("extension", opts.ExtensionDir))
	}
	if opts.Data {
		result.Items = append(result.Items, inspectUninstallItem("data", paths.Root))
	}
	if goos == "windows" && !opts.DryRun {
		result.Deleted = false
		result.ManualRequired = true
		result.RecommendedActions = []string{"close running lovart processes, delete lovart.exe manually, then remove the extension and data directories if desired"}
		return result, nil
	}
	if opts.DryRun {
		result.Deleted = false
		return result, nil
	}
	for i := range result.Items {
		if !result.Items[i].Exists {
			continue
		}
		if err := removeUninstallTarget(result.Items[i]); err != nil {
			return result, err
		}
		result.Items[i].Removed = true
	}
	return result, nil
}

func normalizeUninstallOptions(opts UninstallOptions) UninstallOptions {
	if opts.ExtensionDir == "" {
		opts.ExtensionDir = paths.ExtensionDir
	} else if normalized, err := normalizePath(opts.ExtensionDir); err == nil {
		opts.ExtensionDir = normalized
	}
	return opts
}

func inspectUninstallItem(scope string, path string) UninstallItem {
	_, err := os.Lstat(path)
	return UninstallItem{Scope: scope, Path: path, Exists: err == nil}
}

func removeUninstallTarget(item UninstallItem) error {
	switch item.Scope {
	case "binary":
		if err := os.Remove(item.Path); err != nil && !os.IsNotExist(err) {
			return internalError("remove lovart binary", map[string]any{"path": item.Path, "error": err.Error()})
		}
	default:
		if err := os.RemoveAll(filepath.Clean(item.Path)); err != nil {
			return internalError("remove lovart data", map[string]any{"path": item.Path, "error": err.Error(), "scope": item.Scope})
		}
	}
	return nil
}
