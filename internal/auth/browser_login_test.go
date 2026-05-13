package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aporicho/lovart/internal/paths"
)

func TestRunBrowserExtensionLoginSavesSessionWithoutExposingSecrets(t *testing.T) {
	resetAuthRuntimeRoot(t)
	var openedURL string
	result, err := RunBrowserExtensionLogin(context.Background(), BrowserLoginOptions{
		Timeout: 5 * time.Second,
		OpenBrowser: func(loginURL string) error {
			openedURL = loginURL
			go completeLogin(t, loginURL)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("RunBrowserExtensionLogin: %v", err)
	}
	if !result.Authenticated || !result.OpenedBrowser || result.Status.Available != true {
		t.Fatalf("unexpected result: %#v", result)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"secret-cookie", "secret-token", "secret-csrf", "cid-123"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("login result leaked %s: %s", secret, data)
		}
	}
	if openedURL == "" || !strings.Contains(openedURL, "lovart_cli_auth=1") {
		t.Fatalf("browser was not opened with login URL: %q", openedURL)
	}
	status := GetStatus()
	if !status.Available || !status.ProjectContextReady || status.Source != LoginSourceBrowserExtension {
		t.Fatalf("status not saved correctly: %#v", status)
	}
}

func TestRunBrowserExtensionLoginRequiresBrowserOpenWhenRequested(t *testing.T) {
	resetAuthRuntimeRoot(t)
	result, err := RunBrowserExtensionLogin(context.Background(), BrowserLoginOptions{
		Timeout:              time.Second,
		RequireBrowserOpened: true,
		OpenBrowser: func(loginURL string) error {
			return errAuthBrowserOpen
		},
	})
	if err == nil || !strings.Contains(err.Error(), "input_error") {
		t.Fatalf("expected input error, got result=%#v err=%v", result, err)
	}
	if result.LoginURL == "" || result.CallbackPort == 0 {
		t.Fatalf("missing manual login metadata: %#v", result)
	}
}

var errAuthBrowserOpen = &authBrowserOpenError{}

type authBrowserOpenError struct{}

func (e *authBrowserOpenError) Error() string { return "browser open failed" }

func completeLogin(t *testing.T, loginURL string) {
	t.Helper()
	parsed, err := url.Parse(loginURL)
	if err != nil {
		t.Errorf("parse login URL: %v", err)
		return
	}
	port := parsed.Query().Get("port")
	if _, err := strconv.Atoi(port); err != nil {
		t.Errorf("invalid port: %q", port)
		return
	}
	sessionURL := "http://127.0.0.1:" + port + "/lovart/auth/session"
	resp, err := http.Get(sessionURL)
	if err != nil {
		t.Errorf("get login session: %v", err)
		return
	}
	defer resp.Body.Close()
	var session struct {
		State string `json:"state"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		t.Errorf("decode login session: %v", err)
		return
	}
	body := []byte(`{"state":"` + session.State + `","cookie":"secret-cookie","token":"secret-token","csrf":"secret-csrf","project_id":"project-123","cid":"cid-123"}`)
	resp, err = http.Post("http://127.0.0.1:"+port+"/lovart/auth/complete", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Errorf("post login complete: %v", err)
		return
	}
	_ = resp.Body.Close()
}

func resetAuthRuntimeRoot(t *testing.T) {
	t.Helper()
	t.Cleanup(paths.Reset)
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LOVART_HOME", dir)
	paths.Reset()
}
