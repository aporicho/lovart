package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aporicho/lovart/internal/paths"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	credsPath := filepath.Join(dir, "creds.json")
	os.MkdirAll(filepath.Dir(credsPath), 0700)
	t.Setenv("LOVART_HOME", dir)
	paths.Reset()

	c := &Credentials{
		Cookie: "test-cookie",
		Token:  "test-token",
		CSRF:   "test-csrf",
	}

	if err := Save(c); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Cookie != c.Cookie {
		t.Errorf("Cookie = %q, want %q", loaded.Cookie, c.Cookie)
	}
	if loaded.Token != c.Token {
		t.Errorf("Token = %q, want %q", loaded.Token, c.Token)
	}
}

func TestLoadDerivesTokenAndWebIDFromCookie(t *testing.T) {
	dir := t.TempDir()
	credsPath := filepath.Join(dir, "creds.json")
	os.MkdirAll(filepath.Dir(credsPath), 0700)
	t.Setenv("LOVART_HOME", dir)
	paths.Reset()

	if err := SaveSession(Session{
		Cookie: "foo=bar; usertoken=secret-token; webid=web-123",
		Source: "browser_extension",
	}); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Token != "secret-token" {
		t.Fatalf("Token = %q, want derived token", loaded.Token)
	}
	if loaded.WebID != "web-123" {
		t.Fatalf("WebID = %q, want derived webid", loaded.WebID)
	}

	pc, err := LoadProjectContext()
	if err != nil {
		t.Fatalf("LoadProjectContext: %v", err)
	}
	if pc.CID != "web-123" {
		t.Fatalf("CID = %q, want derived webid", pc.CID)
	}

	status := GetStatus()
	if !containsString(status.Fields, "token") {
		t.Fatalf("status fields = %#v, want derived token field", status.Fields)
	}
}

func TestSetProject(t *testing.T) {
	dir := t.TempDir()
	credsPath := filepath.Join(dir, "creds.json")
	os.MkdirAll(filepath.Dir(credsPath), 0700)
	t.Setenv("LOVART_HOME", dir)
	paths.Reset()

	if err := SetProjectContext("proj-123", "cid-456"); err != nil {
		t.Fatalf("SetProjectContext: %v", err)
	}

	pc, err := LoadProjectContext()
	if err != nil {
		t.Fatalf("LoadProjectContext: %v", err)
	}

	if pc.ProjectID != "proj-123" {
		t.Errorf("ProjectID = %q, want %q", pc.ProjectID, "proj-123")
	}
	if pc.CID != "cid-456" {
		t.Errorf("CID = %q, want %q", pc.CID, "cid-456")
	}

	if err := SetProject("proj-789"); err != nil {
		t.Fatalf("SetProject: %v", err)
	}
	pc, err = LoadProjectContext()
	if err != nil {
		t.Fatalf("LoadProjectContext after SetProject: %v", err)
	}
	if pc.ProjectID != "proj-789" || pc.CID != "cid-456" {
		t.Errorf("project context after SetProject = %#v", pc)
	}
}

func TestClearProjectContextPreservesCIDAndCredentials(t *testing.T) {
	dir := t.TempDir()
	credsPath := filepath.Join(dir, "creds.json")
	os.MkdirAll(filepath.Dir(credsPath), 0700)
	t.Setenv("LOVART_HOME", dir)
	paths.Reset()

	if err := SaveSession(Session{
		Cookie:    "secret-cookie",
		Token:     "secret-token",
		CSRF:      "secret-csrf",
		ProjectID: "project-123",
		CID:       "cid-123",
		Source:    "test",
	}); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	if err := ClearProjectContext(); err != nil {
		t.Fatalf("ClearProjectContext: %v", err)
	}

	creds, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if creds.Cookie != "secret-cookie" || creds.Token != "secret-token" || creds.CSRF != "secret-csrf" {
		t.Fatalf("credentials were not preserved: %#v", creds)
	}
	pc, err := LoadProjectContext()
	if err != nil {
		t.Fatalf("LoadProjectContext: %v", err)
	}
	if pc.ProjectID != "" || pc.CID != "cid-123" {
		t.Fatalf("project context after clear = %#v", pc)
	}
	status := GetStatus()
	if status.ProjectIDPresent || status.ProjectContextReady || containsString(status.Fields, "project_id") {
		t.Fatalf("status after clear = %#v", status)
	}
}

func TestGetStatus(t *testing.T) {
	dir := t.TempDir()
	credsPath := filepath.Join(dir, "creds.json")
	os.MkdirAll(filepath.Dir(credsPath), 0700)
	t.Setenv("LOVART_HOME", dir)
	paths.Reset()

	// Before save: not available.
	s := GetStatus()
	if s.Available {
		t.Error("status should not be available before save")
	}

	// After save: available.
	Save(&Credentials{Cookie: "c", Token: "t"})
	s = GetStatus()
	if !s.Available {
		t.Error("status should be available after save")
	}
}

func TestSaveSessionPreservesProjectMetadataAndStatusIsSafe(t *testing.T) {
	dir := t.TempDir()
	credsPath := filepath.Join(dir, "creds.json")
	os.MkdirAll(filepath.Dir(credsPath), 0700)
	t.Setenv("LOVART_HOME", dir)
	paths.Reset()

	session := Session{
		Cookie:    "secret-cookie",
		Token:     "secret-token",
		CSRF:      "secret-csrf",
		ProjectID: "project-123",
		CID:       "cid-123",
		Source:    "test",
	}
	if err := SaveSession(session); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	pc, err := LoadProjectContext()
	if err != nil {
		t.Fatalf("LoadProjectContext: %v", err)
	}
	if pc.ProjectID != "project-123" || pc.CID != "cid-123" {
		t.Fatalf("project context = %#v", pc)
	}

	status := GetStatus()
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	if !status.ProjectIDPresent || !status.ProjectContextReady || status.UpdatedAt == "" {
		t.Fatalf("status = %#v", status)
	}
	if !containsString(status.Fields, "project_id") || containsString(status.Fields, "cid") {
		t.Fatalf("status fields = %#v", status.Fields)
	}
	if status.Source != "test" || status.CredentialPath == "" {
		t.Fatalf("status source/path = %#v", status)
	}
	for _, leaked := range []string{"secret-cookie", "secret-token", "secret-csrf", "cid-123", "cid_present"} {
		if strings.Contains(string(data), leaked) {
			t.Fatalf("status leaked %s: %s", leaked, data)
		}
	}
	if !strings.Contains(string(data), "project_context_ready") {
		t.Fatalf("status missing project_context_ready: %s", data)
	}
	if strings.Contains(string(data), "secret-cookie") || strings.Contains(string(data), "secret-token") || strings.Contains(string(data), "secret-csrf") {
		t.Fatalf("status leaked secrets: %s", data)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
