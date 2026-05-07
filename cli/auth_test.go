package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/paths"
)

func TestAuthStatusDoesNotLeakSecrets(t *testing.T) {
	resetCLIRuntimeRoot(t)
	if err := auth.SaveSession(auth.Session{
		Cookie:    "secret-cookie",
		Token:     "secret-token",
		ProjectID: "project-123",
	}); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	output := captureStdout(t, func() {
		cmd := newAuthCmd()
		cmd.SetArgs([]string{"status"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute status: %v", err)
		}
	})
	for _, secret := range []string{"secret-cookie", "secret-token"} {
		if strings.Contains(output, secret) {
			t.Fatalf("status leaked %s: %s", secret, output)
		}
	}
	for _, want := range []string{`"available":true`, `"project_id_present":true`, `"project_context_ready":false`} {
		if !strings.Contains(output, want) {
			t.Fatalf("status missing %s: %s", want, output)
		}
	}
	for _, forbidden := range []string{"cid_present", `"cid"`} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("status exposed %s: %s", forbidden, output)
		}
	}
}

func TestAuthCommandDoesNotExposeImport(t *testing.T) {
	cmd := newAuthCmd()
	if found, _, err := cmd.Find([]string{"import"}); err == nil && found != cmd {
		t.Fatalf("auth command still exposes import: %s", found.CommandPath())
	}
	cmd.SetArgs([]string{"import"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("auth import should be rejected")
	}
}

func TestAuthLogoutRequiresYesAndDeletesPrimaryCreds(t *testing.T) {
	resetCLIRuntimeRoot(t)
	if err := auth.Save(&auth.Credentials{Cookie: "secret-cookie"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	output := captureStdout(t, func() {
		cmd := newAuthCmd()
		cmd.SetArgs([]string{"logout"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute logout: %v", err)
		}
	})
	if !strings.Contains(output, `"ok":false`) {
		t.Fatalf("logout without --yes should fail: %s", output)
	}
	if _, err := os.Stat(paths.CredsFile); err != nil {
		t.Fatalf("creds should still exist: %v", err)
	}

	output = captureStdout(t, func() {
		cmd := newAuthCmd()
		cmd.SetArgs([]string{"logout", "--yes"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute logout --yes: %v", err)
		}
	})
	if !strings.Contains(output, `"logged_out":true`) {
		t.Fatalf("logout output = %s", output)
	}
	if _, err := os.Stat(paths.CredsFile); !os.IsNotExist(err) {
		t.Fatalf("creds should be deleted, stat err = %v", err)
	}
}

func resetCLIRuntimeRoot(t *testing.T) {
	t.Helper()
	t.Cleanup(paths.Reset)
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LOVART_HOME", dir)
	paths.Reset()
}
