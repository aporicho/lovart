package project

import (
	"context"
	"fmt"

	"github.com/aporicho/lovart/internal/http"
)

// Delete removes a Lovart project by ID.
func Delete(ctx context.Context, client *http.Client, projectID string) error {
	path := "/api/canva/project/batchDeleteProject"

	body := map[string]any{
		"projectIdList": []string{projectID},
	}

	var resp struct {
		Code int  `json:"code"`
		Data bool `json:"data"`
	}

	if err := client.PostJSON(ctx, http.WWWBase, path, body, &resp); err != nil {
		return fmt.Errorf("project: delete: %w", err)
	}

	if resp.Code != 0 {
		return fmt.Errorf("project: delete returned code %d", resp.Code)
	}

	return nil
}
