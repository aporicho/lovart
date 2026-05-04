package project

import (
	"github.com/aporicho/lovart/internal/auth"
)

// Current returns the active project context from saved credentials.
func Current() (*auth.ProjectContext, error) {
	return auth.LoadProjectContext()
}
