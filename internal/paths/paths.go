// Package paths resolves runtime directories for credentials, downloads, and state.
package paths

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultHome = ".lovart"
	envHome     = "LOVART_HOME"
)

// Resolved directories.
var (
	Root                 string
	CredsFile            string
	RunsDir              string
	DownloadsDir         string
	MetadataDir          string
	GeneratorListFile    string
	GeneratorSchemaFile  string
	MetadataManifestFile string
	SignerDir            string
	SignerWASMFile       string
	SignerManifestFile   string
	TmpDir               string
	ExtensionDir         string
)

func init() {
	resetPaths()
}

// Reset re-evaluates runtime paths. Used by tests after changing env vars.
func Reset() {
	resetPaths()
}

// PrepareRuntime creates runtime directories and removes stale intermediate files.
func PrepareRuntime() error {
	if err := EnsureRuntimeDirs(); err != nil {
		return err
	}
	return CleanupIntermediateFiles(24 * time.Hour)
}

// EnsureRuntimeDirs creates the directories owned by the Lovart runtime.
func EnsureRuntimeDirs() error {
	for _, dir := range []string{Root, MetadataDir, SignerDir, RunsDir, DownloadsDir, TmpDir, filepath.Dir(ExtensionDir)} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("paths: create %s: %w", dir, err)
		}
	}
	if err := os.Chmod(Root, 0700); err != nil {
		return fmt.Errorf("paths: chmod %s: %w", Root, err)
	}
	return nil
}

// CleanupIntermediateFiles removes stale temporary files from the runtime root.
func CleanupIntermediateFiles(maxAge time.Duration) error {
	now := time.Now()
	if pathExists(TmpDir) {
		entries, err := os.ReadDir(TmpDir)
		if err != nil {
			return fmt.Errorf("paths: read tmp dir: %w", err)
		}
		for _, entry := range entries {
			path := filepath.Join(TmpDir, entry.Name())
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if now.Sub(info.ModTime()) > maxAge {
				if err := os.RemoveAll(path); err != nil {
					return fmt.Errorf("paths: remove stale tmp %s: %w", path, err)
				}
			}
		}
	}
	if !pathExists(Root) {
		return nil
	}
	return filepath.WalkDir(Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !isAtomicTempName(d.Name()) {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if now.Sub(info.ModTime()) <= maxAge {
			return nil
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("paths: remove stale intermediate %s: %w", path, err)
		}
		return nil
	})
}

func resetPaths() {
	if env := os.Getenv(envHome); env != "" {
		Root = filepath.Clean(expandHome(env))
	} else {
		home, _ := os.UserHomeDir()
		Root = filepath.Join(home, defaultHome)
	}

	CredsFile = filepath.Join(Root, "creds.json")
	MetadataDir = filepath.Join(Root, "metadata")
	GeneratorListFile = filepath.Join(MetadataDir, "generator_list.json")
	GeneratorSchemaFile = filepath.Join(MetadataDir, "generator_schema.json")
	MetadataManifestFile = filepath.Join(MetadataDir, "manifest.json")
	SignerDir = filepath.Join(Root, "signing")
	SignerWASMFile = filepath.Join(SignerDir, "current.wasm")
	SignerManifestFile = filepath.Join(SignerDir, "manifest.json")
	RunsDir = filepath.Join(Root, "runs")
	DownloadsDir = filepath.Join(Root, "downloads")
	TmpDir = filepath.Join(Root, "tmp")
	ExtensionDir = filepath.Join(Root, "extension", "lovart-connector")
}

func isAtomicTempName(name string) bool {
	return strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".tmp")
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
