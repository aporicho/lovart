// Package cli defines the Lovart CLI command tree using cobra.
package cli

import (
	"github.com/aporicho/lovart/internal/envelope"
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
				printEnvelope(envelope.OK(versionData()))
				return nil
			}
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
		newBalanceCmd(),
		newGenerateCmd(),
		newJobsCmd(),
		newUpdateCmd(),
		newDoctorCmd(),
		newSelfTestCmd(),
	)

	root.AddCommand(newVersionCmd())
	root.Flags().BoolVar(&showVersion, "version", false, "print version information as JSON")
	return root
}
