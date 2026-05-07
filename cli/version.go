package cli

import (
	"github.com/aporicho/lovart/internal/version"
)

func versionData() map[string]any {
	return version.Data()
}
