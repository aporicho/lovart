package auth

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/aporicho/lovart/internal/paths"
)

// Save persists credentials to the creds file.
func Save(c *Credentials) error {
	if c == nil {
		return fmt.Errorf("auth: cannot save nil credentials")
	}
	data, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("auth: marshal credentials: %w", err)
	}
	if err := os.WriteFile(paths.CredsFile, data, 0600); err != nil {
		return fmt.Errorf("auth: write creds file: %w", err)
	}
	return nil
}

// SetProject saves the project context alongside existing credentials.
// It preserves any existing non-project fields in the creds file.
func SetProject(projectID, cid string) error {
	// Read existing creds to preserve non-project fields.
	existing := make(map[string]any)
	if data, err := os.ReadFile(paths.CredsFile); err == nil {
		_ = json.Unmarshal(data, &existing)
	}

	existing["project_id"] = projectID
	existing["projectId"] = projectID
	existing["cid"] = cid

	data, err := json.Marshal(existing)
	if err != nil {
		return fmt.Errorf("auth: marshal project context: %w", err)
	}
	if err := os.WriteFile(paths.CredsFile, data, 0600); err != nil {
		return fmt.Errorf("auth: write creds file: %w", err)
	}
	return nil
}
