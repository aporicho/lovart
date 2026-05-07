// Package cli defines the Lovart CLI command tree using cobra.
package cli

import (
	"github.com/spf13/cobra"
)

// NewRootCommand returns the root cobra command for the Lovart CLI.
func NewRootCommand() *cobra.Command {
	var showVersion bool

	root := &cobra.Command{
		Use:   "lovart",
		Short: "Lovart generation CLI",
		Long:  "Lovart is a CLI and MCP tool for interacting with the Lovart AI image generation platform.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				printEnvelope(okLocal(versionData()))
				return nil
			}
			return cmd.Help()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		newAuthCmd(),
		newSetupCmd(),
		newProjectCmd(),
		newModelsCmd(),
		newConfigCmd(),
		newQuoteCmd(),
		newBalanceCmd(),
		newGenerateCmd(),
		newJobsCmd(),
		newCleanCmd(),
		newUpdateCmd(),
		newDoctorCmd(),
		newDevCmd(),
		newSelfTestCmd(),
	)

	root.Flags().BoolVarP(&showVersion, "version", "v", false, "print version information as JSON")
	return root
}
