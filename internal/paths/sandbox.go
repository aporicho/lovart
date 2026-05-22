package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveWithinBase resolves userPath for local writes.
//
// Relative paths are resolved under base and must remain inside base after
// cleaning and resolving existing symlinks. Absolute paths are treated as an
// explicit user-selected destination and are allowed outside base, but are still
// cleaned and resolved through existing symlinks.
func ResolveWithinBase(base, userPath string) (string, error) {
	if strings.TrimSpace(base) == "" {
		return "", fmt.Errorf("paths: base path is empty")
	}
	if strings.TrimSpace(userPath) == "" {
		return "", fmt.Errorf("paths: writable path is empty")
	}

	if filepath.IsAbs(userPath) {
		resolved, err := resolveExistingPrefix(userPath)
		if err != nil {
			return "", err
		}
		return filepath.Clean(resolved), nil
	}

	resolvedBase, err := resolveExistingPrefix(base)
	if err != nil {
		return "", err
	}
	resolvedBase = filepath.Clean(resolvedBase)

	candidate, err := resolveExistingPrefix(filepath.Join(resolvedBase, userPath))
	if err != nil {
		return "", err
	}
	candidate = filepath.Clean(candidate)
	if !isWithinBase(resolvedBase, candidate) {
		return "", fmt.Errorf("paths: relative path %q escapes base %q", userPath, base)
	}
	return candidate, nil
}

// EnsureWritablePath resolves the destination for a local write using the same
// sandbox rules as ResolveWithinBase. It does not create the path; callers keep
// ownership of directory creation and file permissions.
func EnsureWritablePath(base, userPath string) (string, error) {
	return ResolveWithinBase(base, userPath)
}

func isWithinBase(base, candidate string) bool {
	rel, err := filepath.Rel(base, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}

func resolveExistingPrefix(path string) (string, error) {
	abs, err := filepath.Abs(filepath.Clean(expandHome(path)))
	if err != nil {
		return "", fmt.Errorf("paths: resolve absolute path %q: %w", path, err)
	}

	current := abs
	missing := make([]string, 0)
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("paths: resolve symlink %q: %w", current, err)
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("paths: resolve existing prefix for %q: %w", path, err)
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}
