package project

import (
	"context"
	"fmt"

	"github.com/aporicho/lovart/internal/http"
)

// Rename updates the name of an existing Lovart project.
func Rename(ctx context.Context, client *http.Client, projectID, newName string) error {
	path := "/api/canva/project/updateProjectName"

	body := map[string]any{
		"projectId":   projectID,
		"projectName": newName,
	}

	var resp struct {
		Code int  `json:"code"`
		Data bool `json:"data"`
	}

	if err := client.PostJSON(ctx, http.WWWBase, path, body, &resp); err != nil {
		return fmt.Errorf("project: rename: %w", err)
	}

	if resp.Code != 0 {
		return fmt.Errorf("project: rename returned code %d", resp.Code)
	}

	return nil
}
