package project

import (
	"fmt"

	"github.com/aporicho/lovart/internal/auth"
)

// Current returns the active project context from saved credentials.
func Current() (*auth.ProjectContext, error) {
	return auth.LoadProjectContext()
}

// CanvasURL returns the browser URL for a Lovart project canvas.
func CanvasURL(projectID string) string {
	return fmt.Sprintf("https://www.lovart.ai/canvas?projectId=%s", projectID)
}
