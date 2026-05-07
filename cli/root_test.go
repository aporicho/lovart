package cli

import "testing"

func TestRootCommandExcludesRemovedRouteCommand(t *testing.T) {
	cmd := NewRootCommand()
	removed := "pl" + "an"
	if found, _, err := cmd.Find([]string{removed}); err == nil && found != cmd {
		t.Fatalf("root command still exposes removed route command: %s", found.CommandPath())
	}
}

func TestRootCommandMovesSignUnderDev(t *testing.T) {
	cmd := NewRootCommand()
	if found, _, err := cmd.Find([]string{"sign"}); err == nil && found != cmd {
		t.Fatalf("root command still exposes sign at top level: %s", found.CommandPath())
	}

	found, _, err := cmd.Find([]string{"dev", "sign"})
	if err != nil {
		t.Fatalf("dev command missing sign: %v", err)
	}
	if got, want := found.CommandPath(), "lovart dev sign"; got != want {
		t.Fatalf("sign command path = %q, want %q", got, want)
	}
}

func TestRootCommandExposesDevAuthLogin(t *testing.T) {
	cmd := NewRootCommand()
	found, _, err := cmd.Find([]string{"dev", "auth-login"})
	if err != nil {
		t.Fatalf("dev command missing auth-login: %v", err)
	}
	if got, want := found.CommandPath(), "lovart dev auth-login"; got != want {
		t.Fatalf("auth-login command path = %q, want %q", got, want)
	}
}
