package main

import (
	"testing"

	"github.com/aporicho/lovart/cli"
)

func TestMCPCommandIncludesSmoke(t *testing.T) {
	root := cli.NewRootCommand()
	root.AddCommand(newMCPCommand())
	cmd, _, err := root.Find([]string{"mcp", "smoke"})
	if err != nil || cmd == nil || cmd.Name() != "smoke" {
		t.Fatalf("missing mcp smoke command: cmd=%v err=%v", cmd, err)
	}
	for _, name := range []string{"model", "prompt", "body-file", "mode", "submit", "allow-paid", "max-credits"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Fatalf("mcp smoke missing --%s flag", name)
		}
	}
}
