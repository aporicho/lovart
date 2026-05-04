// Package cli defines the Lovart CLI command tree using cobra.
package cli

import (
	"github.com/spf13/cobra"
)

// NewRootCommand returns the root cobra command for the Lovart CLI.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "lovart",
		Short: "Lovart generation CLI",
		Long:  "Lovart is a CLI and MCP tool for interacting with the Lovart AI image generation platform.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		newSignCmd(),
		newSetupCmd(),
		newProjectCmd(),
		newModelsCmd(),
		newConfigCmd(),
		newPlanCmd(),
		newQuoteCmd(),
		newGenerateCmd(),
		newJobsCmd(),
		newUpdateCmd(),
		newDoctorCmd(),
		newSelfTestCmd(),
	)

	root.AddCommand(newVersionCmd())
	return root
}
