// Package auth manages Lovart credentials (token, cookie, csrf, and project context).
package auth

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/aporicho/lovart/internal/paths"
)

// Credentials holds Lovart authentication values.
type Credentials struct {
	Cookie string `json:"cookie"`
	Token  string `json:"token"`
	CSRF   string `json:"csrf"`
}

// ProjectContext binds generation requests to a Lovart project.
type ProjectContext struct {
	ProjectID string `json:"project_id"`
	CID       string `json:"cid"`
}

// Status reports whether credentials are available without exposing values.
type Status struct {
	Available bool     `json:"available"`
	Source    string   `json:"source"`
	Fields    []string `json:"fields"` // field names only, never values
}

// Load reads credentials from the persisted creds file.
func Load() (*Credentials, error) {
	data, err := os.ReadFile(paths.CredsFile)
	if err != nil {
		return nil, fmt.Errorf("auth: read creds file: %w (run `lovart setup`)", err)
	}
	var c Credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("auth: parse creds: %w", err)
	}
	if c.Cookie == "" && c.Token == "" {
		return nil, fmt.Errorf("auth: creds file exists but no token or cookie found")
	}
	return &c, nil
}

// LoadProjectContext reads project context from the same creds file.
func LoadProjectContext() (*ProjectContext, error) {
	data, err := os.ReadFile(paths.CredsFile)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	pc := &ProjectContext{}
	pc.ProjectID, _ = raw["project_id"].(string)
	if pc.ProjectID == "" {
		pc.ProjectID, _ = raw["projectId"].(string)
	}
	pc.CID, _ = raw["cid"].(string)
	if pc.CID == "" {
		pc.CID, _ = raw["webid"].(string)
	}
	if pc.CID == "" {
		pc.CID, _ = raw["webId"].(string)
	}
	return pc, nil
}

// GetStatus reports credential availability without leaking values.
func GetStatus() Status {
	s := Status{Source: paths.CredsFile}
	c, err := Load()
	if err != nil {
		return s
	}
	s.Available = true
	if c.Cookie != "" {
		s.Fields = append(s.Fields, "cookie")
	}
	if c.Token != "" {
		s.Fields = append(s.Fields, "token")
	}
	if c.CSRF != "" {
		s.Fields = append(s.Fields, "csrf")
	}
	pc, _ := LoadProjectContext()
	if pc != nil && pc.ProjectID != "" {
		s.Fields = append(s.Fields, "project_id")
	}
	return s
}
