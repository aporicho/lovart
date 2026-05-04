package project

import (
	"context"
	"fmt"

	"github.com/aporicho/lovart/internal/http"
)

// Query returns detailed information about a single Lovart project.
func Query(ctx context.Context, client *http.Client, projectID, cid string) (*Project, error) {
	path := "/api/canva/project/queryProject"

	body := map[string]any{
		"projectId": projectID,
		"cid":       cid,
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			ProjectName string `json:"projectName"`
			ProjectID   string `json:"projectId"`
			Canvas      string `json:"canvas"`
		} `json:"data"`
	}

	if err := client.PostJSON(ctx, http.WWWBase, path, body, &resp); err != nil {
		return nil, fmt.Errorf("project: query: %w", err)
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("project: query returned code %d", resp.Code)
	}

	return &Project{
		ID:   projectID,
		Name: resp.Data.ProjectName,
	}, nil
}
