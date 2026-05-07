package main

import (
	"testing"

	"github.com/aporicho/lovart/cli"
)

func TestBinaryRootIncludesSelfManagementCommands(t *testing.T) {
	root := cli.NewRootCommand()
	root.AddCommand(newMCPCommand())
	root.AddCommand(newUpgradeCommand())
	root.AddCommand(newUninstallCommand())
	for _, name := range []string{"upgrade", "uninstall"} {
		if cmd, _, err := root.Find([]string{name}); err != nil || cmd == nil || cmd.Name() != name {
			t.Fatalf("missing %s command: cmd=%v err=%v", name, cmd, err)
		}
	}
}
