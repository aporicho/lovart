package devauth

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/paths"
)

type fakeCapturer struct {
	session auth.Session
	info    CaptureInfo
	err     error
}

func (f fakeCapturer) Capture(context.Context, Options) (auth.Session, CaptureInfo, error) {
	return f.session, f.info, f.err
}

type recordingVerifier struct {
	got     auth.Session
	session auth.Session
	info    VerifyInfo
	err     error
}

func (v *recordingVerifier) Verify(_ context.Context, session auth.Session) (auth.Session, VerifyInfo, error) {
	v.got = session
	return v.session, v.info, v.err
}

func TestRunSavesVerifiedSessionAndReturnsSafeResult(t *testing.T) {
	resetRuntimeRoot(t)
	verifier := &recordingVerifier{
		session: auth.Session{
			Cookie:    "secret-cookie",
			Token:     "secret-token",
			CSRF:      "secret-csrf",
			ProjectID: "project-123",
			CID:       "cid-123",
		},
		info: VerifyInfo{ProjectName: "Design Project", ProjectCount: 2},
	}

	result, err := runWith(context.Background(), Options{Timeout: time.Second}, fakeCapturer{
		session: auth.Session{
			Cookie:    "secret-cookie",
			Token:     "secret-token",
			ProjectID: "project-123",
			CID:       "cid-123",
		},
		info: CaptureInfo{BrowserRestarted: true},
	}, verifier)
	if err != nil {
		t.Fatalf("runWith: %v", err)
	}
	if verifier.got.Source != auth.LoginSourceDevBrowserCapture {
		t.Fatalf("verifier saw source %q, want %q", verifier.got.Source, auth.LoginSourceDevBrowserCapture)
	}
	if !result.Authenticated || !result.ProjectContextReady || !result.BrowserRestarted {
		t.Fatalf("result = %#v", result)
	}
	if result.ProjectID != "project-123" || result.ProjectName != "Design Project" || result.ProjectCount != 2 {
		t.Fatalf("project result = %#v", result)
	}

	status := auth.GetStatus()
	if !status.Available || status.Source != auth.LoginSourceDevBrowserCapture || !status.ProjectContextReady {
		t.Fatalf("status = %#v", status)
	}
	creds, err := auth.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if creds.Cookie != "secret-cookie" || creds.Token != "secret-token" || creds.CSRF != "secret-csrf" {
		t.Fatalf("saved creds = %#v", creds)
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal result: %v", err)
	}
	for _, secret := range []string{"secret-cookie", "secret-token", "secret-csrf", "cid-123"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("result leaked %s: %s", secret, data)
		}
	}
}

func TestRunDoesNotOverwriteExistingSessionOnVerifyFailure(t *testing.T) {
	resetRuntimeRoot(t)
	if err := auth.SaveSession(auth.Session{
		Cookie:    "old-cookie",
		Token:     "old-token",
		ProjectID: "old-project",
		CID:       "old-cid",
		Source:    "existing",
	}); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	_, err := runWith(context.Background(), Options{Timeout: time.Second}, fakeCapturer{
		session: auth.Session{Cookie: "new-cookie", CID: "new-cid"},
	}, &recordingVerifier{err: errors.New("verification failed")})
	if err == nil {
		t.Fatal("runWith unexpectedly succeeded")
	}

	creds, err := auth.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if creds.Cookie != "old-cookie" || creds.Token != "old-token" {
		t.Fatalf("existing creds were overwritten: %#v", creds)
	}
	project, err := auth.LoadProjectContext()
	if err != nil {
		t.Fatalf("LoadProjectContext: %v", err)
	}
	if project.ProjectID != "old-project" || project.CID != "old-cid" {
		t.Fatalf("existing project context was overwritten: %#v", project)
	}
}

func TestRunRejectsIncompleteVerifiedProjectContext(t *testing.T) {
	resetRuntimeRoot(t)
	_, err := runWith(context.Background(), Options{Timeout: time.Second}, fakeCapturer{
		session: auth.Session{Cookie: "secret-cookie", CID: "cid-123"},
	}, &recordingVerifier{
		session: auth.Session{Cookie: "secret-cookie", CID: "cid-123"},
	})
	if err == nil {
		t.Fatal("runWith unexpectedly accepted incomplete project context")
	}
	if status := auth.GetStatus(); status.Available {
		t.Fatalf("incomplete session should not be saved: %#v", status)
	}
}

func resetRuntimeRoot(t *testing.T) {
	t.Helper()
	t.Cleanup(paths.Reset)
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LOVART_HOME", dir)
	paths.Reset()
}
