package cli

import "testing"

func TestGenerateCommandExposesProjectOverrides(t *testing.T) {
	cmd := newGenerateCmd()

	for _, name := range []string{"project-id", "cid"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Fatalf("generate command missing --%s flag", name)
		}
	}

	if got := cmd.Use; got != "generate <model> --body-file <file> [--project-id <id>] [--mode auto|fast|relax]" {
		t.Fatalf("generate use = %q", got)
	}
}
