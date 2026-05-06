package cli

import "testing"

func TestSetupCommandDoesNotExposeOfflineMode(t *testing.T) {
	cmd := newSetupCmd()
	if cmd.Flags().Lookup("offline") != nil {
		t.Fatal("setup command should not expose --offline")
	}
}
