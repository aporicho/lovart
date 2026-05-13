package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aporicho/lovart/internal/paths"
)

// Save persists credentials to the creds file.
func Save(c *Credentials) error {
	if c == nil {
		return fmt.Errorf("auth: cannot save nil credentials")
	}
	return SaveSession(Session{Cookie: c.Cookie, Token: c.Token, CSRF: c.CSRF, CID: c.WebID})
}

// SaveSession persists credentials and project context to the creds file.
func SaveSession(session Session) error {
	normalized := NormalizeCredentials(&Credentials{
		Cookie: session.Cookie,
		Token:  session.Token,
		CSRF:   session.CSRF,
		WebID:  session.CID,
	})
	session.Cookie = normalized.Cookie
	session.Token = normalized.Token
	session.CSRF = normalized.CSRF
	session.CID = normalized.WebID
	if session.Cookie == "" && session.Token == "" {
		return fmt.Errorf("auth: cannot save session without cookie or token")
	}
	if session.UpdatedAt == "" {
		session.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	data := map[string]any{
		"updated_at": session.UpdatedAt,
	}
	if session.Cookie != "" {
		data["cookie"] = session.Cookie
	}
	if session.Token != "" {
		data["token"] = session.Token
	}
	if session.CSRF != "" {
		data["csrf"] = session.CSRF
	}
	if session.ProjectID != "" {
		data["project_id"] = session.ProjectID
		data["projectId"] = session.ProjectID
	}
	if session.CID != "" {
		data["cid"] = session.CID
	}
	if session.ProjectID != "" || session.CID != "" {
		ids := map[string]any{}
		if session.ProjectID != "" {
			ids["project_id"] = session.ProjectID
			ids["projectId"] = session.ProjectID
		}
		if session.CID != "" {
			ids["cid"] = session.CID
		}
		data["ids"] = ids
	}
	if session.Source != "" {
		data["source"] = session.Source
	}
	return writeCredentialMap(data)
}

// Delete removes the primary v2 credentials file.
func Delete() error {
	if err := os.Remove(paths.CredsFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("auth: delete creds file: %w", err)
	}
	return nil
}

// SetProject saves the selected project alongside existing credentials.
// It preserves the internal browser context already stored in the creds file.
func SetProject(projectID string) error {
	cid := ""
	if pc, err := LoadProjectContext(); err == nil && pc != nil {
		cid = pc.CID
	}
	return SetProjectContext(projectID, cid)
}

// SetProjectContext saves project context alongside existing credentials.
// It preserves any existing non-project fields in the creds file.
func SetProjectContext(projectID, cid string) error {
	existing := make(map[string]any)
	if data, err := os.ReadFile(paths.CredsFile); err == nil {
		_ = json.Unmarshal(data, &existing)
	}

	existing["project_id"] = projectID
	existing["projectId"] = projectID
	if cid != "" {
		existing["cid"] = cid
	}

	if ids, ok := existing["ids"].(map[string]any); ok {
		ids["project_id"] = projectID
		ids["projectId"] = projectID
		if cid != "" {
			ids["cid"] = cid
		}
	} else if cid != "" {
		existing["ids"] = map[string]any{
			"project_id": projectID,
			"projectId":  projectID,
			"cid":        cid,
		}
	}
	if _, ok := existing["updated_at"]; !ok {
		existing["updated_at"] = time.Now().UTC().Format(time.RFC3339)
	}

	return writeCredentialMap(existing)
}

// ClearProjectContext removes the selected project while preserving credentials and CID.
func ClearProjectContext() error {
	existing := make(map[string]any)
	data, err := os.ReadFile(paths.CredsFile)
	if err != nil {
		return fmt.Errorf("auth: read creds file: %w", err)
	}
	if err := json.Unmarshal(data, &existing); err != nil {
		return fmt.Errorf("auth: parse creds file: %w", err)
	}

	delete(existing, "project_id")
	delete(existing, "projectId")
	if ids, ok := existing["ids"].(map[string]any); ok {
		delete(ids, "project_id")
		delete(ids, "projectId")
		if len(ids) == 0 {
			delete(existing, "ids")
		}
	}
	if _, ok := existing["updated_at"]; !ok {
		existing["updated_at"] = time.Now().UTC().Format(time.RFC3339)
	}
	return writeCredentialMap(existing)
}

func writeCredentialMap(value map[string]any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("auth: marshal credentials: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.CredsFile), 0700); err != nil {
		return fmt.Errorf("auth: create creds directory: %w", err)
	}
	if err := os.WriteFile(paths.CredsFile, data, 0600); err != nil {
		return fmt.Errorf("auth: write creds file: %w", err)
	}
	return nil
}
