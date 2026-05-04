// Package paths resolves runtime directories for credentials, downloads, and state.
package paths

import (
	"os"
	"path/filepath"
)

// Default user root for Lovart runtime data.
const defaultHome = ".lovart-reverse"

// Resolved directories.
var (
	Root        string
	CredsFile   string
	RunsDir     string
	DownloadsDir string
)

func init() {
	resetPaths()
}

// Reset re-evaluates runtime paths. Used by tests after changing env vars.
func Reset() {
	resetPaths()
}

func resetPaths() {
	// LOVART_REVERSE_ROOT overrides the entire runtime tree.
	if env := os.Getenv("LOVART_REVERSE_ROOT"); env != "" {
		Root = env
	} else if repo := findRepoRoot(); repo != "" {
		Root = repo
	} else {
		home, _ := os.UserHomeDir()
		Root = filepath.Join(home, defaultHome)
	}

	dotDir := filepath.Join(Root, ".lovart")
	os.MkdirAll(dotDir, 0700)

	CredsFile = filepath.Join(dotDir, "creds.json")
	RunsDir = filepath.Join(Root, "runs")
	DownloadsDir = filepath.Join(Root, "downloads")

	os.MkdirAll(RunsDir, 0755)
	os.MkdirAll(DownloadsDir, 0755)
}

// findRepoRoot returns the repo root if go.mod exists in the parent chain.
func findRepoRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}
