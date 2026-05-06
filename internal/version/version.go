// Package version exposes build-time version metadata for CLI output.
package version

import "runtime"

var (
	Package = "lovart"
	Version = "2.0.0-dev"
)

// Data returns the public version payload used by JSON CLI commands.
func Data() map[string]any {
	return map[string]any{
		"package":    Package,
		"version":    Version,
		"go_version": GoVersion(),
	}
}

// GoVersion returns the compiler version used for the current binary.
func GoVersion() string {
	return runtime.Version()
}
