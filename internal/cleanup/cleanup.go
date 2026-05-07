// Package cleanup previews and removes persistent Lovart runtime data.
package cleanup

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/aporicho/lovart/internal/paths"
)

const (
	ScopeRuns      = "runs"
	ScopeDownloads = "downloads"
	ScopeCache     = "cache"
	ScopeAuth      = "auth"
	ScopeExtension = "extension"
)

// Options selects which persistent runtime data should be previewed or removed.
type Options struct {
	DryRun    bool
	Yes       bool
	Runs      bool
	Downloads bool
	Cache     bool
	Auth      bool
	Extension bool
	All       bool
}

// Item describes one filesystem target in a clean operation.
type Item struct {
	Scope   string `json:"scope"`
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
	Bytes   int64  `json:"bytes"`
	Removed bool   `json:"removed"`
}

// Result is the machine-readable clean report.
type Result struct {
	Root       string   `json:"root"`
	DryRun     bool     `json:"dry_run"`
	Deleted    bool     `json:"deleted"`
	Scopes     []string `json:"scopes"`
	Items      []Item   `json:"items"`
	TotalBytes int64    `json:"total_bytes"`
}

type target struct {
	scope string
	path  string
}

// Run previews or removes the selected Lovart runtime data.
func Run(opts Options) (Result, error) {
	scopes := selectedScopes(opts)
	if len(scopes) == 0 {
		return Result{}, fmt.Errorf("cleanup: no scopes selected")
	}
	if !opts.DryRun && !opts.Yes {
		return Result{}, fmt.Errorf("cleanup: --yes is required to delete data")
	}

	result := Result{
		Root:    paths.Root,
		DryRun:  opts.DryRun,
		Deleted: !opts.DryRun,
		Scopes:  scopes,
	}
	for _, item := range selectedTargets(scopes) {
		report, err := inspectTarget(item)
		if err != nil {
			return result, err
		}
		if !opts.DryRun && report.Exists {
			if err := os.RemoveAll(item.path); err != nil {
				return result, fmt.Errorf("cleanup: remove %s: %w", item.path, err)
			}
			report.Removed = true
		}
		result.TotalBytes += report.Bytes
		result.Items = append(result.Items, report)
	}
	return result, nil
}

func selectedScopes(opts Options) []string {
	enabled := map[string]bool{}
	if opts.All {
		for _, scope := range []string{ScopeRuns, ScopeDownloads, ScopeCache, ScopeAuth, ScopeExtension} {
			enabled[scope] = true
		}
	} else {
		if opts.Runs {
			enabled[ScopeRuns] = true
		}
		if opts.Downloads {
			enabled[ScopeDownloads] = true
		}
		if opts.Cache {
			enabled[ScopeCache] = true
		}
		if opts.Auth {
			enabled[ScopeAuth] = true
		}
		if opts.Extension {
			enabled[ScopeExtension] = true
		}
	}

	scopes := make([]string, 0, len(enabled))
	for _, scope := range []string{ScopeRuns, ScopeDownloads, ScopeCache, ScopeAuth, ScopeExtension} {
		if enabled[scope] {
			scopes = append(scopes, scope)
		}
	}
	return scopes
}

func selectedTargets(scopes []string) []target {
	var targets []target
	for _, scope := range scopes {
		switch scope {
		case ScopeRuns:
			targets = append(targets, target{scope: scope, path: paths.RunsDir})
		case ScopeDownloads:
			targets = append(targets, target{scope: scope, path: paths.DownloadsDir})
		case ScopeCache:
			targets = append(targets,
				target{scope: scope, path: paths.MetadataDir},
				target{scope: scope, path: paths.SignerDir},
				target{scope: scope, path: paths.TmpDir},
			)
		case ScopeAuth:
			targets = append(targets, target{scope: scope, path: paths.CredsFile})
		case ScopeExtension:
			targets = append(targets, target{scope: scope, path: paths.ExtensionDir})
		}
	}
	sort.SliceStable(targets, func(i, j int) bool {
		if targets[i].scope == targets[j].scope {
			return targets[i].path < targets[j].path
		}
		return targets[i].scope < targets[j].scope
	})
	return targets
}

func inspectTarget(item target) (Item, error) {
	report := Item{Scope: item.scope, Path: item.path}
	info, err := os.Lstat(item.path)
	if os.IsNotExist(err) {
		return report, nil
	}
	if err != nil {
		return report, fmt.Errorf("cleanup: inspect %s: %w", item.path, err)
	}
	report.Exists = true
	if !info.IsDir() {
		report.Bytes = info.Size()
		return report, nil
	}
	size, err := dirSize(item.path)
	if err != nil {
		return report, err
	}
	report.Bytes = size
	return report, nil
}

func dirSize(path string) (int64, error) {
	var total int64
	err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("cleanup: measure %s: %w", path, err)
	}
	return total, nil
}
