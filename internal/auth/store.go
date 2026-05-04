// Package auth manages Lovart credentials (token, cookie, CSRF) and project context.
package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
	Fields    []string `json:"fields"`
}

// Load reads credentials from the persisted creds file.
// Supports both v2 flat format and v1 nested format {headers:{cookie,token}}.
func Load() (*Credentials, error) {
	data, err := os.ReadFile(paths.CredsFile)
	if err != nil {
		// Try legacy path (v1 scripts/creds.json)
		legacy := filepath.Join(paths.Root, "scripts", "creds.json")
		if d, e2 := os.ReadFile(legacy); e2 == nil {
			data = d
		} else {
			return nil, fmt.Errorf("auth: no credentials found (run `lovart-reverse start` to capture)")
		}
	}

	// Try v2 flat format first.
	var c Credentials
	if err := json.Unmarshal(data, &c); err == nil && (c.Cookie != "" || c.Token != "") {
		return &c, nil
	}

	// Try v1 nested format: {"headers": {"cookie": "...", "token": "..."}}.
	var v1 struct {
		Headers Credentials `json:"headers"`
	}
	if err := json.Unmarshal(data, &v1); err == nil && (v1.Headers.Cookie != "" || v1.Headers.Token != "") {
		return &v1.Headers, nil
	}

	return nil, fmt.Errorf("auth: creds file found but no token or cookie")
}

// LoadProjectContext reads project context from the creds file.
func LoadProjectContext() (*ProjectContext, error) {
	data, err := os.ReadFile(paths.CredsFile)
	if err != nil {
		legacy := filepath.Join(paths.Root, "scripts", "creds.json")
		if d, e2 := os.ReadFile(legacy); e2 == nil {
			data = d
		} else {
			return nil, fmt.Errorf("auth: no creds file for project context")
		}
	}

	pc := &ProjectContext{}

	// Try v2 flat format.
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	pc.ProjectID, _ = raw["project_id"].(string)
	if pc.ProjectID == "" {
		pc.ProjectID, _ = raw["projectId"].(string)
	}
	pc.CID, _ = raw["cid"].(string)
	if pc.CID == "" {
		pc.CID, _ = raw["webid"].(string)
	}

	// If not found in top-level, try v1 nested "ids" sub-object.
	if pc.ProjectID == "" || pc.CID == "" {
		if ids, ok := raw["ids"].(map[string]any); ok {
			if pid, ok := ids["project_id"].(string); ok && pid != "" {
				pc.ProjectID = pid
			}
			if pid, ok := ids["projectId"].(string); ok && pid != "" {
				pc.ProjectID = pid
			}
			if cid, ok := ids["cid"].(string); ok && cid != "" {
				pc.CID = cid
			}
			if cid, ok := ids["webid"].(string); ok && cid != "" {
				pc.CID = cid
			}
		}
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
