package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aporicho/lovart/internal/paths"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	credsPath := filepath.Join(dir, ".lovart", "creds.json")
	os.MkdirAll(filepath.Dir(credsPath), 0700)
	t.Setenv("LOVART_REVERSE_ROOT", dir)
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

func TestSetProject(t *testing.T) {
	dir := t.TempDir()
	credsPath := filepath.Join(dir, ".lovart", "creds.json")
	os.MkdirAll(filepath.Dir(credsPath), 0700)
	t.Setenv("LOVART_REVERSE_ROOT", dir)
	paths.Reset()

	if err := SetProject("proj-123", "cid-456"); err != nil {
		t.Fatalf("SetProject: %v", err)
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
}

func TestGetStatus(t *testing.T) {
	dir := t.TempDir()
	credsPath := filepath.Join(dir, ".lovart", "creds.json")
	os.MkdirAll(filepath.Dir(credsPath), 0700)
	t.Setenv("LOVART_REVERSE_ROOT", dir)
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
