package cli

import (
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Metadata drift detection and sync",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "check",
		Short: "Check for metadata drift",
		RunE: func(cmd *cobra.Command, args []string) error {
			printEnvelope(envelope.OK(map[string]any{"status": "not implemented"}))
			return nil
		},
	})

	return cmd
}
