// Package auth manages Lovart credentials (token, cookie, CSRF) and project context.
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
	WebID  string `json:"webid"`
}

// Session is the persisted browser login state used by the CLI.
type Session struct {
	Cookie    string `json:"cookie,omitempty"`
	Token     string `json:"token,omitempty"`
	CSRF      string `json:"csrf,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
	CID       string `json:"cid,omitempty"`
	Source    string `json:"source,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// ProjectContext binds generation requests to a Lovart project.
type ProjectContext struct {
	ProjectID string `json:"project_id"`
	CID       string `json:"cid"`
}

// Status reports whether credentials are available without exposing values.
type Status struct {
	Available           bool     `json:"available"`
	Source              string   `json:"source,omitempty"`
	CredentialPath      string   `json:"credential_path,omitempty"`
	Fields              []string `json:"fields"`
	ProjectIDPresent    bool     `json:"project_id_present"`
	ProjectContextReady bool     `json:"project_context_ready"`
	UpdatedAt           string   `json:"updated_at,omitempty"`
}

// Load reads credentials from the persisted creds file.
// Supports both v2 flat format and v1 nested format {headers:{cookie,token}}.
func Load() (*Credentials, error) {
	data, _, err := credentialData()
	if err != nil {
		return nil, err
	}
	session, err := parseSession(data)
	if err != nil {
		return nil, err
	}
	return &Credentials{Cookie: session.Cookie, Token: session.Token, CSRF: session.CSRF, WebID: session.CID}, nil
}

// LoadProjectContext reads project context from the creds file.
func LoadProjectContext() (*ProjectContext, error) {
	data, _, err := credentialData()
	if err != nil {
		return nil, fmt.Errorf("auth: no creds file for project context")
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	session := sessionFromMap(raw)
	return &ProjectContext{ProjectID: session.ProjectID, CID: session.CID}, nil
}

// GetStatus reports credential availability without leaking values.
func GetStatus() Status {
	data, credentialPath, err := credentialData()
	status := Status{CredentialPath: paths.CredsFile, Fields: []string{}}
	if credentialPath != "" {
		status.CredentialPath = credentialPath
	}
	if err != nil {
		return status
	}
	session, err := parseSession(data)
	if err != nil {
		return status
	}
	status.Available = true
	status.Source = firstNonEmpty(session.Source, "unknown")
	status.UpdatedAt = session.UpdatedAt
	if session.Cookie != "" {
		status.Fields = append(status.Fields, "cookie")
	}
	if session.Token != "" {
		status.Fields = append(status.Fields, "token")
	}
	if session.CSRF != "" {
		status.Fields = append(status.Fields, "csrf")
	}
	if session.ProjectID != "" {
		status.Fields = append(status.Fields, "project_id")
		status.ProjectIDPresent = true
	}
	status.ProjectContextReady = session.ProjectID != "" && session.CID != ""
	return status
}

func credentialData() ([]byte, string, error) {
	data, err := os.ReadFile(paths.CredsFile)
	if err == nil {
		return data, paths.CredsFile, nil
	}
	return nil, "", fmt.Errorf("auth: no credentials found (run `lovart auth login`)")
}

func parseSession(data []byte) (Session, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return Session{}, fmt.Errorf("auth: parse creds file: %w", err)
	}
	session := sessionFromMap(raw)
	if session.Cookie == "" && session.Token == "" {
		return Session{}, fmt.Errorf("auth: creds file found but no token or cookie")
	}
	return session, nil
}

func sessionFromMap(raw map[string]any) Session {
	headers := anyMap(raw, "headers")
	ids := anyMap(raw, "ids")
	session := Session{
		Cookie:    firstString(raw, "cookie"),
		Token:     firstString(raw, "token", "authorization", "x-auth-token", "x-access-token"),
		CSRF:      firstString(raw, "csrf", "x-csrf-token", "x-xsrf-token", "csrf-token"),
		ProjectID: firstNonEmpty(firstString(raw, "project_id", "projectId"), firstString(ids, "project_id", "projectId")),
		CID:       firstNonEmpty(firstString(raw, "cid", "webid", "webId"), firstString(ids, "cid", "webid", "webId")),
		Source:    firstString(raw, "source", "source_capture"),
		UpdatedAt: firstString(raw, "updated_at"),
	}
	session = session.mergeHeaders(headers)
	return session.mergeCookieHints()
}

func (s Session) mergeHeaders(headers map[string]any) Session {
	if s.Cookie == "" {
		s.Cookie = firstString(headers, "cookie")
	}
	if s.Token == "" {
		s.Token = firstString(headers, "token", "authorization", "x-auth-token", "x-access-token")
	}
	if s.CSRF == "" {
		s.CSRF = firstString(headers, "csrf", "x-csrf-token", "x-xsrf-token", "csrf-token")
	}
	return s
}

func (s Session) mergeCookieHints() Session {
	if s.Token == "" {
		s.Token = cookieValue(s.Cookie, "usertoken")
	}
	if s.CID == "" {
		s.CID = cookieValue(s.Cookie, "webid")
	}
	return s
}

func cookieValue(header, name string) string {
	start := 0
	for start <= len(header) {
		end := len(header)
		for i := start; i < len(header); i++ {
			if header[i] == ';' {
				end = i
				break
			}
		}
		part := trimCookiePart(header[start:end])
		if key, value, ok := cutCookiePart(part); ok && stringsEqualFold(key, name) {
			return value
		}
		if end == len(header) {
			break
		}
		start = end + 1
	}
	return ""
}

func cutCookiePart(part string) (string, string, bool) {
	for i := 0; i < len(part); i++ {
		if part[i] == '=' {
			return part[:i], part[i+1:], true
		}
	}
	return "", "", false
}

func trimCookiePart(part string) string {
	start := 0
	for start < len(part) && (part[start] == ' ' || part[start] == '\t') {
		start++
	}
	end := len(part)
	for end > start && (part[end-1] == ' ' || part[end-1] == '\t') {
		end--
	}
	return part[start:end]
}

func anyMap(raw map[string]any, key string) map[string]any {
	if raw == nil {
		return nil
	}
	if value, ok := raw[key]; ok {
		if result, ok := value.(map[string]any); ok {
			return result
		}
	}
	return nil
}

func firstString(raw map[string]any, names ...string) string {
	if raw == nil {
		return ""
	}
	for _, name := range names {
		for key, value := range raw {
			if stringsEqualFold(key, name) {
				if text, ok := value.(string); ok && text != "" {
					return text
				}
			}
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func stringsEqualFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
