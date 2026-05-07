package project

import (
	"context"
	"fmt"

	"github.com/aporicho/lovart/internal/http"
)

// Project represents a Lovart canvas project.
type Project struct {
	ID       string `json:"projectId"`
	Name     string `json:"projectName"`
	PicCount int    `json:"picCount"`
	Type     int    `json:"projectType"`
}

// projectListResponse mirrors the Lovart project list API envelope.
type projectListResponse struct {
	Code int `json:"code"`
	Data struct {
		Data    []Project `json:"data"`
		Total   int       `json:"total"`
		HasMore bool      `json:"hasMore"`
	} `json:"data"`
}

// List returns the user's Lovart projects.
func List(ctx context.Context, client *http.Client) ([]Project, error) {
	path := "/api/canva/project/lovartProjectList"

	const pageSize = 50
	var projects []Project
	for page := 1; ; page++ {
		body := map[string]any{
			"page":     page,
			"pageSize": pageSize,
		}

		var resp projectListResponse
		if err := client.PostJSON(ctx, http.WWWBase, path, body, &resp); err != nil {
			return nil, fmt.Errorf("project: list: %w", err)
		}

		if resp.Code != 0 {
			return nil, fmt.Errorf("project: list returned code %d", resp.Code)
		}
		projects = append(projects, resp.Data.Data...)
		if !resp.Data.HasMore || len(resp.Data.Data) == 0 {
			break
		}
	}

	return projects, nil
}

// FindByID returns the project with id from projects.
func FindByID(projects []Project, id string) (*Project, bool) {
	for i := range projects {
		if projects[i].ID == id {
			return &projects[i], true
		}
	}
	return nil, false
}
