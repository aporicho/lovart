package cli

import "github.com/spf13/cobra"

func newDevCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Developer and maintenance diagnostics",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newSignCmd())
	return cmd
}
