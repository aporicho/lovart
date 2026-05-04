package project

import (
	"context"
	"fmt"

	"github.com/aporicho/lovart/internal/http"
)

// createProjectResponse mirrors the saveProject API response.
type createProjectResponse struct {
	Code int `json:"code"`
	Data struct {
		ProjectID      string `json:"projectId"`
		NewCreated     bool   `json:"newCreated"`
		ValidProjectID bool   `json:"validProjectId"`
		Version        string `json:"version"`
	} `json:"data"`
}

// Create makes a new Lovart project. If name is non-empty, renames after creation.
func Create(ctx context.Context, client *http.Client, cid, name string) (*Project, error) {
	path := "/api/canva/project/saveProject"

	body := map[string]any{
		"projectType": 3,
	}
	if cid != "" {
		body["cid"] = cid
	}

	var resp createProjectResponse
	if err := client.PostJSON(ctx, http.WWWBase, path, body, &resp); err != nil {
		return nil, fmt.Errorf("project: create: %w", err)
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("project: create returned code %d", resp.Code)
	}

	p := &Project{
		ID:   resp.Data.ProjectID,
		Name: name,
	}

	// If a name was requested, rename the new project.
	if name != "" && name != "Untitled" {
		if err := Rename(ctx, client, resp.Data.ProjectID, name); err != nil {
			return p, fmt.Errorf("project: created but rename failed: %w", err)
		}
	}

	return p, nil
}
