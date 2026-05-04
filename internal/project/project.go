// Package project manages Lovart project context for CLI generation.
// Generation tasks must be scoped to a project to appear in the web UI.
package project

import (
	"github.com/aporicho/lovart/internal/auth"
)

// Project represents a Lovart project (canvas workspace).
type Project struct {
	ID        string `json:"project_id"`
	Name      string `json:"name"`
	CanvasURL string `json:"canvas_url"`
}

// Current returns the active project context from saved credentials.
func Current() (*auth.ProjectContext, error) {
	return auth.LoadProjectContext()
}

// List returns known projects.
// TODO: implement when Lovart project list API is captured.
func List() ([]Project, error) {
	pc, err := auth.LoadProjectContext()
	if err != nil || pc.ProjectID == "" {
		return nil, nil
	}
	return []Project{{ID: pc.ProjectID, Name: "current project"}}, nil
}

// Set saves the project context to credentials.
// TODO: implement when capture extracts more project info.
func Set(projectID, cid string) error {
	return nil
}
