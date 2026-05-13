// Package connector manages the local Lovart Connector browser extension files.
package connector

import (
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	internalbrowser "github.com/aporicho/lovart/internal/browser"
	lovarterrors "github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/paths"
)

const ChromeExtensionsURL = "chrome://extensions/"

// OpenFunc opens a URL in a browser.
type OpenFunc func(string) error

// Options controls extension status, install, and open operations.
type Options struct {
	SourceDir       string
	ExtensionDir    string
	DryRun          bool
	Open            bool
	OpenURL         OpenFunc
	WindowsPathFunc PathConvertFunc
}

// PathConvertFunc converts a Linux path into a Windows-readable path when
// running under WSL.
type PathConvertFunc func(string) (string, error)

// Result is the safe JSON payload returned by extension operations.
type Result struct {
	Status              string   `json:"status"`
	Exists              bool     `json:"exists"`
	Installed           bool     `json:"installed"`
	DryRun              bool     `json:"dry_run"`
	SourceDir           string   `json:"source_dir,omitempty"`
	ExtensionDir        string   `json:"extension_dir"`
	WindowsExtensionDir string   `json:"windows_extension_dir,omitempty"`
	ManifestPath        string   `json:"manifest_path"`
	ManifestName        string   `json:"manifest_name,omitempty"`
	Version             string   `json:"version,omitempty"`
	ChromeURL           string   `json:"chrome_url"`
	OpenedBrowser       bool     `json:"opened_browser"`
	OpenError           string   `json:"open_error,omitempty"`
	ManualSteps         []string `json:"manual_steps"`
}

type manifestInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Status reports whether the local unpacked extension directory is ready.
func Status(opts Options) (Result, error) {
	extensionDir, err := extensionDir(opts.ExtensionDir)
	if err != nil {
		return Result{}, err
	}
	result := baseResult(extensionDir, opts.WindowsPathFunc)
	info, err := readManifest(result.ManifestPath)
	if err != nil {
		result.Status = "missing"
		return result, nil
	}
	result.Exists = true
	result.Installed = true
	result.Status = "installed"
	result.ManifestName = info.Name
	result.Version = info.Version
	return result, nil
}

// Install copies or verifies the unpacked Lovart Connector extension directory.
func Install(opts Options) (Result, error) {
	extensionDir, err := extensionDir(opts.ExtensionDir)
	if err != nil {
		return Result{}, err
	}
	sourceDir, err := sourceDir(opts.SourceDir, extensionDir)
	if err != nil {
		return Result{}, err
	}
	result := baseResult(extensionDir, opts.WindowsPathFunc)
	result.SourceDir = sourceDir
	result.DryRun = opts.DryRun

	info, err := validateSource(sourceDir)
	if err != nil {
		return result, err
	}
	result.ManifestName = info.Name
	result.Version = info.Version
	result.Exists = pathExists(result.ManifestPath)

	sameDir := samePath(sourceDir, extensionDir)
	if opts.DryRun {
		result.Status = "dry_run"
		result.Installed = result.Exists || sameDir
		return result, nil
	}
	if !sameDir {
		if err := replaceDirectory(sourceDir, extensionDir); err != nil {
			return result, err
		}
	}
	result.Status = "installed"
	result.Exists = true
	result.Installed = true
	if opts.Open {
		result.OpenedBrowser, result.OpenError = openChromeExtensions(opts.OpenURL)
	}
	return result, nil
}

// Open opens Chrome's extension management UI and returns manual loading steps.
func Open(opts Options) (Result, error) {
	result, err := Status(opts)
	if err != nil {
		return Result{}, err
	}
	result.OpenedBrowser, result.OpenError = openChromeExtensions(opts.OpenURL)
	return result, nil
}

func baseResult(extensionDir string, converter PathConvertFunc) Result {
	windowsPath := windowsExtensionPath(extensionDir, converter)
	selectPath := extensionDir
	if windowsPath != "" {
		selectPath = windowsPath
	}
	return Result{
		Status:              "unknown",
		ExtensionDir:        extensionDir,
		WindowsExtensionDir: windowsPath,
		ManifestPath:        filepath.Join(extensionDir, "manifest.json"),
		ChromeURL:           ChromeExtensionsURL,
		ManualSteps: []string{
			"open chrome://extensions/",
			"enable Developer mode",
			"click Load unpacked",
			"select " + selectPath,
		},
	}
}

func extensionDir(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		value = paths.ExtensionDir
	}
	return normalizePath(value)
}

func sourceDir(explicit string, targetDir string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return normalizePath(explicit)
	}
	candidates := defaultSourceCandidates(targetDir)
	for _, candidate := range candidates {
		if _, err := os.Stat(filepath.Join(candidate, "manifest.json")); err == nil {
			return candidate, nil
		}
	}
	return "", lovarterrors.InputError("Lovart Connector extension source not found", map[string]any{
		"source_candidates": candidates,
		"recommended_actions": []string{
			"run from a source checkout",
			"pass --source-dir pointing at the extension directory",
			"re-run the release installer to unpack extension files",
		},
	})
}

func defaultSourceCandidates(targetDir string) []string {
	candidates := []string{}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, "extension"))
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "extension"))
	}
	candidates = append(candidates, targetDir)
	deduped := make([]string, 0, len(candidates))
	seen := map[string]bool{}
	for _, candidate := range candidates {
		normalized, err := normalizePath(candidate)
		if err != nil || seen[normalized] {
			continue
		}
		seen[normalized] = true
		deduped = append(deduped, normalized)
	}
	return deduped
}

func validateSource(sourceDir string) (manifestInfo, error) {
	info, err := readManifest(filepath.Join(sourceDir, "manifest.json"))
	if err != nil {
		return manifestInfo{}, lovarterrors.InputError("extension manifest not found or invalid", map[string]any{
			"source_dir": sourceDir,
			"error":      err.Error(),
		})
	}
	if !pathExists(filepath.Join(sourceDir, "src")) {
		return manifestInfo{}, lovarterrors.InputError("extension src directory not found", map[string]any{
			"source_dir": sourceDir,
			"path":       filepath.Join(sourceDir, "src"),
		})
	}
	return info, nil
}

func readManifest(path string) (manifestInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return manifestInfo{}, err
	}
	var info manifestInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return manifestInfo{}, err
	}
	if strings.TrimSpace(info.Name) == "" {
		return manifestInfo{}, lovarterrors.InputError("extension manifest missing name", map[string]any{"path": path})
	}
	return info, nil
}

func replaceDirectory(sourceDir string, targetDir string) error {
	if err := os.MkdirAll(filepath.Dir(targetDir), 0755); err != nil {
		return lovarterrors.Internal("create extension parent directory", map[string]any{"path": filepath.Dir(targetDir), "error": err.Error()})
	}
	staging, err := os.MkdirTemp(filepath.Dir(targetDir), "."+filepath.Base(targetDir)+".")
	if err != nil {
		return lovarterrors.Internal("create extension staging directory", map[string]any{"path": targetDir, "error": err.Error()})
	}
	defer os.RemoveAll(staging)
	if err := copyDirectory(sourceDir, staging); err != nil {
		return err
	}
	if err := os.RemoveAll(targetDir); err != nil {
		return lovarterrors.Internal("remove old extension directory", map[string]any{"path": targetDir, "error": err.Error()})
	}
	if err := os.Rename(staging, targetDir); err != nil {
		return lovarterrors.Internal("replace extension directory", map[string]any{"path": targetDir, "error": err.Error()})
	}
	return nil
}

func copyDirectory(sourceDir string, targetDir string) error {
	return filepath.WalkDir(sourceDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return lovarterrors.Internal("read extension source", map[string]any{"path": path, "error": walkErr.Error()})
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return lovarterrors.Internal("resolve extension source path", map[string]any{"path": path, "error": err.Error()})
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(targetDir, rel)
		info, err := entry.Info()
		if err != nil {
			return lovarterrors.Internal("inspect extension source", map[string]any{"path": path, "error": err.Error()})
		}
		if entry.IsDir() {
			if err := os.MkdirAll(target, info.Mode()); err != nil {
				return lovarterrors.Internal("create extension directory", map[string]any{"path": target, "error": err.Error()})
			}
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return lovarterrors.Internal("create extension parent directory", map[string]any{"path": filepath.Dir(target), "error": err.Error()})
		}
		in, err := os.Open(path)
		if err != nil {
			return lovarterrors.Internal("open extension source file", map[string]any{"path": path, "error": err.Error()})
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		if err != nil {
			return lovarterrors.Internal("create extension target file", map[string]any{"path": target, "error": err.Error()})
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		if copyErr != nil {
			return lovarterrors.Internal("copy extension target file", map[string]any{"path": target, "error": copyErr.Error()})
		}
		if closeErr != nil {
			return lovarterrors.Internal("close extension target file", map[string]any{"path": target, "error": closeErr.Error()})
		}
		return nil
	})
}

func openChromeExtensions(opener OpenFunc) (bool, string) {
	if opener == nil {
		opener = internalbrowser.OpenURL
	}
	if err := opener(ChromeExtensionsURL); err != nil {
		return false, err.Error()
	}
	return true, ""
}

func windowsExtensionPath(path string, converter PathConvertFunc) string {
	if converter == nil {
		converter = defaultWindowsPath
	}
	converted, err := converter(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(converted)
}

func defaultWindowsPath(path string) (string, error) {
	if !internalbrowser.IsWSL() {
		return "", nil
	}
	wslpath, err := exec.LookPath("wslpath")
	if err != nil {
		return "", err
	}
	output, err := exec.Command(wslpath, "-w", path).Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func normalizePath(path string) (string, error) {
	abs, err := filepath.Abs(expandHome(path))
	if err != nil {
		return "", lovarterrors.InputError("resolve path", map[string]any{"path": path, "error": err.Error()})
	}
	return abs, nil
}

func expandHome(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func samePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && leftAbs == rightAbs
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
